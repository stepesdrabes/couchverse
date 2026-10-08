"""Render representative UI states without contacting a server or saving settings."""

import os
os.environ["QT_QPA_PLATFORM"] = "offscreen"

from pathlib import Path
from unittest.mock import Mock, patch

from PySide6.QtGui import QFontDatabase
from PySide6.QtWidgets import QApplication

from couchpush.api.models import Title
from couchpush.config import Config
from couchpush.core.parser import MovieFile
from couchpush.ui.main_window import MainWindow
from couchpush.ui.movie_picker import MoviePicker
from couchpush.ui.theme import apply_theme


def main():
    app = QApplication([])
    for filename in ("segoeui.ttf", "segoeuib.ttf", "consola.ttf"):
        font = Path("C:/Windows/Fonts") / filename
        if font.exists():
            QFontDatabase.addApplicationFont(str(font))
    apply_theme(app)
    output = Path(__file__).resolve().parents[1] / "preview"
    output.mkdir(exist_ok=True)
    with patch("couchpush.ui.main_window.Config.load", return_value=Config()), \
         patch("couchpush.ui.main_window.load_password", return_value=""), \
         patch("couchpush.ui.main_window.load_session_cookie", return_value=None), \
         patch.object(Config, "save"):
        window = MainWindow()
        window._async = Mock()
        window.client = Mock()
        window.conn_status.setText("Connected: admin")
        window.movies = [Title("iron", "Iron Man", 2008, "movie", 0, 0, "ready", 900000000, 1080),
                         Title("iron2", "Iron Man 2", 2010, "movie", 0, 0, "draft", 0, 0),
                         Title("thing1", "The Thing", 1982, "movie", 0, 0, "ready", 0, 0),
                         Title("thing2", "The Thing", 2011, "movie", 0, 0, "ready", 0, 0)]
        filenames = [("Iron Man (2008) 1080p.mkv", "Iron Man", 2008),
                     ("Iron Man 2 (2010) UHDRDV cz en.mkv", "Iron Man 2", 2010),
                     ("Iron Man 3 (2013) UHD.mkv", "Iron Man 3", 2013),
                     ("The.Thing.1080p.mkv", "The Thing", None)]
        window.movie_files = [MovieFile(f"C:/Movies/{filename}", filename, name, year)
                              for filename, name, year in filenames]
        window.folder_edit.setText("C:/Movies/Marvel")
        window.lang_combo.setCurrentText("en, cs")
        window.quality_combo.setCurrentIndex(1)
        window._rebuild_rows()
        window.show()
        app.processEvents()
        window.grab().save(str(output / "queue.png"))
        window.resize(1000, 720)
        app.processEvents()
        window.grab().save(str(output / "queue-compact.png"))
        window.resize(1280, 900)
        window.tabs.setCurrentIndex(1)
        app.processEvents()
        window.grab().save(str(output / "settings.png"))
        picker = MoviePicker(window.movie_rows[3], window.movies, window)
        picker.show()
        app.processEvents()
        picker.grab().save(str(output / "movie-picker.png"))
        picker.reject()
        window.close()
    print(f"UI previews saved to {output}")


if __name__ == "__main__":
    main()
