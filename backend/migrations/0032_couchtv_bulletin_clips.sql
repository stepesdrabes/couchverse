-- +goose Up
-- Couch TV's bulletins on air (docs/couch-tv-plan.md 6.10): a weather or news bulletin is
-- rendered as a clip of its own kind, at most once (clip_id), shortly before a break's slot can
-- air it. A clip whose bulletin is gone or long expired is pruned with its headlines.
ALTER TABLE couchtv_clips DROP CONSTRAINT couchtv_clips_kind_check;
ALTER TABLE couchtv_clips
    ADD CONSTRAINT couchtv_clips_kind_check CHECK (kind IN ('ident_full', 'ident_short', 'sting', 'promo_body',
        'promo_end', 'evergreen_end', 'short_promo', 'strip_promo', 'upnext', 'lineup', 'squeeze', 'standby',
        'weather', 'news'));
ALTER TABLE couchtv_bulletins ADD COLUMN clip_id uuid REFERENCES couchtv_clips (id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE couchtv_bulletins DROP COLUMN clip_id;
DELETE FROM couchtv_clips WHERE kind IN ('weather', 'news');
ALTER TABLE couchtv_clips DROP CONSTRAINT couchtv_clips_kind_check;
ALTER TABLE couchtv_clips
    ADD CONSTRAINT couchtv_clips_kind_check CHECK (kind IN ('ident_full', 'ident_short', 'sting', 'promo_body',
        'promo_end', 'evergreen_end', 'short_promo', 'strip_promo', 'upnext', 'lineup', 'squeeze', 'standby'));
