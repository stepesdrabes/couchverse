-- +goose Up
CREATE TABLE upload_sessions (
    id uuid PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    filename text NOT NULL,
    declared_size bigint NOT NULL,
    received_bytes bigint NOT NULL DEFAULT 0,
    temp_path text NOT NULL,
    status text NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'complete', 'aborted')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);

CREATE TABLE home_rows (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    position int NOT NULL,
    kind text NOT NULL
        CHECK (kind IN ('continue_watching', 'recently_added', 'genre', 'recently_played_music')),
    genre_id bigint REFERENCES genres (id) ON DELETE CASCADE,
    label text NOT NULL DEFAULT '',
    enabled boolean NOT NULL DEFAULT true
);

INSERT INTO home_rows (position, kind, label) VALUES
    (1, 'continue_watching', 'Continue Watching'),
    (2, 'recently_added', 'Up on the Marquee'),
    (3, 'recently_played_music', 'Recently Played');

-- +goose Down
DROP TABLE home_rows;
DROP TABLE upload_sessions;
