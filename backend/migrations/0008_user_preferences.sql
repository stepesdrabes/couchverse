-- +goose Up
-- generic per-user settings blob (subtitle styling today, extensible later)
ALTER TABLE users ADD COLUMN preferences jsonb NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE users DROP COLUMN preferences;
