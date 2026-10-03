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
