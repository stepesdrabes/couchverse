"""Extract embedded text subtitle tracks to sidecar WebVTT files.

Only text codecs are handled (subrip/ass/mov_text/...); bitmap subs (PGS/VOBSUB)
would need OCR and are skipped. A single failing track is dropped rather than
failing the whole file."""

from __future__ import annotations

import os
import subprocess

from .ffprobe import ProbeResult
from .plan import TEXT_SUBTITLE_CODECS, normalize_lang

_NO_WINDOW = getattr(subprocess, "CREATE_NO_WINDOW", 0)


def extract_text_subs(ffmpeg_path: str, in_path: str, out_dir: str,
                      probe: ProbeResult) -> list[tuple[str, str]]:
    """Return [(lang, vtt_path)] for every text subtitle track. lang is a
    2-letter code, or "und" when the source tag is missing."""
    os.makedirs(out_dir, exist_ok=True)
    results: list[tuple[str, str]] = []
    for s in probe.subtitles:
        if s.codec not in TEXT_SUBTITLE_CODECS:
            continue
        lang = normalize_lang(s.language) or "und"
        out = os.path.join(out_dir, f"sub_{s.typed_index}_{lang}.vtt")
        cmd = [
            ffmpeg_path, "-hide_banner", "-loglevel", "error", "-nostdin", "-y",
            "-i", in_path, "-map", f"0:s:{s.typed_index}", "-c:s", "webvtt", out,
        ]
        try:
            proc = subprocess.run(
                cmd, capture_output=True, text=True, encoding="utf-8",
                errors="replace", creationflags=_NO_WINDOW,
            )
        except FileNotFoundError:
            return results
        if proc.returncode == 0 and os.path.exists(out) and os.path.getsize(out) > 0:
            results.append((lang, out))
    return results
