-- +goose Up
CREATE TABLE artists (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    sort_name text NOT NULL DEFAULT '',
    musicbrainz_id text,
    added_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (name)
);
CREATE INDEX artists_name_trgm_idx ON artists USING gin (name gin_trgm_ops);

CREATE TABLE albums (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    artist_id uuid NOT NULL REFERENCES artists (id) ON DELETE CASCADE,
    name text NOT NULL,
    year int,
    status text NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'processing', 'published', 'hidden')),
    musicbrainz_id text,
    added_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (artist_id, name)
);
CREATE INDEX albums_name_trgm_idx ON albums USING gin (name gin_trgm_ops);

CREATE TABLE tracks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    album_id uuid NOT NULL REFERENCES albums (id) ON DELETE CASCADE,
    disc_number int NOT NULL DEFAULT 1,
    track_number int NOT NULL DEFAULT 0,
    name text NOT NULL,
    duration_seconds int NOT NULL DEFAULT 0,
    track_artist text,
    UNIQUE (album_id, disc_number, track_number, name)
);
CREATE INDEX tracks_name_trgm_idx ON tracks USING gin (name gin_trgm_ops);

CREATE TABLE album_genres (
    album_id uuid NOT NULL REFERENCES albums (id) ON DELETE CASCADE,
    genre_id bigint NOT NULL REFERENCES genres (id) ON DELETE CASCADE,
    PRIMARY KEY (album_id, genre_id)
);

-- +goose Down
DROP TABLE album_genres;
DROP TABLE tracks;
DROP TABLE albums;
DROP TABLE artists;
