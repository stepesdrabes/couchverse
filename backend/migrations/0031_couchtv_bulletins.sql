-- +goose Up
-- Couch TV's bulletins (docs/couch-tv-plan.md 6.10): the weather and the news headlines the
-- couchtv_bulletins job fetches. A run that yields a usable snapshot adds a row, so the newest
-- row of a kind is the last good one; data is the snapshot as adminGetCouchTvBulletins serves
-- it. The hourly cleanup keeps a few per kind and none past a day, as long as iROZHLAS lets a
-- reader keep its texts.
CREATE TABLE couchtv_bulletins (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind text NOT NULL CHECK (kind IN ('weather', 'news')),
    fetched_at timestamptz NOT NULL,
    valid_until timestamptz NOT NULL,
    data jsonb NOT NULL
);
CREATE INDEX couchtv_bulletins_kind_idx ON couchtv_bulletins (kind, fetched_at DESC);

-- +goose Down
DROP TABLE couchtv_bulletins;
