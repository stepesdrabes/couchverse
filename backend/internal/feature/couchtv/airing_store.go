package couchtv

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/db"
)

// Removed are the programs from a moment on that an admin took off.
func (s *Store) Removed(ctx context.Context, from time.Time) ([]Airing, error) {
	rows, err := s.q.Query(ctx, `SELECT `+airingCols+` FROM couchtv_airings
		WHERE kind = 'program' AND state = 'replaced' AND starts_at >= $1 ORDER BY starts_at`, from)
	if err != nil {
		return nil, err
	}
	return scanAirings(rows)
}

// Airing reads one row of the schedule.
func (s *Store) Airing(ctx context.Context, id string) (*Airing, error) {
	rows, err := s.airings(ctx, `id = $1`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, db.ErrNotFound
	}
	return &rows[0], nil
}

// programMedia is what an admin's program would air.
type programMedia struct {
	Kind      string // movie or series
	Published bool
	Excluded  bool
	// EpisodeOK: the episode is the series' (or none was given for a movie)
	EpisodeOK bool
	Media     Media // FileID empty when there is nothing to play
}

// ProgramMedia reads a movie (episodeID empty) or an episode of a series as the planner
// would air it: its playable file, credits marker and measured cut. A title that does
// not exist is db.ErrNotFound.
func (s *Store) ProgramMedia(ctx context.Context, titleID, episodeID string) (programMedia, error) {
	var p programMedia
	var fileID string
	var seconds float64
	var credits *int
	var cutMs, cutCredits int64
	err := s.q.QueryRow(ctx, `
		SELECT t.kind, t.status = 'published', coalesce(ct.excluded, false),
		       CASE WHEN $2 = '' THEN t.kind = 'movie' ELSE t.kind = 'series' AND EXISTS (SELECT 1 FROM episodes e
		           JOIN seasons se ON se.id = e.season_id WHERE e.id::text = $2 AND se.title_id = t.id) END,
		       coalesce(mf.id::text, ''), coalesce(mf.duration_seconds::float8, 0), coalesce(`+readySQL+`, false),
		       `+creditsSQL+`, `+cutSQL+`
		FROM titles t
		LEFT JOIN couchtv_titles ct ON ct.title_id = t.id
		LEFT JOIN LATERAL (`+fmt.Sprintf(primaryFile, "($2 = '' AND title_id = t.id OR episode_id::text = $2)")+`) mf ON true
		WHERE t.id = $1`, titleID, episodeID).
		Scan(&p.Kind, &p.Published, &p.Excluded, &p.EpisodeOK, &fileID, &seconds, &p.Media.Ready, &credits, &cutMs, &cutCredits)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, db.ErrNotFound
	}
	if err != nil {
		return p, err
	}
	p.Media.FileID, p.Media.Duration, p.Media.CreditsAt = fileID, secondsToDuration(seconds), msToDuration(credits)
	p.Media.Cut, p.Media.CutCredits = time.Duration(cutMs)*time.Millisecond, time.Duration(cutCredits)*time.Millisecond
	return p, nil
}

// FixedBetween are the programs overlapping [from, to) that replanning keeps (admin
// rows, locked rows and those that started by now), leaving out the given ids.
func (s *Store) FixedBetween(ctx context.Context, from, to, now time.Time, except []string) ([]Airing, error) {
	if except == nil {
		except = []string{} // a nil array is NULL, which would match nothing
	}
	return s.airings(ctx, `kind = 'program' AND starts_at < $2 AND ends_at > $1 AND (source = 'admin' OR locked OR starts_at <= $3)
		AND NOT id::text = ANY($4::text[])`, from, to, now, except)
}

// OnAirReaches reports whether an airing that started by now runs to at or past t: a
// program starting then would change what is already on air (the break leading into
// it announces it).
func (s *Store) OnAirReaches(ctx context.Context, t, now time.Time) (bool, error) {
	var reaches bool
	err := s.q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM couchtv_airings
		WHERE state <> 'replaced' AND starts_at <= $2 AND ends_at >= $1)`, t, now).Scan(&reaches)
	return reaches, err
}

// SetLocked locks a program through replanning, or hands it back.
func (s *Store) SetLocked(ctx context.Context, id string, locked bool) error {
	_, err := s.q.Exec(ctx, `UPDATE couchtv_airings SET locked = $2 WHERE id = $1`, id, locked)
	return err
}

// UpdateProgram stores an admin's program in place of a row (same id): what airs and
// when, as an admin row the planner keeps. The breaks on either side, before and
// after, lose their compositions.
func (s *Store) UpdateProgram(ctx context.Context, old, a Airing) error {
	var credits *int64
	if a.CreditsAt > 0 {
		ms := a.CreditsAt.Milliseconds()
		credits = &ms
	}
	if _, err := s.q.Exec(ctx, `
		UPDATE couchtv_airings SET starts_at = $2, ends_at = $3, title_id = $4, episode_id = NULLIF($5, '')::uuid,
		       media_file_id = NULLIF($6, '')::uuid, fill = NULLIF($7, ''), source = 'admin', state = 'planned', credits_at_ms = $8
		WHERE id = $1`, a.ID, a.StartsAt, a.EndsAt, a.TitleID, a.EpisodeID, a.MediaFileID, a.Fill, credits); err != nil {
		return err
	}
	return s.resetBreaksAround(ctx, []Airing{old, a})
}

// InsertProgram stores an admin's new program and returns its id.
func (s *Store) InsertProgram(ctx context.Context, a Airing) (string, error) {
	var credits *int64
	if a.CreditsAt > 0 {
		ms := a.CreditsAt.Milliseconds()
		credits = &ms
	}
	var id string
	err := s.q.QueryRow(ctx, `
		INSERT INTO couchtv_airings (starts_at, ends_at, kind, title_id, episode_id, media_file_id, slot, fill, source, state, credits_at_ms)
		VALUES ($1, $2, 'program', $3, NULLIF($4, '')::uuid, NULLIF($5, '')::uuid, '', NULLIF($6, ''), 'admin', 'planned', $7)
		RETURNING id`, a.StartsAt, a.EndsAt, a.TitleID, a.EpisodeID, a.MediaFileID, a.Fill, credits).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, s.resetBreaksAround(ctx, []Airing{a})
}

// RemoveProgram takes a program off. The planner's own choice is kept as a removal
// (StateReplaced), so replanning its day does not pick it again; an admin's program is
// deleted, and the planner fills its place as it would have.
func (s *Store) RemoveProgram(ctx context.Context, a Airing) error {
	if a.Source == SourceAdmin {
		_, err := s.q.Exec(ctx, `DELETE FROM couchtv_airings WHERE id = $1`, a.ID)
		return err
	}
	_, err := s.q.Exec(ctx, `UPDATE couchtv_airings SET state = 'replaced', source = 'admin', locked = false WHERE id = $1`, a.ID)
	return err
}
