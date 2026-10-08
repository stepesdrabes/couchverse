from __future__ import annotations

import subprocess
import tempfile
import threading
import unittest
from pathlib import Path
from unittest.mock import MagicMock, patch

import requests

from couchpush.api.models import EmbeddedAudioStream, MediaFile, TitleDetail
from couchpush.api.client import CouchverseError, OffsetConflict
from couchpush.core.matcher import ACTION_REPLACE
from couchpush.core.pipeline import BatchSettings, Callbacks, Job, Pipeline, _Ready
from couchpush.core.uploader import UploadCancelled, UploadError, upload_file
from couchpush.media.ffmpeg import TranscodeError
from couchpush.media.ffprobe import AudioStream, ProbeResult


class Recorder(Callbacks):
    def __init__(self):
        self.logs = []
        self.failures = []
        self.results = []
        self.audio = []

    def log(self, message):
        self.logs.append(message)

    def item_failed(self, index, error):
        self.failures.append((index, error))

    def finished(self, summary):
        self.results.append(summary.copy())

    def item_audio(self, index, language, role):
        self.audio.append((index, language, role))


def media(file_id, lang, role="primary", episode_id=None, languages=None):
    streams = [EmbeddedAudioStream(lang=language) for language in languages] if languages is not None else None
    return MediaFile(file_id, episode_id, "h264", "aac", lang, role, True, 1080, None, streams)


def source_probe(has_video=True, codec="hevc", language="cze"):
    audio = AudioStream(1, 0, "aac", 2, language, "")
    return ProbeResult(1.0, has_video, codec, "yuv420p", "Main", 1920, 1080, [audio], [])


class PipelineTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.temp_path = Path(self.temp.name)
        self.outputs = self.temp_path / "outputs"
        self.callbacks = Recorder()
        self.client = MagicMock()
        self.client.upload_complete.return_value = "new-media"
        self.client.title_detail.return_value = TitleDetail("movie", "Movie", "movie", [], [], [])
        self.settings = BatchSettings("en, cs", str(self.outputs), library_kind="movies", lookahead=1)

    def job(self, index=0, **kwargs):
        source = self.temp_path / f"source-{index}.mkv"
        source.write_bytes(b"source")
        return Job(index, str(source), source.name, f"Movie {index}", **kwargs)

    def pipeline(self, jobs):
        result = Pipeline(self.client, jobs, self.settings, self.callbacks)
        self.addCleanup(result._cleanup_all_temp)
        return result

    def ready(self, pipeline, job, language="cs", languages=None):
        work_dir = self.outputs / str(job.index)
        work_dir.mkdir(parents=True, exist_ok=True)
        output = work_dir / "output.mp4"
        output.write_bytes(b"encoded")
        pipeline._track_temp(str(work_dir))
        return _Ready(job, str(output), str(work_dir), audio_language=language,
                      audio_languages=list(languages or []), multi_audio=len(languages or []) > 1)

    @staticmethod
    def write_transcode(cmd, **kwargs):
        Path(cmd[-1]).write_bytes(b"encoded")

    @staticmethod
    def upload_bytes(client, path, on_session, **kwargs):
        session_id = Path(path).parent.name
        on_session(session_id)
        return session_id

    def run_bounded(self, pipeline):
        thread = threading.Thread(target=pipeline.run, daemon=True)
        thread.start()
        thread.join(timeout=3)
        self.assertFalse(thread.is_alive(), "pipeline did not finish")
        self.assertEqual(len(self.callbacks.results), 1)
        self.assertEqual(list(self.outputs.iterdir()), [])

    def test_plan_error_and_probe_timeout_fail_only_their_items(self):
        jobs = [self.job(i) for i in range(3)]
        pipeline = self.pipeline(jobs)
        probe_results = [source_probe(has_video=False), subprocess.TimeoutExpired("ffprobe", 1), source_probe()]
        with patch("couchpush.core.pipeline.probe", side_effect=probe_results), \
                patch("couchpush.core.pipeline.extract_text_subs", return_value=[]), \
                patch("couchpush.core.pipeline.run_transcode", side_effect=self.write_transcode), \
                patch("couchpush.core.pipeline.upload_file", side_effect=self.upload_bytes), \
                patch.object(pipeline, "_check_disk"):
            self.run_bounded(pipeline)
        self.assertEqual(pipeline.summary["failed"], [0, 1])
        self.assertEqual(pipeline.summary["done"], [2])
        self.client.set_media_audio.assert_called_once_with("new-media", "cs", "primary")

    def test_request_failure_during_completion_does_not_strand_producer(self):
        jobs = [self.job(i) for i in range(5)]
        pipeline = self.pipeline(jobs)
        self.client.upload_complete.side_effect = [requests.ConnectionError("connection lost")] + ["media"] * 4
        with patch.object(pipeline, "_transcode_one", side_effect=lambda job: self.ready(pipeline, job)), \
                patch("couchpush.core.pipeline.upload_file", side_effect=self.upload_bytes):
            self.run_bounded(pipeline)
        self.assertEqual(pipeline.summary["failed"], [0])
        self.assertEqual(pipeline.summary["done"], [1, 2, 3, 4])
        self.assertIsNone(pipeline._active_upload_id)
        self.client.upload_abort.assert_called_once_with("0")

    def test_cancel_with_full_lookahead_buffer_finishes_and_removes_temps(self):
        pipeline = self.pipeline([self.job(i) for i in range(6)])
        third_encoded = threading.Event()

        def encode(job):
            item = self.ready(pipeline, job)
            if job.index == 2:
                third_encoded.set()
            return item

        def cancel_upload(item):
            self.assertTrue(third_encoded.wait(1), "lookahead buffer did not fill")
            raise UploadCancelled()

        with patch.object(pipeline, "_transcode_one", side_effect=encode), \
                patch.object(pipeline, "_upload_one", side_effect=cancel_upload):
            self.run_bounded(pipeline)
        self.assertTrue(pipeline.summary["cancelled"])
        self.assertEqual(pipeline.summary["failed"], [])

    def test_nvenc_error_retries_cpu_and_carries_actual_language(self):
        job = self.job()
        pipeline = self.pipeline([job])
        commands = []

        def transcode(cmd, **kwargs):
            commands.append(cmd)
            if len(commands) == 1:
                raise TranscodeError("InitializeEncoder failed: Invalid Level")
            self.write_transcode(cmd)

        with patch("couchpush.core.pipeline.probe", return_value=source_probe()), \
                patch("couchpush.core.pipeline.extract_text_subs", return_value=[]), \
                patch("couchpush.core.pipeline.run_transcode", side_effect=transcode), \
                patch.object(pipeline, "_check_disk"):
            ready = pipeline._transcode_one(job)
        self.assertEqual(ready.audio_language, "cs")
        self.assertIn("h264_nvenc", commands[0])
        self.assertIn("libx264", commands[1])
        self.assertTrue(any("retrying with CPU" in entry for entry in self.callbacks.logs))

    def test_failed_cpu_retry_reports_both_errors_and_cleans_output(self):
        pipeline = self.pipeline([self.job()])
        with patch("couchpush.core.pipeline.probe", return_value=source_probe()), \
                patch("couchpush.core.pipeline.extract_text_subs", return_value=[]), \
                patch("couchpush.core.pipeline.run_transcode", side_effect=[TranscodeError("gpu error"), TranscodeError("cpu error")]), \
                patch.object(pipeline, "_check_disk"):
            with self.assertRaisesRegex(TranscodeError, "(?s)gpu error.*cpu error"):
                pipeline._transcode_one(pipeline.jobs[0])
        self.assertEqual(list(self.outputs.iterdir()), [])

    def test_remux_and_cancelled_nvenc_never_retry_cpu(self):
        for codec, cancel in [("h264", False), ("hevc", True)]:
            with self.subTest(codec=codec, cancel=cancel):
                pipeline = self.pipeline([self.job()])

                def fail(cmd, **kwargs):
                    if cancel:
                        pipeline._cancel.set()
                    raise TranscodeError("encoder failed")

                with patch("couchpush.core.pipeline.probe", return_value=source_probe(codec=codec)), \
                        patch("couchpush.core.pipeline.extract_text_subs", return_value=[]), \
                        patch("couchpush.core.pipeline.run_transcode", side_effect=fail) as transcode, \
                        patch.object(pipeline, "_check_disk"):
                    with self.assertRaises(TranscodeError):
                        pipeline._transcode_one(pipeline.jobs[0])
                self.assertEqual(transcode.call_count, 1)

    def test_movie_replace_preserves_other_languages_and_audio_role(self):
        job = self.job(action=ACTION_REPLACE, title_id="movie")
        pipeline = self.pipeline([job])
        self.client.title_detail.return_value.media_files = [
            media("english", "en"), media("czech", "cze", "audio_alt"), media("legacy", "")]
        with patch("couchpush.core.pipeline.upload_file", side_effect=self.upload_bytes):
            pipeline._upload_one(self.ready(pipeline, job))
        self.client.delete_media_file.assert_called_once_with("czech")
        self.client.set_media_audio.assert_called_once_with("new-media", "cs", "audio_alt")
        methods = [call[0] for call in self.client.mock_calls]
        self.assertLess(methods.index("title_detail"), methods.index("upload_complete"))
        self.assertLess(methods.index("set_media_audio"), methods.index("delete_media_file"))

    def test_series_replace_resolves_fallback_language_and_episode_scope(self):
        self.settings.library_kind = "series"
        self.settings.title_id = "show"
        job = self.job(action=ACTION_REPLACE, episode_id="episode", replace_ids=["english"])
        pipeline = self.pipeline([job])
        self.client.title_detail.return_value.media_files = [
            media("english", "en", episode_id="episode"),
            media("czech", "cs", "audio_alt", "episode"),
            media("other-episode", "cs", episode_id="another")]
        with patch("couchpush.core.pipeline.upload_file", side_effect=self.upload_bytes):
            pipeline._upload_one(self.ready(pipeline, job))
        self.client.delete_media_file.assert_called_once_with("czech")
        self.client.set_media_audio.assert_called_once_with("new-media", "cs", "audio_alt")

    def test_known_language_keeps_legacy_untagged_primary(self):
        job = self.job(action=ACTION_REPLACE, title_id="movie")
        pipeline = self.pipeline([job])
        self.client.title_detail.return_value.media_files = [media("legacy", "")]
        with patch("couchpush.core.pipeline.upload_file", side_effect=self.upload_bytes):
            pipeline._upload_one(self.ready(pipeline, job, "en"))
        self.client.delete_media_file.assert_not_called()
        self.client.set_media_audio.assert_called_once_with("new-media", "en", "audio_alt")

    def test_tag_failure_leaves_all_existing_files_intact(self):
        job = self.job(action=ACTION_REPLACE, title_id="movie")
        pipeline = self.pipeline([job])
        self.client.title_detail.return_value.media_files = [media("czech", "cs")]
        self.client.set_media_audio.side_effect = requests.ConnectionError("tag failed")
        with patch.object(pipeline, "_transcode_one", side_effect=lambda job: self.ready(pipeline, job)), \
                patch("couchpush.core.pipeline.upload_file", side_effect=self.upload_bytes):
            self.run_bounded(pipeline)
        self.assertEqual(pipeline.summary["failed"], [0])
        self.client.delete_media_file.assert_not_called()
        self.client.upload_abort.assert_not_called()

    def test_explicit_new_movie_uses_created_title_and_retains_it_for_retry(self):
        job = self.job(create_new_title=True, new_title_name="Iron Man", new_title_year=2008)
        pipeline = self.pipeline([job])
        self.client.create_movie.return_value = "draft-movie"
        with patch("couchpush.core.pipeline.upload_file", side_effect=self.upload_bytes):
            pipeline._upload_one(self.ready(pipeline, job))
        self.client.create_movie.assert_called_once_with("Iron Man", 2008)
        self.client.upload_complete.assert_called_once_with("0", "movies", title_id="draft-movie")
        self.assertEqual(job.title_id, "draft-movie")
        methods = [call[0] for call in self.client.mock_calls]
        self.assertLess(methods.index("create_movie"), methods.index("upload_complete"))

    def test_rejected_movie_target_fails_before_probe_encode_or_upload(self):
        job = self.job(create_new_title=True, new_title_name="Harry Potter 1 a Kámen mudrců", new_title_year=2001)
        pipeline = self.pipeline([job])
        self.client.create_movie.side_effect = CouchverseError(400, "bad_request", "invalid request body")
        with patch("couchpush.core.pipeline.probe") as probe_call, \
                patch("couchpush.core.pipeline.run_transcode") as encode, \
                patch("couchpush.core.pipeline.upload_file") as upload:
            self.run_bounded(pipeline)
        self.assertEqual(pipeline.summary["failed"], [0])
        probe_call.assert_not_called()
        encode.assert_not_called()
        upload.assert_not_called()
        self.client.upload_complete.assert_not_called()

    def test_new_movie_target_is_created_once_before_encoding_and_reused_on_retry(self):
        job = self.job(create_new_title=True, new_title_name="Harry Potter 1 a Kámen mudrců", new_title_year=2001)
        pipeline = self.pipeline([job])
        self.client.create_movie.return_value = "potter-one"

        def encode(cmd, **kwargs):
            self.assertEqual(job.title_id, "potter-one")
            self.write_transcode(cmd)

        with patch("couchpush.core.pipeline.probe", return_value=source_probe()), \
                patch("couchpush.core.pipeline.extract_text_subs", return_value=[]), \
                patch("couchpush.core.pipeline.run_transcode", side_effect=encode), \
                patch("couchpush.core.pipeline.upload_file", side_effect=self.upload_bytes), \
                patch.object(pipeline, "_check_disk"):
            ready = pipeline._transcode_one(job)
            pipeline._upload_one(ready)
            pipeline._ensure_movie_target(job)
        self.client.create_movie.assert_called_once_with("Harry Potter 1 a Kámen mudrců", 2001)
        self.client.upload_complete.assert_called_once_with(unittest.mock.ANY, "movies", title_id="potter-one")

    def test_cancel_does_not_block_on_abort_request(self):
        pipeline = self.pipeline([])
        pipeline._active_upload_id = "active"
        entered = threading.Event()
        release = threading.Event()

        def abort(upload_id):
            entered.set()
            release.wait(2)

        self.client.upload_abort.side_effect = abort
        caller = threading.Thread(target=pipeline.cancel, daemon=True)
        caller.start()
        caller.join(0.2)
        try:
            self.assertFalse(caller.is_alive())
            self.assertTrue(entered.wait(1))
            self.assertTrue(pipeline._cancelled())
        finally:
            release.set()

    def test_multiple_audio_uses_actual_languages_and_reports_missing_requests(self):
        self.settings.audio_mode = "multiple"
        self.settings.audio_lang = "en, cs, de"
        job = self.job()
        pipeline = self.pipeline([job])
        source = source_probe(language="eng")
        source.audio.append(AudioStream(2, 1, "aac", 2, "cze", ""))
        with patch("couchpush.core.pipeline.probe", return_value=source), \
                patch("couchpush.core.pipeline.extract_text_subs", return_value=[]), \
                patch("couchpush.core.pipeline.run_transcode", side_effect=self.write_transcode), \
                patch.object(pipeline, "_check_disk"):
            ready = pipeline._transcode_one(job)
        self.assertTrue(ready.multi_audio)
        self.assertEqual(ready.audio_languages, ["en", "cs"])
        self.assertEqual(ready.audio_language, "en")
        self.assertIn((0, "en, cs", "primary"), self.callbacks.audio)
        self.assertTrue(any("requested audio not found: de" in line for line in self.callbacks.logs))
        self.assertTrue(any("HLS remux" in line for line in self.callbacks.logs))

    def test_combined_file_tags_first_language_and_demotes_unknown_old_primary(self):
        job = self.job(action=ACTION_REPLACE, title_id="movie")
        pipeline = self.pipeline([job])
        self.client.title_detail.return_value.media_files = [media("old", "en")]
        with patch("couchpush.core.pipeline.upload_file", side_effect=self.upload_bytes):
            pipeline._upload_one(self.ready(pipeline, job, "en", ["en", "cs"]))
        tag_calls = [call.args for call in self.client.set_media_audio.call_args_list]
        self.assertEqual(tag_calls, [("new-media", "en", "primary"), ("old", "en", "audio_alt")])
        self.assertIn((0, "en, cs", "primary"), self.callbacks.audio)
        self.client.delete_media_file.assert_not_called()

    def test_combined_replace_deletes_only_proven_complete_language_coverage(self):
        job = self.job(action=ACTION_REPLACE, title_id="movie")
        pipeline = self.pipeline([job])
        self.client.title_detail.return_value.media_files = [
            media("english", "eng", languages=["eng"]),
            media("czech", "cs", "audio_alt", languages=["cze"]),
            media("more-languages", "en", languages=["en", "cs", "de"]),
            media("unknown-inventory", "en"),
            media("unknown-track", "en", languages=["en", "und"]),
            media("german", "de", "audio_alt", languages=["de"]),
            media("legacy", "")]
        with patch("couchpush.core.pipeline.upload_file", side_effect=self.upload_bytes):
            pipeline._upload_one(self.ready(pipeline, job, "en", ["en", "cs"]))
        deleted = [call.args[0] for call in self.client.delete_media_file.call_args_list]
        self.assertEqual(deleted, ["english", "czech"])
        self.client.set_media_audio.assert_any_call("english", "eng", "audio_alt")
        self.client.set_media_audio.assert_any_call("more-languages", "en", "audio_alt")
        self.client.set_media_audio.assert_any_call("legacy", "", "audio_alt")

    def test_combined_replace_can_replace_known_combined_file_with_same_languages(self):
        job = self.job(action=ACTION_REPLACE, title_id="movie")
        pipeline = self.pipeline([job])
        self.client.title_detail.return_value.media_files = [media("combined", "en", languages=["en", "cs"])]
        with patch("couchpush.core.pipeline.upload_file", side_effect=self.upload_bytes):
            pipeline._upload_one(self.ready(pipeline, job, "en", ["en", "cs"]))
        self.client.delete_media_file.assert_called_once_with("combined")

    def test_single_replace_preserves_known_combined_file_extra_language(self):
        job = self.job(action=ACTION_REPLACE, title_id="movie")
        pipeline = self.pipeline([job])
        self.client.title_detail.return_value.media_files = [media("combined", "en", languages=["en", "cs"])]
        with patch("couchpush.core.pipeline.upload_file", side_effect=self.upload_bytes):
            pipeline._upload_one(self.ready(pipeline, job, "en"))
        self.client.delete_media_file.assert_not_called()
        self.client.set_media_audio.assert_called_once_with("new-media", "en", "primary")

    def test_single_replace_preserves_embedded_inventory_with_unknown_track(self):
        job = self.job(action=ACTION_REPLACE, title_id="movie")
        pipeline = self.pipeline([job])
        self.client.title_detail.return_value.media_files = [media("combined", "en", languages=["en", "und"])]
        with patch("couchpush.core.pipeline.upload_file", side_effect=self.upload_bytes):
            pipeline._upload_one(self.ready(pipeline, job, "en"))
        self.client.delete_media_file.assert_not_called()

    def test_failed_combined_primary_promotion_keeps_all_old_files(self):
        job = self.job(action=ACTION_REPLACE, title_id="movie")
        pipeline = self.pipeline([job])
        self.client.title_detail.return_value.media_files = [media("old", "en", languages=["en"])]

        def tag(media_id, language, role):
            if media_id == "old":
                raise requests.ConnectionError("cannot update primary")

        self.client.set_media_audio.side_effect = tag
        with patch.object(pipeline, "_transcode_one",
                          side_effect=lambda job: self.ready(pipeline, job, "en", ["en", "cs"])), \
                patch("couchpush.core.pipeline.upload_file", side_effect=self.upload_bytes):
            self.run_bounded(pipeline)
        self.assertEqual(pipeline.summary["failed"], [0])
        self.assertIn("All existing files were kept", self.callbacks.failures[0][1])
        self.client.delete_media_file.assert_not_called()
        self.client.upload_abort.assert_not_called()

    def test_combined_series_promotion_is_scoped_and_includes_cached_primary(self):
        self.settings.library_kind = "series"
        self.settings.title_id = "show"
        job = self.job(action=ACTION_REPLACE, episode_id="episode")
        pipeline = self.pipeline([job])
        cached = media("cached-primary", "en", episode_id="episode", languages=["en"])
        cached.source_deleted_at = "2026-10-01T00:00:00Z"
        self.client.title_detail.return_value.media_files = [cached,
            media("other-episode", "en", episode_id="another", languages=["en"])]
        with patch("couchpush.core.pipeline.upload_file", side_effect=self.upload_bytes):
            pipeline._upload_one(self.ready(pipeline, job, "en", ["en", "cs"]))
        calls = [call.args for call in self.client.set_media_audio.call_args_list]
        self.assertEqual(calls, [("new-media", "en", "primary"), ("cached-primary", "en", "audio_alt")])
        self.client.delete_media_file.assert_not_called()


