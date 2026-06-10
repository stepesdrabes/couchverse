package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/db"
)

type Playlist struct {
	ID         string    `json:"id"`
	UserID     int64     `json:"userId"`
	Name       string    `json:"name"`
	TrackCount int       `json:"trackCount"`
	CoverID    *string   `json:"coverId"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type PlaylistEntry struct {
	EntryID  string    `json:"entryId"`
	Position int       `json:"position"`
	Track    TrackItem `json:"track"`
}

const playlistSelect = `
	SELECT p.id, p.user_id, p.name,
		(SELECT count(*) FROM playlist_tracks pt WHERE pt.playlist_id = p.id),
		(SELECT a.id FROM artwork a
		 JOIN tracks t ON t.album_id::text = a.owner_id
		 JOIN playlist_tracks pt ON pt.track_id = t.id
		 WHERE pt.playlist_id = p.id AND a.owner_kind = 'album' AND a.kind = 'album_cover'
		 ORDER BY pt.position LIMIT 1),
		p.created_at, p.updated_at
	FROM playlists p`

func scanPlaylist(row pgx.Row) (*Playlist, error) {
	var p Playlist
	err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.TrackCount, &p.CoverID, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) ListPlaylists(ctx context.Context, userID int64) ([]Playlist, error) {
	rows, err := s.pool.Query(ctx, playlistSelect+`
		WHERE p.user_id = $1 ORDER BY p.updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	playlists := []Playlist{}
	for rows.Next() {
		p, err := scanPlaylist(rows)
		if err != nil {
			return nil, err
		}
		playlists = append(playlists, *p)
	}
	return playlists, rows.Err()
}

// PlaylistForUser fetches a playlist owned by the user (404 otherwise).
func (s *Store) PlaylistForUser(ctx context.Context, userID int64, playlistID string) (*Playlist, error) {
	return scanPlaylist(s.pool.QueryRow(ctx, playlistSelect+`
		WHERE p.id = $1 AND p.user_id = $2`, playlistID, userID))
}

func (s *Store) CreatePlaylist(ctx context.Context, userID int64, name string) (*Playlist, error) {
	return scanPlaylist(s.pool.QueryRow(ctx,
		`WITH ins AS (INSERT INTO playlists (user_id, name) VALUES ($1, $2) RETURNING *)
		 SELECT id, user_id, name, 0, NULL::uuid, created_at, updated_at FROM ins`, userID, name))
}

func (s *Store) RenamePlaylist(ctx context.Context, userID int64, playlistID string, name string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE playlists SET name = $3, updated_at = now() WHERE id = $1 AND user_id = $2`,
		playlistID, userID, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}

func (s *Store) DeletePlaylist(ctx context.Context, userID int64, playlistID string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM playlists WHERE id = $1 AND user_id = $2`, playlistID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}

func (s *Store) PlaylistEntries(ctx context.Context, playlistID string) ([]PlaylistEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT pt.id, pt.position,
			t.id, t.album_id, t.disc_number, t.track_number, t.name, t.duration_seconds,
			t.track_artist, mf.id, al.name, ar.id, ar.name,
			(SELECT a.id FROM artwork a WHERE a.owner_kind = 'album' AND a.owner_id = al.id::text AND a.kind = 'album_cover')
		FROM playlist_tracks pt
		JOIN tracks t ON t.id = pt.track_id
		JOIN albums al ON al.id = t.album_id
		JOIN artists ar ON ar.id = al.artist_id
		LEFT JOIN media_files mf ON mf.track_id = t.id
		WHERE pt.playlist_id = $1
		ORDER BY pt.position`, playlistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []PlaylistEntry{}
	for rows.Next() {
		var e PlaylistEntry
		t := &e.Track
		if err := rows.Scan(&e.EntryID, &e.Position,
			&t.ID, &t.AlbumID, &t.DiscNumber, &t.TrackNumber, &t.Name, &t.DurationSeconds,
			&t.TrackArtist, &t.MediaFileID, &t.AlbumName, &t.ArtistID, &t.ArtistName, &t.CoverID); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *Store) AddPlaylistTrack(ctx context.Context, playlistID, trackID string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO playlist_tracks (playlist_id, track_id, position)
		 VALUES ($1, $2, COALESCE((SELECT max(position) FROM playlist_tracks WHERE playlist_id = $1), 0) + 1)`,
		playlistID, trackID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `UPDATE playlists SET updated_at = now() WHERE id = $1`, playlistID)
	return err
}

func (s *Store) RemovePlaylistEntry(ctx context.Context, playlistID, entryID string) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM playlist_tracks WHERE id = $1 AND playlist_id = $2`, entryID, playlistID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}

// ReorderPlaylist rewrites entry positions to match the given entry id order.
func (s *Store) ReorderPlaylist(ctx context.Context, playlistID string, entryIDs []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for i, entryID := range entryIDs {
		if _, err := tx.Exec(ctx,
			`UPDATE playlist_tracks SET position = $3 WHERE id = $1 AND playlist_id = $2`,
			entryID, playlistID, i+1); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE playlists SET updated_at = now() WHERE id = $1`, playlistID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
