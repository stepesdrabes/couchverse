-- +goose Up
-- Couch TV's trailers (docs/couch-tv-plan.md, D43): the long promo is cut from the title's own
-- scenes, their picture and their sound in the original language. couchtv_analyze picks the
-- scenes (trailer: for each its picture file, audio file and stream, start, length and lines
-- with their text); trailer_version is the picker that did, apart from analyzed_version, so a
-- better picker never measures the loudness of whole films again, and an admin's scenes
-- (trailer_source) are kept. The two clip kinds are a trailer's body and its end card.
ALTER TABLE couchtv_titles
    ADD COLUMN trailer jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN trailer_source text CHECK (trailer_source IN ('auto', 'admin')),
    ADD COLUMN trailer_version int NOT NULL DEFAULT 0;
ALTER TABLE couchtv_clips DROP CONSTRAINT couchtv_clips_kind_check;
ALTER TABLE couchtv_clips
    ADD CONSTRAINT couchtv_clips_kind_check CHECK (kind IN ('ident_full', 'ident_short', 'sting', 'promo_body',
        'promo_end', 'evergreen_end', 'short_promo', 'strip_promo', 'upnext', 'lineup', 'squeeze', 'standby',
        'weather', 'news', 'trailer_body', 'trailer_end'));

-- +goose Down
DELETE FROM couchtv_clips WHERE kind IN ('trailer_body', 'trailer_end');
ALTER TABLE couchtv_clips DROP CONSTRAINT couchtv_clips_kind_check;
ALTER TABLE couchtv_clips
    ADD CONSTRAINT couchtv_clips_kind_check CHECK (kind IN ('ident_full', 'ident_short', 'sting', 'promo_body',
        'promo_end', 'evergreen_end', 'short_promo', 'strip_promo', 'upnext', 'lineup', 'squeeze', 'standby',
        'weather', 'news'));
ALTER TABLE couchtv_titles
    DROP COLUMN trailer_version,
    DROP COLUMN trailer_source,
    DROP COLUMN trailer;
