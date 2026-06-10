-- +goose Up
CREATE TABLE watch_progress (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title_id uuid REFERENCES titles (id) ON DELETE CASCADE,
    episode_id uuid REFERENCES episodes (id) ON DELETE CASCADE,
    position_seconds int NOT NULL DEFAULT 0,
    duration_seconds int NOT NULL DEFAULT 0,
    completed boolean NOT NULL DEFAULT false,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (num_nonnulls(title_id, episode_id) = 1)
);
CREATE UNIQUE INDEX watch_progress_title_uq ON watch_progress (user_id, title_id) WHERE title_id IS NOT NULL;
CREATE UNIQUE INDEX watch_progress_episode_uq ON watch_progress (user_id, episode_id) WHERE episode_id IS NOT NULL;
CREATE INDEX watch_progress_user_updated_idx ON watch_progress (user_id, updated_at DESC);

CREATE TABLE watchlist (
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title_id uuid NOT NULL REFERENCES titles (id) ON DELETE CASCADE,
    added_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, title_id)
);

CREATE TABLE play_history (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title_id uuid REFERENCES titles (id) ON DELETE CASCADE,
    episode_id uuid REFERENCES episodes (id) ON DELETE CASCADE,
    track_id uuid REFERENCES tracks (id) ON DELETE CASCADE,
    started_at timestamptz NOT NULL DEFAULT now(),
    completed boolean NOT NULL DEFAULT false,
    CHECK (num_nonnulls(title_id, episode_id, track_id) = 1)
);
CREATE INDEX play_history_user_idx ON play_history (user_id, started_at DESC);

CREATE TABLE playlists (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE playlist_tracks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    playlist_id uuid NOT NULL REFERENCES playlists (id) ON DELETE CASCADE,
    track_id uuid NOT NULL REFERENCES tracks (id) ON DELETE CASCADE,
    position int NOT NULL,
    added_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX playlist_tracks_order_idx ON playlist_tracks (playlist_id, position);

-- +goose Down
DROP TABLE playlist_tracks;
DROP TABLE playlists;
DROP TABLE play_history;
DROP TABLE watchlist;
DROP TABLE watch_progress;
