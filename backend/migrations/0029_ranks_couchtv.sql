-- +goose Up
-- What members watched of Couch TV, for ranks (docs/couch-tv-plan.md D12, 6.9): the seconds
-- of each airing per hour of the channel's day, in the channel's time zone, so "after two at
-- night" is the channel's night. Programs and breaks only: the standby card and the slate
-- are not television. The room writes it every 20 s; reactions and the largest audience go
-- to user_counters. airing_id has no foreign key on purpose: XP must outlive an airing that
-- leaves with its title, while the achievements read an airing's fill and time on air by
-- joining couchtv_airings.
CREATE TABLE user_tv_viewing (
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    airing_id uuid NOT NULL,
    day date NOT NULL,
    hour smallint NOT NULL CHECK (hour BETWEEN 0 AND 23),
    seconds bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, airing_id, day, hour)
);

-- +goose Down
DROP TABLE user_tv_viewing;
