"""Resumable chunked upload of one file to a Couchverse upload session.

Everything is driven off the server-confirmed offset, which makes it idempotent
across retries: a 409 means "you are not where the server is", so we jump to the
server's offset and continue; a transient 5xx/network error backs off, re-reads
the server's receivedBytes, and resumes from there."""

from __future__ import annotations

import os
import time
from typing import Callable

import requests

from ..api.client import CouchverseClient, CouchverseError, OffsetConflict

CHUNK_SIZE = 8 * 1024 * 1024  # 8 MiB, matching the web client and well under the 64 MiB cap
_TRANSIENT_STATUS = {408, 425, 429, 500, 502, 503, 504}


class UploadError(RuntimeError):
    pass


class UploadCancelled(Exception):
    pass


def upload_file(
    client: CouchverseClient,
    file_path: str,
    on_session: Callable[[str], None] | None = None,
    on_progress: Callable[[int, int], None] | None = None,
    should_cancel: Callable[[], bool] | None = None,
    chunk_size: int = CHUNK_SIZE,
    max_retries: int = 6,
) -> str:
    """Upload file_path to a new session and return its upload id once every byte
    is acked (not completed - the caller calls upload_complete). on_session fires
    with the new upload id immediately so the caller can abort it on cancel."""
    if chunk_size <= 0:
        raise ValueError("upload chunk size must be positive")
    _check_cancel(should_cancel)
    size = os.path.getsize(file_path)
    upload_id = client.upload_create(os.path.basename(file_path), size)
    if on_session:
        on_session(upload_id)

    offset = 0
    attempts = 0
    delay = 1.0
    with open(file_path, "rb") as f:
        while offset < size:
            _check_cancel(should_cancel)
            f.seek(offset)
            chunk = f.read(chunk_size)
            if not chunk:
                raise UploadError("source file ended before all upload bytes were acknowledged")
            try:
                new_offset = _validate_offset(client.upload_append(upload_id, offset, chunk), size)
                if new_offset <= offset:
                    raise UploadError("upload did not acknowledge new bytes")
                offset = new_offset
                attempts, delay = 0, 1.0
                if on_progress:
                    on_progress(offset, size)
            except OffsetConflict as e:
                offset = _validate_offset(e.server_offset, size)
                attempts += 1
                if attempts > max_retries:
                    raise UploadError("upload repeatedly rejected the confirmed byte offset") from e
                if on_progress:
                    on_progress(offset, size)
                _retry_wait(delay, should_cancel)
                delay = min(delay * 2, 30.0)
            except (CouchverseError, requests.RequestException) as e:
                if isinstance(e, CouchverseError) and e.status not in _TRANSIENT_STATUS:
                    raise
                attempts += 1
                if attempts > max_retries:
                    raise UploadError(f"upload failed after {max_retries} retries: {e}") from e
                _retry_wait(delay, should_cancel)
                delay = min(delay * 2, 30.0)
                offset = _validate_offset(_resync(client, upload_id, offset), size)

    _check_cancel(should_cancel)
    return upload_id


def _resync(client: CouchverseClient, upload_id: str, fallback: int) -> int:
    """Ask the server how many bytes it actually has, so a retry resumes correctly."""
    try:
        st = client.upload_status(upload_id)
        return int(st.get("receivedBytes", fallback))
    except (CouchverseError, ValueError, TypeError, requests.RequestException):
        return fallback


def _validate_offset(offset: int, size: int) -> int:
    if not isinstance(offset, int) or not 0 <= offset <= size:
        raise UploadError(f"server returned an invalid upload offset: {offset}")
    return offset


def _check_cancel(should_cancel: Callable[[], bool] | None) -> None:
    if should_cancel and should_cancel():
        raise UploadCancelled()


def _retry_wait(delay: float, should_cancel: Callable[[], bool] | None) -> None:
    deadline = time.monotonic() + delay
    while True:
        _check_cancel(should_cancel)
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            return
        time.sleep(min(remaining, 0.1))
