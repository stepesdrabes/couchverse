-- +goose Up
-- Couch TV rendering (docs/couch-tv-plan.md 6.5, 6.7). A clip keeps the spec the sidecar
-- renders, the template version it rendered with and when a break first needs it (render
-- order and the render health). Kinds follow the templates a break uses.
ALTER TABLE couchtv_clips DROP CONSTRAINT couchtv_clips_kind_check;
ALTER TABLE couchtv_clips
    ADD CONSTRAINT couchtv_clips_kind_check CHECK (kind IN ('ident_full', 'ident_short', 'sting', 'promo_body',
        'promo_end', 'evergreen_end', 'short_promo', 'strip_promo', 'upnext', 'lineup', 'squeeze', 'standby')),
    ADD COLUMN spec jsonb NOT NULL DEFAULT '{}',
    ADD COLUMN version int NOT NULL DEFAULT 0,
    ADD COLUMN error text NOT NULL DEFAULT '',
    ADD COLUMN needed_at timestamptz,
    -- wall time of the render, for the render speed the health report extrapolates from
    ADD COLUMN render_ms int NOT NULL DEFAULT 0,
    ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();
CREATE INDEX couchtv_clips_state_idx ON couchtv_clips (state, needed_at);

-- A break is the composition of one airing, or the evergreen fallback (no airing) the live
-- stream airs when a break is not ready. path is the assembled package (renditions with
-- rendition.json, like a Playback v2 package) under DATA_DIR.
ALTER TABLE couchtv_breaks DROP CONSTRAINT couchtv_breaks_pkey;
ALTER TABLE couchtv_breaks
    ADD COLUMN id uuid NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    ALTER COLUMN airing_id DROP NOT NULL,
    ADD CONSTRAINT couchtv_breaks_airing_key UNIQUE (airing_id),
    ADD COLUMN kind text NOT NULL DEFAULT 'airing' CHECK (kind IN ('airing', 'evergreen')),
    ADD CONSTRAINT couchtv_breaks_kind_airing CHECK ((kind = 'airing') = (airing_id IS NOT NULL)),
    DROP COLUMN audio_path,
    ADD COLUMN path text NOT NULL DEFAULT '',
    ADD COLUMN duration_ms int NOT NULL DEFAULT 0,
    ADD COLUMN size_bytes bigint NOT NULL DEFAULT 0,
    ADD COLUMN error text NOT NULL DEFAULT '',
    ADD COLUMN assembled_at timestamptz;

-- +goose Down
ALTER TABLE couchtv_breaks
    DROP COLUMN assembled_at,
    DROP COLUMN error,
    DROP COLUMN size_bytes,
    DROP COLUMN duration_ms,
    DROP COLUMN path,
    ADD COLUMN audio_path text NOT NULL DEFAULT '',
    DROP CONSTRAINT couchtv_breaks_kind_airing,
    DROP COLUMN kind,
    DROP CONSTRAINT couchtv_breaks_airing_key,
    DROP COLUMN id;
DELETE FROM couchtv_breaks WHERE airing_id IS NULL;
ALTER TABLE couchtv_breaks ALTER COLUMN airing_id SET NOT NULL, ADD PRIMARY KEY (airing_id);
DROP INDEX couchtv_clips_state_idx;
DELETE FROM couchtv_clips WHERE kind IN ('evergreen_end', 'short_promo', 'strip_promo', 'standby');
ALTER TABLE couchtv_clips
    DROP COLUMN updated_at,
    DROP COLUMN render_ms,
    DROP COLUMN needed_at,
    DROP COLUMN error,
    DROP COLUMN version,
    DROP COLUMN spec,
    DROP CONSTRAINT couchtv_clips_kind_check,
    ADD CONSTRAINT couchtv_clips_kind_check CHECK (kind IN ('ident_full', 'ident_short', 'sting', 'promo_body',
        'promo_end', 'upnext', 'lineup', 'squeeze'));
