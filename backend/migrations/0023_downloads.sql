-- +goose Up
-- A download is a device-ready MP4 made by the prepare_download job. Requests that
-- would make the same MP4 (same media file, same plan) share one download_files row;
-- every user who asked for it has a downloads row. The hourly cleanup deletes files
-- nobody asked for anymore and ready ones past their retention.
CREATE TABLE download_files (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    media_file_id uuid NOT NULL REFERENCES media_files (id) ON DELETE CASCADE,
    spec text NOT NULL,
    plan jsonb NOT NULL,
    status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'preparing', 'ready', 'failed')),
    progress int NOT NULL DEFAULT 0,
    error text NOT NULL DEFAULT '',
    size_bytes bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    ready_at timestamptz,
    requested_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (media_file_id, spec)
);

CREATE TABLE downloads (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    file_id uuid NOT NULL REFERENCES download_files (id) ON DELETE CASCADE,
    title_id uuid NOT NULL REFERENCES titles (id) ON DELETE CASCADE,
    episode_id uuid REFERENCES episodes (id) ON DELETE CASCADE,
    quality text NOT NULL CHECK (quality IN ('original', '1080p', '720p', '480p')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, file_id)
);

CREATE INDEX downloads_user_idx ON downloads (user_id, created_at DESC);
CREATE INDEX downloads_file_idx ON downloads (file_id);

-- +goose Down
DROP TABLE downloads;
DROP TABLE download_files;
