"""Map parsed files to Couchverse episodes and decide per-row defaults.

For each parsed (season, episode): find the existing episode (or mark it for
creation), inspect the episode's existing live media files to choose the audio
role (primary vs alternate) and a default action (skip if the batch language is
already present). Every default is user-overridable in the table."""

from __future__ import annotations

import unicodedata
from dataclasses import dataclass, field
from difflib import SequenceMatcher

from ..api.models import Title, TitleDetail
from ..media.plan import normalize_lang
from .parser import MovieFile, ParsedFile

ACTION_UPLOAD = "upload"
ACTION_SKIP = "skip"
ACTION_REPLACE = "replace"

ROLE_PRIMARY = "primary"
ROLE_ALT = "audio_alt"


@dataclass
class MatchRow:
    parsed: ParsedFile
    season: int
    episode: int
    episode_id: str | None      # None until created at upload time
    episode_name: str
    exists: bool
    existing_langs: list[str]
    has_same_lang: bool
    audio_role: str
    action: str
    note: str = ""
    selected: bool = True
    replace_ids: list[str] = field(default_factory=list)  # same-lang files to delete on REPLACE

    @property
    def will_create(self) -> bool:
        return not self.exists


def build_match_rows(detail: TitleDetail, parsed_files: list[ParsedFile],
                     batch_lang: str) -> list[MatchRow]:
    want = normalize_lang(batch_lang)
    rows: list[MatchRow] = []
    seen: dict[tuple[int, int], str] = {}

    for pf in parsed_files:
        if not pf.ok:
            rows.append(MatchRow(
                parsed=pf, season=pf.season or 0, episode=pf.episode or 0,
                episode_id=None, episode_name="", exists=False, existing_langs=[],
                has_same_lang=False, audio_role=ROLE_PRIMARY, action=ACTION_SKIP,
                note="could not parse season/episode", selected=False))
            continue

        ep = detail.find_episode(pf.season, pf.episode)
        if ep is not None:
            files = detail.live_files_for_episode(ep.id)
            langs = sorted({normalize_lang(f.audio_lang) for f in files if f.audio_lang})
            untagged = any(not f.audio_lang for f in files)
            has_same = (want in langs) or (want == "" and untagged)
            same_lang_ids = [f.id for f in files
                             if normalize_lang(f.audio_lang) == want or (want == "" and not f.audio_lang)]
            if has_same:
                role, action, note = ROLE_PRIMARY, ACTION_SKIP, "already has this language"
            elif files:
                role, action, note = ROLE_ALT, ACTION_UPLOAD, "adds alternate audio"
            else:
                role, action, note = ROLE_PRIMARY, ACTION_UPLOAD, ""
            row = MatchRow(
                parsed=pf, season=pf.season, episode=pf.episode, episode_id=ep.id,
                episode_name=ep.name, exists=True, existing_langs=langs,
                has_same_lang=has_same, audio_role=role, action=action, note=note,
                selected=(action == ACTION_UPLOAD), replace_ids=same_lang_ids)
        else:
            row = MatchRow(
                parsed=pf, season=pf.season, episode=pf.episode, episode_id=None,
                episode_name="", exists=False, existing_langs=[], has_same_lang=False,
                audio_role=ROLE_PRIMARY, action=ACTION_UPLOAD,
                note="will create episode", selected=True)

        key = (pf.season, pf.episode)
        if key in seen:
            row.note = (row.note + "; " if row.note else "") + "duplicate mapping"
            row.selected = False
        else:
            seen[key] = pf.rel_path
        rows.append(row)

    return rows


# ---- movies ----


@dataclass
class MovieRow:
    movie: MovieFile
    title_id: str | None        # matched existing movie title; None means create new
    target_name: str            # display label for the target movie
    exists: bool
    has_media: bool             # matched title already has a file -> default skip / offer replace
    audio_role: str
    action: str
    note: str = ""
    selected: bool = True
    target_title: str = ""
    target_year: int | None = None
    needs_review: bool = False
    duplicate_of: int | None = None

    @property
    def output_base(self) -> str:
        return self.target_name


def _norm_title(s: str) -> str:
    s = unicodedata.normalize("NFKD", s.casefold())
    s = "".join(c for c in s if not unicodedata.combining(c))
    return " ".join("".join(c if c.isalnum() else " " for c in s).split())


def _movie_label(name: str, year: int | None) -> str:
    return f"{name} ({year})" if year is not None else name


