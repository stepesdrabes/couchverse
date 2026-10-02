package auth

import (
	"context"
	"time"

	"couchverse/internal/db"
)

// NewSession describes a session to create: a browser (cookie) or a named
// device (bearer token).
type NewSession struct {
	TokenHash  []byte
	UserID     int64
	Kind       string // browser | device
	DeviceName string
	Platform   string
	UserAgent  string
}

// CreateSession stores a session that expires SessionTTL from now and returns its
// public id.
func (s *Store) CreateSession(ctx context.Context, n NewSession) (string, error) {
	var id string
	err := s.db.QueryRow(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at, user_agent, kind, device_name, platform)
		 VALUES ($1, $2, now() + make_interval(secs => $3), $4, $5, $6, $7)
		 RETURNING id`,
		n.TokenHash, n.UserID, SessionTTL.Seconds(), n.UserAgent, n.Kind, n.DeviceName, n.Platform).Scan(&id)
	return id, err
}

// UserBySession resolves a token hash to its active, non-expired session and
// user. Expiry slides: a session used within SessionTTL never expires, but the
// row is only rewritten when last_seen_at is 5 minutes stale or the expiry is
// more than a day behind, so busy clients do not write on every request.
// extended reports that the expiry moved, so a browser's cookie is reissued.
func (s *Store) UserBySession(ctx context.Context, tokenHash []byte) (u *User, sess *Session, extended bool, err error) {
	sess = &Session{}
	var lastSeen time.Time
	u, err = scanUser(s.db.QueryRow(ctx,
		`SELECT `+userColumns+`, se.id, se.kind, se.expires_at, se.last_seen_at`+userFrom+`
		 JOIN sessions se ON se.user_id = u.id
		 WHERE se.token_hash = $1 AND se.expires_at > now() AND NOT u.disabled`,
		tokenHash), &sess.ID, &sess.Kind, &sess.ExpiresAt, &lastSeen)
	if err != nil {
		return nil, nil, false, err
	}

	now := time.Now()
	extended = sess.ExpiresAt.Before(now.Add(SessionTTL - 24*time.Hour))
	if extended || now.Sub(lastSeen) > 5*time.Minute {
		err = s.db.QueryRow(ctx,
			`UPDATE sessions SET last_seen_at = now(),
			   expires_at = CASE WHEN $2 THEN now() + make_interval(secs => $3) ELSE expires_at END
			 WHERE token_hash = $1
			 RETURNING expires_at`,
			tokenHash, extended, SessionTTL.Seconds()).Scan(&sess.ExpiresAt)
		if err != nil {
			return nil, nil, false, err
		}
	}
	return u, sess, extended, nil
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}

// DeleteUserSession revokes one of the user's own sessions.
func (s *Store) DeleteUserSession(ctx context.Context, userID int64, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM sessions WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}

// SessionRow is a session as the devices list shows it.
type SessionRow struct {
	ID         string
	Kind       string
	DeviceName string
	Platform   string
	UserAgent  string
	CreatedAt  time.Time
	LastSeenAt time.Time
}

// UserSessions lists the user's unexpired sessions, most recently used first.
func (s *Store) UserSessions(ctx context.Context, userID int64) ([]SessionRow, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, kind, device_name, platform, user_agent, created_at, last_seen_at
		 FROM sessions WHERE user_id = $1 AND expires_at > now()
		 ORDER BY last_seen_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SessionRow{}
	for rows.Next() {
		var r SessionRow
		if err := rows.Scan(&r.ID, &r.Kind, &r.DeviceName, &r.Platform, &r.UserAgent, &r.CreatedAt, &r.LastSeenAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	return tag.RowsAffected(), err
}
