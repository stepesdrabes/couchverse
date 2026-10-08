"""The transcode -> upload pipeline.

Two threads connected by a bounded queue: a transcoder (one ffmpeg at a time)
and an uploader (one upload at a time). The transcoder may run up to `lookahead`
files ahead of the uploader; when the buffer is full it blocks. This overlaps GPU
and network work without ever doubling up either. Pure Python so it can be tested
headlessly; the Qt layer runs run() in a worker thread and turns the callbacks
into signals."""

from __future__ import annotations

import os
import queue
import re
import shutil
import threading
import time
import uuid
from dataclasses import dataclass, field

import requests

from ..api.client import CouchverseClient, CouchverseError
from ..media.ffmpeg import (TranscodeCancelled, TranscodeError, TranscodeProgress,
                            build_transcode_cmd, run_transcode)
from ..media.ffprobe import probe
from ..media.plan import build_plan, normalize_lang, parse_language_preferences
from ..media.subs import extract_text_subs
from .matcher import ACTION_REPLACE, ACTION_UPLOAD
from .uploader import UploadCancelled, upload_file

_MIN_FREE_BYTES = 2 * 1024 ** 3  # always keep at least 2 GiB headroom in the temp dir


@dataclass
class BatchSettings:
    audio_lang: str
    temp_dir: str
    library_kind: str = "series"    # "series" or "movies"
    title_id: str = ""              # series: the show; movies: unused (per-job title_id)
    quality_preset: str = "high"
    encoder: str = "nvenc"          # "nvenc" or "cpu"
    keep_channels: bool = False
    lookahead: int = 3
    ffmpeg_path: str = "ffmpeg"
    ffprobe_path: str = "ffprobe"
    max_height: int = 0
    audio_mode: str = "single"       # "single" preference or "multiple" selected languages


@dataclass
class Job:
    index: int                  # row index in the UI table
    source_path: str
    rel_path: str               # for display/logging
    output_base: str            # clean temp output filename (no ext): "S01E02" or "Movie (1999)"
    audio_role: str = "primary"
    action: str = ACTION_UPLOAD  # upload | skip | replace
    episode_id: str | None = None   # series: existing episode
    title_id: str | None = None     # movies: existing title (None => create new from filename)
    replace_ids: list[str] = field(default_factory=list)  # series: files to delete on replace
    create_new_title: bool = False
    new_title_name: str = ""
    new_title_year: int | None = None


@dataclass
class _Ready:
    job: Job
    output_path: str
    work_dir: str
    subs: list[tuple[str, str]] = field(default_factory=list)
    audio_language: str = ""
    audio_languages: list[str] = field(default_factory=list)
    multi_audio: bool = False
    previous_primaries: list[tuple[str, str]] = field(default_factory=list)


class Callbacks:
    """Override these; all are invoked from worker threads."""

    def log(self, message: str) -> None: ...
    def item_state(self, index: int, state: str) -> None: ...
    def item_progress(self, index: int, phase: str, pct: float) -> None: ...
    def item_subs(self, index: int, langs: list[str]) -> None: ...
    def item_audio(self, index: int, language: str, role: str) -> None: ...
    def item_timing(self, index: int, phase: str, seconds: float) -> None: ...
    def item_done(self, index: int, media_file_id: str) -> None: ...
    def item_failed(self, index: int, error: str) -> None: ...
    def overall(self, pct: float, eta_seconds: float) -> None: ...
    def finished(self, summary: dict) -> None: ...


