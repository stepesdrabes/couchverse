from __future__ import annotations

import os
import json
import shutil
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from dataclasses import replace
from pathlib import Path
from unittest.mock import patch

from couchpush.media.ffmpeg import (
    TranscodeCancelled,
    TranscodeError,
    build_transcode_cmd,
    run_transcode,
)
from couchpush.media.ffprobe import AudioStream, FfprobeError, ProbeResult, _parse_probe, probe
from couchpush.media.plan import (
    PlanError, build_plan, mp4_language_tag, parse_language_preferences, preferred_language,
)


def source(**changes) -> ProbeResult:
    result = ProbeResult(120, True, "hevc", "yuv420p10le", "Main 10", 3840, 2160, [
        AudioStream(1, 0, "ac3", 6, "cze", "Czech"),
        AudioStream(2, 1, "aac", 2, "eng", "English"),
    ], [])
    return replace(result, **changes)


class MediaPlanTests(unittest.TestCase):
    def test_preferences_are_ordered_normalized_and_deduplicated(self):
        self.assertEqual(parse_language_preferences(" ENG, cz; de en-US ces "), ["en", "cs", "de"])
        self.assertEqual(preferred_language("en, cs"), "en")
        self.assertEqual(preferred_language(""), "")
        with self.assertRaises(ValueError):
            parse_language_preferences("English, cs")

    def test_iron_man_language_list_selects_english_even_when_czech_is_first(self):
        plan = build_plan(source(), "en, cs", False)
        self.assertEqual(plan.audio_map, "0:a:1")
        self.assertEqual(plan.audio_language, "en")
        self.assertTrue(plan.audio_lang_matched)
        self.assertTrue(plan.audio_copy)

    def test_second_preference_is_used_when_first_is_absent(self):
        plan = build_plan(source(audio=source().audio[:1]), "en, cs", False)
        self.assertEqual(plan.audio_language, "cs")
        self.assertTrue(plan.audio_lang_matched)
        self.assertTrue(plan.audio_downmix)

    def test_multiple_audio_selects_one_track_per_language_in_requested_order(self):
        extra_english = replace(source().audio[1], index=3, typed_index=2, title="English commentary")
        plan = build_plan(source(audio=source().audio + [extra_english]), "eng, cs, en, cze", False,
                          audio_mode="multiple")
        self.assertTrue(plan.multi_audio)
        self.assertEqual(plan.audio_languages, ["en", "cs"])
        self.assertEqual([track.map for track in plan.audio_tracks], ["0:a:1", "0:a:0"])
        self.assertEqual(plan.missing_audio_languages, [])
        self.assertEqual(plan.audio_language, "en")
        self.assertEqual(plan.chosen_audio, source().audio[1])
        self.assertTrue(plan.audio_tracks[0].copy)
        self.assertFalse(plan.audio_tracks[0].downmix)
        self.assertFalse(plan.audio_tracks[1].copy)
        self.assertTrue(plan.audio_tracks[1].downmix)

    def test_multiple_audio_keeps_available_languages_without_fallback(self):
        plan = build_plan(source(), "de, cs, en", True, audio_mode="multiple")
        self.assertEqual(plan.audio_languages, ["cs", "en"])
        self.assertEqual(plan.missing_audio_languages, ["de"])
        self.assertFalse(plan.audio_tracks[0].downmix)
        partial = build_plan(source(audio=source().audio[:1]), "en, cs", False, audio_mode="multiple")
        self.assertEqual(partial.audio_languages, ["cs"])
        self.assertEqual(partial.missing_audio_languages, ["en"])
        self.assertFalse(partial.multi_audio)
        self.assertTrue(partial.audio_lang_matched)

    def test_multiple_audio_rejects_empty_no_matches_and_unsupported_mode(self):
        with self.assertRaisesRegex(PlanError, "at least one"):
            build_plan(source(), "", False, audio_mode="multiple")
        with self.assertRaisesRegex(PlanError, "No requested audio"):
            build_plan(source(), "de, fr", False, audio_mode="multiple")
        with self.assertRaisesRegex(PlanError, "No requested audio"):
            build_plan(source(audio=[]), "en", False, audio_mode="multiple")
        with self.assertRaises(PlanError):
            build_plan(source(), "en, cs", False, audio_mode="all")
        unknown = replace(source().audio[0], language="und")
        with self.assertRaisesRegex(PlanError, "No requested audio"):
            build_plan(source(audio=[unknown]), "en", False, audio_mode="multiple")

    def test_mp4_languages_use_recognized_three_letter_tags(self):
        self.assertEqual(mp4_language_tag("en"), "eng")
        self.assertEqual(mp4_language_tag("cze"), "ces")
        self.assertEqual(mp4_language_tag("de"), "deu")
        self.assertEqual(mp4_language_tag("zz"), "und")
        self.assertEqual(parse_language_preferences("isl, baq, lat"), ["is", "eu", "la"])
        for tag in ("foo", "zz", "en, cs"):
            unknown = replace(source().audio[0], language=tag)
            self.assertEqual(build_plan(source(audio=[unknown]), "en", False).audio_language, "")

    def test_multiple_audio_command_encodes_video_once_and_configures_each_track(self):
        plan = build_plan(source(), "en, cs", False, audio_mode="multiple")
        command = build_transcode_cmd("ffmpeg", "input.mkv", "output.mp4", plan, encoder="cpu")
        self.assertEqual(command.count("-c:v"), 1)
        self.assertEqual(command.count("-map"), 3)
        self.assertEqual(command[command.index("-c:a:0") + 1], "copy")
        self.assertEqual(command[command.index("-c:a:1") + 1], "aac")
        self.assertEqual(command[command.index("-ac:a:1") + 1], "2")
        self.assertNotIn("-ac:a:0", command)
        self.assertEqual(command[command.index("-disposition:a:0") + 1], "default")
        self.assertEqual(command[command.index("-disposition:a:1") + 1], "0")
        self.assertIn("language=eng", command)
        self.assertIn("language=ces", command)

    def test_fallback_keeps_actual_language_and_does_not_invent_unknown_tags(self):
        plan = build_plan(source(audio=source().audio[:1]), "de", False)
        self.assertFalse(plan.audio_lang_matched)
        self.assertEqual(plan.audio_language, "cs")
        untagged = replace(source().audio[0], language="und")
        self.assertEqual(build_plan(source(audio=[untagged]), "en", False).audio_language, "")

    def test_smart_copy_requires_sdr_eight_bit_four_two_zero(self):
        self.assertTrue(build_plan(source(video_codec="h264", pix_fmt="yuv420p", profile="High"), "en", False).video_copy)
        for pix_fmt in ("yuv420p10le", "yuv420p12le", "yuv422p", "yuv444p", ""):
            with self.subTest(pix_fmt=pix_fmt):
                self.assertFalse(build_plan(source(video_codec="h264", pix_fmt=pix_fmt), "en", False).video_copy)
        self.assertTrue(source(pix_fmt="yuv420p12le").is_10bit)
        self.assertTrue(source(pix_fmt="p016le").is_10bit)

    def test_hdr10_hlg_and_dolby_vision_hdr_base_are_tone_mapped(self):
        for transfer in ("smpte2084", "arib-std-b67"):
            self.assertTrue(build_plan(source(color_transfer=transfer), "en", False).hdr_tonemap)
        dv7 = build_plan(source(dovi_profile=7), "en", False)
        self.assertTrue(dv7.hdr_tonemap)
        self.assertEqual(dv7.color_transfer, "smpte2084")
        dv8 = build_plan(source(dovi_profile=8, dovi_compatibility_id=4), "en", False)
        self.assertEqual(dv8.color_transfer, "arib-std-b67")

    def test_no_video_and_dolby_vision_only_sources_fail_with_clear_reason(self):
        with self.assertRaisesRegex(PlanError, "No playable video"):
            build_plan(source(has_video=False), "en", False)
        with self.assertRaisesRegex(PlanError, "Dolby Vision profile 5"):
            build_plan(source(dovi_profile=5), "en", False)
        with self.assertRaisesRegex(PlanError, "no supported"):
            build_plan(source(dovi_profile=8), "en", False)
        with self.assertRaisesRegex(PlanError, "no supported"):
            build_plan(source(dovi_profile=8, color_transfer="smpte2084"), "en", False)

    def test_nvenc_uhd_has_no_forced_level_and_maps_the_probed_stream(self):
        plan = build_plan(source(video_index=3), "en, cs", False)
        command = build_transcode_cmd("ffmpeg", "movie.mkv", "movie.mp4", plan)
        self.assertNotIn("-level", command)
        self.assertNotIn("-level:v", command)
        self.assertEqual(command[command.index("-map") + 1], "0:3")
        self.assertIn("h264_nvenc", command)

    def test_remux_has_no_gpu_decode_or_filters_and_no_audio_means_one_map(self):
        plan = build_plan(source(video_codec="h264", pix_fmt="yuv420p", profile="High", audio=[]), "en", False)
        command = build_transcode_cmd("ffmpeg", "movie.mkv", "movie.mp4", plan)
        self.assertNotIn("-hwaccel", command)
        self.assertNotIn("-vf", command)
        self.assertEqual(command.count("-map"), 1)

    def test_resolution_selection_forces_encode_only_when_source_exceeds_box(self):
        original = source(video_codec="h264", pix_fmt="yuv420p", profile="High")
        plan = build_plan(original, "en", False, max_height=1080)
        self.assertFalse(plan.video_copy)
        self.assertTrue(plan.video_resize)
        self.assertEqual((plan.output_width, plan.output_height), (1920, 1080))
        smaller = build_plan(replace(original, width=1280, height=720), "en", False, max_height=1080)
        self.assertTrue(smaller.video_copy)
        self.assertFalse(smaller.video_resize)
        self.assertEqual((smaller.output_width, smaller.output_height), (1280, 720))
        unchanged = build_plan(original, "en", False)
        self.assertTrue(unchanged.video_copy)
        self.assertEqual((unchanged.output_width, unchanged.output_height), (3840, 2160))

    def test_resolution_preserves_cinemascope_and_portrait_and_never_upscales(self):
        for width, height, expected in ((3840, 1608, (1920, 804)),
                                         (1080, 1920, (606, 1080)),
                                         (640, 360, (640, 360))):
            with self.subTest(width=width, height=height):
                plan = build_plan(source(width=width, height=height), "en", False, max_height=1080)
                self.assertEqual((plan.output_width, plan.output_height), expected)
                self.assertLessEqual(plan.output_width, width)
                self.assertLessEqual(plan.output_height, height)
                self.assertEqual(plan.output_width % 2, 0)
                self.assertEqual(plan.output_height % 2, 0)
                self.assertAlmostEqual(plan.output_width / plan.output_height, width / height, delta=0.003)

    def test_resolution_limits_and_filter_order(self):
        for height in (0, 480, 720, 1080, 2160):
            plan = build_plan(source(color_transfer="smpte2084"), "en", False, max_height=height)
            if height:
                self.assertLessEqual(plan.output_height, height)
        with self.assertRaises(PlanError):
            build_plan(source(), "en", False, max_height=1440)
        plan = build_plan(source(color_transfer="smpte2084"), "en", False, max_height=1080)
        command = build_transcode_cmd("ffmpeg", "movie.mkv", "movie.mp4", plan)
        filters = command[command.index("-vf") + 1]
        self.assertTrue(filters.startswith("scale=1920:1080"))
        self.assertLess(filters.index("scale="), filters.index("tonemap="))

    def test_probe_skips_cover_art_and_preserves_video_index_and_hdr_metadata(self):
        data = {"streams": [
            {"index": 0, "codec_type": "video", "codec_name": "mjpeg", "disposition": {"attached_pic": 1}},
            {"index": 1, "codec_type": "audio", "codec_name": "aac", "channels": 2, "tags": {"language": "eng"}},
            {"index": 2, "codec_type": "video", "codec_name": "hevc", "pix_fmt": "yuv420p10le",
             "width": 3840, "height": 2160, "color_transfer": "smpte2084", "color_primaries": "bt2020",
             "color_space": "bt2020nc", "side_data_list": [{"dv_profile": 8, "dv_bl_signal_compatibility_id": 1}]},
        ]}
        result = _parse_probe(data)
        self.assertEqual(result.video_index, 2)
        self.assertEqual(result.video_codec, "hevc")
        self.assertEqual(result.dovi_profile, 8)
        self.assertTrue(result.is_hdr)
        self.assertEqual(build_plan(result, "en", False).video_map, "0:2")

    def test_probe_timeout_is_an_actionable_media_error(self):
        with patch("couchpush.media.ffprobe.subprocess.run", side_effect=subprocess.TimeoutExpired("ffprobe", 10)):
            with self.assertRaisesRegex(FfprobeError, "timed out after 10"):
                probe("ffprobe", "movie.mkv", timeout=10)


