package store

import (
	"context"
)

// UpsertArtist returns the artist id for a name, creating it when new.
func (s *Store) UpsertArtist(ctx context.Context, name string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO artists (name, sort_name) VALUES ($1, $1)
		 ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id`, name).Scan(&id)
	return id, err
}

func (s *Store) UpsertAlbum(ctx context.Context, artistID int64, name string, year int) (int64, error) {
	var yearVal *int
	if year > 0 {
		yearVal = &year
	}
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO albums (artist_id, name, year, status) VALUES ($1, $2, $3, 'published')
		 ON CONFLICT (artist_id, name) DO UPDATE
			SET year = COALESCE(albums.year, EXCLUDED.year)
		 RETURNING id`, artistID, name, yearVal).Scan(&id)
	return id, err
}

func (s *Store) UpsertTrack(ctx context.Context, albumID int64, disc, num int, name string, durationSeconds int, trackArtist string) (int64, error) {
	var artistVal *string
	if trackArtist != "" {
		artistVal = &trackArtist
	}
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO tracks (album_id, disc_number, track_number, name, duration_seconds, track_artist)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (album_id, disc_number, track_number, name) DO UPDATE
			SET duration_seconds = EXCLUDED.duration_seconds,
				track_artist = EXCLUDED.track_artist
		 RETURNING id`,
		albumID, disc, num, name, durationSeconds, artistVal).Scan(&id)
	return id, err
}

func (s *Store) SetAlbumGenre(ctx context.Context, albumID int64, genre string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var genreID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO genres (name) VALUES ($1)
		 ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id`, genre).Scan(&genreID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO album_genres (album_id, genre_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		albumID, genreID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
