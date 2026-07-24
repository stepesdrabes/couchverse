-- +goose Up
-- A profile banner is just another artwork row, so it inherits the resizing,
-- caching and accent extraction the rest of the app already has.
ALTER TABLE artwork DROP CONSTRAINT artwork_kind_check;
ALTER TABLE artwork ADD CONSTRAINT artwork_kind_check
    CHECK (kind IN ('poster', 'backdrop', 'thumb', 'album_cover', 'artist_photo', 'avatar', 'banner'));

-- Public-profile bio, authored as markdown and rendered client-side with raw
-- HTML disabled. A column rather than a preferences key: it is public profile
-- content, it needs a length bound, and profiles select it by join.
ALTER TABLE users ADD COLUMN bio text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users DROP COLUMN bio;
DELETE FROM artwork WHERE kind = 'banner';
ALTER TABLE artwork DROP CONSTRAINT artwork_kind_check;
ALTER TABLE artwork ADD CONSTRAINT artwork_kind_check
    CHECK (kind IN ('poster', 'backdrop', 'thumb', 'album_cover', 'artist_photo', 'avatar'));