class Pipeline:
    def __init__(self, client: CouchverseClient, jobs: list[Job],
                 settings: BatchSettings, callbacks: Callbacks):
        from .eta import EtaEstimator

        self.client = client
        self.jobs = jobs
        self.settings = settings
        self.cb = callbacks
        self._cancel = threading.Event()
        self._producer_done = threading.Event()
        self._queue: queue.Queue = queue.Queue(maxsize=max(1, settings.lookahead))
        self._active_upload_id: str | None = None
        self._upload_lock = threading.Lock()
        self._temp: set[str] = set()
        self._temp_lock = threading.Lock()
        self.summary = {"done": [], "failed": [], "cancelled": False}
        os.makedirs(settings.temp_dir, exist_ok=True)
        sizes = {j.index: _safe_size(j.source_path) for j in jobs}
        self.eta = EtaEstimator([j.index for j in jobs], sizes)

    # ---- control ----

    def cancel(self) -> None:
        self._cancel.set()
        with self._upload_lock:
            upload_id = self._active_upload_id
        if upload_id:
            # HTTP aborts can take the client's full timeout; keep the UI responsive.
            threading.Thread(target=self._abort_upload, args=(upload_id,),
                             name="cp-abort", daemon=True).start()

    def _cancelled(self) -> bool:
        return self._cancel.is_set()

    def run(self) -> None:
        transcoder = threading.Thread(target=self._transcode_loop, name="cp-transcode", daemon=True)
        uploader = threading.Thread(target=self._upload_loop, name="cp-upload", daemon=True)
        try:
            transcoder.start()
            uploader.start()
            while transcoder.is_alive() or uploader.is_alive():
                pct, eta = self.eta.snapshot()
                self.cb.overall(pct, eta)
                time.sleep(0.1)
            transcoder.join()
            uploader.join()
        finally:
            self._cleanup_all_temp()
            self.summary["cancelled"] = self._cancelled()
            pct, eta = self.eta.snapshot()
            self.cb.overall(pct, 0.0 if not self._cancelled() else eta)
            self.cb.finished(self.summary)

    # ---- transcode side ----

    def _transcode_loop(self) -> None:
        try:
            for job in self.jobs:
                if self._cancelled():
                    break
                try:
                    ready = self._transcode_one(job)
                except TranscodeCancelled:
                    self._cancel.set()
                    break
                except Exception as e:
                    # Invalid input and external-tool errors belong to the row; never
                    # leave the upload worker waiting on a crashed producer.
                    self._fail_item(job.index, e, encoding=True)
                    continue
                # hand off, but stay responsive to cancel while the buffer is full
                handed_off = False
                while not self._cancelled():
                    try:
                        self._queue.put(ready, timeout=0.5)
                        handed_off = True
                        break
                    except queue.Full:
                        continue
                if not handed_off:
                    self._cleanup_item(ready)
        finally:
            # A sentinel put could block forever if cancellation stopped the consumer
            # with a full lookahead buffer. Completion is independent of queue space.
            self._producer_done.set()

    def _transcode_one(self, job: Job) -> _Ready:
        t0 = time.monotonic()
        work_dir = os.path.join(self.settings.temp_dir, uuid.uuid4().hex)
        os.makedirs(work_dir, exist_ok=True)
        self._track_temp(work_dir)
        try:
            return self._prepare_one(job, work_dir, t0)
        except Exception:
            self._cleanup_work_dir(work_dir)
            raise

    def _prepare_one(self, job: Job, work_dir: str, t0: float) -> _Ready:
        idx = job.index
        self._ensure_movie_target(job)
        self.cb.item_state(idx, "Probing")
        pr = probe(self.settings.ffprobe_path, job.source_path)
        if self._cancelled():
            raise TranscodeCancelled()
        self.eta.set_duration(idx, pr.duration)
        self._check_disk(_safe_size(job.source_path))

        plan = build_plan(pr, self.settings.audio_lang, self.settings.keep_channels,
                          self.settings.max_height, audio_mode=self.settings.audio_mode)
        if plan.video_resize:
            self.cb.log(f"{job.rel_path}: resizing {pr.width}x{pr.height} to "
                        f"{plan.output_width}x{plan.output_height}")
        self.cb.item_audio(idx, ", ".join(plan.audio_languages), job.audio_role)
        preferences = parse_language_preferences(self.settings.audio_lang)
        if plan.missing_audio_languages:
            self.cb.log(f"{job.rel_path}: requested audio not found: "
                        f"{', '.join(plan.missing_audio_languages)}; "
                        f"including {', '.join(plan.audio_languages) or 'untagged audio'}")
        if (self.settings.audio_mode == "single" and plan.has_audio
                and not plan.audio_lang_matched and preferences):
            selected = plan.audio_language or "untagged"
            self.cb.log(f"{job.rel_path}: no audio matching {', '.join(preferences)}; "
                        f"using first track ({selected})")
        if plan.hdr_tonemap:
            self.cb.log(f"{job.rel_path}: converting HDR to SDR for browser playback")
        if plan.multi_audio:
            self.cb.log(f"{job.rel_path}: including {', '.join(plan.audio_languages)} audio; "
                        "CouchVerse will prepare an HLS remux for language switching")

        self.cb.item_state(idx, "Extracting subtitles")
        subs = extract_text_subs(self.settings.ffmpeg_path, job.source_path,
                                 os.path.join(work_dir, "subs"), pr)
        self.cb.item_subs(idx, [lang for lang, _ in subs])
        if self._cancelled():
            raise TranscodeCancelled()

        # clean basename so the server can parse a movie name from new-movie uploads
        out = os.path.join(work_dir, f"{_safe_name(job.output_base)}.mp4")
        self.cb.item_state(idx, "Remuxing" if plan.video_copy else "Transcoding")
        cmd = build_transcode_cmd(self.settings.ffmpeg_path, job.source_path, out, plan,
                                  self.settings.quality_preset, self.settings.encoder)
        dur_us = pr.duration * 1_000_000

        def on_prog(p: TranscodeProgress) -> None:
            frac = min(1.0, p.out_time_us / dur_us) if dur_us else 0.0
            self.eta.on_encode_progress(idx, frac, p.speed)
            self.cb.item_progress(idx, "transcode", frac * 100)

        try:
            run_transcode(cmd, on_progress=on_prog, should_cancel=self._cancelled)
        except TranscodeError as gpu_error:
            if self.settings.encoder != "nvenc" or plan.video_copy or self._cancelled():
                raise
            self.cb.log(f"{job.rel_path}: NVENC failed ({_error_reason(gpu_error)}); "
                        "retrying with CPU")
            self.cb.item_state(idx, "Transcoding (CPU fallback)")
            self.eta.on_encode_progress(idx, 0.0, 0.0)
            try:
                cmd = build_transcode_cmd(self.settings.ffmpeg_path, job.source_path, out, plan,
                                          self.settings.quality_preset, "cpu")
                run_transcode(cmd, on_progress=on_prog, should_cancel=self._cancelled)
            except TranscodeCancelled:
                raise
            except Exception as cpu_error:
                raise TranscodeError(f"NVENC failed: {gpu_error}\n"
                                     f"CPU retry failed: {cpu_error}") from cpu_error
        self.cb.item_timing(idx, "transcode", time.monotonic() - t0)
        self.eta.on_encode_done(idx, _safe_size(out))
        return _Ready(job=job, output_path=out, work_dir=work_dir, subs=subs,
                      audio_language=plan.audio_language,
                      audio_languages=list(plan.audio_languages), multi_audio=plan.multi_audio)

    # ---- upload side ----

    def _upload_loop(self) -> None:
        while True:
            try:
                item = self._queue.get(timeout=0.1)
            except queue.Empty:
                if self._producer_done.is_set():
                    break
                continue
            if self._cancelled():
                self._cleanup_item(item)
                continue
            try:
                self._upload_one(item)
            except (UploadCancelled, TranscodeCancelled):
                self._cancel.set()
            except Exception as e:
                self._fail_item(item.job.index, e)
            finally:
                self._cleanup_item(item)
        self._drain_queue()

    def _upload_one(self, item: _Ready) -> None:
        idx = item.job.index
        job = item.job
        start = time.monotonic()

        if self._cancelled():
            raise UploadCancelled()
        self._ensure_movie_target(job)
        first_language = item.audio_languages[0] if item.audio_languages else item.audio_language
        normalized_language = normalize_lang(first_language)
        item.audio_language = normalized_language if re.fullmatch(r"[a-z]{2}", normalized_language) else ""
        old_media_ids, audio_role = self._resolve_existing(item)
        self.cb.item_audio(idx, ", ".join(item.audio_languages) or item.audio_language, audio_role)

        def on_prog(acked: int, total: int) -> None:
            frac = acked / total if total else 0.0
            elapsed = time.monotonic() - start
            self.eta.on_upload_progress(idx, frac, acked / elapsed if elapsed > 0 else 0.0)
            self.cb.item_progress(idx, "upload", frac * 100)

        def on_session(uid: str) -> None:
            with self._upload_lock:
                self._active_upload_id = uid

        self.cb.item_state(idx, "Uploading")
        completed = False
        try:
            upload_id = upload_file(self.client, item.output_path, on_session=on_session,
                                    on_progress=on_prog, should_cancel=self._cancelled)
            if self._cancelled():
                raise UploadCancelled()

            if self.settings.library_kind == "series":
                if job.episode_id:
                    media_file_id = self.client.upload_complete(upload_id, "series", episode_id=job.episode_id)
                else:  # new episode: server resolves SxxExx from the filename
                    media_file_id = self.client.upload_complete(upload_id, "series", title_id=self.settings.title_id)
            else:
                media_file_id = self.client.upload_complete(upload_id, "movies", title_id=job.title_id)
            completed = True
        finally:
            with self._upload_lock:
                active_id = self._active_upload_id
                self._active_upload_id = None
            if active_id and not completed:
                self._abort_upload(active_id)

        self.cb.item_state(idx, "Tagging audio")
        self.client.set_media_audio(media_file_id, item.audio_language, audio_role)
        if item.multi_audio:
            self._promote_combined(item, media_file_id)

        if item.subs:
            self.cb.item_state(idx, "Uploading subtitles")
            for sub_lang, sub_path in item.subs:
                if self._cancelled():
                    break
                try:
                    self.client.upload_subtitle(media_file_id, sub_lang, sub_path)
                except (CouchverseError, OSError, ValueError, requests.RequestException) as e:
                    self.cb.log(f"{job.rel_path}: subtitle '{sub_lang}' failed: {e}")

        if job.action == ACTION_REPLACE:
            for old_id in old_media_ids:
                if self._cancelled():
                    break
                if old_id == media_file_id:
                    continue
                try:
                    self.client.delete_media_file(old_id)
                except (CouchverseError, requests.RequestException) as e:
                    self.cb.log(f"{job.rel_path}: could not delete old file {old_id}: {e}")

        self.cb.item_timing(idx, "upload", time.monotonic() - start)
        self.eta.on_upload_done(idx)
        self._cleanup_item(item)
        self.summary["done"].append(idx)
        self.cb.item_state(idx, "Done")
        self.cb.item_done(idx, media_file_id)

    def _ensure_movie_target(self, job: Job) -> None:
        if self.settings.library_kind != "movies" or not job.create_new_title or job.title_id:
            return
        if self._cancelled():
            raise TranscodeCancelled()
        if not job.new_title_name.strip():
            raise ValueError("new movie title needs a name")
        self.cb.item_state(job.index, "Creating movie")
        self.cb.log(f"{job.rel_path}: creating movie target '{job.new_title_name}'")
        job.title_id = self.client.create_movie(job.new_title_name, job.new_title_year)
        if self._cancelled():
            raise TranscodeCancelled()

    def _resolve_existing(self, item: _Ready) -> tuple[list[str], str]:
        job = item.job
        item.previous_primaries.clear()
        title_id = job.title_id if self.settings.library_kind == "movies" else self.settings.title_id
        if not title_id or (self.settings.library_kind == "series" and not job.episode_id):
            return [], "primary" if item.multi_audio else job.audio_role
        detail = self.client.title_detail(title_id)
        if self.settings.library_kind == "series":
            owner_files = [m for m in detail.media_files if m.episode_id == job.episode_id]
        else:
            owner_files = detail.media_files
        files = [m for m in owner_files if m.is_live]
        lang = normalize_lang(item.audio_language)
        output_languages = {normalize_lang(a) for a in item.audio_languages if a}
        if lang:
            output_languages.add(lang)
        matching = []
        replaceable = []
        for media_file in files:
            known_languages = _known_audio_languages(media_file)
            tagged_language = normalize_lang(media_file.audio_lang)
            same_tag = tagged_language == lang if not item.multi_audio else tagged_language in output_languages
            if same_tag or (known_languages and known_languages.intersection(output_languages)):
                matching.append(media_file)
                if known_languages is not None:
                    safe = bool(known_languages) and known_languages.issubset(output_languages)
                else:
                    # Older servers do not report embedded streams. Keep their
                    # single-language behavior, but never infer multi-track coverage.
                    safe = (not item.multi_audio and same_tag
                            and media_file.audio_streams is None)
                if safe:
                    replaceable.append(media_file)
                elif job.action == ACTION_REPLACE:
                    self.cb.log(f"{job.rel_path}: keeping existing file {media_file.id}; "
                                "its full audio languages are unknown or not covered by this upload")
        if item.multi_audio:
            audio_role = "primary"
            item.previous_primaries = [(m.id, m.audio_lang) for m in owner_files
                                      if (m.audio_role or "primary") == "primary"]
        elif matching:
            # Replacing alternate Czech audio must not turn it into the primary file.
            audio_role = next((m.audio_role or "primary" for m in matching
                               if (m.audio_role or "primary") == "primary"),
                              matching[0].audio_role or "primary")
        else:
            audio_role = "audio_alt" if files else job.audio_role
        old_ids = [m.id for m in replaceable] if job.action == ACTION_REPLACE else []
        if job.action == ACTION_REPLACE and files and not matching:
            label = ", ".join(item.audio_languages) or lang or "untagged"
            role_label = "combined audio" if item.multi_audio else "alternate audio"
            self.cb.log(f"{job.rel_path}: no existing {label} audio to replace; "
                        f"keeping existing files and adding {role_label}")
        return old_ids, audio_role

    def _promote_combined(self, item: _Ready, media_file_id: str) -> None:
        for old_id, old_language in item.previous_primaries:
            if old_id == media_file_id:
                continue
            try:
                self.client.set_media_audio(old_id, old_language, "audio_alt")
            except Exception as error:
                raise RuntimeError("Combined audio uploaded, but could not make it the primary file: "
                                   f"{error}. All existing files were kept.") from error

    def _abort_upload(self, upload_id: str) -> None:
        try:
            self.client.upload_abort(upload_id)
        except (CouchverseError, OSError, requests.RequestException):
            pass

    def _fail_item(self, index: int, error: Exception, encoding: bool = False) -> None:
        self.cb.item_failed(index, str(error) or type(error).__name__)
        self.summary["failed"].append(index)
        if encoding:
            self.eta.on_encode_done(index, 0)
        self.eta.on_upload_done(index)

    # ---- temp + disk helpers ----

    def _check_disk(self, source_size: int) -> None:
        free = shutil.disk_usage(self.settings.temp_dir).free
        budget = max(_MIN_FREE_BYTES, int(source_size * 1.3))
        if free < budget:
            raise OSError(f"low disk space: {free // (1024**2)} MB free, need ~{budget // (1024**2)} MB")

    def _track_temp(self, path: str) -> None:
        with self._temp_lock:
            self._temp.add(path)

    def _cleanup_item(self, item: _Ready) -> None:
        self._cleanup_work_dir(item.work_dir)

    def _cleanup_work_dir(self, work_dir: str) -> None:
        _rmtree(work_dir)
        with self._temp_lock:
            self._temp.discard(work_dir)

    def _drain_queue(self) -> None:
        while True:
            try:
                item = self._queue.get_nowait()
            except queue.Empty:
                break
            self._cleanup_item(item)

    def _cleanup_all_temp(self) -> None:
        with self._temp_lock:
            paths = list(self._temp)
            self._temp.clear()
        for p in paths:
            _rmtree(p)


def _safe_size(path: str) -> int:
    try:
        return os.path.getsize(path)
    except OSError:
        return 0


def _rmtree(path: str) -> None:
    if path:
        shutil.rmtree(path, ignore_errors=True)


def _safe_name(name: str) -> str:
    cleaned = re.sub(r'[\\/:*?"<>|]+', " ", name).strip()
    return " ".join(cleaned.split()) or "output"


def _error_reason(error: Exception) -> str:
    return next((line.strip()[:240] for line in str(error).splitlines() if line.strip()),
                type(error).__name__)


def _known_audio_languages(media_file) -> set[str] | None:
    if not media_file.audio_streams:
        return None
    languages = {normalize_lang(a.lang) for a in media_file.audio_streams}
    if any(not re.fullmatch(r"[a-z]{2}", lang) for lang in languages):
        return None
    return languages
