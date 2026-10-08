"""Thin ffprobe wrapper returning just what the tool needs to plan a transcode."""

from __future__ import annotations

import json
import math
import re
import subprocess
from dataclasses import dataclass

# Avoid a console window flashing when ffprobe is spawned from the GUI on Windows.
_NO_WINDOW = getattr(subprocess, "CREATE_NO_WINDOW", 0)


class FfprobeError(RuntimeError):
    pass


@dataclass
class AudioStream:
    index: int          # absolute stream index in the file
    typed_index: int    # index among audio streams (for -map 0:a:<n>)
    codec: str
    channels: int
    language: str       # raw container tag, e.g. "eng", "cze", ""
    title: str


@dataclass
class SubtitleStream:
    index: int
    typed_index: int
    codec: str
    language: str
    title: str


@dataclass
class ProbeResult:
    duration: float
    has_video: bool
    video_codec: str
    pix_fmt: str
    profile: str
    width: int
    height: int
    audio: list[AudioStream]
    subtitles: list[SubtitleStream]
    video_index: int = 0
    color_transfer: str = ""
    color_primaries: str = ""
    color_space: str = ""
    color_range: str = ""
    dovi_profile: int = 0
    dovi_compatibility_id: int = 0

    @property
    def is_10bit(self) -> bool:
        """Include higher bit depths, which also need conversion for browser H.264."""
        pf = (self.pix_fmt or "").lower()
        if re.search(r"(?:p0|p)(?:10|12|14|16)(?:le|be)?$", pf):
            return True
        return any(depth in (self.profile or "").lower() for depth in ("10", "12", "16"))

    @property
    def is_hdr(self) -> bool:
        return self.color_transfer in {"smpte2084", "arib-std-b67"} or self.dovi_profile > 0


def probe(ffprobe_path: str, file_path: str, timeout: float = 120.0) -> ProbeResult:
    cmd = [ffprobe_path, "-v", "error", "-show_format", "-show_streams", "-of", "json", file_path]
    try:
        proc = subprocess.run(
            cmd, capture_output=True, text=True, encoding="utf-8", errors="replace",
            timeout=timeout, creationflags=_NO_WINDOW,
        )
    except FileNotFoundError as e:
        raise FfprobeError(f"ffprobe not found: {ffprobe_path}") from e
    except subprocess.TimeoutExpired as e:
        raise FfprobeError(f"ffprobe timed out after {timeout:g} seconds") from e
    if proc.returncode != 0:
        raise FfprobeError(proc.stderr.strip() or f"ffprobe exited {proc.returncode}")
    try:
        data = json.loads(proc.stdout)
    except ValueError as e:
        raise FfprobeError("could not parse ffprobe output") from e

    if not isinstance(data, dict):
        raise FfprobeError("ffprobe returned an invalid media description")
    return _parse_probe(data)


def _parse_probe(data: dict) -> ProbeResult:

    duration = _to_float((data.get("format") or {}).get("duration"))
    has_video = False
    video_codec = pix_fmt = profile = ""
    width = height = 0
    video_index = dovi_profile = dovi_compatibility_id = 0
    color_transfer = color_primaries = color_space = color_range = ""
    audio: list[AudioStream] = []
    subtitles: list[SubtitleStream] = []
    a_n = s_n = 0

    for st in data.get("streams") or []:
        kind = st.get("codec_type")
        tags = st.get("tags") or {}
        if kind == "video" and not has_video:
            # ignore cover-art/mjpeg attachments disguised as video
            if (st.get("disposition") or {}).get("attached_pic"):
                continue
            has_video = True
            video_codec = (st.get("codec_name") or "").lower()
            pix_fmt = st.get("pix_fmt") or ""
            profile = st.get("profile") or ""
            width = _to_int(st.get("width"))
            height = _to_int(st.get("height"))
            video_index = _to_int(st.get("index"))
            color_transfer = (st.get("color_transfer") or "").lower()
            color_primaries = (st.get("color_primaries") or "").lower()
            color_space = (st.get("color_space") or "").lower()
            color_range = (st.get("color_range") or "").lower()
            for side in st.get("side_data_list") or []:
                if "dv_profile" in side:
                    dovi_profile = _to_int(side.get("dv_profile"))
                    dovi_compatibility_id = _to_int(side.get("dv_bl_signal_compatibility_id"))
            if duration <= 0:
                duration = _to_float(st.get("duration"))
        elif kind == "audio":
            audio.append(AudioStream(
                index=_to_int(st.get("index")),
                typed_index=a_n,
                codec=(st.get("codec_name") or "").lower(),
                channels=_to_int(st.get("channels")),
                language=(tags.get("language") or "").lower(),
                title=tags.get("title") or "",
            ))
            a_n += 1
        elif kind == "subtitle":
            subtitles.append(SubtitleStream(
                index=_to_int(st.get("index")),
                typed_index=s_n,
                codec=(st.get("codec_name") or "").lower(),
                language=(tags.get("language") or "").lower(),
                title=tags.get("title") or "",
            ))
            s_n += 1

    return ProbeResult(duration, has_video, video_codec, pix_fmt, profile,
                       width, height, audio, subtitles, video_index,
                       color_transfer, color_primaries, color_space, color_range,
                       dovi_profile, dovi_compatibility_id)


def _to_float(v) -> float:
    try:
        value = float(v)
        return value if math.isfinite(value) else 0.0
    except (TypeError, ValueError):
        return 0.0


def _to_int(v) -> int:
    try:
        return int(v)
    except (TypeError, ValueError):
        return 0
