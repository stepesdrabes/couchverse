"""Typed views over the Couchverse admin JSON the tool consumes.

Field names mirror the live API responses (camelCase in JSON, snake_case here).
Parsing is defensive: missing keys fall back to sensible empties rather than
raising, so a server tweak degrades gracefully instead of crashing a batch.
"""

from __future__ import annotations

from dataclasses import dataclass


@dataclass
class Title:
    """A catalog title row from /admin/library - a movie or a series."""

    id: str
    name: str
    year: int | None
    kind: str
    season_count: int
    episode_count: int
    status: str
    size_bytes: int
    max_height: int

    @classmethod
    def from_json(cls, d: dict) -> "Title":
        return cls(
            id=d.get("id", ""),
            name=d.get("name", ""),
            year=d.get("year"),
            kind=d.get("kind", ""),
            season_count=d.get("seasonCount", 0),
            episode_count=d.get("episodeCount", 0),
            status=d.get("status", ""),
            size_bytes=d.get("sizeBytes", 0),
            max_height=d.get("maxHeight", 0),
        )

    @property
    def has_media(self) -> bool:
        # the library row only carries rolled-up stats, but either being non-zero
        # means at least one media file is attached (and probed)
        return self.size_bytes > 0 or self.max_height > 0


@dataclass
class Episode:
    id: str
    episode_number: int
    name: str
    season_id: str

    @classmethod
    def from_json(cls, d: dict) -> "Episode":
        return cls(
            id=d.get("id", ""),
            episode_number=d.get("episodeNumber", 0),
            name=d.get("name", ""),
            season_id=d.get("seasonId", ""),
        )


@dataclass
class Season:
    id: str
    season_number: int
    name: str
    episodes: list[Episode]

    @classmethod
    def from_json(cls, d: dict) -> "Season":
        return cls(
            id=d.get("id", ""),
            season_number=d.get("seasonNumber", 0),
            name=d.get("name", ""),
            episodes=[Episode.from_json(e) for e in (d.get("episodes") or [])],
        )


@dataclass
class EmbeddedAudioStream:
    index: int = 0
    codec: str = ""
    lang: str = ""
    title: str = ""
    channels: int = 0
    default: bool = False

    @classmethod
    def from_json(cls, d: dict) -> "EmbeddedAudioStream":
        return cls(
            index=d.get("index", 0),
            codec=d.get("codec", ""),
            lang=d.get("lang", "") or "",
            title=d.get("title", "") or "",
            channels=d.get("channels", 0),
            default=bool(d.get("default", False)),
        )


@dataclass
class MediaFile:
    id: str
    episode_id: str | None
    video_codec: str
    audio_codec: str
    audio_lang: str
    audio_role: str
    direct_play: bool
    height: int
    source_deleted_at: str | None
    audio_streams: list[EmbeddedAudioStream] | None = None

    @classmethod
    def from_json(cls, d: dict) -> "MediaFile":
        streams = d.get("audioStreams")
        return cls(
            id=d.get("id", ""),
            episode_id=d.get("episodeId"),
            video_codec=d.get("videoCodec", ""),
            audio_codec=d.get("audioCodec", ""),
            audio_lang=d.get("audioLang", "") or "",
            audio_role=d.get("audioRole", "") or "",
            direct_play=bool(d.get("directPlay", False)),
            height=d.get("height", 0),
            source_deleted_at=d.get("sourceDeletedAt"),
            audio_streams=([EmbeddedAudioStream.from_json(a) for a in streams]
                           if isinstance(streams, list) and all(isinstance(a, dict) for a in streams)
                           else None),
        )

    @property
    def is_live(self) -> bool:
        return not self.source_deleted_at


@dataclass
class TitleDetail:
    id: str
    name: str
    kind: str
    metadata_languages: list[str]
    seasons: list[Season]
    media_files: list[MediaFile]

    @classmethod
    def from_json(cls, d: dict) -> "TitleDetail":
        title = d.get("title") or {}
        streams_by_file = d.get("audioStreamsByFile")
        files = []
        for entry in d.get("mediaFiles") or []:
            if isinstance(streams_by_file, dict) and entry.get("id") in streams_by_file:
                entry = {**entry, "audioStreams": streams_by_file[entry["id"]]}
            files.append(MediaFile.from_json(entry))
        return cls(
            id=title.get("id", ""),
            name=title.get("name", ""),
            kind=title.get("kind", ""),
            metadata_languages=title.get("metadataLanguages") or [],
            seasons=[Season.from_json(s) for s in (d.get("seasons") or [])],
            media_files=files,
        )

    def find_episode(self, season_number: int, episode_number: int) -> Episode | None:
        for s in self.seasons:
            if s.season_number == season_number:
                for e in s.episodes:
                    if e.episode_number == episode_number:
                        return e
        return None

    def find_season(self, season_number: int) -> Season | None:
        for s in self.seasons:
            if s.season_number == season_number:
                return s
        return None

    def live_files_for_episode(self, episode_id: str) -> list[MediaFile]:
        return [m for m in self.media_files if m.episode_id == episode_id and m.is_live]
