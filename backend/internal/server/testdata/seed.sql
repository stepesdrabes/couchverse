-- A small but complete catalog for the API tests: a published movie and series
-- with translations, artwork, files, subtitles and audio tracks, a draft title,
-- viewing history, rankings state and jobs. Runs after migrations and the admin
-- bootstrap; every member reuses the admin's password hash ("admin").

INSERT INTO users (id, username, display_name, password_hash, role, preferences, bio)
OVERRIDING SYSTEM VALUE
SELECT 2, 'nora', 'Nora', password_hash, 'member',
       '{"publicProfile": true, "subtitles": {"size": "large"}}', '# Hi\n\nI watch **everything**.'
FROM users WHERE username = 'admin';
INSERT INTO users (id, username, display_name, password_hash, role, preferences)
OVERRIDING SYSTEM VALUE
SELECT 3, 'piet', 'Piet', password_hash, 'member', '{"publicProfile": false}'
FROM users WHERE username = 'admin';
INSERT INTO users (id, username, display_name, password_hash, role, disabled)
OVERRIDING SYSTEM VALUE
SELECT 4, 'gone', 'Gone', password_hash, 'member', true
FROM users WHERE username = 'admin';
SELECT setval(pg_get_serial_sequence('users', 'id'), 10);

INSERT INTO libraries (id, name, kind, path, managed) OVERRIDING SYSTEM VALUE VALUES
    (1, 'Movies', 'movies', '/srv/media/movies', true),
    (2, 'Series', 'series', '/srv/media/series', true)
ON CONFLICT DO NOTHING;

INSERT INTO genres (id, name, translations) OVERRIDING SYSTEM VALUE VALUES
    (101, 'Drama', '{"cs": {"name": "Drama"}}'),
    (102, 'Science Fiction', '{"cs": {"name": "Sci-fi"}}'),
    (103, 'Comedy', '{}');
SELECT setval(pg_get_serial_sequence('genres', 'id'), 200);

INSERT INTO titles (id, kind, name, slug, sort_name, overview, year, release_date, content_rating,
                    runtime_minutes, status, tmdb_id, translations, metadata_languages, allow_random_playback)
VALUES
    ('00000000-0000-4000-8000-000000000101', 'movie', 'Glass Harbor', 'glass-harbor-2025', 'Glass Harbor',
     'A lighthouse keeper finds a door in the sea.', 2025, '2025-03-14', 'PG-13', 112, 'published', 1001,
     '{"cs": {"name": "Skleněný přístav", "overview": "Strážce majáku najde dveře v moři."}}', '{en,cs}', false),
    ('00000000-0000-4000-8000-000000000102', 'series', 'Static Bloom', 'static-bloom-2024', 'Static Bloom',
     'Radio waves start growing flowers.', 2024, NULL, 'TV-14', NULL, 'published', 2002,
     '{}', '{en}', true),
    ('00000000-0000-4000-8000-000000000103', 'movie', 'Unfinished', 'unfinished-2026', 'Unfinished',
     '', 2026, NULL, '', NULL, 'draft', NULL, '{}', '{en}', false);

INSERT INTO title_genres (title_id, genre_id) VALUES
    ('00000000-0000-4000-8000-000000000101', 101),
    ('00000000-0000-4000-8000-000000000101', 102),
    ('00000000-0000-4000-8000-000000000102', 102);

INSERT INTO seasons (id, title_id, season_number, name, overview, translations) VALUES
    ('00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000000102', 1, 'Season 1', 'Seeds.', '{}'),
    ('00000000-0000-4000-8000-000000000202', '00000000-0000-4000-8000-000000000102', 2, 'Season 2', '', '{}');

INSERT INTO episodes (id, season_id, episode_number, name, overview, air_date, runtime_minutes, translations) VALUES
    ('00000000-0000-4000-8000-000000000301', '00000000-0000-4000-8000-000000000201', 1, 'Pilot', 'It begins.',
     '2024-01-05', 42, '{"cs": {"name": "Pilot", "overview": "Začíná to."}}'),
    ('00000000-0000-4000-8000-000000000302', '00000000-0000-4000-8000-000000000201', 2, 'Interference', '', NULL, 44, '{}'),
    ('00000000-0000-4000-8000-000000000303', '00000000-0000-4000-8000-000000000202', 1, 'Rebroadcast', '', NULL, NULL, '{}');

