-- +goose Up
-- Promo beds from the web (docs/couch-tv-plan.md 6.3, "Promo audio from the web"): the
-- official 30-second previews iTunes and Deezer offer, matched to a title by the
-- couchtv_soundtrack job. A title's candidates are its last search's, best first by
-- confidence; its pick (the finder's confident match or an admin's choice) is one of them
-- and is kept through later searches.
CREATE TABLE couchtv_soundtracks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title_id uuid NOT NULL REFERENCES titles (id) ON DELETE CASCADE,
    provider text NOT NULL CHECK (provider IN ('itunes', 'deezer')),
    track_id text NOT NULL CHECK (track_id ~ '^[0-9]{1,20}$'),
    track text NOT NULL,
    artist text NOT NULL,
    album text NOT NULL DEFAULT '',
    year int,
    -- the whole track's length, not the preview's
    duration_ms int NOT NULL DEFAULT 0,
    preview_url text NOT NULL,
    confidence double precision NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (title_id, provider, track_id)
);

-- The pick plays under the title's promos once its preview is downloaded (path, under
-- DATA_DIR) and measured (the excerpt within the preview): an admin's excerpt of the
-- title's own audio still comes first, the analysis' excerpt only after the pick.
ALTER TABLE couchtv_titles
    ADD COLUMN soundtrack_id uuid REFERENCES couchtv_soundtracks (id) ON DELETE SET NULL,
    ADD COLUMN soundtrack_source text CHECK (soundtrack_source IN ('auto', 'admin')),
    ADD COLUMN soundtrack_path text,
    ADD COLUMN soundtrack_start_ms int CHECK (soundtrack_start_ms >= 0),
    ADD COLUMN soundtrack_duration_ms int CHECK (soundtrack_duration_ms > 0),
    -- the admin's own query of the last search; empty for the finder's
    ADD COLUMN soundtrack_query text NOT NULL DEFAULT '',
    ADD COLUMN soundtrack_version int NOT NULL DEFAULT 0,
    ADD COLUMN soundtrack_searched_at timestamptz;

-- +goose Down
ALTER TABLE couchtv_titles
    DROP COLUMN soundtrack_searched_at,
    DROP COLUMN soundtrack_version,
    DROP COLUMN soundtrack_query,
    DROP COLUMN soundtrack_duration_ms,
    DROP COLUMN soundtrack_start_ms,
    DROP COLUMN soundtrack_path,
    DROP COLUMN soundtrack_source,
    DROP COLUMN soundtrack_id;
DROP TABLE couchtv_soundtracks;
