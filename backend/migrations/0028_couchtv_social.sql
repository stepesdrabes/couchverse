-- +goose Up
-- Couch TV's guests and its watch time (docs/couch-tv-plan.md D12, D23, 6.9).
-- A guest joined through the channel link with a name: their cookie (or header) carries
-- a random token, stored hashed. link_hash is the link they joined through, so a rotated
-- link ends them; turning the link off or rotating it deletes every guest anyway.
CREATE TABLE couchtv_guests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash text NOT NULL UNIQUE,
    name text NOT NULL,
    link_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    seen_at timestamptz NOT NULL DEFAULT now()
);

-- Seconds watched on Couch TV, the channel's own bucket beside watch_time_daily and
-- couch_watch_time_daily: user_id is null for guests, title_id for breaks and fillers.
CREATE TABLE tv_watch_time_daily (
    day date NOT NULL,
    user_id bigint REFERENCES users (id) ON DELETE CASCADE,
    title_id uuid REFERENCES titles (id) ON DELETE CASCADE,
    seconds bigint NOT NULL DEFAULT 0,
    UNIQUE NULLS NOT DISTINCT (day, user_id, title_id)
);

-- +goose Down
DROP TABLE tv_watch_time_daily;
DROP TABLE couchtv_guests;
