"""Build and run the transcode command, parsing ffmpeg's -progress stream.

Default encoder is NVENC (RTX), with CUDA decode and CPU color conversion.
PQ/HLG sources are tone-mapped to SDR before the 8-bit H.264 encode. Output is
always MP4 / h264 / AAC. Multiple selected audio tracks use the server's
selectable-audio HLS preparation."""

from __future__ import annotations

import subprocess
import threading
from dataclasses import dataclass
from typing import Callable

from .plan import TranscodePlan, mp4_language_tag

_NO_WINDOW = getattr(subprocess, "CREATE_NO_WINDOW", 0)

# preset -> NVENC preset (p1 fast .. p7 best), cq / x264 crf, and VBV ceilings.
# The common tiers use p4: ~1.5x faster than p5 on a 3060 with near-identical size.
# Each +3 cq is roughly -27% file size.
QUALITY_PRESETS: dict[str, dict] = {
    "max":   {"nv_preset": "p6", "quality": 19, "maxrate": "60M", "bufsize": "120M"},
    "high":  {"nv_preset": "p4", "quality": 23, "maxrate": "40M", "bufsize": "80M"},
    "small": {"nv_preset": "p4", "quality": 26, "maxrate": "24M", "bufsize": "48M"},
    "tiny":  {"nv_preset": "p4", "quality": 29, "maxrate": "16M", "bufsize": "32M"},
}


class TranscodeError(RuntimeError):
    pass


class TranscodeCancelled(Exception):
    pass


@dataclass
class TranscodeProgress:
    out_time_us: int
    total_size: int
    speed: float   # realtime multiple parsed from "2.5x"; 0.0 if unknown
    done: bool


def build_transcode_cmd(ffmpeg_path: str, in_path: str, out_path: str, plan: TranscodePlan,
                        preset_name: str = "high", encoder: str = "nvenc") -> list[str]:
    q = QUALITY_PRESETS.get(preset_name, QUALITY_PRESETS["high"])
    cmd = [ffmpeg_path, "-hide_banner", "-nostdin", "-y", "-loglevel", "error"]

    # GPU decode only pays off when we actually re-encode the video.
    if not plan.video_copy and encoder == "nvenc":
        cmd += ["-hwaccel", "cuda"]
    cmd += ["-i", in_path]

    # Video is mapped once; audio order determines the default player language.
    cmd += ["-map", plan.video_map]
    for track in plan.audio_tracks:
        cmd += ["-map", track.map]

    cmd += _video_args(plan, q, encoder)
    if not plan.video_copy:
        filters = []
        if plan.video_resize:
            # Resize before float HDR processing to reduce CPU and memory work on UHD.
            filters.append(f"scale={plan.output_width}:{plan.output_height}:flags=lanczos")
        if plan.hdr_tonemap:
            filters.append(_hdr_filter(plan))
        # 4:2:0 encoders need even dimensions; pad one edge without cropping the image.
        filters.append("pad=ceil(iw/2)*2:ceil(ih/2)*2")
        cmd += ["-vf", ",".join(filters)]
        if plan.hdr_tonemap:
            cmd += ["-color_primaries", "bt709", "-color_trc", "bt709",
                    "-colorspace", "bt709", "-color_range", "tv"]

    for index, track in enumerate(plan.audio_tracks):
        if track.copy:
            cmd += [f"-c:a:{index}", "copy"]
        else:
            cmd += [f"-c:a:{index}", "aac", f"-b:a:{index}", "192k"]
            if track.downmix:
                cmd += [f"-ac:a:{index}", "2"]
        cmd += [f"-metadata:s:a:{index}", f"language={mp4_language_tag(track.language)}",
                f"-disposition:a:{index}", "default" if index == 0 else "0"]

    cmd += ["-movflags", "+faststart", "-progress", "pipe:1", "-nostats", out_path]
    return cmd


def _video_args(plan: TranscodePlan, q: dict, encoder: str) -> list[str]:
    if plan.video_copy:
        return ["-c:v", "copy"]
    if encoder == "cpu":
        return [
            "-c:v", "libx264", "-preset", "medium", "-crf", str(q["quality"]),
            "-pix_fmt", "yuv420p", "-profile:v", "high",
        ]
    return [
        "-c:v", "h264_nvenc", "-preset", q["nv_preset"], "-tune", "hq",
        "-rc", "vbr", "-cq", str(q["quality"]), "-b:v", "0",
        "-maxrate", q["maxrate"], "-bufsize", q["bufsize"],
        # Let NVENC choose a level that fits the resolution, frame rate and bitrate.
        "-profile:v", "high", "-pix_fmt", "yuv420p",
        "-rc-lookahead", "20", "-bf", "3", "-b_ref_mode", "middle",
        "-spatial_aq", "1", "-temporal_aq", "1", "-aq-strength", "8",
    ]


