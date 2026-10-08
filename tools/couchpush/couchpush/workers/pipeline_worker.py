"""Bridge the pure-Python Pipeline to Qt.

PipelineEmitter implements the Pipeline.Callbacks duck-type; each method emits a
Qt signal. Because the emitter lives in the UI thread, signals fired from the
pipeline's worker threads are delivered as queued connections, so UI slots always
run on the UI thread. The pipeline itself is driven from a plain background thread
(it manages its own transcode/upload threads internally)."""

from __future__ import annotations

import threading

from PySide6.QtCore import QObject, Signal

from ..core.pipeline import BatchSettings, Job, Pipeline


class PipelineEmitter(QObject):
    sig_log = Signal(str)
    sig_item_state = Signal(int, str)
    sig_item_progress = Signal(int, str, float)
    sig_item_subs = Signal(int, object)
    sig_item_audio = Signal(int, str, str)
    sig_item_timing = Signal(int, str, float)
    sig_item_done = Signal(int, str)
    sig_item_failed = Signal(int, str)
    sig_overall = Signal(float, float)
    sig_finished = Signal(dict)

    # Pipeline.Callbacks interface (called from worker threads):
    def log(self, message: str) -> None:
        self.sig_log.emit(message)

    def item_state(self, index: int, state: str) -> None:
        self.sig_item_state.emit(index, state)

    def item_progress(self, index: int, phase: str, pct: float) -> None:
        self.sig_item_progress.emit(index, phase, pct)

    def item_subs(self, index: int, langs: list[str]) -> None:
        self.sig_item_subs.emit(index, langs)

    def item_audio(self, index: int, language: str, role: str) -> None:
        self.sig_item_audio.emit(index, language, role)

    def item_timing(self, index: int, phase: str, seconds: float) -> None:
        self.sig_item_timing.emit(index, phase, seconds)

    def item_done(self, index: int, media_file_id: str) -> None:
        self.sig_item_done.emit(index, media_file_id)

    def item_failed(self, index: int, error: str) -> None:
        self.sig_item_failed.emit(index, error)

    def overall(self, pct: float, eta_seconds: float) -> None:
        self.sig_overall.emit(pct, eta_seconds)

    def finished(self, summary: dict) -> None:
        self.sig_finished.emit(summary)


class PipelineRunner:
    def __init__(self, client, jobs: list[Job], settings: BatchSettings):
        self.emitter = PipelineEmitter()
        self.pipeline = Pipeline(client, jobs, settings, self.emitter)
        self._thread: threading.Thread | None = None

    def start(self) -> None:
        if self._thread is not None:
            raise RuntimeError("this batch has already been started")
        self._thread = threading.Thread(target=self.pipeline.run, name="cp-pipeline", daemon=True)
        self._thread.start()

    def cancel(self) -> None:
        self.pipeline.cancel()

    def is_running(self) -> bool:
        return self._thread is not None and self._thread.is_alive()
