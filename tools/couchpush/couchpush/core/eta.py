"""Self-calibrating ETA across the overlapped transcode+upload pipeline.

Overall percent is job-count based (each episode is half transcode, half upload)
so the bar moves intuitively. The time estimate is separate: it seeds an encode
speed (realtime multiple) and an upload rate (bytes/s), refines both with an EWMA
as real measurements arrive, and reports remaining wall-clock as the max of the
remaining transcode tail and remaining upload total (the two run in parallel).
Honest, not exact. Keyed by job index so sparse table-row indices work."""

from __future__ import annotations

import threading


class EtaEstimator:
    def __init__(self, indices: list[int], sizes: dict[int, int],
                 seed_speed: float = 4.0, seed_rate: float = 6_000_000.0):
        self._lock = threading.Lock()
        self.indices = list(indices)
        self.durations = {i: 0.0 for i in indices}       # source seconds
        self.sizes = dict(sizes)                          # source bytes (upload proxy)
        self.out_sizes: dict[int, int] = {}               # actual output bytes once known
        self.enc_frac = {i: 0.0 for i in indices}
        self.up_frac = {i: 0.0 for i in indices}
        self.transcoded = {i: False for i in indices}
        self.uploaded = {i: False for i in indices}
        self.speed = seed_speed
        self.rate = seed_rate

    def set_duration(self, i: int, duration: float) -> None:
        with self._lock:
            self.durations[i] = duration

    def on_encode_progress(self, i: int, frac: float, speed: float) -> None:
        with self._lock:
            self.enc_frac[i] = frac
            if speed and speed > 0:
                self.speed = self._ewma(self.speed, speed)

    def on_encode_done(self, i: int, output_size: int) -> None:
        with self._lock:
            self.enc_frac[i] = 1.0
            self.transcoded[i] = True
            if output_size:
                self.out_sizes[i] = output_size

    def on_upload_progress(self, i: int, frac: float, rate: float) -> None:
        with self._lock:
            self.up_frac[i] = frac
            if rate and rate > 0:
                self.rate = self._ewma(self.rate, rate)

    def on_upload_done(self, i: int) -> None:
        with self._lock:
            self.up_frac[i] = 1.0
            self.uploaded[i] = True

    def snapshot(self) -> tuple[float, float]:
        """Return (overall_percent, eta_seconds)."""
        with self._lock:
            n = len(self.indices)
            if n == 0:
                return 100.0, 0.0
            t_remain = sum(self._est_enc(i) * (1 - self.enc_frac[i])
                           for i in self.indices if not self.transcoded[i])
            u_remain = sum(self._est_up(i) * (1 - self.up_frac[i])
                           for i in self.indices if not self.uploaded[i])
            eta = max(t_remain, u_remain)
            done = sum(0.5 * self.enc_frac[i] + 0.5 * self.up_frac[i] for i in self.indices)
            return done / n * 100.0, eta

    def _est_enc(self, i: int) -> float:
        d = self.durations[i]
        return max(d / self.speed, 0.5) if d else 1.0

    def _est_up(self, i: int) -> float:
        size = self.out_sizes.get(i) or self.sizes.get(i, 0)
        return size / self.rate if self.rate > 0 else 0.0

    @staticmethod
    def _ewma(old: float, new: float, alpha: float = 0.3) -> float:
        return new if old <= 0 else (1 - alpha) * old + alpha * new
