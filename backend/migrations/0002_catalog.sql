-- +goose Up
CREATE TABLE titles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind text NOT NULL CHECK (kind IN ('movie', 'series')),
    name text NOT NULL,
    slug text NOT NULL UNIQUE,
    sort_name text NOT NULL DEFAULT '',
    overview text NOT NULL DEFAULT '',
    year int,
    release_date date,
    content_rating text NOT NULL DEFAULT '',
    runtime_minutes int,
    status text NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'processing', 'published', 'hidden')),
    tmdb_id int,
    added_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX titles_name_trgm_idx ON titles USING gin (name gin_trgm_ops);
CREATE INDEX titles_kind_status_idx ON titles (kind, status);
CREATE INDEX titles_added_idx ON titles (added_at DESC);

CREATE TABLE seasons (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title_id uuid NOT NULL REFERENCES titles (id) ON DELETE CASCADE,
    season_number int NOT NULL,
    name text NOT NULL DEFAULT '',
    overview text NOT NULL DEFAULT '',
    UNIQUE (title_id, season_number)
);

CREATE TABLE episodes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    season_id uuid NOT NULL REFERENCES seasons (id) ON DELETE CASCADE,
    episode_number int NOT NULL,
    name text NOT NULL DEFAULT '',
    overview text NOT NULL DEFAULT '',
    air_date date,
    runtime_minutes int,
    UNIQUE (season_id, episode_number)
);

CREATE TABLE genres (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name text NOT NULL UNIQUE
);

CREATE TABLE title_genres (
    title_id uuid NOT NULL REFERENCES titles (id) ON DELETE CASCADE,
    genre_id bigint NOT NULL REFERENCES genres (id) ON DELETE CASCADE,
    PRIMARY KEY (title_id, genre_id)
);

-- +goose Down
DROP TABLE title_genres;
DROP TABLE genres;
DROP TABLE episodes;
DROP TABLE seasons;
DROP TABLE titles;
