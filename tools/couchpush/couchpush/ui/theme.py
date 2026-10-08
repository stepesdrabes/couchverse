"""CouchVerse colors shared by the desktop queue and its dialogs."""

from PySide6.QtGui import QColor, QFont, QPalette
from PySide6.QtWidgets import QApplication


def apply_theme(app: QApplication) -> None:
    app.setStyle("Fusion")
    app.setFont(QFont("Segoe UI", 10))
    palette = QPalette()
    for role, color in (
        (QPalette.Window, "#0b0d14"), (QPalette.WindowText, "#f2f4f8"),
        (QPalette.Base, "#11141e"), (QPalette.AlternateBase, "#151925"),
        (QPalette.Text, "#f2f4f8"), (QPalette.Button, "#1d2230"),
        (QPalette.ButtonText, "#f2f4f8"), (QPalette.Highlight, "#3b1c29"),
        (QPalette.HighlightedText, "#ffffff"), (QPalette.ToolTipBase, "#252b3b"),
        (QPalette.ToolTipText, "#f2f4f8"),
        (QPalette.PlaceholderText, "#8993aa"),
    ):
        palette.setColor(role, QColor(color))
    palette.setColor(QPalette.Disabled, QPalette.Text, QColor("#626a80"))
    palette.setColor(QPalette.Disabled, QPalette.ButtonText, QColor("#626a80"))
    app.setPalette(palette)
    app.setStyleSheet("""
        QMainWindow, QDialog { background: #0b0d14; }
        QLabel { background: transparent; }
        QLabel#brand { font-size: 25px; font-weight: 700; }
        QLabel#brandMark { background: #e50914; color: white; border-radius: 12px;
                           font-size: 22px; font-weight: 800; padding: 8px; }
        QLabel#muted { color: #969eb3; }
        QLabel#sectionTitle { font-size: 19px; font-weight: 650; }
        QLabel#stat { font-size: 23px; font-weight: 700; }
        QLabel#pill { color: #aab2c8; background: #1b2030;
                      border-radius: 13px; padding: 6px 12px; }
        QGroupBox { background: #11141e; border: 1px solid #262c3d;
                    border-radius: 12px; margin-top: 12px; padding: 16px 12px 10px; }
        QGroupBox::title { subcontrol-origin: margin; left: 15px;
                          padding: 0 6px; color: #bec5d7; font-weight: 600; }
        QLineEdit, QComboBox, QSpinBox { background: #191e2b; border: 1px solid #30384b;
                      border-radius: 7px; padding: 7px 9px; min-height: 18px; }
        QLineEdit:focus, QComboBox:focus, QSpinBox:focus { border-color: #ea5961; }
        QLineEdit:disabled, QComboBox:disabled, QSpinBox:disabled { color: #626a80; }
        QComboBox { padding-right: 24px; }
        QComboBox QAbstractItemView { background: #191e2b; selection-background-color: #3b2634; }
        QPushButton { background: #202637; color: #e8ecf5; border: 1px solid #354057;
                      border-radius: 7px; padding: 8px 13px; font-weight: 600; }
        QPushButton:hover { background: #2b3347; border-color: #65718c; }
        QPushButton:pressed { background: #171c29; }
        QPushButton:disabled { color: #626a80; border-color: #262c3d; background: #171b27; }
        QPushButton#primary { background: #df1522; border-color: #ed2633; color: white; }
        QPushButton#primary:hover { background: #f02835; }
        QPushButton#primary:disabled { background: #49212a; border-color: #49212a; color: #9a6f7a; }
        QCheckBox { spacing: 7px; }
        QCheckBox::indicator { width: 17px; height: 17px; }
        QTabWidget::pane { border: none; }
        QTabBar::tab { color: #909ab1; padding: 11px 22px; border-bottom: 2px solid transparent; }
        QTabBar::tab:selected { color: #ffffff; border-bottom: 2px solid #e50914; }
        QTabBar::tab:hover { color: #ffffff; background: #151925; }
        QTableWidget, QListWidget { border: 1px solid #262c3d; border-radius: 8px;
                                   background: #11141e; alternate-background-color: #151925;
                                   gridline-color: #242a3a; selection-background-color: #342334; }
        QTableWidget::item { padding: 6px; border-bottom: 1px solid #202636; }
        QTableWidget::item:selected { background: #342334; }
        QHeaderView::section { background: #191e2b; color: #9ea8bf; border: none;
                              border-bottom: 1px solid #30384b; padding: 10px 7px; }
        QListWidget::item { padding: 12px; border-bottom: 1px solid #242a3a; }
        QListWidget::item:selected { background: #342334; }
        QPlainTextEdit { background: #0d1018; color: #aab4ca; border: 1px solid #262c3d;
                         border-radius: 7px; padding: 8px; font-family: Consolas; font-size: 11px; }
        QProgressBar { background: #242a3a; border: none; border-radius: 5px;
                       min-height: 12px; max-height: 12px; color: transparent; }
        QProgressBar::chunk { background: #e50914; border-radius: 5px; }
        QScrollArea { border: none; background: transparent; }
        QScrollBar:vertical { background: #11141e; width: 10px; margin: 0; }
        QScrollBar::handle:vertical { background: #394158; border-radius: 5px; min-height: 24px; }
        QScrollBar::add-line:vertical, QScrollBar::sub-line:vertical { height: 0; }
        QScrollBar::add-page:vertical, QScrollBar::sub-page:vertical { background: none; }
        QToolTip { background: #252b3b; color: #f2f4f8; border: 1px solid #4b5672; padding: 6px; }
        QSplitter::handle { background: #262c3d; height: 3px; }
    """)
