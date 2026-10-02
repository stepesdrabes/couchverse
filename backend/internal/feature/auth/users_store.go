package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"couchverse/internal/db"
)

// Store owns users and sessions SQL over the shared pool.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	DisplayName  string    `json:"displayName"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role" enum:"admin,member"`
	Disabled     bool      `json:"disabled"`
	AvatarID     *string   `json:"avatarId" doc:"Artwork id of the profile picture."`
	BannerID     *string   `json:"bannerId" doc:"Artwork id of the profile banner."`
	Bio          string    `json:"bio" doc:"Markdown, rendered with raw HTML disabled."`
	CreatedAt    time.Time `json:"createdAt"`
}

const (
	userColumns = `u.id, u.username, u.display_name, u.password_hash, u.role, u.disabled,
	       av.id, bn.id, u.bio, u.created_at`
	userFrom = `
	FROM users u
	LEFT JOIN artwork av ON av.owner_kind = 'user' AND av.owner_id = u.id::text AND av.kind = 'avatar'
	LEFT JOIN artwork bn ON bn.owner_kind = 'user' AND bn.owner_id = u.id::text AND bn.kind = 'banner'`
	userSelect = `SELECT ` + userColumns + userFrom
)

// scanUser reads userColumns, then any extra columns selected after them.
func scanUser(row pgx.Row, extra ...any) (*User, error) {
	var u User
	dest := []any{&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &u.Role, &u.Disabled,
		&u.AvatarID, &u.BannerID, &u.Bio, &u.CreatedAt}
	err := row.Scan(append(dest, extra...)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) UserByUsername(ctx context.Context, username string) (*User, error) {
	return scanUser(s.db.QueryRow(ctx, userSelect+` WHERE u.username = $1`, username))
}

func (s *Store) UserByID(ctx context.Context, id int64) (*User, error) {
	return scanUser(s.db.QueryRow(ctx, userSelect+` WHERE u.id = $1`, id))
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}

// ErrUsernameTaken is returned by CreateUser for a username already in use.
var ErrUsernameTaken = errors.New("username already exists")

func (s *Store) CreateUser(ctx context.Context, username, displayName, passwordHash, role string) (*User, error) {
	var id int64
	err := s.db.QueryRow(ctx,
		`INSERT INTO users (username, display_name, password_hash, role)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		username, displayName, passwordHash, role).Scan(&id)
	if db.IsUniqueViolation(err) {
		return nil, ErrUsernameTaken
	}
	if err != nil {
		return nil, err
	}
	return s.UserByID(ctx, id)
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.Query(ctx, userSelect+` ORDER BY u.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []User{}
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
	Bio          *string
}

func (s *Store) UpdateUser(ctx context.Context, id int64, up UserUpdate) (*User, error) {
	tag, err := s.db.Exec(ctx,
		`UPDATE users SET
			display_name = COALESCE($2, display_name),
			role = COALESCE($3, role),
			disabled = COALESCE($4, disabled),
			password_hash = COALESCE($5, password_hash),
			bio = COALESCE($6, bio)
		 WHERE id = $1`,
		id, up.DisplayName, up.Role, up.Disabled, up.PasswordHash, up.Bio)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, db.ErrNotFound
	}
	return s.UserByID(ctx, id)
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}

// Preferences are a user's client settings, stored as a jsonb blob. Only the
// keys modelled here are served; any other stored key stays in the blob
// untouched.
type Preferences struct {
	Subtitles     *SubtitlePreferences `json:"subtitles,omitempty"`
	Language      *string              `json:"language,omitempty" doc:"Saved display language (ISO 639-1), restored on sign-in."`
	PublicProfile *bool                `json:"publicProfile,omitempty" doc:"Appear on public profiles and leaderboards; absent means yes."`
}

// SubtitlePreferences style the player's subtitles. Absent fields fall back to
// the player's defaults.
type SubtitlePreferences struct {
	FontSizePct       *int    `json:"fontSizePct,omitempty" minimum:"50" maximum:"200" doc:"Text size as a percentage of the default."`
	Color             *string `json:"color,omitempty" doc:"Text colour as #rrggbb."`
	FontFamily        *string `json:"fontFamily,omitempty" enum:"sans,serif,mono,rounded"`
	BackgroundOpacity *int    `json:"backgroundOpacity,omitempty" minimum:"0" maximum:"100" doc:"Opacity of the box behind the text, in percent."`
}

// decodePreferences reads the stored blob key by key: the blob predates this
// shape, so a value stored with another type reads as unset rather than
// failing the whole read.
func decodePreferences(raw []byte) *Preferences {
	var keys map[string]json.RawMessage
	_ = json.Unmarshal(raw, &keys)
	p := &Preferences{
		Language:      jsonValue[string](keys["language"]),
		PublicProfile: jsonValue[bool](keys["publicProfile"]),
	}
	var sub map[string]json.RawMessage
	if json.Unmarshal(keys["subtitles"], &sub) == nil && sub != nil {
		p.Subtitles = &SubtitlePreferences{
			FontSizePct:       jsonValue[int](sub["fontSizePct"]),
			Color:             jsonValue[string](sub["color"]),
			FontFamily:        jsonValue[string](sub["fontFamily"]),
			BackgroundOpacity: jsonValue[int](sub["backgroundOpacity"]),
		}
	}
	return p
}

// jsonValue decodes raw as a T, or nil when it is absent, null or another type.
func jsonValue[T any](raw json.RawMessage) *T {
	var v *T
	if json.Unmarshal(raw, &v) != nil {
		return nil
	}
	return v
}

func (s *Store) UserPreferences(ctx context.Context, id int64) (*Preferences, error) {
	var raw []byte
	err := s.db.QueryRow(ctx, `SELECT preferences FROM users WHERE id = $1`, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodePreferences(raw), nil
}

// MergeUserPreferences writes the keys set in patch. An object value (subtitles)
// merges into the stored object rather than replacing it, so keys this server
// does not model survive a client's read-modify-write.
func (s *Store) MergeUserPreferences(ctx context.Context, id int64, patch Preferences) (*Preferences, error) {
	body, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}
	var raw []byte
	err = s.db.QueryRow(ctx,
		`UPDATE users u SET preferences = u.preferences || (
			SELECT COALESCE(jsonb_object_agg(p.key, CASE
				WHEN jsonb_typeof(p.value) = 'object' AND jsonb_typeof(u.preferences -> p.key) = 'object'
				THEN (u.preferences -> p.key) || p.value
				ELSE p.value END), '{}'::jsonb)
			FROM jsonb_each($2::jsonb) p)
		 WHERE u.id = $1
		 RETURNING u.preferences`, id, body).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodePreferences(raw), nil
}

// DeleteUserSessions removes all sessions of a disabled user.
func (s *Store) DeleteUserSessions(ctx context.Context, userID int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete user sessions: %w", err)
	}
	return nil
}
