"""Queue behavior checks with Qt offscreen and no live server or saved credentials."""

import os
os.environ.setdefault("QT_QPA_PLATFORM", "offscreen")

import unittest
from unittest.mock import Mock, patch

from PySide6.QtCore import Qt
from PySide6.QtWidgets import QApplication, QDialog

from couchpush.api.models import Title, TitleDetail
from couchpush.config import Config
from couchpush.core.matcher import build_movie_row, build_movie_rows
from couchpush.core.parser import MovieFile
from couchpush.ui.main_window import COL_ACTION, COL_AUDIO, COL_SEL, MainWindow
from couchpush.ui.movie_picker import MoviePicker
from couchpush.ui.theme import apply_theme
from couchpush.workers.pipeline_worker import PipelineEmitter


def movie_title(id="iron", name="Iron Man", year=2008, has_media=False):
    return Title(id, name, year, "movie", 0, 0, "ready", 100 if has_media else 0, 1080 if has_media else 0)


class FakeRunner:
    def __init__(self, client, jobs, settings):
        self.jobs, self.settings = jobs, settings
        self.emitter = PipelineEmitter()
        self.start = Mock()
        self.cancel = Mock()


class QueueTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.app = QApplication.instance() or QApplication([])
        apply_theme(cls.app)

    def setUp(self):
        self.patches = [patch("couchpush.ui.main_window.Config.load", return_value=Config()),
                        patch("couchpush.ui.main_window.load_password", return_value=""),
                        patch("couchpush.ui.main_window.load_session_cookie", return_value=None),
                        patch.object(Config, "save")]
        for item in self.patches:
            item.start()
        self.window = MainWindow()
        self.window._warn = Mock()
        self.window.client = Mock()
        self.window.movies = [movie_title()]
        self.window._lang_timer.stop()
        self.window._async = Mock()

    def tearDown(self):
        self.window._running = False
        self.window.close()
        self.window.deleteLater()
        self.app.processEvents()
        for item in reversed(self.patches):
            item.stop()

    def queue(self, files):
        self.window.movie_files = files
        self.window._rebuild_rows()

    def test_corrected_new_target_is_explicit_in_job(self):
        source = MovieFile("C:/source/odd.mkv", "odd.mkv", "Wrong", None)
        row = build_movie_row(source, new_name="Iron Man 2", new_year=2010)
        self.window.movie_rows = [row]
        self.window._populate_table()
        job = self.window._collect_jobs()[0]
        self.assertEqual(job.output_base, "Iron Man 2 (2010)")
        self.assertTrue(job.create_new_title)
        self.assertEqual((job.new_title_name, job.new_title_year), ("Iron Man 2", 2010))

    def test_hidden_selection_remains_in_batch_and_bulk_only_changes_visible(self):
        self.queue([MovieFile("C:/Iron.mkv", "Iron Man.mkv", "Iron Man", 2008),
                    MovieFile("C:/Thor.mkv", "Thor.mkv", "Thor", 2011)])
        self.window.search_edit.setText("Thor")
        self.window._check_all(False)
        jobs = self.window._collect_jobs()
        self.assertEqual([job.index for job in jobs], [0])
        self.assertIn("1 selected", self.window.count_label.text())

    def test_duplicate_manual_targets_cannot_start(self):
        self.queue([MovieFile("C:/one.mkv", "one.mkv", "Iron Man", 2008),
                    MovieFile("C:/two.mkv", "two.mkv", "Iron Man", 2008)])
        self.window.table.item(1, COL_SEL).setCheckState(Qt.Checked)
        with patch("couchpush.ui.main_window.PipelineRunner") as runner:
            self.window._on_start()
            runner.assert_not_called()
        self.assertIn("both target", self.window._warn.call_args.args[0])

    def test_scanning_again_replaces_old_queue(self):
        self.queue([MovieFile("C:/one.mkv", "one.mkv", "First", 2000)])
        self.queue([MovieFile("C:/two.mkv", "two.mkv", "Second", 2001)])
        self.assertEqual(self.window.table.rowCount(), 1)
        self.assertEqual(self.window.movie_rows[0].target_title, "Second")

    def test_language_edit_preserves_corrected_target_and_selection(self):
        source = MovieFile("C:/one.mkv", "one.mkv", "Wrong", 2000)
        row = build_movie_row(source, new_name="Correct", new_year=2001)
        self.window.movie_rows = [row]
        self.window._populate_table()
        self.window.table.item(0, COL_SEL).setCheckState(Qt.Unchecked)
        self.window.lang_combo.setCurrentText("en, cs")
        self.window._on_language_changed()
        self.assertEqual(self.window.movie_rows[0].target_title, "Correct")
        self.assertFalse(self.window._checked(0))
        self.assertIn("en, cs", self.window.table.item(0, COL_AUDIO).text())

    def test_running_locks_mode_and_queue_mutation(self):
        self.queue([MovieFile("C:/one.mkv", "one.mkv", "First", 2000)])
        with patch("couchpush.ui.main_window.PipelineRunner", FakeRunner), patch("couchpush.ui.main_window.shutil.which", return_value="ffmpeg"):
            self.window._on_start()
        self.assertTrue(self.window._running)
        self.assertFalse(self.window.mode_combo.isEnabled())
        self.assertFalse(self.window.clear_btn.isEnabled())
        self.assertFalse(self.window.choose_target_btn.isEnabled())
        self.assertFalse(self.window.table.isEnabled())
        self.assertFalse(self.window.resolution_combo.isEnabled())
        self.assertFalse(self.window.audio_mode_combo.isEnabled())

    def test_multiple_mode_reaches_batch_and_persists(self):
        self.queue([MovieFile("C:/one.mkv", "one.mkv", "First", 2000)])
        self.window.lang_combo.setCurrentText("en, cs")
        self.assertEqual(self.window.audio_mode_combo.currentData(), "multiple")
        with patch("couchpush.ui.main_window.PipelineRunner", FakeRunner), patch("couchpush.ui.main_window.shutil.which", return_value="ffmpeg"):
            self.window._on_start()
        self.assertEqual(self.window.runner.settings.audio_mode, "multiple")
        self.assertEqual(self.window.runner.settings.audio_lang, "en, cs")
        self.assertEqual(self.window.cfg.audio_mode, "multiple")
        self.window._on_item_audio(0, "en, cs", "primary")
        self.assertEqual(self.window.table.item(0, COL_AUDIO).text(), "en, cs (switchable)")

    def test_mode_change_preserves_target_selection_and_clarifies_action(self):
        self.window.movies = [movie_title(has_media=True)]
        self.queue([MovieFile("C:/one.mkv", "one.mkv", "Iron Man", 2008)])
        self.window.lang_combo.setCurrentText("en, cs")
        self.window._on_language_changed()
        self.assertFalse(self.window._checked(0))
        self.assertEqual(self.window.table.item(0, COL_ACTION).text(), "Upload languages")
        self.window.audio_mode_combo.setCurrentIndex(self.window.audio_mode_combo.findData("single"))
        self.assertFalse(self.window._checked(0))
        self.assertEqual(self.window.movie_rows[0].title_id, "iron")
        self.assertEqual(self.window.table.item(0, COL_ACTION).text(), "Replace language")
        self.assertIn("first available", self.window.audio_hint.text())
        with patch("couchpush.ui.main_window.PipelineRunner", FakeRunner), patch("couchpush.ui.main_window.shutil.which", return_value="ffmpeg"):
            self.window.table.item(0, COL_SEL).setCheckState(Qt.Checked)
            self.window._on_start()
        self.assertEqual(self.window.runner.settings.audio_mode, "single")

    def test_multiple_mode_requires_languages_but_single_allows_first_track(self):
        self.queue([MovieFile("C:/one.mkv", "one.mkv", "First", 2000)])
        self.window.lang_combo.setCurrentText("")
        self.window._on_language_changed()
        self.assertFalse(self.window.start_btn.isEnabled())
        with patch("couchpush.ui.main_window.PipelineRunner") as runner:
            self.window._on_start()
            runner.assert_not_called()
        self.assertIn("List the audio languages", self.window._warn.call_args.args[0])
        self.window.audio_mode_combo.setCurrentIndex(self.window.audio_mode_combo.findData("single"))
        self.assertTrue(self.window.start_btn.isEnabled())

    def test_chosen_resolution_reaches_batch_settings(self):
        self.queue([MovieFile("C:/one.mkv", "one.mkv", "First", 2000)])
        self.assertEqual(self.window.resolution_combo.currentData(), 1080)
        self.window.resolution_combo.setCurrentIndex(self.window.resolution_combo.findData(720))
        with patch("couchpush.ui.main_window.PipelineRunner", FakeRunner), patch("couchpush.ui.main_window.shutil.which", return_value="ffmpeg"):
            self.window._on_start()
        self.assertEqual(self.window.runner.settings.max_height, 720)

    def test_retry_only_selects_failed_rows_and_retains_created_id(self):
        self.queue([MovieFile("C:/one.mkv", "one.mkv", "First", 2000),
                    MovieFile("C:/two.mkv", "two.mkv", "Second", 2001)])
        self.window._active_jobs = {j.index: j for j in self.window._collect_jobs()}
        self.window._active_jobs[1].title_id = "created-title"
        self.window._on_item_done(0, "media-one")
        self.window._on_item_failed(1, "network failure")
        self.window._on_start = Mock()
        self.window._retry_failed()
        self.assertFalse(self.window._checked(0))
        self.assertTrue(self.window._checked(1))
        self.assertEqual(self.window._collect_jobs()[0].title_id, "created-title")

    def test_cancel_retains_target_created_during_preparation(self):
        self.queue([MovieFile("C:/one.mkv", "one.mkv", "First", 2000)])
        self.window._active_jobs = {j.index: j for j in self.window._collect_jobs()}
        self.window._active_jobs[0].title_id = "prepared-draft"
        self.window._on_finished({"done": [], "failed": [], "cancelled": True})
        self.assertEqual(self.window._collect_jobs()[0].title_id, "prepared-draft")

    def test_switching_mode_discards_stale_scan_without_staying_busy(self):
        self.window._loading = True
        self.window.mode_combo.setCurrentIndex(self.window.mode_combo.findData("series"))
        self.assertFalse(self.window._loading)
        self.assertTrue(self.window.scan_btn.isEnabled())

    def test_background_series_load_preserves_movie_selection(self):
        self.queue([MovieFile("C:/one.mkv", "one.mkv", "First", 2000)])
        self.window.table.item(0, COL_SEL).setCheckState(Qt.Unchecked)
        self.window.series_combo.blockSignals(True)
        self.window.series_combo.addItem("A Series", "series-id")
        self.window.series_combo.blockSignals(False)
        self.window._on_detail_loaded("series-id", TitleDetail("series-id", "A Series", "series", [], [], []))
        self.assertFalse(self.window._checked(0))

    def test_picker_search_and_new_metadata(self):
        source = MovieFile("C:/odd.mkv", "odd.mkv", "Iron Man", 2008)
        picker = MoviePicker(build_movie_row(source), [movie_title(), movie_title("thor", "Thor", 2011)], self.window)
        picker.search.setText("2011")
        self.assertEqual(picker.list.count(), 1)
        picker.tabs.setCurrentIndex(1)
        picker.name.setText("Iron Man 2")
        picker.year.setValue(2010)
        picker.accept()
        self.assertEqual(picker.result(), QDialog.Accepted)
        self.assertEqual(picker.result_row.output_base, "Iron Man 2 (2010)")
        picker.deleteLater()


if __name__ == "__main__":
    unittest.main()
