-- +goose Up
CREATE TABLE libraries (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('movies', 'series', 'music')),
    path text NOT NULL UNIQUE,
    managed boolean NOT NULL DEFAULT false,
    last_scanned_at timestamptz
);

CREATE TABLE media_files (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    library_id bigint NOT NULL REFERENCES libraries (id) ON DELETE CASCADE,
    title_id bigint REFERENCES titles (id) ON DELETE CASCADE,
    episode_id bigint REFERENCES episodes (id) ON DELETE CASCADE,
    track_id bigint REFERENCES tracks (id) ON DELETE CASCADE,
    path text NOT NULL,
    size_bytes bigint NOT NULL DEFAULT 0,
    container text NOT NULL DEFAULT '',
    video_codec text NOT NULL DEFAULT '',
    audio_codec text NOT NULL DEFAULT '',
    width int NOT NULL DEFAULT 0,
    height int NOT NULL DEFAULT 0,
    duration_seconds numeric NOT NULL DEFAULT 0,
    bitrate bigint NOT NULL DEFAULT 0,
    channels int NOT NULL DEFAULT 0,
    sample_rate int NOT NULL DEFAULT 0,
    video_range text NOT NULL DEFAULT 'sdr'
        CHECK (video_range IN ('sdr', 'hdr10', 'hlg', 'dv')),
    direct_play boolean NOT NULL DEFAULT false,
    probe jsonb,
    file_mtime timestamptz,
    scanned_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (library_id, path),
    CHECK (num_nonnulls(title_id, episode_id, track_id) = 1)
);
CREATE INDEX media_files_title_idx ON media_files (title_id);
CREATE INDEX media_files_episode_idx ON media_files (episode_id);
CREATE INDEX media_files_track_idx ON media_files (track_id);

CREATE TABLE subtitles (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    media_file_id bigint NOT NULL REFERENCES media_files (id) ON DELETE CASCADE,
    lang text NOT NULL DEFAULT 'und',
    label text NOT NULL DEFAULT '',
    source text NOT NULL CHECK (source IN ('embedded', 'uploaded')),
    forced boolean NOT NULL DEFAULT false,
    path text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX subtitles_media_file_idx ON subtitles (media_file_id);

CREATE TABLE artwork (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    owner_kind text NOT NULL CHECK (owner_kind IN ('title', 'season', 'episode', 'artist', 'album')),
    owner_id bigint NOT NULL,
    kind text NOT NULL CHECK (kind IN ('poster', 'backdrop', 'thumb', 'album_cover', 'artist_photo')),
    path text NOT NULL,
    width int NOT NULL DEFAULT 0,
    height int NOT NULL DEFAULT 0,
    source text NOT NULL DEFAULT 'uploaded' CHECK (source IN ('tmdb', 'uploaded', 'embedded')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (owner_kind, owner_id, kind)
);

CREATE TABLE transcode_variants (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    media_file_id bigint NOT NULL REFERENCES media_files (id) ON DELETE CASCADE,
    name text NOT NULL,
    width int NOT NULL DEFAULT 0,
    height int NOT NULL DEFAULT 0,
    video_bitrate bigint NOT NULL DEFAULT 0,
    audio_bitrate bigint NOT NULL DEFAULT 0,
    mode text NOT NULL CHECK (mode IN ('copy', 'transcode')),
    status text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'processing', 'ready', 'failed')),
    playlist_path text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    UNIQUE (media_file_id, name)
);

-- +goose Down
DROP TABLE transcode_variants;
DROP TABLE artwork;
DROP TABLE subtitles;
DROP TABLE media_files;
DROP TABLE libraries;
