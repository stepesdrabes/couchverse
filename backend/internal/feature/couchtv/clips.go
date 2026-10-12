package couchtv

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/db"
)

// Clip kinds (couchtv_clips.kind).
const (
	ClipIdentFull    = "ident_full"
	ClipIdentShort   = "ident_short"
	ClipSting        = "sting"
	ClipPromoBody    = "promo_body"
	ClipPromoEnd     = "promo_end"
	ClipEvergreenEnd = "evergreen_end"
	ClipShortPromo   = "short_promo"
	ClipStripPromo   = "strip_promo"
	ClipUpNext       = "upnext"
	ClipLineup       = "lineup"
	ClipSqueeze      = "squeeze"
	ClipStandby      = "standby"
)

// Clip states.
const (
	ClipPending   = "pending"
	ClipRendering = "rendering"
	ClipReady     = "ready"
	ClipFailed    = "failed"
)

// Break states: composed (clips requested), rendering (being assembled), ready, failed.
const (
	BreakComposed  = "composed"
	BreakRendering = "rendering"
	BreakReady     = "ready"
	BreakFailed    = "failed"
)

// Clip is a cutscene clip: wanted by a break, rendered once and shared by every break with
// the same key.
type Clip struct {
	ID    string
	Kind  string
	Key   string
	State string
	Spec  json.RawMessage
	// Frames is the length the composer counted on; once ready, the rendered length.
	Frames int
	// Path is the rendered package under DATA_DIR (couchtv/clips/<key>).
	Path    string
	Version int
}

