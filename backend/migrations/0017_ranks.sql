-- +goose Up
-- Unlocked achievements. The rules are re-derivable from activity at any time,
-- but the unlock moment is not: it drives "unlocked 3 days ago" and the
-- first-unlock celebration.
CREATE TABLE user_achievements (
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    code text NOT NULL,
    unlocked_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, code)
);

-- Counters for activity that leaves no other trace: couch sessions live only in
-- the hub's memory and couch_watch_time_daily has no user dimension, so hosting,
-- joining, party size and emoji have nowhere else to go. Keys are Go constants
-- and every value is a monotonic bigint with one producer and one consumer, so a
-- generic table beats a column per counter and a new counter needs no migration.
CREATE TABLE user_counters (
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    key text NOT NULL,
    value bigint NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, key)
);

-- Hour-of-day rollup behind the "when you watch" clock and the night-owl /
-- early-bird achievements. Bucketed in the app's local zone, not UTC, so "night"
-- means night. Written from inside analytics.RecordWatch/RecordListen, so no
-- caller changes and at most 48 rows per user per day.
CREATE TABLE watch_time_hourly (
    day date NOT NULL,
    hour smallint NOT NULL CHECK (hour BETWEEN 0 AND 23),
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('video', 'music')),
    seconds bigint NOT NULL DEFAULT 0,
    UNIQUE (day, hour, user_id, kind)
);

CREATE INDEX watch_time_hourly_user_idx ON watch_time_hourly (user_id, day);

-- The daily rollup is now scanned per user (profiles) as well as per day (admin
-- charts); its unique index leads with day, so add the user-leading one.
CREATE INDEX watch_time_daily_user_day_idx ON watch_time_daily (user_id, day);

-- +goose Down
DROP INDEX watch_time_daily_user_day_idx;
DROP TABLE watch_time_hourly;
DROP TABLE user_counters;
DROP TABLE user_achievements;
