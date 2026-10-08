"""Desktop upload queue with explicit catalog targets and live batch status."""

from __future__ import annotations

import os
import shutil

from PySide6.QtCore import Qt, QTimer
from PySide6.QtGui import QColor
from PySide6.QtWidgets import (
    QAbstractItemView, QAbstractSpinBox, QCheckBox, QComboBox, QCompleter, QDialog, QFileDialog, QGridLayout, QGroupBox,
    QHBoxLayout, QHeaderView, QLabel, QLineEdit, QMainWindow, QMessageBox,
    QPlainTextEdit, QProgressBar, QPushButton, QScrollArea, QSpinBox, QTableWidget,
    QSplitter, QTabWidget, QTableWidgetItem, QVBoxLayout, QWidget,
)

from ..api.client import CouchverseClient
from ..api.models import Title, TitleDetail
from ..config import (Config, clear_password, clear_session_cookie, load_password,
                      load_session_cookie, save_password, save_session_cookie)
from ..core.matcher import (ACTION_REPLACE, ACTION_UPLOAD, ROLE_ALT, MatchRow, MovieRow,
                            build_match_rows, build_movie_rows, movie_selection_errors, movie_target_key)
from ..core.parser import (VIDEO_EXTS, MovieFile, ParsedFile, RegexError, compile_regex,
                           parse_movie, parse_name, scan_folder, scan_movies)
from ..core.pipeline import BatchSettings, Job
from ..media.plan import parse_language_preferences, preferred_language
from ..workers.async_call import AsyncCall
from ..workers.pipeline_worker import PipelineRunner
from .movie_picker import MoviePicker

COMMON_LANGS = ["cs", "en", "sk", "de", "es", "fr", "it", "pl", "ru", "pt", "nl", "hu", "ja", "ko"]
QUALITY_LABELS = [("max", "Max quality (slow, largest)"),
                  ("high", "High - faster + smaller (default)"),
                  ("small", "Small - good for cartoons"),
                  ("tiny", "Smallest")]
ENCODER_LABELS = [("nvenc", "NVIDIA NVENC (GPU)"), ("cpu", "libx264 (CPU)")]
LIBRARY_LABELS = [("series", "TV series"), ("movies", "Movies")]
RESOLUTION_LABELS = [(1080, "1080p (Full HD)"), (720, "720p (HD)"),
                     (480, "480p (SD)"), (2160, "2160p (4K)"), (0, "Original size")]
AUDIO_MODE_LABELS = [("multiple", "All listed languages"), ("single", "One preferred language")]

COL_SEL, COL_FILE, COL_SE, COL_EP, COL_AUDIO, COL_SUBS, COL_ACTION, COL_STATUS, COL_TIME = range(9)


