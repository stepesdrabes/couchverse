-- +goose Up
-- Cached vibrant accent colour (hex, e.g. #3a7bd5) extracted from each artwork
-- image on the server, so the UI themes banners and cards from a value in the
-- payload instead of re-analysing images in the browser. Empty = not computed.
ALTER TABLE artwork ADD COLUMN accent text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE artwork DROP COLUMN accent;
