-- +goose Up
-- The version of the footage shots' pick (couchtv.ShotsVersion), apart from the analysis'
-- (analyzed_version), so a better pick re-picks every title's shots without measuring its
-- loudness again, which decodes a whole film. Existing rows are the first pick (0).
ALTER TABLE couchtv_titles ADD COLUMN shots_version int NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE couchtv_titles DROP COLUMN shots_version;
