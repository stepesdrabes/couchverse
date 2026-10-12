-- Couch TV spike S2 (docs/couch-tv-plan.md): a 60 s break for the render sidecar to render
-- and assemble, cut from one real file, on a throwaway database the app has migrated. The
-- squeeze shows the file's frames from start_ms (footage decoded per frame, like a promo);
-- the rest are the brand and card templates with stand-in artwork. Run it with
--   psql "$S2_DB" -v file='Iron Man (2008)/Iron Man (2008).mkv' -v start_ms=7200000 -f couchtv-s2.sql
-- (file is relative to the managed movies library, DATA_DIR/media/movies), then start the
-- sidecar: it renders the seven clips (1500 frames) and assembles the break.

INSERT INTO media_files (library_id, path)
SELECT id, :'file' FROM libraries WHERE kind = 'movies' AND managed
ON CONFLICT (library_id, path) DO NOTHING;

CREATE TEMP TABLE s2 AS
SELECT mf.id::text AS file,
       (SELECT jsonb_agg(format('/media/v1/frames/%s/%s_288/%s.jpg', mf.id, :start_ms, n) ORDER BY n)
        FROM generate_series(0, 287) n) AS frames,
       format('/media/v1/audio/%s/%s_11520.flac', mf.id, :start_ms) AS bed,
       '{"name": "Spike Two", "genres": ["Drama"], "accents": {"backdrop": "#2f6fd0"}, "backdrop": "", "poster": ""}'::jsonb AS title
FROM media_files mf JOIN libraries l ON l.id = mf.library_id
WHERE l.kind = 'movies' AND l.managed AND mf.path = :'file';

INSERT INTO couchtv_clips (id, kind, key, spec, duration_ms, needed_at, last_used_at)
SELECT c.id::uuid, c.kind, 's2-' || c.kind || c.n, c.spec, c.frames * 40, now(), now()
FROM s2, LATERAL (VALUES
    ('00000000-0000-4000-8000-00000000e201', 1, 'squeeze', 275, jsonb_build_object('template', 'squeeze', 'ending', s2.title,
        'program', jsonb_build_object('kind', 'frames', 'frames', s2.frames), 'handover', 'line', 'clock', '20:13',
        'next', jsonb_build_object('title', s2.title, 'time', '20:15'),
        'bed', jsonb_build_object('url', s2.bed, 'in', 0, 'out', 11.52))),
    ('00000000-0000-4000-8000-00000000e202', 1, 'ident_short', 125, '{"template": "ident", "cut": "short"}'::jsonb),
    ('00000000-0000-4000-8000-00000000e203', 1, 'short_promo', 225, jsonb_build_object('template', 'shortPromo', 'title', s2.title,
        'airing', '{"eyebrow": "Tomorrow", "time": "21:00"}'::jsonb, 'duration', 9)),
    ('00000000-0000-4000-8000-00000000e204', 1, 'lineup', 300, jsonb_build_object('template', 'lineup', 'duration', 12, 'items', jsonb_build_array(
        jsonb_build_object('time', '20:15', 'title', s2.title, 'sub', 'Movie · Drama · 126 min', 'state', 'next', 'badge', 'Next · in 1 min'),
        jsonb_build_object('time', '22:30', 'title', s2.title, 'sub', 'S01E02 · Pilot')))),
    ('00000000-0000-4000-8000-00000000e205', 1, 'sting', 150, '{"template": "sting", "duration": 6}'::jsonb),
    ('00000000-0000-4000-8000-00000000e206', 2, 'short_promo', 225, jsonb_build_object('template', 'shortPromo', 'title', s2.title,
        'airing', '{"eyebrow": "Premiere", "day": "Saturday", "time": "20:15"}'::jsonb, 'duration', 9)),
    ('00000000-0000-4000-8000-00000000e207', 1, 'upnext', 200, jsonb_build_object('template', 'upNext', 'title', s2.title,
        'time', '20:15', 'clock', '20:14', 'duration', 8))
) AS c(id, n, kind, frames, spec)
ON CONFLICT (key) DO NOTHING;

-- the break's sound: every clip's spec at its start, mixed at -20 LUFS
INSERT INTO couchtv_breaks (id, kind, state, duration_ms, recipe)
SELECT '00000000-0000-4000-8000-00000000e2ff', 'evergreen', 'composed', 60000, jsonb_build_object(
    'key', 's2', 'frames', 1500, 'promos', '[]'::jsonb,
    'clips', jsonb_agg(jsonb_build_object('clipId', id, 'kind', kind, 'at', at, 'frames', duration_ms / 40) ORDER BY at),
    'sound', jsonb_build_object('template', 'break', 'loudness', -20,
        'items', jsonb_agg(jsonb_build_object('at', at / 25.0, 'spec', spec) ORDER BY at)))
FROM (SELECT id, kind, duration_ms, spec, (sum(duration_ms) OVER (ORDER BY id) - duration_ms) / 40 AS at
      FROM couchtv_clips WHERE key LIKE 's2-%') clips
ON CONFLICT DO NOTHING;

INSERT INTO jobs (type, payload, priority)
SELECT 'couchtv_render', jsonb_build_object('clipId', id), 0 FROM couchtv_clips WHERE key LIKE 's2-%' AND state <> 'ready';
