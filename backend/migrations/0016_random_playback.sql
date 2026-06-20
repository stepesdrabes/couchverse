-- +goose Up
-- Per-title opt-in for random ("shuffle") playback: a button on the title page and
-- an in-player toggle that auto-advances to a random episode. Suits non-serialized
-- shows (Simpsons, Futurama); off by default so serialized shows keep playing in order.
ALTER TABLE titles ADD COLUMN allow_random_playback boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE titles DROP COLUMN allow_random_playback;
