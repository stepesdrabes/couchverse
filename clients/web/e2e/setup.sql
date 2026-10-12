-- Applied by scripts/e2e-server.sh after backend/internal/server/testdata/seed.sql, with
-- psql variables naming the generated clips (relative to their library) and their sizes:
-- `movie`/`movie_size`, the movie's Czech file `alt`/`alt_size`, and the series' first two
-- episodes `episode1`/`episode2` (`episode_size`).

UPDATE media_files
   SET library_id = (SELECT id FROM libraries WHERE kind = 'movies' ORDER BY id LIMIT 1),
       path = :'movie', size_bytes = :movie_size, duration_seconds = 60
 WHERE id = '00000000-0000-4000-8000-000000000501';

UPDATE media_files
   SET library_id = (SELECT id FROM libraries WHERE kind = 'movies' ORDER BY id LIMIT 1),
       path = :'alt', size_bytes = :alt_size, duration_seconds = 60,
       video_codec = 'h264', width = 320, height = 180
 WHERE id = '00000000-0000-4000-8000-000000000502';

-- plain H.264 clips that play directly, the second with the ladder rung the seed prepared
UPDATE media_files
   SET library_id = (SELECT id FROM libraries WHERE kind = 'series' ORDER BY id LIMIT 1),
       path = CASE episode_id WHEN '00000000-0000-4000-8000-000000000301' THEN :'episode1'
                              ELSE :'episode2' END,
       size_bytes = :episode_size, duration_seconds = 25, container = 'mp4', video_codec = 'h264',
       audio_codec = 'aac', width = 320, height = 180, channels = 2, video_range = 'sdr',
       direct_play = true
 WHERE id IN ('00000000-0000-4000-8000-000000000503', '00000000-0000-4000-8000-000000000504');
DELETE FROM audio_streams WHERE media_file_id = '00000000-0000-4000-8000-000000000503';
UPDATE watch_progress
   SET position_seconds = LEAST(position_seconds, 10), duration_seconds = 25
 WHERE episode_id IN ('00000000-0000-4000-8000-000000000301', '00000000-0000-4000-8000-000000000302');

-- the seeded resume points lie far past the end of the one-minute clip
UPDATE watch_progress
   SET position_seconds = 20, duration_seconds = 60
 WHERE title_id = '00000000-0000-4000-8000-000000000101';

-- Members whose state a spec changes (watch progress, display language), so specs running
-- in parallel never trip over each other's writes to nora.
INSERT INTO users (username, display_name, password_hash, role)
SELECT v.username, v.display_name, password_hash, 'member'
  FROM users, (VALUES ('otto', 'Otto'), ('vera', 'Vera')) AS v (username, display_name)
 WHERE users.username = 'admin';

-- Couch TV, on for the run: the series' first episode (its generated clip) airs to its credits
-- at 24 s, every 32 s from a few minutes back to three hours on, with the built-in slate in the
-- breaks (nothing is rendered). Locked admin rows, so a replan (the admin spec's) airs around
-- them, and reruns, so they move no strip on.
INSERT INTO settings (key, value)
VALUES ('features', '{"couchEnabled": true, "rankingsEnabled": true, "downloadsEnabled": true, "couchTvEnabled": true}')
ON CONFLICT (key) DO UPDATE SET value = settings.value || '{"couchTvEnabled": true}';

-- the channel airs the episode from its package; scripts/e2e-server.sh waits for this job
INSERT INTO jobs (id, type, payload)
OVERRIDING SYSTEM VALUE VALUES
    (905, 'transcode_hls', '{"mediaFileId": "00000000-0000-4000-8000-000000000503", "variant": "package"}');

-- clear of anything a plan run put there before the seed
DELETE FROM couchtv_airings
 WHERE ends_at > date_trunc('minute', now()) - interval '10 minutes'
   AND starts_at < now() + interval '3 hours 1 minute';
INSERT INTO couchtv_airings (starts_at, ends_at, kind, title_id, episode_id, media_file_id, fill, source,
                             locked, credits_at_ms)
SELECT at, at + interval '24 seconds', 'program', '00000000-0000-4000-8000-000000000102',
       '00000000-0000-4000-8000-000000000301', '00000000-0000-4000-8000-000000000503', 'reruns', 'admin',
       true, 24000
  FROM generate_series(date_trunc('minute', now()) - interval '10 minutes', now() + interval '3 hours',
                       interval '32 seconds') AS at;
INSERT INTO couchtv_airings (starts_at, ends_at, kind)
SELECT starts_at + interval '24 seconds', starts_at + interval '32 seconds', 'break'
  FROM couchtv_airings WHERE kind = 'program' AND source = 'admin' AND locked;
