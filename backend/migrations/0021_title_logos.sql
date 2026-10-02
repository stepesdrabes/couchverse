-- +goose Up
-- Title logos are transparent wordmarks in a content language, so artwork gains
-- an optional language and a slot becomes (owner, kind, language): one logo per
-- title and language, while art that is not tied to a language keeps a null one.
ALTER TABLE artwork ADD COLUMN lang text CHECK (lang ~ '^[a-z]{2}$');
ALTER TABLE artwork DROP CONSTRAINT artwork_owner_kind_owner_id_kind_key;
ALTER TABLE artwork ADD CONSTRAINT artwork_slot_key
    UNIQUE NULLS NOT DISTINCT (owner_kind, owner_id, kind, lang);
ALTER TABLE artwork DROP CONSTRAINT artwork_kind_check;
ALTER TABLE artwork ADD CONSTRAINT artwork_kind_check
    CHECK (kind IN ('poster', 'backdrop', 'thumb', 'avatar', 'banner', 'logo'));

-- +goose Down
DELETE FROM artwork WHERE kind = 'logo' OR lang IS NOT NULL;
ALTER TABLE artwork DROP CONSTRAINT artwork_kind_check;
ALTER TABLE artwork ADD CONSTRAINT artwork_kind_check
    CHECK (kind IN ('poster', 'backdrop', 'thumb', 'avatar', 'banner'));
ALTER TABLE artwork DROP CONSTRAINT artwork_slot_key;
ALTER TABLE artwork ADD CONSTRAINT artwork_owner_kind_owner_id_kind_key
    UNIQUE (owner_kind, owner_id, kind);
ALTER TABLE artwork DROP COLUMN lang;
