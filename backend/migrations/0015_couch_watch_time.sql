-- +goose Up
-- A separate "On Couch watch-time" stat: the aggregate seconds followers spend
-- watching on a couch, scoped per title and kept apart from normal watch-time.
-- No user dimension (anonymous followers count too); the couch hub flushes here.
CREATE TABLE couch_watch_time_daily (
    day date NOT NULL,
    title_id uuid NOT NULL REFERENCES titles (id) ON DELETE CASCADE,
    seconds bigint NOT NULL DEFAULT 0,
    UNIQUE (day, title_id)
);

CREATE INDEX couch_watch_time_daily_title_idx ON couch_watch_time_daily (title_id);

-- +goose Down
DROP TABLE couch_watch_time_daily;
