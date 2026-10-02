-- +goose Up
-- Music is removed from Couchverse: drop its tables and rows, and tighten every
-- CHECK that listed a music value so none can come back. Media and artwork files
-- on disk are left for the admin to remove by hand.

-- jobs point at media files only through their payload, so nothing cascades there
WITH gone AS (
    DELETE FROM media_files
    WHERE track_id IS NOT NULL
       OR library_id IN (SELECT id FROM libraries WHERE kind = 'music')
    RETURNING id
)
DELETE FROM jobs WHERE payload->>'mediaFileId' IN (SELECT id::text FROM gone);

DELETE FROM libraries WHERE kind = 'music';
ALTER TABLE libraries DROP CONSTRAINT libraries_kind_check;
ALTER TABLE libraries ADD CONSTRAINT libraries_kind_check
    CHECK (kind IN ('movies', 'series'));

ALTER TABLE media_files DROP CONSTRAINT media_files_check;
ALTER TABLE media_files DROP COLUMN track_id;
ALTER TABLE media_files ADD CONSTRAINT media_files_check
    CHECK (num_nonnulls(title_id, episode_id) <= 1);

-- genres created from audio tags that no title uses would list as empty genres
DELETE FROM genres g
WHERE EXISTS (SELECT 1 FROM album_genres ag WHERE ag.genre_id = g.id)
  AND NOT EXISTS (SELECT 1 FROM title_genres tg WHERE tg.genre_id = g.id)
  AND NOT EXISTS (SELECT 1 FROM home_rows hr WHERE hr.genre_id = g.id);

DROP TABLE playlist_tracks;
DROP TABLE playlists;
DROP TABLE play_history;
DROP TABLE album_genres;
DROP TABLE tracks;
DROP TABLE albums;
DROP TABLE artists;

DELETE FROM artwork
WHERE owner_kind IN ('artist', 'album') OR kind IN ('album_cover', 'artist_photo');
ALTER TABLE artwork DROP CONSTRAINT artwork_owner_kind_check;
ALTER TABLE artwork ADD CONSTRAINT artwork_owner_kind_check
    CHECK (owner_kind IN ('title', 'season', 'episode', 'user'));
ALTER TABLE artwork DROP CONSTRAINT artwork_kind_check;
ALTER TABLE artwork ADD CONSTRAINT artwork_kind_check
    CHECK (kind IN ('poster', 'backdrop', 'thumb', 'avatar', 'banner'));

DELETE FROM watch_time_daily WHERE kind = 'music';
ALTER TABLE watch_time_daily DROP CONSTRAINT watch_time_daily_kind_check;
ALTER TABLE watch_time_daily ADD CONSTRAINT watch_time_daily_kind_check
    CHECK (kind = 'video');

DELETE FROM watch_time_hourly WHERE kind = 'music';
ALTER TABLE watch_time_hourly DROP CONSTRAINT watch_time_hourly_kind_check;
ALTER TABLE watch_time_hourly ADD CONSTRAINT watch_time_hourly_kind_check
    CHECK (kind = 'video');

DELETE FROM home_rows WHERE kind = 'recently_played_music';
ALTER TABLE home_rows DROP CONSTRAINT home_rows_kind_check;
ALTER TABLE home_rows ADD CONSTRAINT home_rows_kind_check
    CHECK (kind IN ('continue_watching', 'recently_added', 'genre'));

DELETE FROM user_achievements
WHERE code IN ('tracks_100', 'listen_50h', 'artists_25', 'playlist_50');

-- guarded on shape: the readers tolerate a malformed value, so this must too
UPDATE settings SET value = value - 'musicEnabled'
WHERE key = 'features' AND jsonb_typeof(value) = 'object';
UPDATE settings SET value = value #- '{rates,musicMinute}'
WHERE key = 'ranks' AND jsonb_typeof(value -> 'rates') = 'object';

-- +goose Down
-- Irreversible: the music rows are deleted, so there is nothing to restore.