func (s *Store) Clip(ctx context.Context, id string) (*Clip, error) {
	var c Clip
	var ms int
	err := s.q.QueryRow(ctx, `SELECT id, kind, key, state, spec, duration_ms, path, version FROM couchtv_clips WHERE id = $1`, id).
		Scan(&c.ID, &c.Kind, &c.Key, &c.State, &c.Spec, &ms, &c.Path, &c.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	c.Frames = ms * FPS / 1000
	return &c, err
}

// RequestClip makes sure a clip exists for key and marks it in use: when a break first
// needs it (the earliest wins) and now. It reports the clip's id and state.
func (s *Store) RequestClip(ctx context.Context, kind, key string, spec Spec, frames int, needed time.Time) (id, state string, err error) {
	body, err := json.Marshal(spec)
	if err != nil {
		return "", "", err
	}
	err = s.q.QueryRow(ctx, `
		INSERT INTO couchtv_clips (kind, key, spec, duration_ms, needed_at, last_used_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (key) DO UPDATE SET last_used_at = now(),
			needed_at = CASE WHEN couchtv_clips.state = 'ready' THEN couchtv_clips.needed_at
			                 ELSE least(couchtv_clips.needed_at, EXCLUDED.needed_at) END
		RETURNING id, state`, kind, key, body, frames*1000/FPS, needed).Scan(&id, &state)
	return id, state, err
}

// RetryClip puts a failed clip back in the queue.
func (s *Store) RetryClip(ctx context.Context, id string) error {
	_, err := s.q.Exec(ctx, `UPDATE couchtv_clips SET state = 'pending', error = '', updated_at = now() WHERE id = $1 AND state = 'failed'`, id)
	return err
}

// ReadyFrames maps the given keys of rendered clips to their length in frames.
func (s *Store) ReadyFrames(ctx context.Context, keys []string) (map[string]int, error) {
	rows, err := s.q.Query(ctx, `SELECT key, duration_ms FROM couchtv_clips WHERE key = ANY($1) AND state = 'ready'`, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var key string
		var ms int
		if err := rows.Scan(&key, &ms); err != nil {
			return nil, err
		}
		out[key] = ms * FPS / 1000
	}
	return out, rows.Err()
}

func (s *Store) setClipState(ctx context.Context, id, state, msg string) error {
	_, err := s.q.Exec(ctx, `UPDATE couchtv_clips SET state = $2, error = $3, updated_at = now() WHERE id = $1`, id, state, msg)
	return err
}

// clipRendered records a finished render.
func (s *Store) clipRendered(ctx context.Context, id string, frames int, path string, size int64, version int, took time.Duration) error {
	_, err := s.q.Exec(ctx, `
		UPDATE couchtv_clips SET state = 'ready', error = '', updated_at = now(), duration_ms = $2, path = $3, size_bytes = $4, version = $5,
		       render_ms = $6, rendered_at = now()
		WHERE id = $1`, id, frames*1000/FPS, path, size, version, took.Milliseconds())
	return err
}

// Break is a break's composition and, once assembled, its package.
type Break struct {
	ID       string
	AiringID string // empty for the evergreen break
	Kind     string
	State    string
	Recipe   Recipe
	// StartsAt is when it airs; zero for the evergreen break.
	StartsAt time.Time
	Path     string
	Frames   int
}

// Recipe is what couchtv_breaks.recipe stores: the clips in order and the break's sound.
type Recipe struct {
	// Key identifies the composition (the evergreen break is recomposed when it changes).
	Key    string       `json:"key"`
	Frames int          `json:"frames"`
	Clips  []RecipeClip `json:"clips"`
	Sound  BreakSpec    `json:"sound"`
	Promos []string     `json:"promos"`
}

// RecipeClip is one clip of a recipe, placed in frames.
type RecipeClip struct {
	ClipID string `json:"clipId"`
	Kind   string `json:"kind"`
	At     int    `json:"at"`
	Frames int    `json:"frames"`
}

const breakCols = `b.id, coalesce(b.airing_id::text, ''), b.kind, b.state, b.recipe, coalesce(a.starts_at, 'epoch'), b.path, b.duration_ms`

func scanBreak(row pgx.Row) (*Break, error) {
	var b Break
	var recipe []byte
	var ms int
	err := row.Scan(&b.ID, &b.AiringID, &b.Kind, &b.State, &recipe, &b.StartsAt, &b.Path, &ms)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if b.AiringID == "" {
		b.StartsAt = time.Time{}
	}
	b.Frames = ms * FPS / 1000
	return &b, json.Unmarshal(recipe, &b.Recipe)
}

func (s *Store) Break(ctx context.Context, id string) (*Break, error) {
	return scanBreak(s.q.QueryRow(ctx, `SELECT `+breakCols+` FROM couchtv_breaks b
		LEFT JOIN couchtv_airings a ON a.id = b.airing_id WHERE b.id = $1`, id))
}

// ReadyBreak is the assembled break of an airing, or db.ErrNotFound while it is not ready.
func (s *Store) ReadyBreak(ctx context.Context, airingID string) (*Break, error) {
	return scanBreak(s.q.QueryRow(ctx, `SELECT `+breakCols+` FROM couchtv_breaks b
		LEFT JOIN couchtv_airings a ON a.id = b.airing_id WHERE b.airing_id = $1 AND b.state = 'ready'`, airingID))
}

// Evergreen is the newest assembled evergreen break: what the live stream airs, cut to
// the gap, where a break is not ready.
func (s *Store) Evergreen(ctx context.Context) (*Break, error) {
	return scanBreak(s.q.QueryRow(ctx, `SELECT `+breakCols+` FROM couchtv_breaks b
		LEFT JOIN couchtv_airings a ON a.id = b.airing_id
		WHERE b.kind = 'evergreen' AND b.state = 'ready' ORDER BY b.created_at DESC LIMIT 1`))
}

// Standby is the newest rendered standby card ("Technical difficulties"), a seamless
// loop the live stream repeats when it has nothing else.
func (s *Store) Standby(ctx context.Context) (*Clip, error) {
	var id string
	err := s.q.QueryRow(ctx, `SELECT id FROM couchtv_clips WHERE kind = 'standby' AND state = 'ready'
		ORDER BY rendered_at DESC LIMIT 1`).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.Clip(ctx, id)
}

// saveBreak stores an airing's composition (or a new evergreen one, airingID empty).
func (s *Store) saveBreak(ctx context.Context, airingID string, r Recipe, state, msg string) (string, error) {
	body, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	kind := "airing"
	var airing *string
	if airingID != "" {
		airing = &airingID
	} else {
		kind = "evergreen"
	}
	var id string
	err = s.q.QueryRow(ctx, `
		INSERT INTO couchtv_breaks (airing_id, kind, recipe, state, error, duration_ms)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (airing_id) DO UPDATE SET recipe = EXCLUDED.recipe, state = EXCLUDED.state, error = EXCLUDED.error,
			duration_ms = EXCLUDED.duration_ms, path = '', updated_at = now()
		RETURNING id`, airing, kind, body, state, msg, r.Frames*1000/FPS).Scan(&id)
	return id, err
}

func (s *Store) setBreakState(ctx context.Context, id, state, msg string) error {
	_, err := s.q.Exec(ctx, `UPDATE couchtv_breaks SET state = $2, error = $3, updated_at = now() WHERE id = $1`, id, state, msg)
	return err
}

func (s *Store) breakAssembled(ctx context.Context, id, path string, frames int, size int64) error {
	_, err := s.q.Exec(ctx, `
		UPDATE couchtv_breaks SET state = 'ready', error = '', path = $2, duration_ms = $3, size_bytes = $4,
		       assembled_at = now(), updated_at = now()
		WHERE id = $1`, id, path, frames*1000/FPS, size)
	return err
}

// assemblable lists the composed breaks using a clip whose clips are all rendered now, with
// when they air (zero for the evergreen break).
func (s *Store) assemblable(ctx context.Context, clipID string) ([]Break, error) {
	rows, err := s.q.Query(ctx, `
		SELECT `+breakCols+` FROM couchtv_breaks b LEFT JOIN couchtv_airings a ON a.id = b.airing_id
		WHERE b.state = 'composed'
		  AND b.recipe->'clips' @> jsonb_build_array(jsonb_build_object('clipId', $1::text))
		  AND NOT EXISTS (SELECT 1 FROM jsonb_array_elements(b.recipe->'clips') c
		                  JOIN couchtv_clips k ON k.id = (c->>'clipId')::uuid WHERE k.state <> 'ready')`, clipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Break
	for rows.Next() {
		b, err := scanBreak(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *b)
	}
	return out, rows.Err()
}
