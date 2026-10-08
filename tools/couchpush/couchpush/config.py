"""Persistent settings for Couchpush.

Non-secret settings live in a JSON file under %APPDATA%\\couchpush. The Couchverse
session cookie is deliberately kept out of that file and stored in the OS keyring
(Windows Credential Manager) so no credential is ever written to disk in plaintext.
If keyring is unavailable the cookie simply is not persisted and the user re-logs in.
"""

from __future__ import annotations

import json
import os
import tempfile
from dataclasses import asdict, dataclass, field, fields
from pathlib import Path

APP_NAME = "couchpush"
SESSION_COOKIE_NAME = "couchverse_session"

_KEYRING_SERVICE = "couchpush"
_KEYRING_COOKIE_USER = "session"
_KEYRING_PASSWORD_USER = "password"

# Handles both "s23e13"/"S23E13" and "35x07" naming, with named groups. The \b
# anchors keep it from matching quality tags like "1920x1080".
DEFAULT_EPISODE_REGEX = r"\b[Ss]?(?P<season>\d{1,2})[\s._-]*[XxEe](?P<episode>\d{1,3})\b"

QUALITY_PRESETS = ("max", "high", "small", "tiny")


def config_dir() -> Path:
    base = os.environ.get("APPDATA") or str(Path.home())
    d = Path(base) / APP_NAME
    d.mkdir(parents=True, exist_ok=True)
    return d


def config_path() -> Path:
    return config_dir() / "config.json"


def default_temp_dir() -> str:
    return str(Path(tempfile.gettempdir()) / APP_NAME)


@dataclass
class Config:
    server_url: str = ""
    username: str = ""
    ffmpeg_path: str = "ffmpeg"
    ffprobe_path: str = "ffprobe"
    temp_dir: str = field(default_factory=default_temp_dir)
    episode_regex: str = DEFAULT_EPISODE_REGEX
    last_series_id: str = ""
    audio_lang: str = "en"
    audio_mode: str = "multiple"
    quality_preset: str = "high"
    encoder: str = "nvenc"
    keep_channels: bool = False
    lookahead: int = 3
    library_kind: str = "movies"
    source_folder: str = ""
    max_height: int = 1080

    @classmethod
    def load(cls) -> "Config":
        p = config_path()
        if not p.exists():
            return cls()
        try:
            data = json.loads(p.read_text("utf-8"))
        except (OSError, ValueError):
            return cls()
        known = {f.name for f in fields(cls)}
        return cls(**{k: v for k, v in data.items() if k in known})

    def save(self) -> None:
        config_path().write_text(json.dumps(asdict(self), indent=2), "utf-8")


def save_session_cookie(value: str, expires_at: float | None) -> None:
    """Persist the session cookie to the OS keyring. No-op if keyring is missing."""
    try:
        import keyring

        keyring.set_password(
            _KEYRING_SERVICE,
            _KEYRING_COOKIE_USER,
            json.dumps({"value": value, "expiresAt": expires_at}),
        )
    except Exception:
        pass


def load_session_cookie() -> tuple[str, float | None] | None:
    """Return (cookie_value, expires_at) from the keyring, or None if absent."""
    try:
        import keyring

        raw = keyring.get_password(_KEYRING_SERVICE, _KEYRING_COOKIE_USER)
    except Exception:
        return None
    if not raw:
        return None
    try:
        d = json.loads(raw)
    except ValueError:
        return None
    return d.get("value", ""), d.get("expiresAt")


def clear_session_cookie() -> None:
    try:
        import keyring

        keyring.delete_password(_KEYRING_SERVICE, _KEYRING_COOKIE_USER)
    except Exception:
        pass


def save_password(password: str) -> None:
    """Store the admin password in the OS keyring (not in the JSON config)."""
    try:
        import keyring

        keyring.set_password(_KEYRING_SERVICE, _KEYRING_PASSWORD_USER, password)
    except Exception:
        pass


def load_password() -> str:
    try:
        import keyring

        return keyring.get_password(_KEYRING_SERVICE, _KEYRING_PASSWORD_USER) or ""
    except Exception:
        return ""


def clear_password() -> None:
    try:
        import keyring

        keyring.delete_password(_KEYRING_SERVICE, _KEYRING_PASSWORD_USER)
    except Exception:
        pass
