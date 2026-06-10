-- +goose Up
-- Daily watch/listen time rollup, upserted on every progress beacon and
-- scrobble. title_id is NULL for music (one slot per user/day thanks to
-- NULLS NOT DISTINCT).
CREATE TABLE watch_time_daily (
    day date NOT NULL,
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title_id uuid REFERENCES titles (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('video', 'music')),
    seconds bigint NOT NULL DEFAULT 0,
    UNIQUE NULLS NOT DISTINCT (day, user_id, kind, title_id)
);

CREATE INDEX watch_time_daily_title_idx ON watch_time_daily (title_id);

-- +goose Down
DROP TABLE watch_time_daily;