INSERT INTO artwork (id, owner_kind, owner_id, kind, path, width, height, source, accent, created_at) VALUES
    ('00000000-0000-4000-8000-000000000401', 'title', '00000000-0000-4000-8000-000000000101', 'poster',
     'artwork/glass-poster.jpg', 500, 750, 'tmdb', '#3a6ea5', '2026-01-01T10:00:00Z'),
    ('00000000-0000-4000-8000-000000000402', 'title', '00000000-0000-4000-8000-000000000101', 'backdrop',
     'artwork/glass-backdrop.jpg', 1280, 720, 'tmdb', '#204060', '2026-01-01T10:00:00Z'),
    ('00000000-0000-4000-8000-000000000403', 'title', '00000000-0000-4000-8000-000000000102', 'poster',
     'artwork/bloom-poster.jpg', 500, 750, 'uploaded', '', '2026-01-02T10:00:00Z'),
    ('00000000-0000-4000-8000-000000000404', 'episode', '00000000-0000-4000-8000-000000000301', 'thumb',
     'artwork/bloom-s1e1.jpg', 300, 169, 'tmdb', '#556b2f', '2026-01-02T10:00:00Z'),
    ('00000000-0000-4000-8000-000000000405', 'user', '2', 'avatar',
     'artwork/nora-avatar.jpg', 256, 256, 'uploaded', '#aa3366', '2026-01-03T10:00:00Z'),
    ('00000000-0000-4000-8000-000000000406', 'user', '2', 'banner',
     'artwork/nora-banner.jpg', 1500, 500, 'uploaded', '#112233', '2026-01-03T10:00:00Z');

INSERT INTO media_files (id, library_id, title_id, episode_id, path, size_bytes, container, video_codec, audio_codec,
                         width, height, duration_seconds, bitrate, channels, sample_rate, video_range, direct_play,
                         probe, file_mtime, scanned_at, created_at, audio_lang, audio_role)
VALUES
    ('00000000-0000-4000-8000-000000000501', 1, '00000000-0000-4000-8000-000000000101', NULL,
     '/srv/media/movies/Glass Harbor (2025)/Glass Harbor (2025).mp4', 1500000000, 'mp4', 'h264', 'aac',
     1920, 1080, 6720.5, 1800000, 2, 48000, 'sdr', true, '{}', '2026-01-01T09:00:00Z', '2026-01-01T09:05:00Z',
     '2026-01-01T09:00:00Z', 'en', 'primary'),
    ('00000000-0000-4000-8000-000000000502', 1, '00000000-0000-4000-8000-000000000101', NULL,
     '/srv/media/movies/Glass Harbor (2025)/Glass Harbor (2025).cs.m4a', 90000000, 'mp4', '', 'aac',
     0, 0, 6720.5, 128000, 2, 48000, 'sdr', true, '{}', '2026-01-01T09:00:00Z', '2026-01-01T09:05:00Z',
     '2026-01-01T09:01:00Z', 'cs', 'audio_alt'),
    ('00000000-0000-4000-8000-000000000503', 2, NULL, '00000000-0000-4000-8000-000000000301',
     '/srv/media/series/Static Bloom/Season 01/Static Bloom S01E01.mkv', 900000000, 'matroska', 'hevc', 'eac3',
     3840, 2160, 2520, 12000000, 6, 48000, 'hdr10', false, '{}', '2026-01-02T09:00:00Z', '2026-01-02T09:05:00Z',
     '2026-01-02T09:00:00Z', '', 'primary'),
    ('00000000-0000-4000-8000-000000000504', 2, NULL, '00000000-0000-4000-8000-000000000302',
     '/srv/media/series/Static Bloom/Season 01/Static Bloom S01E02.mkv', 850000000, 'matroska', 'h264', 'ac3',
     1280, 720, 2640, 5000000, 6, 48000, 'sdr', false, '{}', '2026-01-02T09:00:00Z', '2026-01-02T09:05:00Z',
     '2026-01-02T09:01:00Z', '', 'primary');

INSERT INTO audio_streams (id, media_file_id, stream_index, codec, lang, label, channels, is_default) VALUES
    ('00000000-0000-4000-8000-000000000601', '00000000-0000-4000-8000-000000000503', 1, 'eac3', 'en', 'English 5.1', 6, true),
    ('00000000-0000-4000-8000-000000000602', '00000000-0000-4000-8000-000000000503', 2, 'aac', 'cs', 'Čeština', 2, false);

