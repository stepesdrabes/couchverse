package couchtv

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Store owns the couchtv SQL over the shared pool, or over one planning transaction
// (Locked).
type Store struct {
	db *pgxpool.Pool
	q  querier
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db, q: db}
}

// planLock is the advisory lock every replan holds.
const planLock = 0x636f75636874760a

// Locked runs fn in one transaction holding the planning lock, so the plan job and an
// admin's replan never interleave.
func (s *Store) Locked(ctx context.Context, fn func(*Store) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(planLock)); err != nil {
		return err
	}
	if err := fn(&Store{db: s.db, q: tx}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// readySQL says whether media file mf is prepared for the channel. Until the channel
// prepares its own programs (plan 6.3, couchtv_prepare), that is its Playback v2
// package: the copied source and the audio renditions, plus an H.264 rung for any
// source other than 8-bit SDR H.264.
const readySQL = `(EXISTS (SELECT 1 FROM transcode_variants v WHERE v.media_file_id = mf.id
		AND v.name = 'source' AND v.format = 'fmp4' AND v.status = 'ready')
	AND NOT EXISTS (SELECT 1 FROM transcode_variants v WHERE v.media_file_id = mf.id
		AND v.name = 'audio' AND v.status <> 'ready')
	AND (mf.video_codec = 'h264' AND mf.bit_depth <= 8 AND mf.hdr_format = 'sdr'
		OR EXISTS (SELECT 1 FROM transcode_variants v WHERE v.media_file_id = mf.id
			AND v.mode = 'transcode' AND v.format = 'fmp4' AND v.status = 'ready'
			AND v.name NOT IN ('audio', 'trickplay'))))`

// primaryFile is the playable file of a title or episode, as playback picks it.
const primaryFile = `SELECT * FROM media_files WHERE %s ORDER BY (audio_role = 'primary') DESC, height DESC, id LIMIT 1`

// creditsSQL is where the credits of media file mf start (ms), the admin's marker over
// the analysis'; null without one, or when the admin says there are none.
const creditsSQL = `(SELECT mm.start_ms FROM media_markers mm WHERE mm.media_file_id = mf.id AND mm.kind = 'credits'
	ORDER BY mm.source = 'admin' DESC LIMIT 1)`

// Library loads what the channel can air: published titles that are not excluded,
// with a probed file; series by episodes in seasons 1 and up.
func (s *Store) Library(ctx context.Context) (Library, error) {
	genres, err := s.genres(ctx)
	if err != nil {
		return Library{}, err
	}
	lib := Library{Movies: []Movie{}, Series: []Series{}}
	rows, err := s.q.Query(ctx, `
		SELECT t.id, t.name, t.added_at, t.release_date, t.year, coalesce(t.collection_tmdb_id, 0), t.collection_name,
		       mf.id, mf.duration_seconds::float8, `+readySQL+`, `+creditsSQL+`
		FROM titles t
		JOIN LATERAL (`+fmt.Sprintf(primaryFile, "title_id = t.id")+`) mf ON true
		WHERE t.kind = 'movie' AND t.status = 'published' AND mf.duration_seconds > 0
		  AND NOT EXISTS (SELECT 1 FROM couchtv_titles ct WHERE ct.title_id = t.id AND ct.excluded)
		ORDER BY t.id`)
	if err != nil {
		return Library{}, err
	}
	for rows.Next() {
		var m Movie
		var released *time.Time
		var year, credits *int
		var seconds float64
		if err := rows.Scan(&m.ID, &m.Name, &m.AddedAt, &released, &year, &m.CollectionID, &m.CollectionName,
			&m.FileID, &seconds, &m.Ready, &credits); err != nil {
			rows.Close()
			return Library{}, err
		}
		m.CreditsAt = msToDuration(credits)
		switch {
		case released != nil:
			m.Released = *released
		case year != nil:
			m.Released = time.Date(*year, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		m.Duration = secondsToDuration(seconds)
		m.Genres = genres[m.ID]
		lib.Movies = append(lib.Movies, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Library{}, err
	}

	rows, err = s.q.Query(ctx, `
		SELECT t.id, t.name, e.id, s.season_number, e.episode_number, mf.id, mf.duration_seconds::float8, `+readySQL+`,
		       `+creditsSQL+`
		FROM titles t
		JOIN seasons s ON s.title_id = t.id AND s.season_number > 0
		JOIN episodes e ON e.season_id = s.id
		JOIN LATERAL (`+fmt.Sprintf(primaryFile, "episode_id = e.id")+`) mf ON true
		WHERE t.kind = 'series' AND t.status = 'published' AND mf.duration_seconds > 0
		  AND NOT EXISTS (SELECT 1 FROM couchtv_titles ct WHERE ct.title_id = t.id AND ct.excluded)
		ORDER BY t.id, s.season_number, e.episode_number`)
	if err != nil {
		return Library{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var seriesID, name string
		var e Episode
		var seconds float64
		var credits *int
		if err := rows.Scan(&seriesID, &name, &e.ID, &e.Season, &e.Number, &e.FileID, &seconds, &e.Ready, &credits); err != nil {
			return Library{}, err
		}
		e.Duration, e.CreditsAt = secondsToDuration(seconds), msToDuration(credits)
		if n := len(lib.Series); n == 0 || lib.Series[n-1].ID != seriesID {
			lib.Series = append(lib.Series, Series{ID: seriesID, Name: name, Genres: genres[seriesID]})
		}
		last := &lib.Series[len(lib.Series)-1]
		last.Episodes = append(last.Episodes, e)
	}
	return lib, rows.Err()
}

func secondsToDuration(s float64) time.Duration {
	return time.Duration(s*1000) * time.Millisecond
}

func msToDuration(ms *int) time.Duration {
	if ms == nil {
		return 0
	}
	return time.Duration(*ms) * time.Millisecond
}

// genres maps title ids to their genre names.
func (s *Store) genres(ctx context.Context) (map[string][]string, error) {
	rows, err := s.q.Query(ctx, `SELECT tg.title_id, g.name FROM title_genres tg JOIN genres g ON g.id = tg.genre_id ORDER BY g.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = append(out[id], name)
	}
	return out, rows.Err()
}

const airingCols = `id, starts_at, ends_at, kind, coalesce(title_id::text, ''), coalesce(episode_id::text, ''),
	coalesce(media_file_id::text, ''), slot, coalesce(fill, ''), source, locked, state, credits_at_ms`

func scanAirings(rows pgx.Rows) ([]Airing, error) {
	defer rows.Close()
	var out []Airing
	for rows.Next() {
		var a Airing
		var credits *int
		if err := rows.Scan(&a.ID, &a.StartsAt, &a.EndsAt, &a.Kind, &a.TitleID, &a.EpisodeID, &a.MediaFileID,
			&a.Slot, &a.Fill, &a.Source, &a.Locked, &a.State, &credits); err != nil {
			return nil, err
		}
		if credits != nil {
			a.CreditsAt = time.Duration(*credits) * time.Millisecond
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) airings(ctx context.Context, where string, args ...any) ([]Airing, error) {
	rows, err := s.q.Query(ctx, `SELECT `+airingCols+` FROM couchtv_airings WHERE state <> 'replaced' AND (`+where+`) ORDER BY starts_at, kind DESC`, args...)
	if err != nil {
		return nil, err
	}
	return scanAirings(rows)
}

// History is what the rules read of the programs before a moment: the latest airing of
// every movie and episode per fill. That bounds it by the library rather than by time.
func (s *Store) History(ctx context.Context, before time.Time) ([]Airing, error) {
	rows, err := s.q.Query(ctx, `
		SELECT DISTINCT ON (title_id, episode_id, fill) `+airingCols+`
		FROM couchtv_airings
		WHERE kind = 'program' AND state <> 'replaced' AND starts_at < $1
		ORDER BY title_id, episode_id, fill, starts_at DESC`, before)
	if err != nil {
		return nil, err
	}
	return scanAirings(rows)
}

// Fixed are the programs from a moment on that replanning keeps: admin rows, locked
// rows and the ones that started before now.
func (s *Store) Fixed(ctx context.Context, from, now time.Time) ([]Airing, error) {
	return s.airings(ctx, `kind = 'program' AND starts_at >= $1 AND (source = 'admin' OR locked OR starts_at < $2)`, from, now)
}

// Programs are the programs starting in [from, to).
func (s *Store) Programs(ctx context.Context, from, to time.Time) ([]Airing, error) {
	return s.airings(ctx, `kind = 'program' AND starts_at >= $1 AND starts_at < $2`, from, to)
}

// DayPlanned reports whether the planner has placed programs in [start, end).
func (s *Store) DayPlanned(ctx context.Context, start, end time.Time) (bool, error) {
	var planned bool
	err := s.q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM couchtv_airings
		WHERE kind = 'program' AND source = 'auto' AND starts_at >= $1 AND starts_at < $2)`, start, end).Scan(&planned)
	return planned, err
}

// Cursors are the admins' strip restarts.
func (s *Store) Cursors(ctx context.Context) ([]Cursor, error) {
	rows, err := s.q.Query(ctx, `SELECT title_id, next_episode_id, effective_from FROM couchtv_strips`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Cursor
	for rows.Next() {
		var c Cursor
		if err := rows.Scan(&c.SeriesID, &c.EpisodeID, &c.From); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// rowKey identifies a row by everything planning decides, so an unchanged row keeps
// its id (and whatever later phases hang on it, such as a break's composition).
func rowKey(a Airing) string {
	return strings.Join([]string{a.Kind, fmt.Sprint(a.StartsAt.UnixMicro()), fmt.Sprint(a.EndsAt.UnixMicro()),
		a.TitleID, a.EpisodeID, a.MediaFileID, a.Slot, a.Fill, fmt.Sprint(a.CreditsAt.Milliseconds())}, "|")
}

// Replace makes the schedule from start on match rows (a plan's output): rows that
// planning keeps (they carry their id) stay, identical rows stay, other auto rows go
// and new ones are inserted. Admin and locked rows are never deleted here.
func (s *Store) Replace(ctx context.Context, start time.Time, rows []Airing) (inserted, deleted int, err error) {
	old, err := s.airings(ctx, `starts_at >= $1 OR kind = 'break' AND ends_at >= $1`, start)
	if err != nil {
		return 0, 0, err
	}
	keep := map[string]bool{}
	for _, a := range rows {
		if a.ID != "" {
			keep[a.ID] = true
		}
	}
	stale := map[string][]string{}
	for _, a := range old {
		if !keep[a.ID] && a.Source == SourceAuto && !a.Locked {
			stale[rowKey(a)] = append(stale[rowKey(a)], a.ID)
		}
	}
	var add []Airing
	for _, a := range rows {
		if a.ID != "" {
			continue
		}
		if ids := stale[rowKey(a)]; len(ids) > 0 {
			stale[rowKey(a)] = ids[1:]
			continue
		}
		add = append(add, a)
	}
	var drop []string
	for _, ids := range stale {
		drop = append(drop, ids...)
	}
	if len(drop) > 0 {
		if _, err := s.q.Exec(ctx, `DELETE FROM couchtv_airings WHERE id = ANY($1::uuid[])`, drop); err != nil {
			return 0, 0, err
		}
	}
	if len(add) > 0 {
		if err := s.insert(ctx, add); err != nil {
			return 0, 0, err
		}
	}
	return len(add), len(drop), nil
}

func (s *Store) insert(ctx context.Context, rows []Airing) error {
	n := len(rows)
	starts, ends := make([]time.Time, n), make([]time.Time, n)
	kinds, titles, episodes, files := make([]string, n), make([]string, n), make([]string, n), make([]string, n)
	slots, fills, sources, states := make([]string, n), make([]string, n), make([]string, n), make([]string, n)
	locked := make([]bool, n)
	credits := make([]*int64, n)
	for i, a := range rows {
		starts[i], ends[i], kinds[i] = a.StartsAt, a.EndsAt, a.Kind
		titles[i], episodes[i], files[i] = a.TitleID, a.EpisodeID, a.MediaFileID
		slots[i], fills[i], sources[i], states[i], locked[i] = a.Slot, a.Fill, a.Source, a.State, a.Locked
		if a.CreditsAt > 0 {
			ms := a.CreditsAt.Milliseconds()
			credits[i] = &ms
		}
	}
	_, err := s.q.Exec(ctx, `
		INSERT INTO couchtv_airings (starts_at, ends_at, kind, title_id, episode_id, media_file_id, slot, fill,
		                             source, locked, state, credits_at_ms)
		SELECT s, e, k, NULLIF(t, '')::uuid, NULLIF(ep, '')::uuid, NULLIF(f, '')::uuid, sl, NULLIF(fi, ''), so, l, st, c
		FROM unnest($1::timestamptz[], $2::timestamptz[], $3::text[], $4::text[], $5::text[], $6::text[], $7::text[],
		            $8::text[], $9::text[], $10::bool[], $11::text[], $12::int[])
		     AS r(s, e, k, t, ep, f, sl, fi, so, l, st, c)`,
		starts, ends, kinds, titles, episodes, files, slots, fills, sources, locked, states, credits)
	return err
}

// Swap stores a stand-in for a program (same id) and moves the start of the break
// after it to the stand-in's end; that break's composition no longer fits it.
func (s *Store) Swap(ctx context.Context, a Airing, oldEnd time.Time) error {
	var credits *int64
	if a.CreditsAt > 0 {
		ms := a.CreditsAt.Milliseconds()
		credits = &ms
	}
	if _, err := s.q.Exec(ctx, `
		UPDATE couchtv_airings SET title_id = $2, episode_id = NULLIF($3, '')::uuid, media_file_id = NULLIF($4, '')::uuid,
		       fill = NULLIF($5, ''), ends_at = $6, credits_at_ms = $7
		WHERE id = $1`, a.ID, a.TitleID, a.EpisodeID, a.MediaFileID, a.Fill, a.EndsAt, credits); err != nil {
		return err
	}
	var breakID string
	err := s.q.QueryRow(ctx, `UPDATE couchtv_airings SET starts_at = $1 WHERE kind = 'break' AND starts_at = $2 RETURNING id`,
		a.EndsAt, oldEnd).Scan(&breakID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = s.q.Exec(ctx, `DELETE FROM couchtv_breaks WHERE airing_id = $1`, breakID)
	return err
}

// PlannedUntil is when the last planned airing ends; nil before the first plan.
func (s *Store) PlannedUntil(ctx context.Context) (*time.Time, error) {
	var until *time.Time
	err := s.q.QueryRow(ctx, `SELECT max(ends_at) FROM couchtv_airings WHERE state <> 'replaced'`).Scan(&until)
	return until, err
}

// TitleKinds maps the given title ids that exist to their kind.
func (s *Store) TitleKinds(ctx context.Context, ids []string) (map[string]string, error) {
	rows, err := s.q.Query(ctx, `SELECT id, kind FROM titles WHERE id = ANY($1::uuid[])`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, kind string
		if err := rows.Scan(&id, &kind); err != nil {
			return nil, err
		}
		out[id] = kind
	}
	return out, rows.Err()
}

// Schedule lists the airings overlapping [from, to) as the admin sees them.
func (s *Store) Schedule(ctx context.Context, from, to time.Time) ([]CouchTvAiring, error) {
	rows, err := s.q.Query(ctx, `
		SELECT a.id, a.starts_at, a.ends_at, a.kind, coalesce(a.title_id::text, ''), coalesce(t.name, ''), coalesce(t.kind, ''),
		       coalesce(a.episode_id::text, ''), coalesce(se.season_number, 0), coalesce(e.episode_number, 0), coalesce(e.name, ''),
		       coalesce(a.media_file_id::text, ''), a.slot, coalesce(a.fill, ''), a.source, a.locked, a.state, a.credits_at_ms,
		       a.kind = 'break' OR coalesce(`+readySQL+`, false)
		FROM couchtv_airings a
		LEFT JOIN titles t ON t.id = a.title_id
		LEFT JOIN episodes e ON e.id = a.episode_id
		LEFT JOIN seasons se ON se.id = e.season_id
		LEFT JOIN media_files mf ON mf.id = a.media_file_id
		WHERE a.starts_at < $2 AND a.ends_at > $1 AND a.state <> 'replaced'
		ORDER BY a.starts_at, a.kind DESC`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CouchTvAiring{}
	for rows.Next() {
		var a CouchTvAiring
		if err := rows.Scan(&a.ID, &a.StartsAt, &a.EndsAt, &a.Kind, &a.TitleID, &a.TitleName, &a.TitleKind,
			&a.EpisodeID, &a.SeasonNumber, &a.EpisodeNumber, &a.EpisodeName,
			&a.MediaFileID, &a.Slot, &a.Fill, &a.Source, &a.Locked, &a.State, &a.CreditsAtMs, &a.Ready); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