class MainWindow(QMainWindow):
    def __init__(self) -> None:
        super().__init__()
        self.setWindowTitle("CouchPush - CouchVerse uploader")
        self.resize(1280, 900)
        self.setMinimumSize(1000, 800)
        self.setAcceptDrops(True)
        self.cfg = Config.load()
        self.client: CouchverseClient | None = None
        self.mode = "movies"
        self.series: list[Title] = []
        self.detail: TitleDetail | None = None
        self.parsed: list[ParsedFile] = []
        self.rows: list[MatchRow] = []
        self.movies: list[Title] | None = None      # None = not loaded yet
        self.movie_files: list[MovieFile] = []
        self.movie_rows: list[MovieRow] = []
        self.runner: PipelineRunner | None = None
        self._inflight: set[AsyncCall] = set()
        self._detail_cache: dict[str, TitleDetail] = {}
        self._timings: dict[int, dict] = {}
        self._running = False
        self._loading = False
        self._scan_generation = 0
        self._connection_generation = 0
        self._failed_rows: set[int] = set()
        self._movie_overrides: dict[str, MovieRow] = {}
        self._explicit_paths: list[str] = []
        self._lang_timer = QTimer(self)
        self._lang_timer.setSingleShot(True)
        self._lang_timer.setInterval(250)
        self._lang_timer.timeout.connect(self._on_language_changed)

        root = QWidget()
        outer = QVBoxLayout(root)
        outer.setContentsMargins(24, 20, 24, 16)
        outer.setSpacing(14)
        self.setCentralWidget(root)
        header = QHBoxLayout()
        mark = QLabel("C")
        mark.setObjectName("brandMark")
        mark.setAlignment(Qt.AlignCenter)
        mark.setFixedSize(48, 48)
        header.addWidget(mark)
        brand = QVBoxLayout()
        name = QLabel("CouchPush")
        name.setObjectName("brand")
        brand.addWidget(name)
        subtitle = QLabel("Prepare on your PC. Press play on your couch.")
        subtitle.setObjectName("muted")
        brand.addWidget(subtitle)
        header.addLayout(brand)
        header.addStretch()
        self.conn_status = QLabel("Offline")
        self.conn_status.setObjectName("pill")
        self.conn_status.setFixedHeight(32)
        header.addWidget(self.conn_status)
        settings_btn = QPushButton("Server settings")
        settings_btn.clicked.connect(lambda: self.tabs.setCurrentIndex(1))
        header.addWidget(settings_btn)
        outer.addLayout(header)
        self.tabs = QTabWidget()
        outer.addWidget(self.tabs, 1)
        queue = QWidget()
        self.body = QVBoxLayout(queue)
        self.body.setContentsMargins(0, 6, 0, 0)
        self.body.setSpacing(10)
        self.tabs.addTab(queue, "Upload queue")
        self.settings_page = QWidget()
        settings_scroll = QScrollArea()
        settings_scroll.setWidgetResizable(True)
        settings_scroll.setWidget(self.settings_page)
        self.settings_body = QVBoxLayout(self.settings_page)
        self.settings_body.setContentsMargins(0, 10, 0, 0)
        self.tabs.addTab(settings_scroll, "Connection and encoding")
        self._build_connection()
        self._build_settings()
        self.settings_body.addStretch()
        self._build_source()
        self._build_target()
        self._build_table()
        self._build_run()

        self._apply_config_to_widgets()
        self._on_mode_changed()
        self._update_summary()
        self._try_restore_session()

    # ---------- UI construction ----------

    def _build_connection(self) -> None:
        self.connection_box = box = QGroupBox("Your CouchVerse server")
        g = QGridLayout(box)
        self.url_edit = QLineEdit(placeholderText="http://192.168.0.69:8080")
        self.user_edit = QLineEdit(placeholderText="admin username")
        self.pass_edit = QLineEdit(); self.pass_edit.setEchoMode(QLineEdit.Password)
        self.remember_chk = QCheckBox("Stay signed in")
        self.login_btn = QPushButton("Log in"); self.login_btn.clicked.connect(self._on_login)
        g.addWidget(QLabel("Server URL"), 0, 0); g.addWidget(self.url_edit, 0, 1, 1, 3)
        g.addWidget(QLabel("Username"), 1, 0); g.addWidget(self.user_edit, 1, 1)
        g.addWidget(QLabel("Password"), 1, 2); g.addWidget(self.pass_edit, 1, 3)
        g.addWidget(self.remember_chk, 2, 1); g.addWidget(self.login_btn, 2, 3)
        hint = QLabel("Use a local HTTP address for fast LAN uploads, or your public HTTPS URL.")
        hint.setObjectName("muted")
        g.addWidget(hint, 3, 0, 1, 4)
        self.pass_edit.returnPressed.connect(self._on_login)
        self.settings_body.addWidget(box)

    def _build_settings(self) -> None:
        self.encoding_box = box = QGroupBox("Encoding and storage")
        g = QGridLayout(box)
        self.ffmpeg_edit = QLineEdit()
        self.ffprobe_edit = QLineEdit()
        self.temp_edit = QLineEdit()
        ff_btn = QPushButton("Browse"); ff_btn.clicked.connect(lambda: self._browse_file(self.ffmpeg_edit))
        fp_btn = QPushButton("Browse"); fp_btn.clicked.connect(lambda: self._browse_file(self.ffprobe_edit))
        tmp_btn = QPushButton("Browse"); tmp_btn.clicked.connect(lambda: self._browse_dir(self.temp_edit))
        self.encoder_combo = QComboBox()
        for key, label in ENCODER_LABELS:
            self.encoder_combo.addItem(label, key)
        self.quality_combo = QComboBox()
        for key, label in QUALITY_LABELS:
            self.quality_combo.addItem(label, key)
        self.keep_channels_chk = QCheckBox("Keep original audio channels (default: downmix to stereo)")
        self.lookahead_spin = QSpinBox(); self.lookahead_spin.setRange(1, 8)
        self.lookahead_spin.setButtonSymbols(QAbstractSpinBox.PlusMinus)
        g.addWidget(QLabel("ffmpeg"), 0, 0); g.addWidget(self.ffmpeg_edit, 0, 1); g.addWidget(ff_btn, 0, 2)
        g.addWidget(QLabel("ffprobe"), 1, 0); g.addWidget(self.ffprobe_edit, 1, 1); g.addWidget(fp_btn, 1, 2)
        g.addWidget(QLabel("Temp folder"), 2, 0); g.addWidget(self.temp_edit, 2, 1); g.addWidget(tmp_btn, 2, 2)
        g.addWidget(QLabel("Encoder"), 3, 0); g.addWidget(self.encoder_combo, 3, 1)
        g.addWidget(QLabel("Quality"), 4, 0); g.addWidget(self.quality_combo, 4, 1)
        g.addWidget(self.keep_channels_chk, 5, 1)
        g.addWidget(QLabel("Transcode ahead (files)"), 6, 0); g.addWidget(self.lookahead_spin, 6, 1)
        note = QLabel("Output: H.264 MP4 with the selected AAC audio tracks. Compatible sources are remuxed; HDR is converted to SDR. If NVIDIA encoding fails, CouchPush retries with the CPU.")
        note.setObjectName("muted")
        note.setWordWrap(True)
        g.addWidget(note, 7, 0, 1, 3)
        self.settings_body.addWidget(box)

    def _build_source(self) -> None:
        self.source_box = box = QGroupBox("Source files")
        g = QGridLayout(box)
        self.mode_combo = QComboBox()
        for key, label in LIBRARY_LABELS:
            self.mode_combo.addItem(label, key)
        self.mode_combo.currentIndexChanged.connect(self._on_mode_changed)
        self.folder_edit = QLineEdit(placeholderText="Choose a folder, add files, or drop videos here...")
        self.folder_edit.setAcceptDrops(False)
        folder_btn = QPushButton("Browse"); folder_btn.clicked.connect(self._browse_source)
        self.regex_edit = QLineEdit()
        self.scan_btn = QPushButton("Scan folder"); self.scan_btn.clicked.connect(self._on_scan)
        self.files_btn = QPushButton("Add files..."); self.files_btn.clicked.connect(self._browse_files)
        self.clear_btn = QPushButton("Clear queue"); self.clear_btn.clicked.connect(self._clear_queue)
        self.regex_label = QLabel("Episode regex")
        self.regex_hint = QLabel("Named groups: (?P<season>...) (?P<episode>...)")
        g.addWidget(QLabel("Library"), 0, 0); g.addWidget(self.mode_combo, 0, 1)
        g.addWidget(self.files_btn, 0, 2); g.addWidget(self.clear_btn, 0, 3)
        g.addWidget(QLabel("Folder"), 1, 0); g.addWidget(self.folder_edit, 1, 1); g.addWidget(folder_btn, 1, 2)
        g.addWidget(self.scan_btn, 1, 3)
        g.addWidget(self.regex_label, 2, 0); g.addWidget(self.regex_edit, 2, 1, 1, 3)
        g.addWidget(self.regex_hint, 3, 1, 1, 3)
        g.setColumnStretch(1, 1)
        self.body.addWidget(box)

    def _build_target(self) -> None:
        self.target_box = box = QGroupBox("Destination, audio and resolution")
        g = QGridLayout(box)
        self.series_combo = QComboBox(); self.series_combo.setEnabled(False)
        self.series_combo.setEditable(True)
        self.series_combo.setInsertPolicy(QComboBox.NoInsert)
        self.series_combo.completer().setFilterMode(Qt.MatchContains)
        self.series_combo.completer().setCaseSensitivity(Qt.CaseInsensitive)
        self.series_combo.completer().setCompletionMode(QCompleter.PopupCompletion)
        self.series_combo.currentIndexChanged.connect(self._on_series_changed)
        self.series_combo.editTextChanged.connect(self._on_series_text_changed)
        self.lang_combo = QComboBox(); self.lang_combo.setEditable(True)
        self.lang_combo.addItems(COMMON_LANGS)
        self.lang_combo.setInsertPolicy(QComboBox.NoInsert)
        self.lang_combo.setMinimumWidth(130)
        self.lang_combo.currentTextChanged.connect(lambda _=None: self._lang_timer.start())
        self.audio_mode_combo = QComboBox()
        for key, label in AUDIO_MODE_LABELS:
            self.audio_mode_combo.addItem(label, key)
        self.audio_mode_combo.currentIndexChanged.connect(self._on_audio_mode_changed)
        self.audio_lang_label = QLabel("Languages")
        self.audio_hint = QLabel()
        self.audio_hint.setObjectName("muted")
        self.audio_hint.setWordWrap(True)
        self.series_label = QLabel("Series")
        self.resolution_combo = QComboBox()
        for height, label in RESOLUTION_LABELS:
            self.resolution_combo.addItem(label, height)
        self.resolution_combo.setToolTip("Limits the output to the chosen resolution. Preserves aspect ratio and never upscales smaller files. Original size keeps the source dimensions.")
        g.addWidget(self.series_label, 0, 0); g.addWidget(self.series_combo, 0, 1)
        g.addWidget(self.audio_lang_label, 0, 2); g.addWidget(self.lang_combo, 0, 3)
        g.addWidget(QLabel("Resolution"), 0, 4); g.addWidget(self.resolution_combo, 0, 5)
        g.addWidget(QLabel("Audio mode"), 1, 0); g.addWidget(self.audio_mode_combo, 1, 1)
        g.addWidget(self.audio_hint, 1, 2, 1, 4)
        self.movie_hint = QLabel("Review each target below. Double-click a movie to search your library or correct its title.")
        self.movie_hint.setObjectName("muted")
        self.movie_hint.setWordWrap(True)
        g.addWidget(self.movie_hint, 0, 0, 1, 2)
        g.setColumnStretch(1, 1)
        self.body.addWidget(box)

    def _build_table(self) -> None:
        self.queue_split = QSplitter(Qt.Vertical)
        self.queue_split.setChildrenCollapsible(False)
        self.body.addWidget(self.queue_split, 1)
        box = QWidget()
        v = QVBoxLayout(box)
        v.setContentsMargins(0, 0, 0, 0)
        bar = QHBoxLayout()
        self.search_edit = QLineEdit(placeholderText="Filter files or targets...")
        self.search_edit.setClearButtonEnabled(True)
        self.search_edit.textChanged.connect(self._filter_table)
        bar.addWidget(self.search_edit, 1)
        self.filter_combo = QComboBox()
        for label, key in (("All files", "all"), ("Selected", "selected"), ("New titles", "new"),
                           ("Needs review", "review"), ("Has media", "existing"), ("Failed", "failed")):
            self.filter_combo.addItem(label, key)
        self.filter_combo.currentIndexChanged.connect(self._filter_table)
        bar.addWidget(self.filter_combo)
        self.choose_target_btn = QPushButton("Choose target...")
        self.choose_target_btn.clicked.connect(self._choose_movie_target)
        bar.addWidget(self.choose_target_btn)
        self.all_btn = QPushButton("Select visible")
        self.all_btn.clicked.connect(lambda: self._check_all(True))
        self.none_btn = QPushButton("Clear selection")
        self.none_btn.clicked.connect(lambda: self._check_all(False))
        bar.addWidget(self.all_btn); bar.addWidget(self.none_btn)
        v.addLayout(bar)
        counts = QHBoxLayout()
        self.count_label = QLabel("0 files")
        self.count_label.setObjectName("muted")
        counts.addWidget(self.count_label)
        counts.addStretch()
        self.queue_hint = QLabel("Add videos to build your upload queue")
        self.queue_hint.setObjectName("muted")
        counts.addWidget(self.queue_hint)
        v.addLayout(counts)
        self.table = QTableWidget(0, 9)
        self.table.setHorizontalHeaderLabels(
            ["", "File", "S/E", "Episode", "Audio", "Subs", "Action", "Status", "Time"])
        self.table.setEditTriggers(QAbstractItemView.NoEditTriggers)
        self.table.setSelectionMode(QAbstractItemView.SingleSelection)
        self.table.setSelectionBehavior(QAbstractItemView.SelectRows)
        self.table.setAlternatingRowColors(True)
        self.table.setShowGrid(False)
        self.table.setWordWrap(False)
        self.table.verticalHeader().setVisible(False)
        self.table.verticalHeader().setDefaultSectionSize(44)
        self.table.itemChanged.connect(self._on_table_changed)
        self.table.itemSelectionChanged.connect(self._update_summary)
        self.table.cellDoubleClicked.connect(lambda i, _c: self._choose_movie_target(i))
        hh = self.table.horizontalHeader()
        hh.setSectionResizeMode(COL_FILE, QHeaderView.Stretch)
        hh.setSectionResizeMode(COL_EP, QHeaderView.Stretch)
        for c in (COL_SEL, COL_SE, COL_AUDIO, COL_SUBS, COL_ACTION, COL_STATUS, COL_TIME):
            hh.setSectionResizeMode(c, QHeaderView.ResizeToContents)
        hh.setSectionResizeMode(COL_STATUS, QHeaderView.Interactive)
        self.table.setColumnWidth(COL_STATUS, 175)
        self.table.setMinimumHeight(190)
        v.addWidget(self.table)
        self.queue_split.addWidget(box)
        self.queue_split.setStretchFactor(0, 1)

    def _build_run(self) -> None:
        box = QWidget()
        v = QVBoxLayout(box)
        v.setContentsMargins(0, 8, 0, 0)
        row = QHBoxLayout()
        self.start_btn = QPushButton("Start upload"); self.start_btn.clicked.connect(self._on_start)
        self.start_btn.setObjectName("primary")
        self.cancel_btn = QPushButton("Cancel"); self.cancel_btn.clicked.connect(self._on_cancel)
        self.cancel_btn.setEnabled(False)
        self.overall_bar = QProgressBar(); self.overall_bar.setRange(0, 100)
        self.eta_label = QLabel("Ready")
        self.eta_label.setObjectName("muted")
        self.retry_btn = QPushButton("Retry failed")
        self.retry_btn.clicked.connect(self._retry_failed)
        self.retry_btn.setEnabled(False)
        row.addWidget(self.start_btn); row.addWidget(self.cancel_btn)
        row.addWidget(self.retry_btn)
        row.addWidget(self.overall_bar, 1); row.addWidget(self.eta_label)
        v.addLayout(row)
        self.log_view = QPlainTextEdit(); self.log_view.setReadOnly(True); self.log_view.setMaximumBlockCount(500)
        self.log_view.setMinimumHeight(65)
        self.log_view.setPlaceholderText("Encoding and upload activity will appear here. Select a failed row for the full error.")
        v.addWidget(self.log_view)
        self.queue_split.addWidget(box)
        self.queue_split.setSizes([430, 150])

    # ---------- config ----------

    def _apply_config_to_widgets(self) -> None:
        c = self.cfg
        self.url_edit.setText(c.server_url)
        self.user_edit.setText(c.username)
        self.ffmpeg_edit.setText(c.ffmpeg_path)
        self.ffprobe_edit.setText(c.ffprobe_path)
        self.temp_edit.setText(c.temp_dir)
        self.regex_edit.setText(c.episode_regex)
        self.lookahead_spin.setValue(c.lookahead)
        self.keep_channels_chk.setChecked(c.keep_channels)
        self._select_combo(self.encoder_combo, c.encoder)
        self._select_combo(self.quality_combo, c.quality_preset)
        self._select_combo(self.resolution_combo, c.max_height)
        self._select_combo(self.audio_mode_combo, c.audio_mode)
        self._on_audio_mode_changed()
        if c.audio_lang:
            self.lang_combo.setCurrentText(c.audio_lang)
        self.mode_combo.blockSignals(True)
        self._select_combo(self.mode_combo, c.library_kind)
        self.mode_combo.blockSignals(False)
        self.folder_edit.setText(c.source_folder)
        saved_pw = load_password()
        if saved_pw:
            self.pass_edit.setText(saved_pw)
            self.remember_chk.setChecked(True)

    def _gather_config(self) -> None:
        c = self.cfg
        c.server_url = self.url_edit.text().strip()
        c.username = self.user_edit.text().strip()
        c.ffmpeg_path = self.ffmpeg_edit.text().strip() or "ffmpeg"
        c.ffprobe_path = self.ffprobe_edit.text().strip() or "ffprobe"
        c.temp_dir = self.temp_edit.text().strip()
        c.episode_regex = self.regex_edit.text()
        c.lookahead = self.lookahead_spin.value()
        c.keep_channels = self.keep_channels_chk.isChecked()
        c.encoder = self.encoder_combo.currentData()
        c.quality_preset = self.quality_combo.currentData()
        c.audio_lang = self.lang_combo.currentText().strip()
        c.audio_mode = self.audio_mode_combo.currentData()
        c.library_kind = self.mode
        c.source_folder = self.folder_edit.text().strip()
        c.max_height = self.resolution_combo.currentData()
        if self.detail:
            c.last_series_id = self.detail.id

    def closeEvent(self, event) -> None:  # noqa: N802 (Qt override)
        if self._running:
            self._on_cancel()
            self._on_log("Stopping workers before closing...")
            self._close_when_finished = True
            event.ignore()
            return
        self._gather_config()
        self.cfg.save()
        super().closeEvent(event)

    # ---------- connection ----------

    def _async(self, fn, on_done, on_failed=None) -> None:
        """Run fn() off the UI thread; deliver result/error back on the UI thread."""
        call = AsyncCall(fn)
        self._inflight.add(call)

        def done(result):
            self._inflight.discard(call)
            on_done(result)

        def failed(error):
            self._inflight.discard(call)
            (on_failed or self._warn)(error)

        call.done.connect(done)
        call.failed.connect(failed)
        call.start()

    def _try_restore_session(self) -> None:
        if not self.cfg.server_url:
            return
        saved = load_session_cookie()
        if not saved or not saved[0]:
            return
        client = CouchverseClient(self.cfg.server_url)
        client.restore_session(saved[0])
        self.conn_status.setText("Reconnecting...")
        self._async(client.me,
                    lambda user: self._on_session_ready(client, user),
                    lambda _err: self.conn_status.setText("Offline"))

    def _on_login(self) -> None:
        url = self.url_edit.text().strip()
        if not url:
            self._warn("Enter the server URL.")
            return
        client = CouchverseClient(url)
        username = self.user_edit.text().strip()
        password = self.pass_edit.text()
        self.conn_status.setText("Connecting...")
        self.login_btn.setEnabled(False)
        self._async(lambda: client.login(username, password),
                    lambda user: self._on_login_ok(client, password, user),
                    self._on_login_failed)

    def _on_login_ok(self, client: CouchverseClient, password: str, user: dict) -> None:
        self.login_btn.setEnabled(True)
        if user.get("role") != "admin":
            self.conn_status.setText("Not an admin account")
            self._warn("This account is not an admin.")
            return
        if self.remember_chk.isChecked():
            cookie = client.session_cookie()
            if cookie:
                save_session_cookie(cookie[0], cookie[1])
            save_password(password)
        else:
            clear_session_cookie()
            clear_password()
            self.pass_edit.clear()
        self._on_session_ready(client, user)

    def _on_login_failed(self, error: str) -> None:
        self.login_btn.setEnabled(True)
        self.conn_status.setText("Login failed")
        self._warn(f"Could not log in:\n{error}")

    def _on_session_ready(self, client: CouchverseClient, user: dict) -> None:
        if user.get("role") != "admin":
            self.conn_status.setText("Admin account required")
            return
        self.client = client
        self._connection_generation += 1
        self._detail_cache.clear()
        self._detail_loading = False
        self._movies_loading = False
        self.detail = None
        self.movies = None
        self._movie_overrides.clear()
        self.conn_status.setText(f"Connected: {user.get('username', '?')}")
        self.tabs.setCurrentIndex(0)
        self._load_series()
        if self.mode == "movies":
            self._ensure_movies_loaded(self._rebuild_rows)
        self._update_summary()

    def _load_series(self) -> None:
        if not self.client:
            return
        client = self.client
        generation = self._connection_generation
        self._async(client.list_series,
                    lambda titles: self._on_series_loaded(titles) if generation == self._connection_generation else None,
                    lambda err: self._warn(f"Could not load series:\n{err}") if generation == self._connection_generation else None)

    def _on_series_loaded(self, series: list[Title]) -> None:
        self.series = series
        self.series_combo.blockSignals(True)
        self.series_combo.clear()
        for s in series:
            label = f"{s.name} ({s.year})" if s.year else s.name
            self.series_combo.addItem(label, s.id)
        self.series_combo.setEnabled(True)
        idx = self.series_combo.findData(self.cfg.last_series_id) if self.cfg.last_series_id else -1
        self.series_combo.setCurrentIndex(idx if idx >= 0 else 0)
        self.series_combo.blockSignals(False)
        self._on_series_changed()

    def _on_series_changed(self) -> None:
        sid = self.series_combo.currentData()
        if not sid or not self.client:
            self.detail = None
            self._update_summary()
            return
        cached = self._detail_cache.get(sid)
        if cached is not None:
            self.detail = cached
            if self.mode == "series":
                self._rebuild_rows()
            return
        self._set_detail_loading(True)
        client = self.client
        generation = self._connection_generation
        self._async(lambda: client.title_detail(sid),
                    lambda d: self._on_detail_loaded(sid, d) if generation == self._connection_generation else None,
                    lambda err: self._on_detail_failed(err) if generation == self._connection_generation else None)

    def _on_series_text_changed(self, text: str) -> None:
        if text != self.series_combo.itemText(self.series_combo.currentIndex()):
            self.detail = None
            self._update_summary()

    def _on_detail_loaded(self, sid: str, detail: TitleDetail) -> None:
        self._set_detail_loading(False)
        self._detail_cache[sid] = detail
        if self.series_combo.currentData() == sid and self.series_combo.currentText() == self.series_combo.itemText(self.series_combo.currentIndex()):
            self.detail = detail
            if self.mode == "series":
                self._rebuild_rows()

    def _on_detail_failed(self, error: str) -> None:
        self._set_detail_loading(False)
        self._warn(f"Could not load series:\n{error}")

    def _set_detail_loading(self, loading: bool) -> None:
        self._detail_loading = loading
        self._update_summary()

    # ---------- scanning + matching ----------

    def _on_mode_changed(self) -> None:
        if self._running:
            return
        self.mode = self.mode_combo.currentData()
        is_series = self.mode == "series"
        for w in (self.regex_label, self.regex_edit, self.regex_hint, self.series_label, self.series_combo):
            w.setVisible(is_series)
        self.movie_hint.setVisible(not is_series)
        self.choose_target_btn.setVisible(not is_series)
        cols = ["", "File", "S/E", "Episode"] if is_series else ["", "File", "Year", "Target movie"]
        self.table.setHorizontalHeaderLabels(cols + ["Audio", "Subs", "Action", "Status", "Time"])
        self.rows = []
        self.movie_rows = []
        self._loading = False
        self.table.setRowCount(0)
        self._scan_generation += 1
        self._failed_rows.clear()
        self._movie_overrides.clear()
        self._update_summary()
        folder = self.folder_edit.text().strip()
        if self._explicit_paths:
            self._scan_paths(self._explicit_paths)
        elif folder and os.path.isdir(folder):
            self._on_scan()

    def _ensure_movies_loaded(self, then) -> None:
        if self.movies is not None:
            then()
            return
        if not self.client:
            self.movies = []
            then()
            return
        client = self.client
        generation = self._connection_generation
        self._movies_loading = True
        self._update_summary()

        def loaded(movies):
            if generation != self._connection_generation:
                return
            self._movies_loading = False
            self.movies = movies
            then()
            self._update_summary()

        def failed(error):
            if generation != self._connection_generation:
                return
            self._movies_loading = False
            self._update_summary()
            self._warn(f"Could not load movies:\n{error}")

        self._async(client.list_movies, loaded, failed)

    def _on_scan(self) -> None:
        if self._running:
            return
        folder = self.folder_edit.text().strip()
        if not folder or not os.path.isdir(folder):
            self._warn("Choose a valid source folder.")
            return
        self._explicit_paths = []
        self._movie_overrides.clear()
        pattern = self.regex_edit.text()
        mode = self.mode
        if mode == "movies" and self.client:
            self.movies = None
        self._scan_async(lambda: scan_folder(folder, pattern) if mode == "series" else scan_movies(folder))

    def _scan_async(self, fn) -> None:
        self._scan_generation += 1
        generation = self._scan_generation
        mode = self.mode
        self._loading = True
        self._update_summary()

        def done(files):
            if generation != self._scan_generation or mode != self.mode:
                return
            self._loading = False
            self._failed_rows.clear()
            if mode == "series":
                self.parsed = files
                self._rebuild_rows()
            else:
                self.movie_files = files
                self._ensure_movies_loaded(self._rebuild_rows)
            self._update_summary()

        def failed(error):
            if generation != self._scan_generation:
                return
            self._loading = False
            self._update_summary()
            self._warn(f"Could not scan files:\n{error}")

        self._async(fn, done, failed)

    def _scan_paths(self, paths: list[str]) -> None:
        pattern = self.regex_edit.text()
        mode = self.mode
        paths = list(paths)
        for i, row in enumerate(self.movie_rows):
            row.selected = self._checked(i)
            self._movie_overrides[row.movie.path] = row

        def scan():
            try:
                root = os.path.commonpath([os.path.dirname(path) for path in paths]) if paths else ""
            except ValueError:
                root = ""
            rx = compile_regex(pattern) if mode == "series" else None
            files = []
            for path in paths:
                rel = os.path.relpath(path, root).replace("\\", "/") if root else os.path.basename(path)
                if mode == "series":
                    season, episode = parse_name(rx, rel)
                    files.append(ParsedFile(path, rel, season, episode, "" if season is not None and episode is not None else "no season/episode match"))
                else:
                    name, year = parse_movie(rel)
                    files.append(MovieFile(path, rel, name, year))
            return files

        self._scan_async(scan)

    def _rebuild_rows(self) -> None:
        if self._running:
            return
        try:
            batch_lang = preferred_language(self.lang_combo.currentText())
        except ValueError:
            self._update_summary()
            return
        if self.mode == "series":
            self.rows = build_match_rows(self.detail, self.parsed, batch_lang) if self.detail else []
        else:
            if self.movies is None:
                return
            self.movie_rows = build_movie_rows(self.movies, self.movie_files)
            self.movie_rows = [self._movie_overrides.get(row.movie.path, row) for row in self.movie_rows]
        self._populate_table()

    def _on_language_changed(self) -> None:
        if self._running:
            return
        if self.mode == "series":
            self._rebuild_rows()
        else:
            for i, row in enumerate(self.movie_rows):
                self.table.item(i, COL_AUDIO).setText(self._audio_label(row.audio_role))
            self._update_summary()

    def _on_audio_mode_changed(self, *_args) -> None:
        multiple = self.audio_mode_combo.currentData() == "multiple"
        self.audio_lang_label.setText("Languages" if multiple else "Prefer audio")
        self.audio_hint.setText(
            "en, cs includes both tracks. First language is default. CouchVerse prepares playback for switching."
            if multiple else "en, cs picks the first available language. One audio track is uploaded."
        )
        self.lang_combo.setToolTip(
            "Include one matching track per listed language, in order. Missing languages are reported; no matches fails the file."
            if multiple else "Ordered preferences, such as en, cs. If none match, the first source track is used with a warning."
        )
        if not hasattr(self, "table") or self._running:
            return
        rows = self.rows if self.mode == "series" else self.movie_rows
        for i, row in enumerate(rows):
            self.table.item(i, COL_AUDIO).setText(self._audio_label(row.audio_role))
            action = self._series_action_label(row) if self.mode == "series" else self._movie_action_label(row)
            self.table.item(i, COL_ACTION).setText(action)
        self._update_summary()

    def _populate_table(self) -> None:
        self._timings = {}
        rows = self.rows if self.mode == "series" else self.movie_rows
        if self.mode == "movies":
            seen = {}
            for i, row in enumerate(rows):
                row.note = row.note.split("; duplicate target", 1)[0]
                row.duplicate_of = None
                if row.needs_review:
                    continue
                key = movie_target_key(row)
                if key in seen:
                    row.duplicate_of = seen[key]
                    row.note += "; duplicate target (select only one)"
                    row.selected = False
                else:
                    seen[key] = i
        self.table.blockSignals(True)
        self.table.setUpdatesEnabled(False)
        try:
            self.table.setRowCount(len(rows))
            for i, r in enumerate(rows):
                if self.mode == "series":
                    self._fill_series_row(i, r)
                else:
                    self._fill_movie_row(i, r)
        finally:
            self.table.setUpdatesEnabled(True)
            self.table.blockSignals(False)
        self._filter_table()
        self._update_summary()

    def _fill_series_row(self, i: int, r: MatchRow) -> None:
        self._set_check(i, r.selected)
        self.table.setItem(i, COL_FILE, QTableWidgetItem(r.parsed.rel_path))
        self.table.setItem(i, COL_SE, QTableWidgetItem(f"S{r.season:02d}E{r.episode:02d}" if r.parsed.ok else "?"))
        ep = r.episode_name if r.exists else ("(new episode)" if r.parsed.ok else "")
        self.table.setItem(i, COL_EP, QTableWidgetItem(ep))
        self.table.setItem(i, COL_AUDIO, QTableWidgetItem(self._audio_label(r.audio_role)))
        self.table.setItem(i, COL_SUBS, QTableWidgetItem(""))
        self.table.setItem(i, COL_ACTION, QTableWidgetItem(self._series_action_label(r)))
        self.table.setItem(i, COL_STATUS, QTableWidgetItem(r.note))
        self.table.setItem(i, COL_TIME, QTableWidgetItem(""))

    def _fill_movie_row(self, i: int, r: MovieRow) -> None:
        self._set_check(i, r.selected)
        self.table.setItem(i, COL_FILE, QTableWidgetItem(r.movie.rel_path))
        self.table.setItem(i, COL_SE, QTableWidgetItem(str(r.target_year) if r.target_year else "-"))
        self.table.setItem(i, COL_EP, QTableWidgetItem(r.target_name or "(unparsed)"))
        self.table.item(i, COL_EP).setToolTip("Double-click to choose or correct the target movie")
        self.table.setItem(i, COL_AUDIO, QTableWidgetItem(self._audio_label(r.audio_role)))
        self.table.setItem(i, COL_SUBS, QTableWidgetItem(""))
        self.table.setItem(i, COL_ACTION, QTableWidgetItem(self._movie_action_label(r)))
        self.table.setItem(i, COL_STATUS, QTableWidgetItem(r.note))
        self.table.item(i, COL_STATUS).setToolTip(r.note)
        if r.needs_review or r.duplicate_of is not None:
            self.table.item(i, COL_STATUS).setForeground(QColor("#f5c56c"))
        self.table.setItem(i, COL_TIME, QTableWidgetItem(""))

    def _set_check(self, i: int, checked: bool) -> None:
        sel = QTableWidgetItem()
        sel.setFlags(Qt.ItemIsUserCheckable | Qt.ItemIsEnabled | Qt.ItemIsSelectable)
        sel.setCheckState(Qt.Checked if checked else Qt.Unchecked)
        self.table.setItem(i, COL_SEL, sel)

    def _audio_label(self, role: str) -> str:
        lang = self.lang_combo.currentText().strip() or "-"
        if self.audio_mode_combo.currentData() == "multiple":
            return f"{lang} (listed tracks)"
        return f"{lang} ({'alt' if role == ROLE_ALT else 'primary'})"

    def _series_action_label(self, r: MatchRow) -> str:
        if not r.parsed.ok:
            return "-"
        if self.audio_mode_combo.currentData() == "multiple":
            return "Create + upload" if r.will_create else "Upload languages if checked"
        if r.has_same_lang:
            return "Replace if checked"
        if r.audio_role == ROLE_ALT:
            return "Upload (alt audio)"
        return "Create + upload" if r.will_create else "Upload"

    def _movie_action_label(self, r: MovieRow) -> str:
        if r.needs_review:
            return "Choose target"
        if r.has_media:
            return "Upload languages" if self.audio_mode_combo.currentData() == "multiple" else "Replace language"
        return "Upload" if r.exists else "Create + upload"

    def _checked(self, i: int) -> bool:
        item = self.table.item(i, COL_SEL)
        return item is not None and item.checkState() == Qt.Checked

    def _check_all(self, checked: bool) -> None:
        rows = self.rows if self.mode == "series" else self.movie_rows
        for i in range(min(self.table.rowCount(), len(rows))):
            ok = rows[i].parsed.ok if self.mode == "series" else not rows[i].needs_review and bool(rows[i].target_name)
            duplicate = "duplicate mapping" in rows[i].note if self.mode == "series" else rows[i].duplicate_of is not None
            if not self.table.isRowHidden(i) and (not checked or ok and not duplicate):
                self.table.item(i, COL_SEL).setCheckState(Qt.Checked if checked else Qt.Unchecked)

    def _on_table_changed(self, item: QTableWidgetItem) -> None:
        if item.column() != COL_SEL:
            return
        rows = self.rows if self.mode == "series" else self.movie_rows
        if item.row() < len(rows):
            rows[item.row()].selected = item.checkState() == Qt.Checked
        self._filter_table()
        self._update_summary()

    def _filter_table(self, *_args) -> None:
        rows = self.rows if self.mode == "series" else self.movie_rows
        query = self.search_edit.text().casefold().split()
        mode = self.filter_combo.currentData()
        for i, row in enumerate(rows):
            source = row.parsed.rel_path if self.mode == "series" else row.movie.rel_path
            target = row.episode_name if self.mode == "series" else row.target_name
            text = f"{source} {target}".casefold()
            shown = all(word in text for word in query)
            exists = row.exists
            review = not row.parsed.ok if self.mode == "series" else row.needs_review or row.duplicate_of is not None
            has_media = row.has_same_lang if self.mode == "series" else row.has_media
            shown = shown and (mode == "all" or mode == "selected" and self._checked(i)
                              or mode == "new" and not exists or mode == "review" and review
                              or mode == "existing" and has_media or mode == "failed" and i in self._failed_rows)
            self.table.setRowHidden(i, not shown)
        self._update_summary()

    def _update_summary(self) -> None:
        if not hasattr(self, "table"):
            return
        rows = self.rows if self.mode == "series" else self.movie_rows
        selected = sum(self._checked(i) for i in range(len(rows)))
        visible = sum(not self.table.isRowHidden(i) for i in range(len(rows)))
        self.count_label.setText(f"{len(rows)} files  /  {selected} selected  /  {visible} visible")
        loading = self._loading or (getattr(self, "_movies_loading", False) if self.mode == "movies" else getattr(self, "_detail_loading", False))
        self.scan_btn.setEnabled(not self._running and not loading)
        self.files_btn.setEnabled(not self._running and not loading)
        self.clear_btn.setEnabled(not self._running and bool(rows))
        self.retry_btn.setEnabled(not self._running and bool(self._failed_rows))
        self.choose_target_btn.setEnabled(not self._running and not loading and self.table.currentRow() >= 0)
        try:
            languages = parse_language_preferences(self.lang_combo.currentText())
            valid_lang = bool(languages) or self.audio_mode_combo.currentData() == "single"
        except ValueError:
            valid_lang = False
        ready = bool(self.client) and (self.movies is not None if self.mode == "movies" else self.detail is not None)
        self.start_btn.setEnabled(not self._running and not loading and ready and selected > 0 and valid_lang)
        self.start_btn.setText(f"Upload {selected} {'movie' if self.mode == 'movies' else 'episode'}{'s' if selected != 1 else ''}" if selected else "Start upload")
        if self._running:
            self.queue_hint.setText("Encoding and uploading in parallel")
        elif loading:
            self.queue_hint.setText("Loading files and library...")
        elif not valid_lang:
            self.queue_hint.setText("Use audio codes such as en or en, cs")
        elif not self.client:
            self.queue_hint.setText("Connect your server in settings to upload")
        elif self.mode == "series" and not self.detail:
            self.queue_hint.setText("Choose a series from your library")
        elif not rows:
            self.queue_hint.setText("Add files or drop a folder to get started")
        else:
            self.queue_hint.setText("Only checked files will upload, including hidden selections")

    def _choose_movie_target(self, index=None) -> None:
        if self.mode != "movies" or self._running:
            return
        if isinstance(index, bool) or index is None:
            index = self.table.currentRow()
        if index < 0 or index >= len(self.movie_rows):
            return
        dialog = MoviePicker(self.movie_rows[index], self.movies or [], self)
        if dialog.exec() == QDialog.Accepted and dialog.result_row is not None:
            row = dialog.result_row
            self._movie_overrides[row.movie.path] = row
            self.movie_rows[index] = row
            self._populate_table()
            self.table.selectRow(index)

    def _clear_queue(self) -> None:
        self._scan_generation += 1
        self._loading = False
        self._explicit_paths = []
        self._movie_overrides.clear()
        self._failed_rows.clear()
        self.rows, self.movie_rows, self.parsed, self.movie_files = [], [], [], []
        self._populate_table()

    # ---------- run ----------

    def _collect_jobs(self) -> list[Job]:
        jobs: list[Job] = []
        if self.mode == "series":
            for i, r in enumerate(self.rows):
                if not self._checked(i) or not r.parsed.ok:
                    continue
                action = ACTION_REPLACE if r.has_same_lang else ACTION_UPLOAD
                jobs.append(Job(
                    index=i, source_path=r.parsed.path, rel_path=r.parsed.rel_path,
                    output_base=f"S{r.season:02d}E{r.episode:02d}", audio_role=r.audio_role,
                    action=action, episode_id=r.episode_id, replace_ids=r.replace_ids))
        else:
            for i, r in enumerate(self.movie_rows):
                if not self._checked(i) or r.needs_review or not r.target_name:
                    continue
                action = ACTION_REPLACE if r.has_media else ACTION_UPLOAD
                jobs.append(Job(
                    index=i, source_path=r.movie.path, rel_path=r.movie.rel_path,
                    output_base=r.output_base, audio_role=r.audio_role, action=action, title_id=r.title_id,
                    create_new_title=r.title_id is None, new_title_name=r.target_title, new_title_year=r.target_year))
        return jobs

    def _on_start(self) -> None:
        if self._running or self._loading or (self.mode == "movies" and getattr(self, "_movies_loading", False)):
            return
        if not self.client:
            self._warn("Log in first.")
            return
        if self.mode == "series" and not self.detail:
            self._warn("Pick a series first.")
            return
        try:
            languages = parse_language_preferences(self.lang_combo.currentText())
            if self.audio_mode_combo.currentData() == "multiple" and not languages:
                raise ValueError("List the audio languages to include, such as en, cs.")
        except ValueError as error:
            self._warn(str(error))
            return
        if self.mode == "movies":
            errors = movie_selection_errors(self.movie_rows)
            if errors:
                self._warn("Review the selected movies before uploading:\n\n" + "\n".join(errors))
                return
        else:
            keys = [(r.season, r.episode) for i, r in enumerate(self.rows) if self._checked(i)]
            if len(keys) != len(set(keys)):
                self._warn("Multiple selected files target the same episode. Select one file per episode.")
                return
        jobs = self._collect_jobs()
        if not jobs:
            self._warn("No items selected.")
            return
        temp = self.temp_edit.text().strip()
        if not temp:
            self._warn("Choose a temp folder.")
            return
        self._gather_config()
        self.cfg.save()
        for label, binary in (("ffmpeg", self.cfg.ffmpeg_path), ("ffprobe", self.cfg.ffprobe_path)):
            if not shutil.which(binary):
                self._warn(f"{label} was not found. Set its executable path in Connection and encoding.")
                return
        settings = BatchSettings(
            audio_lang=self.lang_combo.currentText().strip(), temp_dir=temp,
            library_kind=self.mode,
            title_id=self.detail.id if (self.mode == "series" and self.detail) else "",
            quality_preset=self.quality_combo.currentData(),
            encoder=self.encoder_combo.currentData(), keep_channels=self.keep_channels_chk.isChecked(),
            lookahead=self.lookahead_spin.value(), ffmpeg_path=self.cfg.ffmpeg_path,
            ffprobe_path=self.cfg.ffprobe_path,
            max_height=self.resolution_combo.currentData(),
            audio_mode=self.audio_mode_combo.currentData(),
        )
        self._failed_rows.clear()
        self._timings.clear()
        self._active_jobs = {j.index: j for j in jobs}
        self.overall_bar.setValue(0)
        for j in jobs:
            self._set_status(j.index, "Queued")
            self.table.item(j.index, COL_TIME).setText("")
        try:
            self.runner = PipelineRunner(self.client, jobs, settings)
        except OSError as error:
            self._warn(f"Could not prepare the temp folder:\n{error}")
            return
        em = self.runner.emitter
        em.sig_log.connect(self._on_log)
        em.sig_item_state.connect(self._on_item_state)
        em.sig_item_progress.connect(self._on_item_progress)
        em.sig_item_subs.connect(self._on_item_subs)
        em.sig_item_timing.connect(self._on_item_timing)
        em.sig_item_audio.connect(self._on_item_audio)
        em.sig_item_done.connect(self._on_item_done)
        em.sig_item_failed.connect(self._on_item_failed)
        em.sig_overall.connect(self._on_overall)
        em.sig_finished.connect(self._on_finished)
        self._set_running(True)
        self._on_log(f"Starting {len(jobs)} {'movie' if self.mode == 'movies' else 'episode'}(s)...")
        self.runner.start()

    def _on_cancel(self) -> None:
        if self.runner:
            self._on_log("Cancelling...")
            self.cancel_btn.setEnabled(False)
            self.runner.cancel()

    def _set_running(self, running: bool) -> None:
        self._running = running
        self._lang_timer.stop()
        self.cancel_btn.setEnabled(running)
        for w in (self.connection_box, self.encoding_box, self.source_box, self.target_box,
                  self.all_btn, self.none_btn, self.table):
            w.setEnabled(not running)
        self._update_summary()

    def _retry_failed(self) -> None:
        if self._running or not self._failed_rows:
            return
        for i in range(self.table.rowCount()):
            self.table.item(i, COL_SEL).setCheckState(Qt.Checked if i in self._failed_rows else Qt.Unchecked)
        self._on_start()

    # ---------- signal slots (UI thread) ----------

    def _on_log(self, message: str) -> None:
        self.log_view.appendPlainText(message)

    def _on_item_state(self, index: int, state: str) -> None:
        self._set_status(index, state)

    def _on_item_progress(self, index: int, phase: str, pct: float) -> None:
        self._set_status(index, f"{phase.capitalize()} {pct:.0f}%")

    def _on_item_subs(self, index: int, langs: list) -> None:
        item = self.table.item(index, COL_SUBS)
        if item is not None:
            item.setText(", ".join(langs) if langs else "-")

    def _on_item_timing(self, index: int, phase: str, seconds: float) -> None:
        self._timings.setdefault(index, {})[phase] = seconds
        rec = self._timings[index]
        parts = []
        if "transcode" in rec:
            parts.append(f"T {_fmt_secs(rec['transcode'])}")
        if "upload" in rec:
            parts.append(f"U {_fmt_secs(rec['upload'])}")
        item = self.table.item(index, COL_TIME)
        if item is not None:
            item.setText("  ".join(parts))

    def _on_item_done(self, index: int, media_file_id: str) -> None:
        self._set_status(index, "Done", QColor("#34d399"))
        self.table.item(index, COL_SEL).setCheckState(Qt.Unchecked)
        self._remember_movie_id(index, has_media=True)

    def _remember_movie_id(self, index: int, has_media: bool = False) -> None:
        if self.mode != "movies" or index >= len(self.movie_rows):
            return
        job = getattr(self, "_active_jobs", {}).get(index)
        row = self.movie_rows[index]
        if job and job.title_id:
            row.title_id = job.title_id
            row.exists = True
            row.has_media = row.has_media or has_media
            self._movie_overrides[row.movie.path] = row

    def _on_item_audio(self, index: int, lang: str, role: str) -> None:
        item = self.table.item(index, COL_AUDIO)
        if item is not None:
            label = "switchable" if "," in lang else "alt" if role == ROLE_ALT else "primary"
            item.setText(f"{lang or 'unknown'} ({label})")

    def _on_item_failed(self, index: int, error: str) -> None:
        self._failed_rows.add(index)
        self._remember_movie_id(index)
        self._set_status(index, f"Failed: {error}", QColor("#f0635c"))
        self._on_log(f"[row {index + 1}] failed: {error}")
        self._filter_table()

    def _on_overall(self, pct: float, eta_seconds: float) -> None:
        self.overall_bar.setValue(int(pct))
        self.eta_label.setText(f"ETA {_fmt_eta(eta_seconds)}")

    def _on_finished(self, summary: dict) -> None:
        self._set_running(False)
        done, failed = len(summary.get("done", [])), len(summary.get("failed", []))
        if summary.get("cancelled"):
            msg = f"Cancelled. {done} done, {failed} failed."
        else:
            msg = f"Finished: {done} done, {failed} failed."
            self.overall_bar.setValue(100)
            self.eta_label.setText("ETA -")
        self._on_log(msg)
        self.eta_label.setText(msg)
        done_ids = set(summary.get("done", []))
        failed_ids = set(summary.get("failed", []))
        for index in getattr(self, "_active_jobs", {}):
            self._remember_movie_id(index)
            if index not in done_ids and index not in failed_ids:
                self._set_status(index, "Cancelled" if summary.get("cancelled") else "Not completed")
        self._filter_table()
        self.movies = None  # movie has-media flags may have changed; reload on next movies scan
        if self.mode == "movies" and self.client:
            self._ensure_movies_loaded(lambda: None)
        if self.mode == "series" and self.detail and self.client:
            sid = self.detail.id
            client = self.client
            self._async(lambda: client.title_detail(sid),
                        lambda d: self._refresh_detail(sid, d), lambda _e: None)
        if getattr(self, "_close_when_finished", False):
            QTimer.singleShot(0, self.close)

    def _refresh_detail(self, sid: str, detail: TitleDetail) -> None:
        self._detail_cache[sid] = detail
        if self.series_combo.currentData() == sid:
            self.detail = detail

    def _set_status(self, index: int, text: str, color: QColor | None = None) -> None:
        item = self.table.item(index, COL_STATUS)
        if item is None:
            return
        item.setText(text.splitlines()[0][:100])
        item.setToolTip(text)
        item.setForeground(color or QColor("#bec5d7"))

    # ---------- helpers ----------

    def _browse_source(self) -> None:
        d = QFileDialog.getExistingDirectory(self, "Choose source folder", self.folder_edit.text())
        if d:
            self.folder_edit.setText(d)
            self._on_scan()

    def _browse_files(self) -> None:
        extensions = " ".join(f"*{ext}" for ext in sorted(VIDEO_EXTS))
        paths, _ = QFileDialog.getOpenFileNames(self, "Add movies or episodes", self.folder_edit.text(),
                                               f"Video files ({extensions});;All files (*)")
        if paths:
            self._add_files(paths)

    def _add_files(self, paths: list[str]) -> None:
        if self._running:
            return
        current = [row.path for row in (self.parsed if self.mode == "series" else self.movie_files)]
        accepted = [os.path.abspath(path) for path in paths if os.path.isfile(path) and os.path.splitext(path)[1].lower() in VIDEO_EXTS]
        self._explicit_paths = list(dict.fromkeys(current + accepted))
        if self._explicit_paths:
            self._scan_paths(self._explicit_paths)

    def dragEnterEvent(self, event) -> None:  # noqa: N802
        if not self._running and event.mimeData().hasUrls():
            paths = [url.toLocalFile() for url in event.mimeData().urls() if url.isLocalFile()]
            if any(os.path.isdir(path) or os.path.splitext(path)[1].lower() in VIDEO_EXTS for path in paths):
                event.acceptProposedAction()

    def dropEvent(self, event) -> None:  # noqa: N802
        if self._running:
            return
        paths = [url.toLocalFile() for url in event.mimeData().urls() if url.isLocalFile()]
        folders = [path for path in paths if os.path.isdir(path)]
        if folders:
            self.folder_edit.setText(folders[0])
            self._on_scan()
            if len(folders) > 1:
                self._on_log("Scanned the first dropped folder. Add more videos with Add files.")
        else:
            self._add_files(paths)
        event.acceptProposedAction()

    def _browse_dir(self, edit: QLineEdit) -> None:
        d = QFileDialog.getExistingDirectory(self, "Choose folder", edit.text())
        if d:
            edit.setText(d)

    def _browse_file(self, edit: QLineEdit) -> None:
        f, _ = QFileDialog.getOpenFileName(self, "Choose executable", edit.text(), "Programs (*.exe);;All files (*)")
        if f:
            edit.setText(f)

    def _select_combo(self, combo: QComboBox, key: str) -> None:
        idx = combo.findData(key)
        if idx >= 0:
            combo.setCurrentIndex(idx)

    def _warn(self, message: str) -> None:
        QMessageBox.warning(self, "Couchpush", message)


def _fmt_eta(seconds: float) -> str:
    s = int(seconds)
    if s <= 0:
        return "-"
    h, rem = divmod(s, 3600)
    m, sec = divmod(rem, 60)
    if h:
        return f"{h}h {m}m"
    if m:
        return f"{m}m {sec}s"
    return f"{sec}s"


def _fmt_secs(seconds: float) -> str:
    s = int(round(seconds))
    if s >= 60:
        return f"{s // 60}m{s % 60:02d}s"
    return f"{s}s"
