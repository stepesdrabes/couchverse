-- +goose Up
-- Couch TV's live stream (docs/couch-tv-plan.md 6.6). Once an airing goes on air it
-- records what it airs: feed lists its parts (a program from its package, a break's
-- rendition, the slate), each with its time on air, and origin is the numbering of its
-- first segment (media sequence per rendition, discontinuity sequence), chained from
-- the airing before it. Neither changes afterwards, so every request and every restart
-- lists the same playlists.
ALTER TABLE couchtv_airings
    ADD COLUMN origin jsonb,
    ADD COLUMN feed jsonb;
CREATE INDEX couchtv_airings_ends_idx ON couchtv_airings (ends_at);

-- What couchtv_prepare found airable per media file (a row means it is ready): the
-- video rendition the channel airs and its exact time on air, the video cut at the
-- segment boundary nearest the credits it was measured for (null: the end), which
-- makes the schedule's durations exact.
CREATE TABLE couchtv_media (
    media_file_id uuid PRIMARY KEY REFERENCES media_files (id) ON DELETE CASCADE,
    video text NOT NULL,
    on_air_ms int NOT NULL CHECK (on_air_ms > 0),
    credits_at_ms int,
    checked_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE couchtv_media;
DROP INDEX couchtv_airings_ends_idx;
ALTER TABLE couchtv_airings
    DROP COLUMN feed,
    DROP COLUMN origin;
