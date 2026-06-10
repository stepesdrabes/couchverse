package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/httpx"
)

func (s *Store) CreateSession(ctx context.Context, tokenHash []byte, userID int64, expiresAt time.Time, userAgent string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at, user_agent) VALUES ($1, $2, $3, $4)`,
		tokenHash, userID, expiresAt, userAgent)
	return err
}

// UserBySession resolves a session token hash to its (active, non-expired)
// user and touches last_seen_at at most every 5 minutes.
func (s *Store) UserBySession(ctx context.Context, tokenHash []byte) (*User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, userSelect+`
		 JOIN sessions se ON se.user_id = u.id
		 WHERE se.token_hash = $1 AND se.expires_at > now() AND NOT u.disabled`,
		tokenHash))
	if err != nil {
		return nil, err
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE sessions SET last_seen_at = now()
		 WHERE token_hash = $1 AND last_seen_at < now() - interval '5 minutes'`,
		tokenHash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	return u, nil
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash []byte) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	return tag.RowsAffected(), err
}