def build_movie_row(movie: MovieFile, target: Title | None = None, *,
                    new_name: str | None = None, new_year: int | None = None) -> MovieRow:
    """Assign a file to an explicit catalog target, or to a new movie.

    Supplying new_name also makes new_year explicit, including None to omit it.
    The source MovieFile stays unchanged so the filename remains visible in the UI.
    """
    if target is not None:
        if target.kind != "movie":
            raise ValueError("Choose a movie title as the upload target.")
        return MovieRow(
            movie=movie, title_id=target.id,
            target_name=_movie_label(target.name, target.year), exists=True,
            has_media=target.has_media, audio_role=ROLE_PRIMARY,
            action=ACTION_SKIP if target.has_media else ACTION_UPLOAD,
            note="already has a file (check to replace)" if target.has_media else "",
            selected=not target.has_media, target_title=target.name, target_year=target.year)

    name = (movie.name if new_name is None else new_name).strip()
    year = movie.year if new_name is None else new_year
    if not name:
        return MovieRow(
            movie=movie, title_id=None, target_name="", exists=False,
            has_media=False, audio_role=ROLE_PRIMARY, action=ACTION_SKIP,
            note="could not parse movie name; choose a target", selected=False,
            needs_review=True)
    return MovieRow(
        movie=movie, title_id=None, target_name=_movie_label(name, year), exists=False,
        has_media=False, audio_role=ROLE_PRIMARY, action=ACTION_UPLOAD,
        note="will create movie", target_title=name, target_year=year)


def movie_target_key(row: MovieRow) -> tuple:
    """Identity of the upload target, independent of the source's parsed year."""
    if row.title_id:
        return ("existing", row.title_id)
    return ("new", _norm_title(row.target_title or row.movie.name), row.target_year)


def movie_selection_errors(rows: list[MovieRow]) -> list[str]:
    """Validate selected rows before starting a batch, including manual assignments."""
    errors: list[str] = []
    seen: dict[tuple, MovieRow] = {}
    for row in rows:
        if not row.selected:
            continue
        if row.needs_review or not row.target_name:
            errors.append(f"{row.movie.rel_path}: choose a movie target first.")
            continue
        key = movie_target_key(row)
        if key in seen:
            errors.append(
                f"{row.movie.rel_path} and {seen[key].movie.rel_path}: "
                f"both target {row.target_name}. Select only one file for this movie.")
        else:
            seen[key] = row
    return errors


def suggest_movies(movie: MovieFile, movies: list[Title], limit: int = 12) -> list[Title]:
    """Rank selector suggestions by name similarity and release year.

    Fuzzy suggestions are for review only, never automatic upload assignments.
    """
    name = _norm_title(movie.name)

    def rank(title: Title) -> tuple:
        other = _norm_title(title.name)
        score = SequenceMatcher(None, name, other).ratio() if name else 0.0
        return (other == name, score, movie.year is not None and title.year == movie.year,
                -(abs(movie.year - title.year) if movie.year and title.year else 9999))

    return sorted((t for t in movies if t.kind == "movie"), key=rank, reverse=True)[:max(0, limit)]


def build_movie_rows(movies: list[Title], files: list[MovieFile]) -> list[MovieRow]:
    by_norm: dict[str, list[Title]] = {}
    for t in movies:
        if t.kind == "movie":
            by_norm.setdefault(_norm_title(t.name), []).append(t)

    rows: list[MovieRow] = []
    seen: dict[tuple, int] = {}
    for f in files:
        cands = by_norm.get(_norm_title(f.name), [])
        match: Title | None = None
        review = ""
        if cands:
            if f.year is not None:
                same_year = [t for t in cands if t.year == f.year]
                if len(same_year) == 1:
                    match = same_year[0]
                elif same_year:
                    review = "multiple movies match this name and year; choose a target"
                elif any(t.year is None for t in cands):
                    review = "catalog movie has no release year; choose a target"
            else:
                if len(cands) == 1:
                    match = cands[0]
                else:
                    review = "multiple releases share this name; choose a target"

        row = build_movie_row(f, match)
        if review:
            row.needs_review = True
            row.selected = False
            row.action = ACTION_SKIP
            row.note = review
        elif cands and match is None and f.ok:
            row.note = "different release year; will create movie"

        if not row.needs_review:
            key = movie_target_key(row)
            if key in seen:
                row.duplicate_of = seen[key]
                row.note = (row.note + "; " if row.note else "") + "duplicate target (select only one)"
                row.selected = False
            else:
                seen[key] = len(rows)
        rows.append(row)

    return rows