def _hdr_filter(plan: TranscodePlan) -> str:
    primaries = plan.color_primaries if plan.color_primaries not in {"", "unknown", "unspecified"} else "bt2020"
    matrix = plan.color_space if plan.color_space not in {"", "unknown", "unspecified"} else "bt2020nc"
    source_range = "full" if plan.color_range in {"pc", "full"} else "limited"
    return (
        f"zscale=primariesin={primaries}:transferin={plan.color_transfer}:"
        f"matrixin={matrix}:rangein={source_range}:transfer=linear:npl=100,"
        "format=gbrpf32le,zscale=primaries=bt709,tonemap=tonemap=hable:desat=2,"
        "zscale=transfer=bt709:matrix=bt709:range=limited:dither=error_diffusion,"
        # HDR/Dolby Vision side data describes the source, not the SDR output.
        "format=yuv420p,sidedata=mode=delete"
    )


def run_transcode(cmd: list[str], on_progress: Callable[[TranscodeProgress], None] | None = None,
                  should_cancel: Callable[[], bool] | None = None) -> None:
    """Run ffmpeg, feeding parsed -progress blocks to on_progress. Raises
    TranscodeCancelled if should_cancel() turns true, TranscodeError on failure."""
    if should_cancel and should_cancel():
        raise TranscodeCancelled()
    try:
        proc = subprocess.Popen(
            cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            text=True, encoding="utf-8", errors="replace", bufsize=1,
            creationflags=_NO_WINDOW,
        )
    except FileNotFoundError as e:
        raise TranscodeError(f"ffmpeg not found: {cmd[0]}") from e

    stderr_chunks: list[str] = []
    drainer = threading.Thread(target=_drain, args=(proc.stderr, stderr_chunks), daemon=True)
    drainer.start()

    cancelled = threading.Event()
    stop_watcher = threading.Event()

    def watch_cancel() -> None:
        while not stop_watcher.wait(0.2):
            if should_cancel and should_cancel():
                cancelled.set()
                _kill(proc)
                break

    watcher = threading.Thread(target=watch_cancel, daemon=True)
    watcher.start()
    fields: dict[str, str] = {}
    try:
        assert proc.stdout is not None
        for line in proc.stdout:
            if should_cancel and should_cancel():
                cancelled.set()
                _kill(proc)
                break
            line = line.strip()
            if "=" not in line:
                continue
            key, value = line.split("=", 1)
            fields[key] = value
            if key == "progress":
                if on_progress:
                    on_progress(_parse(fields, done=value == "end"))
                if value == "end":
                    break
    except BaseException:
        _kill(proc)
        raise
    finally:
        rc = proc.wait()
        stop_watcher.set()
        watcher.join(timeout=6.0)
        proc.stdout.close() if proc.stdout else None
        drainer.join(timeout=2.0)
        proc.stderr.close() if proc.stderr else None

    if cancelled.is_set():
        raise TranscodeCancelled()
    if rc != 0:
        raise TranscodeError("".join(stderr_chunks).strip() or f"ffmpeg exited {rc}")


def _parse(fields: dict[str, str], done: bool) -> TranscodeProgress:
    return TranscodeProgress(
        out_time_us=_int(fields.get("out_time_us") or fields.get("out_time_ms")),
        total_size=_int(fields.get("total_size")),
        speed=_speed(fields.get("speed")),
        done=done,
    )


def _drain(stream, sink: list[str]) -> None:
    if stream is None:
        return
    for line in stream:
        sink.append(line)


def _kill(proc: subprocess.Popen) -> None:
    if proc.poll() is not None:
        return
    try:
        proc.terminate()
    except OSError:
        return
    try:
        proc.wait(timeout=5)
    except subprocess.TimeoutExpired:
        proc.kill()


def _int(v) -> int:
    try:
        return int(v)
    except (TypeError, ValueError):
        return 0


def _speed(v) -> float:
    if not v:
        return 0.0
    try:
        return float(str(v).rstrip("x").strip())
    except ValueError:
        return 0.0
