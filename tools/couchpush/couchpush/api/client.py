"""HTTP client for the Couchverse admin API.

Auth is a single httpOnly session cookie (`couchverse_session`) held in a
requests cookie jar. The server marks that cookie Secure for its HTTPS
deployment; we drop the Secure flag client-side so the same session also works
against a plain-HTTP LAN URL (e.g. http://192.168.0.69:8080), which is the fast
path for large uploads. That is harmless over HTTPS.
"""

from __future__ import annotations

import os
from urllib.parse import urlparse

import requests

from ..config import SESSION_COOKIE_NAME
from .models import Title, TitleDetail


class CouchverseError(Exception):
    def __init__(self, status: int, code: str, message: str, request: str = ""):
        context = f" ({request})" if request else ""
        super().__init__(f"{status} {code}: {message}{context}")
        self.status = status
        self.code = code
        self.message = message
        self.request = request


class AuthError(CouchverseError):
    """Not logged in, session expired, or the account is not an admin."""


class OffsetConflict(CouchverseError):
    """Chunk PUT rejected (409); `server_offset` is where the server actually is."""

    def __init__(self, server_offset: int):
        super().__init__(409, "offset_mismatch", f"server offset is {server_offset}")
        self.server_offset = server_offset


class CouchverseClient:
    def __init__(self, base_url: str, timeout: float = 30.0, upload_timeout: float = 600.0):
        self.base = base_url.rstrip("/")
        self.api = self.base + "/api/v1"
        self.timeout = timeout
        self.upload_timeout = upload_timeout
        self.session = requests.Session()
        self.session.headers["User-Agent"] = "couchpush/0.1"

    # ---- auth ----

    def login(self, username: str, password: str) -> dict:
        r = self.session.post(
            self.api + "/auth/login",
            json={"username": username, "password": password},
            timeout=self.timeout,
        )
        if r.status_code == 401:
            raise AuthError(401, "invalid_credentials", "invalid username or password")
        self._check(r)
        self._desecure_cookies()
        return r.json()

    def me(self) -> dict:
        r = self.session.get(self.api + "/auth/me", timeout=self.timeout)
        self._check(r)
        return r.json()

    def logout(self) -> None:
        try:
            self.session.post(self.api + "/auth/logout", timeout=self.timeout)
        except requests.RequestException:
            pass

    def restore_session(self, cookie_value: str) -> None:
        """Re-attach a saved session cookie so we can skip the login prompt."""
        host = urlparse(self.base).hostname or ""
        self.session.cookies.set(SESSION_COOKIE_NAME, cookie_value, domain=host, path="/")
        self._desecure_cookies()

    def session_cookie(self) -> tuple[str, float | None] | None:
        for c in self.session.cookies:
            if c.name == SESSION_COOKIE_NAME:
                return c.value, c.expires
        return None

    def _desecure_cookies(self) -> None:
        for c in self.session.cookies:
            if c.secure:
                c.secure = False

    # ---- catalog ----

    def list_series(self, query: str = "") -> list[Title]:
        return self._list_titles("series", query)

    def list_movies(self, query: str = "") -> list[Title]:
        return self._list_titles("movie", query)

    def _list_titles(self, kind: str, query: str) -> list[Title]:
        items: list[Title] = []
        page = 1
        while True:
            r = self.session.get(
                self.api + "/admin/library",
                params={"type": kind, "pageSize": 200, "page": page, "q": query},
                timeout=self.timeout,
            )
            self._check(r)
            data = r.json()
            batch = data.get("items") or []
            items.extend(Title.from_json(it) for it in batch if it.get("id"))
            if not batch or len(items) >= data.get("total", len(items)):
                break
            page += 1
        return items

    def title_detail(self, title_id: str) -> TitleDetail:
        r = self.session.get(self.api + f"/admin/titles/{title_id}", timeout=self.timeout)
        self._check(r)
        return TitleDetail.from_json(r.json())

    def create_movie(self, name: str, year: int | None = None) -> str:
        r = self.session.post(
            self.api + "/admin/titles",
            json={"kind": "movie", "name": name, "year": year},
            timeout=self.timeout,
        )
        self._check(r)
        return r.json()["id"]

    def create_season(self, title_id: str, season_number: int, name: str = "") -> dict:
        r = self.session.post(
            self.api + f"/admin/titles/{title_id}/seasons",
            json={"seasonNumber": season_number, "name": name},
            timeout=self.timeout,
        )
        self._check(r)
        return r.json()

    def create_episode(self, season_id: str, episode_number: int, name: str = "") -> dict:
        r = self.session.post(
            self.api + f"/admin/seasons/{season_id}/episodes",
            json={"episodeNumber": episode_number, "name": name, "overview": "", "runtimeMinutes": None},
            timeout=self.timeout,
        )
        self._check(r)
        return r.json()

    # ---- uploads ----

    def upload_create(self, filename: str, size: int) -> str:
        r = self.session.post(
            self.api + "/admin/uploads",
            json={"filename": filename, "size": size},
            timeout=self.timeout,
        )
        self._check(r)
        return r.json()["id"]

    def upload_status(self, upload_id: str) -> dict:
        r = self.session.get(self.api + f"/admin/uploads/{upload_id}", timeout=self.timeout)
        self._check(r)
        return r.json()

    def upload_append(self, upload_id: str, offset: int, chunk: bytes) -> int:
        """PUT one chunk at `offset`; returns the new server offset. Raises
        OffsetConflict(server_offset) on 409 so the caller can resync."""
        r = self.session.put(
            self.api + f"/admin/uploads/{upload_id}",
            params={"offset": offset},
            data=chunk,
            headers={"Content-Type": "application/octet-stream"},
            timeout=self.upload_timeout,
        )
        if r.status_code == 409:
            try:
                server_offset = int(r.json().get("offset", 0))
            except (ValueError, AttributeError):
                server_offset = 0
            raise OffsetConflict(server_offset)
        self._check(r)
        return int(r.json().get("offset", offset + len(chunk)))

    def upload_complete(self, upload_id: str, library_kind: str = "series",
                        episode_id: str | None = None, title_id: str | None = None) -> str:
        """Finalize an upload. Series: pass episode_id for a known episode, or title_id
        and the server resolves SxxExx from the filename. Movies: pass title_id for a known
        movie, or neither and the server find-or-creates the title from the filename."""
        body: dict = {"libraryKind": library_kind}
        if episode_id:
            body["episodeId"] = episode_id
        elif title_id:
            body["titleId"] = title_id
        r = self.session.post(
            self.api + f"/admin/uploads/{upload_id}/complete", json=body, timeout=self.timeout
        )
        self._check(r)
        return r.json()["mediaFileId"]

    def upload_abort(self, upload_id: str) -> None:
        try:
            self.session.delete(self.api + f"/admin/uploads/{upload_id}", timeout=self.timeout)
        except requests.RequestException:
            pass

    # ---- media files ----

    def set_media_audio(self, media_file_id: str, audio_lang: str, audio_role: str) -> None:
        r = self.session.patch(
            self.api + f"/admin/media-files/{media_file_id}",
            json={"audioLang": audio_lang, "audioRole": audio_role},
            timeout=self.timeout,
        )
        self._check(r)

    def delete_media_file(self, media_file_id: str) -> None:
        r = self.session.delete(
            self.api + f"/admin/media-files/{media_file_id}", timeout=self.timeout
        )
        self._check(r)

    def upload_subtitle(self, media_file_id: str, lang: str, file_path: str) -> dict:
        with open(file_path, "rb") as f:
            r = self.session.post(
                self.api + f"/admin/media-files/{media_file_id}/subtitles",
                files={"file": (os.path.basename(file_path), f, "text/vtt")},
                data={"lang": lang},
                timeout=self.timeout,
            )
        self._check(r)
        return r.json()

    # ---- helpers ----

    def _check(self, r: requests.Response) -> None:
        if r.ok:
            return
        code, message = "error", (r.text or "")[:300]
        try:
            err = r.json().get("error") or {}
            code = err.get("code", code)
            message = err.get("message", message)
        except ValueError:
            pass
        request = r.request
        context = f"{request.method} {urlparse(request.url).path}" if request is not None else ""
        if r.status_code in (401, 403):
            raise AuthError(r.status_code, code, message, context)
        raise CouchverseError(r.status_code, code, message, context)
