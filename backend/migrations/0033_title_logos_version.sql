-- +goose Up
-- The TMDB logo backfill (metadata.LogoBackfill) records the version of itself that looked a
-- title's logos up, found or not, so a title TMDB has none for is not asked again at every
-- start, and a better backfill (a higher version) looks every title up once more.
ALTER TABLE titles ADD COLUMN logos_version int NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE titles DROP COLUMN logos_version;