INSERT INTO subtitles (id, media_file_id, lang, label, source, forced, path, created_at) VALUES
    ('00000000-0000-4000-8000-000000000701', '00000000-0000-4000-8000-000000000501', 'en', 'English', 'uploaded', false,
     'subtitles/glass-en.vtt', '2026-01-01T09:10:00Z'),
    ('00000000-0000-4000-8000-000000000702', '00000000-0000-4000-8000-000000000503', 'cs', 'Čeština', 'embedded', true,
     'subtitles/bloom-cs.vtt', '2026-01-02T09:10:00Z');

INSERT INTO transcode_variants (id, media_file_id, name, width, height, video_bitrate, audio_bitrate, mode, status,
                                playlist_path, created_at, completed_at, size_bytes) VALUES
    ('00000000-0000-4000-8000-000000000801', '00000000-0000-4000-8000-000000000504', '720p', 1280, 720, 3000000, 192000,
     'transcode', 'ready', 'cache/hls/504/720p/index.m3u8', '2026-01-02T10:00:00Z', '2026-01-02T10:30:00Z', 600000000),
    ('00000000-0000-4000-8000-000000000802', '00000000-0000-4000-8000-000000000504', 'source', 1280, 720, 5000000, 192000,
     'copy', 'failed', '', '2026-01-02T10:00:00Z', NULL, 0);

INSERT INTO watch_progress (user_id, title_id, episode_id, position_seconds, duration_seconds, completed, updated_at) VALUES
    (2, '00000000-0000-4000-8000-000000000101', NULL, 1800, 6720, false, now() - interval '1 day'),
    (2, NULL, '00000000-0000-4000-8000-000000000301', 2500, 2520, true, now() - interval '2 days'),
    (2, NULL, '00000000-0000-4000-8000-000000000302', 600, 2640, false, now() - interval '3 hours'),
    (1, '00000000-0000-4000-8000-000000000101', NULL, 120, 6720, false, now() - interval '5 days');

INSERT INTO watchlist (user_id, title_id, added_at) VALUES
    (2, '00000000-0000-4000-8000-000000000101', '2026-01-04T10:00:00Z'),
    (1, '00000000-0000-4000-8000-000000000102', '2026-01-04T10:00:00Z');

INSERT INTO watch_time_daily (day, user_id, title_id, kind, seconds) VALUES
    (current_date, 2, '00000000-0000-4000-8000-000000000101', 'video', 1800),
    (current_date - 1, 2, '00000000-0000-4000-8000-000000000102', 'video', 3100),
    (current_date - 3, 1, '00000000-0000-4000-8000-000000000101', 'video', 120);
INSERT INTO watch_time_hourly (day, hour, user_id, kind, seconds) VALUES
    (current_date, 21, 2, 'video', 1800),
    (current_date - 1, 6, 2, 'video', 3100),
    (current_date - 3, 13, 1, 'video', 120);
INSERT INTO couch_watch_time_daily (day, title_id, seconds) VALUES
    (current_date - 1, '00000000-0000-4000-8000-000000000102', 1500);

INSERT INTO user_achievements (user_id, code, unlocked_at) VALUES
    (2, 'first_play', '2026-01-05T20:00:00Z'),
    (2, 'avatar_set', '2026-01-03T10:00:00Z');
INSERT INTO user_counters (user_id, key, value) VALUES
    (2, 'couch_hosted', 2),
    (2, 'couch_joined', 1),
    (2, 'couch_emoji', 14);

INSERT INTO home_rows (position, kind, genre_id, label, enabled) VALUES
    (3, 'genre', 102, 'Science Fiction', true);

-- fixed ids clear of the cleanup jobs the app enqueues at startup
INSERT INTO jobs (id, type, payload, status, attempts, progress, last_error, created_at, finished_at)
OVERRIDING SYSTEM VALUE VALUES
    (901, 'probe', '{"mediaFileId": "00000000-0000-4000-8000-000000000501"}', 'done', 1, 100, '', now() - interval '1 day', now() - interval '1 day'),
    (902, 'transcode_hls', '{"mediaFileId": "00000000-0000-4000-8000-000000000504", "variant": "source"}', 'failed', 3, 40,
     'ffmpeg exited with status 1', now() - interval '2 hours', now() - interval '1 hour'),
    (903, 'fetch_metadata', '{"titleId": "00000000-0000-4000-8000-000000000101", "tmdbId": 1001}', 'done', 1, 100, NULL,
     now() - interval '3 days', now() - interval '3 days'),
    (904, 'import_episodes', '{"titleId": "00000000-0000-4000-8000-000000000102", "seasons": []}', 'cancelled', 0, 0, NULL,
     now() - interval '3 days', now() - interval '3 days');
