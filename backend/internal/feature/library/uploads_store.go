package library

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/db"
)

type UploadSession struct {
	ID            string    `json:"id"`
	UserID        int64     `json:"userId"`
	Filename      string    `json:"filename"`
	DeclaredSize  int64     `json:"declaredSize"`
	ReceivedBytes int64     `json:"receivedBytes"`
	TempPath      string    `json:"-"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	ExpiresAt     time.Time `json:"expiresAt"`
}

const uploadCols = `id, user_id, filename, declared_size, received_bytes, temp_path, status,
	created_at, updated_at, expires_at`

func scanUpload(row pgx.Row) (*UploadSession, error) {
	var u UploadSession
	err := row.Scan(&u.ID, &u.UserID, &u.Filename, &u.DeclaredSize, &u.ReceivedBytes, &u.TempPath,
		&u.Status, &u.CreatedAt, &u.UpdatedAt, &u.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) CreateUploadSession(ctx context.Context, id string, userID int64, filename string, size int64, tempPath string) (*UploadSession, error) {
	return scanUpload(s.db.QueryRow(ctx,
		`INSERT INTO upload_sessions (id, user_id, filename, declared_size, temp_path, expires_at)
		 VALUES ($1, $2, $3, $4, $5, now() + interval '7 days')
		 RETURNING `+uploadCols,
		id, userID, filename, size, tempPath))
}

func (s *Store) UploadSession(ctx context.Context, id string) (*UploadSession, error) {
	return scanUpload(s.db.QueryRow(ctx,
		`SELECT `+uploadCols+` FROM upload_sessions WHERE id = $1`, id))
}

func (s *Store) SetUploadReceived(ctx context.Context, id string, received int64) error {
	_, err := s.db.Exec(ctx,
		`UPDATE upload_sessions SET received_bytes = $2, updated_at = now() WHERE id = $1`, id, received)
	return err
}

func (s *Store) SetUploadStatus(ctx context.Context, id, status string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE upload_sessions SET status = $2, updated_at = now() WHERE id = $1`, id, status)
	return err
}

func (s *Store) ActiveUploadSessions(ctx context.Context) ([]UploadSession, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+uploadCols+` FROM upload_sessions WHERE status = 'active' ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := []UploadSession{}
	for rows.Next() {
		u, err := scanUpload(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *u)
	}
	return sessions, rows.Err()
}

// ExpiredUploadSessions returns sessions to reap (cleanup job).
func (s *Store) ExpiredUploadSessions(ctx context.Context) ([]UploadSession, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+uploadCols+` FROM upload_sessions
		 WHERE (status = 'active' AND expires_at < now()) OR status = 'aborted'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := []UploadSession{}
	for rows.Next() {
		u, err := scanUpload(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *u)
	}
	return sessions, rows.Err()
}

func (s *Store) DeleteUploadSession(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM upload_sessions WHERE id = $1`, id)
	return err
}

// CreateAssignedMediaFile registers an uploaded file that may carry an explicit
// title/episode assignment (kept by the probe job).
func (s *Store) CreateAssignedMediaFile(ctx context.Context, libraryID int64, path string, size int64, titleID, episodeID *string) (string, error) {
	var id string
	err := s.db.QueryRow(ctx,
		`INSERT INTO media_files (library_id, path, size_bytes, file_mtime, title_id, episode_id)
		 VALUES ($1, $2, $3, now(), $4, $5)
		 ON CONFLICT (library_id, path) DO UPDATE
			SET size_bytes = EXCLUDED.size_bytes, file_mtime = now(), scanned_at = NULL,
				title_id = EXCLUDED.title_id, episode_id = EXCLUDED.episode_id
		 RETURNING id`,
		libraryID, path, size, titleID, episodeID).Scan(&id)
	return id, err
}
