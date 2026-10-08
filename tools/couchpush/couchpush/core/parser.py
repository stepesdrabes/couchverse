"""Scan a folder of episodes and parse season/episode numbers with a user regex.

The regex is matched against each file's path relative to the chosen root (with
'\\' normalized to '/'), so a pattern can pull the season from the folder and the
episode from the filename. Named groups `season` and `episode` are preferred;
two unnamed groups are accepted as (season, episode)."""

from __future__ import annotations

import os
import re
from dataclasses import dataclass
from datetime import date

VIDEO_EXTS = {".mkv", ".mp4", ".m4v", ".avi", ".mov", ".ts", ".m2ts", ".webm", ".wmv", ".mpg", ".mpeg"}


class RegexError(ValueError):
    pass


@dataclass
class ParsedFile:
    path: str          # absolute path
    rel_path: str      # path relative to the scanned root (forward slashes)
    season: int | None
    episode: int | None
    error: str = ""

    @property
    def ok(self) -> bool:
        return self.season is not None and self.episode is not None


def compile_regex(pattern: str) -> re.Pattern:
    try:
        return re.compile(pattern, re.IGNORECASE)
    except re.error as e:
        raise RegexError(f"invalid regex: {e}") from e


def parse_name(rx: re.Pattern, rel_path: str) -> tuple[int | None, int | None]:
    s = rel_path.replace("\\", "/")
    m = rx.search(s)
    if not m:
        return None, None
    gd = m.groupdict()
    season = gd.get("season")
    episode = gd.get("episode")
    if season is None and episode is None:
        groups = m.groups()
        if len(groups) >= 2:
            season, episode = groups[0], groups[1]
    return _to_int(season), _to_int(episode)


def scan_folder(root: str, pattern: str) -> list[ParsedFile]:
    rx = compile_regex(pattern)
    out: list[ParsedFile] = []
    for dirpath, _dirnames, filenames in os.walk(root):
        for fn in sorted(filenames):
            if os.path.splitext(fn)[1].lower() not in VIDEO_EXTS:
                continue
            full = os.path.join(dirpath, fn)
            rel = os.path.relpath(full, root).replace("\\", "/")
            season, episode = parse_name(rx, rel)
            err = "" if (season is not None and episode is not None) else "no season/episode match"
            out.append(ParsedFile(path=full, rel_path=rel, season=season, episode=episode, error=err))
    out.sort(key=lambda p: (p.season if p.season is not None else 9999,
                            p.episode if p.episode is not None else 9999,
                            p.rel_path))
    return out


def _to_int(v) -> int | None:
    if v is None:
        return None
    try:
        return int(v)
    except (TypeError, ValueError):
        return None


# ---- movies ----

_YEAR_RE = re.compile(r"^(.*?)[\s._-]*[\[(](\d{4})[\])]")
_JUNK_RE = re.compile(
    r"(?i)(?:^|[\s._\[(-]+)(1080[pi]|2160p|720p|480p|4k|uhd(?:rdv)?|blu[\s._-]?ray|"
    r"web[\s._-]?dl|webrip|hdtv|x264|x265|h\.?264|h\.?265|hevc|aac|ac3|dts|remux|"
    r"hdr(?:10)?\+?|dv|repack|proper)(?=$|[\s._\])+-]).*$"
)
_GENERIC_MOVIE_NAMES = {"movie", "video", "feature", "main"}


@dataclass
class MovieFile:
    path: str
    rel_path: str
    name: str
    year: int | None

    @property
    def ok(self) -> bool:
        return bool(self.name)


def _clean_name(s: str) -> str:
    s = _JUNK_RE.sub("", s)
    s = s.replace(".", " ").replace("_", " ").strip(" -")
    return " ".join(s.split())


def _split_name_year(s: str) -> tuple[str, int | None]:
    m = _YEAR_RE.match(s)
    if m:
        try:
            return _clean_name(m.group(1)), int(m.group(2))
        except ValueError:
            pass
    # Strip release tags first so numbers in those tags cannot become the year.
    s = _clean_name(s)
    bare = [m for m in re.finditer(r"\b(18\d{2}|19\d{2}|20\d{2})\b", s)
            if 1888 <= int(m.group(1)) <= date.today().year + 1]
    if bare:
        before = _clean_name(s[: bare[-1].start()])
        if before:
            return before, int(bare[-1].group(1))
    return _clean_name(s), None


def parse_movie(rel_path: str) -> tuple[str, int | None]:
    """Best-effort movie name + year from a path: "The Matrix (1999) 1080p.mkv" or a
    "Movie (2024)/movie.mkv" folder."""
    rel = rel_path.replace("\\", "/")
    base = os.path.splitext(rel.rsplit("/", 1)[-1])[0]
    name, year = _split_name_year(base)
    if year is None and "/" in rel:
        # The selected root may contain several levels above the actual movie folder.
        folders = rel.split("/")[:-1]
        for folder in reversed(folders):
            folder_name, folder_year = _split_name_year(folder)
            if folder_name and folder_year is not None:
                name, year = folder_name, folder_year
                break
        else:
            if (not name or name.casefold() in _GENERIC_MOVIE_NAMES) and folders:
                folder_name, _ = _split_name_year(folders[-1])
                if folder_name:
                    name = folder_name
    return name, year


def scan_movies(root: str) -> list[MovieFile]:
    out: list[MovieFile] = []
    for dirpath, _dirnames, filenames in os.walk(root):
        for fn in sorted(filenames):
            if os.path.splitext(fn)[1].lower() not in VIDEO_EXTS:
                continue
            full = os.path.join(dirpath, fn)
            rel = os.path.relpath(full, root).replace("\\", "/")
            name, year = parse_movie(rel)
            out.append(MovieFile(path=full, rel_path=rel, name=name, year=year))
    out.sort(key=lambda m: m.rel_path)
    return out
