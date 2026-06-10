package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/db"
)

type AlbumCard struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Year       *int    `json:"year"`
	ArtistID   string  `json:"artistId"`
	ArtistName string  `json:"artistName"`
	CoverID    *string `json:"coverId"`
	TrackCount int     `json:"trackCount"`
}

type TrackItem struct {
	ID              string  `json:"id"`
	AlbumID         string  `json:"albumId"`
	DiscNumber      int     `json:"discNumber"`
	TrackNumber     int     `json:"trackNumber"`
	Name            string  `json:"name"`
	DurationSeconds int     `json:"durationSeconds"`
	TrackArtist     *string `json:"trackArtist"`
	MediaFileID     *string `json:"mediaFileId"`
	AlbumName       string  `json:"albumName"`
	ArtistID        string  `json:"artistId"`
	ArtistName      string  `json:"artistName"`
	CoverID         *string `json:"coverId"`
}

type ArtistCard struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	AlbumCount int    `json:"albumCount"`
}

const albumCardSelect = `
	SELECT al.id, al.name, al.year, ar.id, ar.name,
		(SELECT a.id FROM artwork a WHERE a.owner_kind = 'album' AND a.owner_id = al.id::text AND a.kind = 'album_cover'),
		(SELECT count(*) FROM tracks t WHERE t.album_id = al.id)
	FROM albums al
	JOIN artists ar ON ar.id = al.artist_id`

