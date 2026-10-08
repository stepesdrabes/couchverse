"""Run a blocking callable off the UI thread and deliver its result via signals.

Used for the catalog network calls (login, list series, title detail) so a slow
response - e.g. a series with hundreds of episodes - never freezes the window."""

from __future__ import annotations

import threading
from typing import Callable

from PySide6.QtCore import QObject, Signal


class AsyncCall(QObject):
    done = Signal(object)
    failed = Signal(str)

    def __init__(self, fn: Callable[[], object]):
        super().__init__()
        self._fn = fn

    def start(self) -> None:
        threading.Thread(target=self._run, name="cp-async", daemon=True).start()

    def _run(self) -> None:
        try:
            result = self._fn()
        except Exception as e:  # any failure becomes a UI-side message
            self.failed.emit(str(e))
            return
        self.done.emit(result)
