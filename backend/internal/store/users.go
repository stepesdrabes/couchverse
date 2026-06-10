package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/db"
)

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	DisplayName  string    `json:"displayName"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	Disabled     bool      `json:"disabled"`
	AvatarID     *string   `json:"avatarId"`
	CreatedAt    time.Time `json:"createdAt"`
}

const userSelect = `
	SELECT u.id, u.username, u.display_name, u.password_hash, u.role, u.disabled, av.id, u.created_at
	FROM users u
	LEFT JOIN artwork av ON av.owner_kind = 'user' AND av.owner_id = u.id::text AND av.kind = 'avatar'`

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &u.Role, &u.Disabled, &u.AvatarID, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) UserByUsername(ctx context.Context, username string) (*User, error) {
	return scanUser(s.pool.QueryRow(ctx, userSelect+` WHERE u.username = $1`, username))
}

func (s *Store) UserByID(ctx context.Context, id int64) (*User, error) {
	return scanUser(s.pool.QueryRow(ctx, userSelect+` WHERE u.id = $1`, id))
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) CreateUser(ctx context.Context, username, displayName, passwordHash, role string) (*User, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (username, display_name, password_hash, role)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		username, displayName, passwordHash, role).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.UserByID(ctx, id)
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, userSelect+` ORDER BY u.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}

type UserUpdate struct {
	DisplayName  *string
	Role         *string
	Disabled     *bool
	PasswordHash *string
}

func (s *Store) UpdateUser(ctx context.Context, id int64, up UserUpdate) (*User, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE users SET
			display_name = COALESCE($2, display_name),
			role = COALESCE($3, role),
			disabled = COALESCE($4, disabled),
			password_hash = COALESCE($5, password_hash)
		 WHERE id = $1`,
		id, up.DisplayName, up.Role, up.Disabled, up.PasswordHash)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, db.ErrNotFound
	}
	return s.UserByID(ctx, id)
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}

func (s *Store) UserPreferences(ctx context.Context, id int64) (json.RawMessage, error) {
	var v json.RawMessage
	err := s.pool.QueryRow(ctx, `SELECT preferences FROM users WHERE id = $1`, id).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	return v, err
}

// MergeUserPreferences shallow-merges the given top-level keys into the user's
// preferences blob (a null value removes its key).
func (s *Store) MergeUserPreferences(ctx context.Context, id int64, patch map[string]json.RawMessage) (json.RawMessage, error) {
	body, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}
	var v json.RawMessage
	err = s.pool.QueryRow(ctx,
		`UPDATE users SET preferences = preferences || $2::jsonb WHERE id = $1
		 RETURNING preferences`, id, body).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	return v, err
}

// DeleteUserSessions removes all sessions of a disabled user.
func (s *Store) DeleteUserSessions(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete user sessions: %w", err)
	}
	return nil
}
