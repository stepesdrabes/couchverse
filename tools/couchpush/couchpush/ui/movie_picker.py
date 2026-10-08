"""Search catalog targets or correct the metadata for a new movie."""

from PySide6.QtCore import Qt
from PySide6.QtWidgets import (
    QAbstractSpinBox, QDialog, QDialogButtonBox, QFormLayout, QLabel, QLineEdit, QListWidget,
    QListWidgetItem, QSpinBox, QTabWidget, QVBoxLayout, QWidget,
)

from ..api.models import Title
from ..core.matcher import MovieRow, build_movie_row, suggest_movies


class MoviePicker(QDialog):
    def __init__(self, row: MovieRow, movies: list[Title], parent=None):
        super().__init__(parent)
        self.setWindowTitle("Choose movie target")
        self.resize(650, 580)
        self.row = row
        self.result_row: MovieRow | None = None
        suggested = suggest_movies(row.movie, movies, limit=12)
        ranks = {movie.id: i for i, movie in enumerate(suggested)}
        self.movies = sorted(movies, key=lambda m: (ranks.get(m.id, 99), m.name.casefold(), m.year or 0))
        layout = QVBoxLayout(self)
        layout.setContentsMargins(24, 20, 24, 20)
        title = QLabel("Where should this movie go?")
        title.setObjectName("sectionTitle")
        layout.addWidget(title)
        source = QLabel(row.movie.rel_path)
        source.setObjectName("muted")
        source.setWordWrap(True)
        layout.addWidget(source)
        self.tabs = QTabWidget()
        existing = QWidget()
        existing_layout = QVBoxLayout(existing)
        self.search = QLineEdit(placeholderText="Search your library by title or year...")
        self.search.setClearButtonEnabled(True)
        existing_layout.addWidget(self.search)
        self.list = QListWidget()
        existing_layout.addWidget(self.list, 1)
        self.empty = QLabel("No matching movies. Use Create new to add one.")
        self.empty.setObjectName("muted")
        existing_layout.addWidget(self.empty)
        hint = QLabel("Titles with media replace only the selected audio language; other languages stay available.")
        hint.setWordWrap(True)
        hint.setObjectName("muted")
        existing_layout.addWidget(hint)
        self.tabs.addTab(existing, "Existing movie")
        new = QWidget()
        form = QFormLayout(new)
        form.setContentsMargins(12, 20, 12, 12)
        self.name = QLineEdit(row.target_title or row.movie.name)
        self.year = QSpinBox()
        self.year.setRange(0, 2100)
        self.year.setButtonSymbols(QAbstractSpinBox.PlusMinus)
        self.year.setSpecialValueText("Unknown")
        self.year.setValue(row.target_year or 0)
        form.addRow("Movie title", self.name)
        form.addRow("Release year", self.year)
        new_hint = QLabel("Creates a draft movie in CouchVerse when the upload completes. You can add artwork and TMDB metadata in the admin.")
        new_hint.setWordWrap(True)
        new_hint.setObjectName("muted")
        form.addRow(new_hint)
        self.tabs.addTab(new, "Create new")
        layout.addWidget(self.tabs, 1)
        self.buttons = QDialogButtonBox(QDialogButtonBox.Ok | QDialogButtonBox.Cancel)
        self.buttons.button(QDialogButtonBox.Ok).setText("Use this target")
        self.buttons.button(QDialogButtonBox.Ok).setObjectName("primary")
        self.buttons.accepted.connect(self.accept)
        self.buttons.rejected.connect(self.reject)
        layout.addWidget(self.buttons)
        self.search.textChanged.connect(self._filter)
        self.list.itemSelectionChanged.connect(self._update_ok)
        self.list.itemDoubleClicked.connect(lambda _item: self.accept())
        self.tabs.currentChanged.connect(self._update_ok)
        self.name.textChanged.connect(self._update_ok)
        self._filter()
        self.tabs.setCurrentIndex(0 if row.title_id or row.needs_review else 1)
        self._update_ok()

    def _filter(self) -> None:
        current = self.list.currentItem()
        chosen = current.data(Qt.UserRole).id if current else self.row.title_id
        query = self.search.text().casefold().split()
        self.list.clear()
        for movie in self.movies:
            label = f"{movie.name} ({movie.year})" if movie.year else movie.name
            if not all(part in label.casefold() for part in query):
                continue
            state = "Has media - same-language replace / alternate audio" if movie.has_media else "Ready for its first upload"
            item = QListWidgetItem(f"{label}\n{state}")
            item.setData(Qt.UserRole, movie)
            self.list.addItem(item)
            if movie.id == chosen:
                self.list.setCurrentItem(item)
        self.empty.setVisible(self.list.count() == 0)
        self._update_ok()

    def _update_ok(self, *_args) -> None:
        enabled = self.list.currentItem() is not None if self.tabs.currentIndex() == 0 else bool(self.name.text().strip())
        self.buttons.button(QDialogButtonBox.Ok).setEnabled(enabled)

    def accept(self) -> None:
        if self.tabs.currentIndex() == 0:
            item = self.list.currentItem()
            if item is None:
                return
            self.result_row = build_movie_row(self.row.movie, item.data(Qt.UserRole))
        else:
            if not self.name.text().strip():
                return
            self.result_row = build_movie_row(self.row.movie, new_name=self.name.text().strip(),
                                              new_year=self.year.value() or None)
        super().accept()