class MediaInventoryTests(unittest.TestCase):
    def test_optional_embedded_audio_inventory_parses_without_breaking_older_servers(self):
        missing = MediaFile.from_json({"id": "old", "audioLang": "en"})
        self.assertIsNone(missing.audio_streams)
        known = MediaFile.from_json({"id": "combined", "audioLang": "en", "audioStreams": [
            {"index": 0, "codec": "aac", "lang": "en", "default": True},
            {"index": 1, "codec": "aac", "lang": "cs"}]})
        self.assertEqual([stream.lang for stream in known.audio_streams], ["en", "cs"])
        self.assertTrue(known.audio_streams[0].default)

    def test_malformed_inventory_is_unknown(self):
        media_file = MediaFile.from_json({"id": "unsafe", "audioStreams": [{"lang": "en"}, None]})
        self.assertIsNone(media_file.audio_streams)

    def test_title_detail_applies_audio_streams_by_file_map(self):
        detail = TitleDetail.from_json({
            "title": {"id": "movie", "kind": "movie"},
            "mediaFiles": [{"id": "combined", "audioLang": "en"}, {"id": "legacy"}],
            "audioStreamsByFile": {"combined": [{"lang": "en"}, {"lang": "cs"}]},
        })
        self.assertEqual([a.lang for a in detail.media_files[0].audio_streams], ["en", "cs"])
        self.assertIsNone(detail.media_files[1].audio_streams)


class UploadTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.path = Path(self.temp.name) / "output.mp4"
        self.path.write_bytes(b"test")
        self.client = MagicMock()
        self.client.upload_create.return_value = "session"

    def test_cancel_interrupts_retry_backoff(self):
        self.client.upload_append.side_effect = requests.ConnectionError("offline")
        cancelled = threading.Event()
        self.client.upload_append.side_effect = lambda *args: self.cancel_and_fail(cancelled)
        with self.assertRaises(UploadCancelled):
            upload_file(self.client, str(self.path), should_cancel=cancelled.is_set)
        self.client.upload_status.assert_not_called()

    @staticmethod
    def cancel_and_fail(cancelled):
        cancelled.set()
        raise requests.ConnectionError("offline")

    def test_invalid_or_stalled_offsets_fail_instead_of_looping(self):
        for offset in (-1, 0, 5):
            with self.subTest(offset=offset):
                self.client.upload_append.return_value = offset
                with self.assertRaises(UploadError):
                    upload_file(self.client, str(self.path))

    def test_network_retry_resumes_from_confirmed_offset(self):
        self.client.upload_append.side_effect = [requests.ConnectionError("connection lost"), 4]
        self.client.upload_status.return_value = {"receivedBytes": 2}
        with patch("couchpush.core.uploader._retry_wait"):
            upload_id = upload_file(self.client, str(self.path))
        self.assertEqual(upload_id, "session")
        self.assertEqual(self.client.upload_append.call_args_list[1].args, ("session", 2, b"st"))

    def test_repeated_offset_conflicts_are_bounded(self):
        self.client.upload_append.side_effect = OffsetConflict(0)
        with patch("couchpush.core.uploader._retry_wait"):
            with self.assertRaisesRegex(UploadError, "repeatedly rejected"):
                upload_file(self.client, str(self.path), max_retries=2)
        self.assertEqual(self.client.upload_append.call_count, 3)


if __name__ == "__main__":
    unittest.main()
