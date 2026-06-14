-- +goose Up
-- Per-language TMDB metadata. Base columns (name/overview) stay the default
-- language (English) and act as the fallback; `translations` holds other
-- languages as {"cs": {"name": "...", "overview": "...", "tagline": "..."}}.
-- `titles.metadata_languages` is the set of content languages a title carries,
-- chosen at create/edit time (e.g. {en,cs} for Dexter, {en,ko} for Squid Game).
ALTER TABLE titles ADD COLUMN translations jsonb NOT NULL DEFAULT '{}';
ALTER TABLE titles ADD COLUMN metadata_languages text[] NOT NULL DEFAULT '{}';
ALTER TABLE seasons ADD COLUMN translations jsonb NOT NULL DEFAULT '{}';
ALTER TABLE episodes ADD COLUMN translations jsonb NOT NULL DEFAULT '{}';
ALTER TABLE genres ADD COLUMN translations jsonb NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE genres DROP COLUMN translations;
ALTER TABLE episodes DROP COLUMN translations;
ALTER TABLE seasons DROP COLUMN translations;
ALTER TABLE titles DROP COLUMN metadata_languages;
ALTER TABLE titles DROP COLUMN translations;
