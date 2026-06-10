-- +goose Up
ALTER TABLE artwork DROP CONSTRAINT artwork_owner_kind_check;
ALTER TABLE artwork ADD CONSTRAINT artwork_owner_kind_check
    CHECK (owner_kind IN ('title', 'season', 'episode', 'artist', 'album', 'user'));
ALTER TABLE artwork DROP CONSTRAINT artwork_kind_check;
ALTER TABLE artwork ADD CONSTRAINT artwork_kind_check
    CHECK (kind IN ('poster', 'backdrop', 'thumb', 'album_cover', 'artist_photo', 'avatar'));

-- +goose Down
DELETE FROM artwork WHERE owner_kind = 'user' OR kind = 'avatar';
ALTER TABLE artwork DROP CONSTRAINT artwork_owner_kind_check;
ALTER TABLE artwork ADD CONSTRAINT artwork_owner_kind_check
    CHECK (owner_kind IN ('title', 'season', 'episode', 'artist', 'album'));
ALTER TABLE artwork DROP CONSTRAINT artwork_kind_check;
ALTER TABLE artwork ADD CONSTRAINT artwork_kind_check
    CHECK (kind IN ('poster', 'backdrop', 'thumb', 'album_cover', 'artist_photo'));