class TranscodeCancellationTests(unittest.TestCase):
    def test_cancel_works_while_encoder_is_silent(self):
        cancelled = threading.Event()
        timer = threading.Timer(0.3, cancelled.set)
        timer.start()
        started = time.monotonic()
        try:
            with self.assertRaises(TranscodeCancelled):
                run_transcode([sys.executable, "-c", "import time; time.sleep(30)"], should_cancel=cancelled.is_set)
        finally:
            timer.cancel()
        self.assertLess(time.monotonic() - started, 5)


@unittest.skipUnless(shutil.which("ffmpeg") and shutil.which("ffprobe"), "ffmpeg and ffprobe are required")
class RealMediaTests(unittest.TestCase):
    def _audio_hash(self, path: str, index: int) -> str:
        result = subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-nostdin",
                                 "-i", path, "-map", f"0:a:{index}", "-c:a", "copy",
                                 "-f", "hash", "-hash", "sha256", "-"],
                                capture_output=True, text=True, timeout=20)
        self.assertEqual(result.returncode, 0, result.stderr)
        return result.stdout.strip()

    def test_two_language_mp4_copies_aac_downmixes_ac3_and_preserves_default_order(self):
        with tempfile.TemporaryDirectory() as directory:
            path = str(Path(directory) / "mixed.mkv")
            command = ["ffmpeg", "-hide_banner", "-loglevel", "error", "-nostdin", "-y",
                       "-f", "lavfi", "-i", "color=gray:size=160x90:rate=24",
                       "-f", "lavfi", "-i", "anullsrc=channel_layout=5.1:sample_rate=48000",
                       "-f", "lavfi", "-i", "sine=frequency=880:sample_rate=48000",
                       "-map", "0:v", "-map", "1:a", "-map", "2:a", "-t", "0.25",
                       "-c:v", "ffv1", "-c:a:0", "ac3", "-b:a:0", "384k",
                       "-c:a:1", "aac", "-ac:a:1", "2", "-b:a:1", "192k",
                       "-metadata:s:a:0", "language=cze", "-metadata:s:a:1", "language=eng", path]
            generated = subprocess.run(command, capture_output=True, text=True, timeout=20)
            self.assertEqual(generated.returncode, 0, generated.stderr)
            plan = build_plan(probe("ffprobe", path), "en, cs, en", False, audio_mode="multiple")
            output = str(Path(directory) / "languages.mp4")
            run_transcode(build_transcode_cmd("ffmpeg", path, output, plan, encoder="cpu"))
            result = probe("ffprobe", output)
            self.assertEqual(result.video_codec, "h264")
            self.assertEqual(result.pix_fmt, "yuv420p")
            self.assertEqual([audio.codec for audio in result.audio], ["aac", "aac"])
            self.assertEqual([audio.language for audio in result.audio], ["eng", "ces"])
            self.assertEqual([audio.channels for audio in result.audio], [2, 2])
            self.assertEqual(self._audio_hash(path, 1), self._audio_hash(output, 0))
            metadata = subprocess.run(["ffprobe", "-v", "error", "-show_streams", "-of", "json", output],
                                      capture_output=True, text=True, timeout=20)
            self.assertEqual(metadata.returncode, 0, metadata.stderr)
            streams = json.loads(metadata.stdout)["streams"]
            self.assertEqual(len([stream for stream in streams if stream["codec_type"] == "video"]), 1)
            audio = [stream for stream in streams if stream["codec_type"] == "audio"]
            self.assertEqual([stream["disposition"]["default"] for stream in audio], [1, 0])

    def _generate_hdr(self, directory: str, size: str = "160x90") -> str:
        path = str(Path(directory) / "hdr.mkv")
        command = ["ffmpeg", "-hide_banner", "-loglevel", "error", "-nostdin", "-y",
                   "-f", "lavfi", "-i", f"color=gray:size={size}:rate=24",
                   "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000",
                   "-f", "lavfi", "-i", "sine=frequency=880:sample_rate=48000",
                   "-map", "0:v", "-map", "1:a", "-map", "2:a", "-t", "0.125",
                   "-c:v", "libx265", "-preset", "ultrafast", "-x265-params", "pools=1:frame-threads=1:log-level=error",
                   "-pix_fmt", "yuv420p10le", "-color_primaries", "bt2020",
                   "-color_trc", "smpte2084", "-colorspace", "bt2020nc",
                   "-vf", "setparams=color_primaries=bt2020:color_trc=smpte2084:colorspace=bt2020nc",
                   "-c:a", "pcm_s16le", "-metadata:s:a:0", "language=cze",
                   "-metadata:s:a:1", "language=eng", path]
        result = subprocess.run(command, capture_output=True, text=True, timeout=30)
        self.assertEqual(result.returncode, 0, result.stderr)
        return path

    def test_hdr10_cpu_output_is_sdr_h264_with_only_selected_aac_track(self):
        with tempfile.TemporaryDirectory() as directory:
            path = self._generate_hdr(directory)
            input_probe = probe("ffprobe", path)
            self.assertTrue(input_probe.is_10bit)
            self.assertTrue(input_probe.is_hdr)
            plan = build_plan(input_probe, "en, cs", False)
            output = str(Path(directory) / "output.mp4")
            run_transcode(build_transcode_cmd("ffmpeg", path, output, plan, encoder="cpu"))
            result = probe("ffprobe", output)
            self.assertEqual(result.video_codec, "h264")
            self.assertEqual(result.pix_fmt, "yuv420p")
            self.assertEqual(result.color_transfer, "bt709")
            self.assertEqual(result.color_primaries, "bt709")
            self.assertEqual(result.color_space, "bt709")
            self.assertEqual(len(result.audio), 1)

            self.assertEqual(result.audio[0].codec, "aac")
            self.assertEqual(result.audio[0].language, "eng")
            self.assertFalse(result.is_hdr)

    @unittest.skipUnless(os.environ.get("COUCHPUSH_NVENC_TESTS") == "1", "set COUCHPUSH_NVENC_TESTS=1 on an NVIDIA PC")
    def test_uhd_nvenc_reproduces_invalid_level_and_encodes_with_auto_level(self):
        with tempfile.TemporaryDirectory() as directory:
            path = self._generate_hdr(directory, "3840x2160")
            plan = build_plan(probe("ffprobe", path), "en, cs", False)
            output = str(Path(directory) / "uhd.mp4")
            command = build_transcode_cmd("ffmpeg", path, output, plan)
            with self.assertRaisesRegex(TranscodeError, "Invalid Level"):
                run_transcode(command[:-1] + ["-level", "4.2", command[-1]])
            run_transcode(command)
            result = probe("ffprobe", output)
            self.assertEqual((result.width, result.height), (3840, 2160))
            self.assertEqual(result.video_codec, "h264")
            self.assertEqual(result.pix_fmt, "yuv420p")
            self.assertEqual(result.color_transfer, "bt709")
            self.assertEqual(len(result.audio), 1)

            resized = build_plan(probe("ffprobe", path), "en, cs", False, max_height=1080)
            downscaled = str(Path(directory) / "1080p.mp4")
            run_transcode(build_transcode_cmd("ffmpeg", path, downscaled, resized))
            result = probe("ffprobe", downscaled)
            self.assertEqual((result.width, result.height), (1920, 1080))
            self.assertEqual((resized.output_width, resized.output_height), (result.width, result.height))
            self.assertEqual(result.video_codec, "h264")
            self.assertEqual(result.pix_fmt, "yuv420p")
            self.assertEqual(result.color_transfer, "bt709")
            self.assertEqual(len(result.audio), 1)


if __name__ == "__main__":
    unittest.main()
