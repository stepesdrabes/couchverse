-- +goose Up
-- The subtitle files a title's trailer pick read (their paths under DATA_DIR, sorted and joined by
-- commas; empty for none). A trailer is picked from subtitles only, so a title whose pick is empty
-- for want of subtitles is picked again once the key of what it has no longer matches.
ALTER TABLE couchtv_titles ADD COLUMN trailer_subs text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE couchtv_titles DROP COLUMN trailer_subs;
