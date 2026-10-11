-- +goose Up
-- Couch TV (docs/couch-tv-plan.md), the live channel. Titles keep TMDB's base-language
-- tagline for promos and the TMDB collection that orders Saga Sunday.
ALTER TABLE titles
    ADD COLUMN tagline text NOT NULL DEFAULT '',
    ADD COLUMN collection_tmdb_id int,
    ADD COLUMN collection_name text NOT NULL DEFAULT '';

-- The schedule: programs and the breaks between them, in UTC. Auto rows are replaced
-- whenever their day is replanned; admin and locked rows survive it. A program's fill
-- says which rule chose it, so the planner reads strip cursors, the movie rotation and
-- the saga back from earlier rows instead of keeping its own state.
CREATE TABLE couchtv_airings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NOT NULL,
    kind text NOT NULL CHECK (kind IN ('program', 'break')),
    title_id uuid REFERENCES titles (id) ON DELETE CASCADE,
    episode_id uuid REFERENCES episodes (id) ON DELETE CASCADE,
    media_file_id uuid REFERENCES media_files (id) ON DELETE SET NULL,
    slot text NOT NULL DEFAULT '',
    fill text CHECK (fill IN ('strip', 'marathon', 'reruns', 'movie', 'matinee', 'premiere', 'saga',
                              'encore', 'filler')),
    source text NOT NULL DEFAULT 'auto' CHECK (source IN ('auto', 'admin')),
    locked boolean NOT NULL DEFAULT false,
    state text NOT NULL DEFAULT 'planned' CHECK (state IN ('planned', 'ready', 'aired', 'failed', 'replaced')),
    -- where the credits squeeze begins, from the program's start; null while unknown
    credits_at_ms int,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (ends_at > starts_at),
    CHECK (kind = 'break' OR title_id IS NOT NULL),
    CHECK (kind = 'program' OR (title_id IS NULL AND episode_id IS NULL AND fill IS NULL))
);
CREATE INDEX couchtv_airings_starts_idx ON couchtv_airings (starts_at);
CREATE INDEX couchtv_airings_title_idx ON couchtv_airings (title_id);
CREATE INDEX couchtv_airings_episode_idx ON couchtv_airings (episode_id);

-- An admin's restart of a strip: from effective_from on, the series' strip continues at
-- next_episode_id. Without a row the cursor is derived from the series' latest strip
-- airing, so replanning a day never has to rewind stored state.
CREATE TABLE couchtv_strips (
    title_id uuid PRIMARY KEY REFERENCES titles (id) ON DELETE CASCADE,
    next_episode_id uuid NOT NULL REFERENCES episodes (id) ON DELETE CASCADE,
    effective_from timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Per-title channel facts: the "Never on Couch TV" switch and what the analysis picked
-- for promos (soundtrack excerpt, footage shots, mood).
CREATE TABLE couchtv_titles (
    title_id uuid PRIMARY KEY REFERENCES titles (id) ON DELETE CASCADE,
    excluded boolean NOT NULL DEFAULT false,
    promo_audio_file_id uuid REFERENCES media_files (id) ON DELETE SET NULL,
    promo_audio_start_ms int,
    promo_audio_duration_ms int,
    promo_audio_source text CHECK (promo_audio_source IN ('auto', 'admin')),
    shots jsonb NOT NULL DEFAULT '[]',
    mood text CHECK (mood IN ('cinematic', 'grit', 'bright')),
    analyzed_version int NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Rendered cutscene clips, shared by every break that needs the same render: key hashes
-- the template version and its inputs.
CREATE TABLE couchtv_clips (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind text NOT NULL CHECK (kind IN ('ident_full', 'ident_short', 'sting', 'promo_body', 'promo_end',
                                       'upnext', 'lineup', 'squeeze')),
    key text NOT NULL UNIQUE,
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'rendering', 'ready', 'failed')),
    duration_ms int NOT NULL DEFAULT 0,
    path text NOT NULL DEFAULT '',
    size_bytes bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    rendered_at timestamptz,
    last_used_at timestamptz
);

-- A break's composition: the clips in order with their offsets and its audio mix.
CREATE TABLE couchtv_breaks (
    airing_id uuid PRIMARY KEY REFERENCES couchtv_airings (id) ON DELETE CASCADE,
    recipe jsonb NOT NULL DEFAULT '[]',
    audio_path text NOT NULL DEFAULT '',
    state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'composed', 'rendering', 'ready', 'failed')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE couchtv_breaks;
DROP TABLE couchtv_clips;
DROP TABLE couchtv_titles;
DROP TABLE couchtv_strips;
DROP TABLE couchtv_airings;
ALTER TABLE titles
    DROP COLUMN collection_name,
    DROP COLUMN collection_tmdb_id,
    DROP COLUMN tagline;
