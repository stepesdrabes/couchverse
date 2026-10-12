-- +goose Up
-- Where things are in a media file (docs/couch-tv-plan.md 6.2): a series episode's intro
-- and end credits, a movie's credits start and the music under them. The analysis writes
-- the auto rows; an admin's row of a kind overrides the auto one and the analysis never
-- touches it. An admin row without a start says the file has none of that kind.
CREATE TABLE media_markers (
    media_file_id uuid NOT NULL REFERENCES media_files (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('intro', 'credits', 'credits_music')),
    source text NOT NULL CHECK (source IN ('auto', 'admin')),
    start_ms int CHECK (start_ms >= 0),
    end_ms int CHECK (end_ms > start_ms),
    -- the analyzer that wrote an auto row; 0 for admin rows
    version int NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (media_file_id, kind, source),
    CHECK (start_ms IS NOT NULL OR source = 'admin' AND end_ms IS NULL)
);

-- The analyzer that last looked at a file, so a file without markers is not analyzed
-- again and a better analyzer re-runs.
ALTER TABLE media_files ADD COLUMN markers_version int NOT NULL DEFAULT 0;

-- What couchtv_analyze measures beyond the promo picks: the program's loudness for the
-- break mix, and whose shots these are (an admin's are kept).
ALTER TABLE couchtv_titles
    ADD COLUMN shots_source text CHECK (shots_source IN ('auto', 'admin')),
    ADD COLUMN loudness_lufs double precision,
    ADD COLUMN analyzed_at timestamptz;

-- +goose Down
ALTER TABLE couchtv_titles
    DROP COLUMN analyzed_at,
    DROP COLUMN loudness_lufs,
    DROP COLUMN shots_source;
ALTER TABLE media_files DROP COLUMN markers_version;
DROP TABLE media_markers;