func (s *Store) scanAlbumCards(ctx context.Context, query string, args ...any) ([]AlbumCard, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cards := []AlbumCard{}
	for rows.Next() {
		var c AlbumCard
		if err := rows.Scan(&c.ID, &c.Name, &c.Year, &c.ArtistID, &c.ArtistName, &c.CoverID, &c.TrackCount); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

func (s *Store) RecentAlbums(ctx context.Context, limit int) ([]AlbumCard, error) {
	return s.scanAlbumCards(ctx, albumCardSelect+`
		WHERE al.status = 'published'
		ORDER BY al.added_at DESC LIMIT $1`, limit)
}

func (s *Store) AlbumsByArtist(ctx context.Context, artistID string) ([]AlbumCard, error) {
	return s.scanAlbumCards(ctx, albumCardSelect+`
		WHERE al.status = 'published' AND al.artist_id = $1
		ORDER BY al.year DESC NULLS LAST, al.name`, artistID)
}

func (s *Store) AlbumCardByID(ctx context.Context, id string) (*AlbumCard, error) {
	cards, err := s.scanAlbumCards(ctx, albumCardSelect+` WHERE al.id = $1`, id)
	if err != nil {
		return nil, err
	}
	if len(cards) == 0 {
		return nil, db.ErrNotFound
	}
	return &cards[0], nil
}

func (s *Store) ListArtists(ctx context.Context) ([]ArtistCard, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT ar.id, ar.name, count(al.id)
		 FROM artists ar
		 JOIN albums al ON al.artist_id = ar.id AND al.status = 'published'
		 GROUP BY ar.id
		 ORDER BY ar.sort_name, ar.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	artists := []ArtistCard{}
	for rows.Next() {
		var a ArtistCard
		if err := rows.Scan(&a.ID, &a.Name, &a.AlbumCount); err != nil {
			return nil, err
		}
		artists = append(artists, a)
	}
	return artists, rows.Err()
}

func (s *Store) ArtistByID(ctx context.Context, id string) (*ArtistCard, error) {
	var a ArtistCard
	err := s.pool.QueryRow(ctx,
		`SELECT ar.id, ar.name,
			(SELECT count(*) FROM albums al WHERE al.artist_id = ar.id AND al.status = 'published')
		 FROM artists ar WHERE ar.id = $1`, id).Scan(&a.ID, &a.Name, &a.AlbumCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

const trackItemSelect = `
	SELECT t.id, t.album_id, t.disc_number, t.track_number, t.name, t.duration_seconds,
		t.track_artist, mf.id, al.name, ar.id, ar.name,
		(SELECT a.id FROM artwork a WHERE a.owner_kind = 'album' AND a.owner_id = al.id::text AND a.kind = 'album_cover')
	FROM tracks t
	JOIN albums al ON al.id = t.album_id
	JOIN artists ar ON ar.id = al.artist_id
	LEFT JOIN media_files mf ON mf.track_id = t.id`

func (s *Store) scanTrackItems(ctx context.Context, query string, args ...any) ([]TrackItem, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tracks := []TrackItem{}
	for rows.Next() {
		var t TrackItem
		if err := rows.Scan(&t.ID, &t.AlbumID, &t.DiscNumber, &t.TrackNumber, &t.Name,
			&t.DurationSeconds, &t.TrackArtist, &t.MediaFileID, &t.AlbumName, &t.ArtistID,
			&t.ArtistName, &t.CoverID); err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

func (s *Store) TracksForAlbum(ctx context.Context, albumID string) ([]TrackItem, error) {
	return s.scanTrackItems(ctx, trackItemSelect+`
		WHERE t.album_id = $1
		ORDER BY t.disc_number, t.track_number, t.name`, albumID)
}

// RecentlyPlayedAlbums powers the music home row.
func (s *Store) RecentlyPlayedAlbums(ctx context.Context, userID int64, limit int) ([]AlbumCard, error) {
	return s.scanAlbumCards(ctx, albumCardSelect+`
		WHERE al.status = 'published' AND al.id IN (
			SELECT DISTINCT t.album_id FROM play_history ph
			JOIN tracks t ON t.id = ph.track_id
			WHERE ph.user_id = $1
		)
		ORDER BY (
			SELECT max(ph2.started_at) FROM play_history ph2
			JOIN tracks t2 ON t2.id = ph2.track_id
			WHERE ph2.user_id = $1 AND t2.album_id = al.id
		) DESC
		LIMIT $2`, userID, limit)
}

func (s *Store) RecordPlay(ctx context.Context, userID int64, trackID string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO play_history (user_id, track_id) VALUES ($1, $2)`, userID, trackID)
	return err
}

// Admin music management

type AdminAlbumRow struct {
	AlbumCard
	Status    string    `json:"status"`
	SizeBytes int64     `json:"sizeBytes"`
	AddedAt   time.Time `json:"addedAt"`
}

var adminAlbumSorts = map[string]string{
	"":      "al.added_at DESC",
	"added": "al.added_at DESC",
	"name":  "al.name ASC",
	"year":  "al.year DESC NULLS LAST",
	"size":  "size_bytes DESC",
}

func (s *Store) AdminListAlbums(ctx context.Context, query, sort string, page, pageSize int) ([]AdminAlbumRow, int, error) {
	orderBy, ok := adminAlbumSorts[sort]
	if !ok {
		return nil, 0, fmt.Errorf("invalid sort %q", sort)
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	if page < 1 {
		page = 1
	}

	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM albums al WHERE $1 = '' OR al.name ILIKE '%' || $1 || '%'`,
		query).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT al.id, al.name, al.year, ar.id, ar.name,
			(SELECT a.id FROM artwork a WHERE a.owner_kind = 'album' AND a.owner_id = al.id::text AND a.kind = 'album_cover'),
			(SELECT count(*) FROM tracks t WHERE t.album_id = al.id),
			al.status, al.added_at,
			COALESCE((SELECT sum(mf.size_bytes) FROM media_files mf
				JOIN tracks t ON t.id = mf.track_id WHERE t.album_id = al.id), 0) AS size_bytes
		FROM albums al
		JOIN artists ar ON ar.id = al.artist_id
		WHERE $1 = '' OR al.name ILIKE '%' || $1 || '%'
		ORDER BY `+orderBy+`
		LIMIT $2 OFFSET $3`,
		query, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []AdminAlbumRow{}
	for rows.Next() {
		var r AdminAlbumRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Year, &r.ArtistID, &r.ArtistName, &r.CoverID,
			&r.TrackCount, &r.Status, &r.AddedAt, &r.SizeBytes); err != nil {
			return nil, 0, err
		}
		items = append(items, r)
	}
	return items, total, rows.Err()
}

func (s *Store) AdminAlbumByID(ctx context.Context, id string) (*AdminAlbumRow, error) {
	var r AdminAlbumRow
	err := s.pool.QueryRow(ctx, `
		SELECT al.id, al.name, al.year, ar.id, ar.name,
			(SELECT a.id FROM artwork a WHERE a.owner_kind = 'album' AND a.owner_id = al.id::text AND a.kind = 'album_cover'),
			(SELECT count(*) FROM tracks t WHERE t.album_id = al.id),
			al.status, al.added_at,
			COALESCE((SELECT sum(mf.size_bytes) FROM media_files mf
				JOIN tracks t ON t.id = mf.track_id WHERE t.album_id = al.id), 0)
		FROM albums al
		JOIN artists ar ON ar.id = al.artist_id
		WHERE al.id = $1`, id).
		Scan(&r.ID, &r.Name, &r.Year, &r.ArtistID, &r.ArtistName, &r.CoverID,
			&r.TrackCount, &r.Status, &r.AddedAt, &r.SizeBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

type AlbumUpdate struct {
	Name       *string `json:"name"`
	Year       *int    `json:"year"`
	Status     *string `json:"status"`
	ArtistName *string `json:"artistName"`
}

func (s *Store) UpdateAlbum(ctx context.Context, id string, up AlbumUpdate) error {
	if up.ArtistName != nil && *up.ArtistName != "" {
		artistID, err := s.UpsertArtist(ctx, *up.ArtistName)
		if err != nil {
			return err
		}
		if _, err := s.pool.Exec(ctx,
			`UPDATE albums SET artist_id = $2 WHERE id = $1`, id, artistID); err != nil {
			return err
		}
	}
	tag, err := s.pool.Exec(ctx,
		`UPDATE albums SET
			name = COALESCE($2, name),
			year = COALESCE($3, year),
			status = COALESCE($4, status)
		 WHERE id = $1`,
		id, up.Name, up.Year, up.Status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteAlbum(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM albums WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}

func (s *Store) UpdateTrackName(ctx context.Context, id string, name string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE tracks SET name = $2 WHERE id = $1`, id, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteTrack(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM tracks WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}

// UpsertArtist returns the artist id for a name, creating it when new.
func (s *Store) UpsertArtist(ctx context.Context, name string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO artists (name, sort_name) VALUES ($1, $1)
		 ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id`, name).Scan(&id)
	return id, err
}

func (s *Store) UpsertAlbum(ctx context.Context, artistID string, name string, year int) (string, error) {
	var yearVal *int
	if year > 0 {
		yearVal = &year
	}
	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO albums (artist_id, name, year, status) VALUES ($1, $2, $3, 'published')
		 ON CONFLICT (artist_id, name) DO UPDATE
			SET year = COALESCE(albums.year, EXCLUDED.year)
		 RETURNING id`, artistID, name, yearVal).Scan(&id)
	return id, err
}

func (s *Store) UpsertTrack(ctx context.Context, albumID string, disc, num int, name string, durationSeconds int, trackArtist string) (string, error) {
	var artistVal *string
	if trackArtist != "" {
		artistVal = &trackArtist
	}
	var id string
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

func (s *Store) SetAlbumGenre(ctx context.Context, albumID string, genre string) error {
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
