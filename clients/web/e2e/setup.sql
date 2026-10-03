-- Applied by scripts/e2e-server.sh after backend/internal/server/testdata/seed.sql, with
-- psql variables `movie` (the generated clip, relative to the Movies library) and
-- `movie_size`.

UPDATE media_files
   SET library_id = (SELECT id FROM libraries WHERE kind = 'movies' ORDER BY id LIMIT 1),
       path = :'movie', size_bytes = :movie_size, duration_seconds = 60
 WHERE id = '00000000-0000-4000-8000-000000000501';

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
