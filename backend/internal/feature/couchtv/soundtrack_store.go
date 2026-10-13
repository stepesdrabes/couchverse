package couchtv

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/db"
)

// soundtrackSubject is what the finder reads of a title.
type soundtrackSubject struct {
	Kind     string
	Name     string // English
	Year     int
	TmdbID   int
	Excluded bool
	Version  int // the finder's version that searched it; 0 before the first search
}

func (s *Store) soundtrackSubject(ctx context.Context, titleID string) (*soundtrackSubject, error) {
	var t soundtrackSubject
	err := s.q.QueryRow(ctx, `
		SELECT t.kind, `+englishTitle+`, coalesce(t.year, 0), coalesce(t.tmdb_id, 0), coalesce(ct.excluded, false),
		       coalesce(ct.soundtrack_version, 0)
		FROM titles t LEFT JOIN couchtv_titles ct ON ct.title_id = t.id WHERE t.id = $1`, titleID).
		Scan(&t.Kind, &t.Name, &t.Year, &t.TmdbID, &t.Excluded, &t.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	return &t, err
}

// StaleSoundtracks are the published titles on the channel, with media, that an older
// finder (or none) searched.
func (s *Store) StaleSoundtracks(ctx context.Context, version int) ([]string, error) {
	rows, err := s.q.Query(ctx, `
		SELECT t.id FROM titles t
		LEFT JOIN couchtv_titles ct ON ct.title_id = t.id
		WHERE t.status = 'published' AND NOT coalesce(ct.excluded, false) AND coalesce(ct.soundtrack_version, 0) < $1
		  AND EXISTS (SELECT 1 FROM media_files mf
		              LEFT JOIN episodes e ON e.id = mf.episode_id
		              LEFT JOIN seasons se ON se.id = e.season_id
		              WHERE (mf.title_id = t.id OR se.title_id = t.id) AND mf.video_codec <> '' AND mf.duration_seconds > 0)
		ORDER BY t.id`, version)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// pickSoundtrackSQL makes the finder's best confident candidate a title's pick, unless
// an admin chose one: a pick that stays keeps its downloaded preview.
const pickSoundtrackSQL = `
	UPDATE couchtv_titles ct SET soundtrack_id = best.id,
		soundtrack_source = CASE WHEN best.id IS NULL THEN NULL ELSE 'auto' END,
		soundtrack_path = CASE WHEN best.id = ct.soundtrack_id THEN ct.soundtrack_path END,
		soundtrack_start_ms = CASE WHEN best.id = ct.soundtrack_id THEN ct.soundtrack_start_ms END,
		soundtrack_duration_ms = CASE WHEN best.id = ct.soundtrack_id THEN ct.soundtrack_duration_ms END,
		updated_at = now()
	FROM (SELECT (SELECT id FROM couchtv_soundtracks WHERE title_id = $1 AND confidence >= $2
	              ORDER BY confidence DESC, provider = 'itunes' DESC, track_id LIMIT 1) AS id) best
	WHERE ct.title_id = $1 AND ct.soundtrack_source IS DISTINCT FROM 'admin'`

// SaveSoundtracks stores a search's candidates in place of the last one's (the pick is
// kept, and a candidate found again keeps its id). The finder's own search also records
// its version and picks its best confident candidate; an admin's query only offers more
// to choose from.
func (s *Store) SaveSoundtracks(ctx context.Context, titleID, query string, version int, found []SoundtrackCandidate) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO couchtv_titles (title_id) VALUES ($1) ON CONFLICT DO NOTHING`, titleID); err != nil {
		return err
	}
	ids := make([]string, 0, len(found))
	for _, c := range found {
		var year *int
		if c.Album.Year > 0 {
			year = &c.Album.Year
		}
		var id string
		if err := tx.QueryRow(ctx, `
			INSERT INTO couchtv_soundtracks (title_id, provider, track_id, track, artist, album, year, duration_ms,
			                                 preview_url, confidence)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (title_id, provider, track_id) DO UPDATE SET track = $4, artist = $5, album = $6, year = $7,
				duration_ms = $8, preview_url = $9, confidence = $10
			RETURNING id`, titleID, c.Provider, c.ID, c.Name, c.Artist, c.Album.Name, year, c.Duration.Milliseconds(),
			c.Preview, c.Confidence).Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM couchtv_soundtracks s WHERE s.title_id = $1 AND NOT s.id = ANY($2::uuid[])
		AND s.id IS DISTINCT FROM (SELECT soundtrack_id FROM couchtv_titles WHERE title_id = $1)`, titleID, ids); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE couchtv_titles SET soundtrack_query = $2, soundtrack_searched_at = now(),
		soundtrack_version = CASE WHEN $2 = '' THEN $3 ELSE soundtrack_version END, updated_at = now()
		WHERE title_id = $1`, titleID, query, version); err != nil {
		return err
	}
	if query == "" {
		if _, err := tx.Exec(ctx, pickSoundtrackSQL, titleID, soundtrackConfident); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// storedSoundtrack is a candidate as the downloads read it.
type storedSoundtrack struct {
	ID         string
	Provider   string
	TrackID    string
	PreviewURL string
}

func (t storedSoundtrack) track() SoundtrackTrack {
	return SoundtrackTrack{Provider: t.Provider, ID: t.TrackID, Preview: t.PreviewURL}
}

// soundtrackPick is a title's pick and whether its preview is ready.
type soundtrackPick struct {
	storedSoundtrack
	Source string
	Ready  bool
}

func (s *Store) soundtrackPick(ctx context.Context, titleID string) (*soundtrackPick, error) {
	var p soundtrackPick
	err := s.q.QueryRow(ctx, `
		SELECT s.id, s.provider, s.track_id, s.preview_url, coalesce(ct.soundtrack_source, 'auto'), ct.soundtrack_path IS NOT NULL
		FROM couchtv_titles ct JOIN couchtv_soundtracks s ON s.id = ct.soundtrack_id
		WHERE ct.title_id = $1`, titleID).Scan(&p.ID, &p.Provider, &p.TrackID, &p.PreviewURL, &p.Source, &p.Ready)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &p, err
}

// soundtrack reads one candidate.
func (s *Store) soundtrack(ctx context.Context, id string) (*storedSoundtrack, error) {
	var t storedSoundtrack
	err := s.q.QueryRow(ctx, `SELECT id, provider, track_id, preview_url FROM couchtv_soundtracks WHERE id = $1`, id).
		Scan(&t.ID, &t.Provider, &t.TrackID, &t.PreviewURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	return &t, err
}

// soundtrackReady records the pick's downloaded preview and the excerpt promos play,
// unless the pick changed meanwhile.
func (s *Store) soundtrackReady(ctx context.Context, titleID, soundtrackID, path string, start, length time.Duration) (bool, error) {
	tag, err := s.q.Exec(ctx, `UPDATE couchtv_titles SET soundtrack_path = $3, soundtrack_start_ms = $4,
		soundtrack_duration_ms = $5, updated_at = now() WHERE title_id = $1 AND soundtrack_id = $2`,
		titleID, soundtrackID, filepath.ToSlash(path), start.Milliseconds(), length.Milliseconds())
	return tag.RowsAffected() > 0, err
}

// dropSoundtrack forgets a candidate whose preview cannot play: a pick of it goes back
// to the finder, which picks its next confident candidate.
func (s *Store) dropSoundtrack(ctx context.Context, titleID, soundtrackID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM couchtv_soundtracks WHERE id = $1`, soundtrackID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE couchtv_titles SET soundtrack_source = NULL, soundtrack_path = NULL,
		soundtrack_start_ms = NULL, soundtrack_duration_ms = NULL WHERE title_id = $1 AND soundtrack_id IS NULL`, titleID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, pickSoundtrackSQL, titleID, soundtrackConfident); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// soundtracks lists a title's candidates, best first.
func (s *Store) soundtracks(ctx context.Context, titleID string) ([]CouchTvSoundtrack, error) {
	rows, err := s.q.Query(ctx, `
		SELECT id, provider, track, artist, album, coalesce(year, 0), duration_ms, confidence FROM couchtv_soundtracks
		WHERE title_id = $1 ORDER BY confidence DESC, provider = 'itunes' DESC, track_id`, titleID)
	if err != nil {
		return nil, err
	}
	out := []CouchTvSoundtrack{}
	for rows.Next() {
		var c CouchTvSoundtrack
		if err := rows.Scan(&c.ID, &c.Provider, &c.Track, &c.Artist, &c.Album, &c.Year, &c.DurationMs, &c.Confidence); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SoundtrackOfTitle reports whether a candidate is one of the title's.
func (s *Store) SoundtrackOfTitle(ctx context.Context, soundtrackID, titleID string) (bool, error) {
	var ok bool
	err := s.q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM couchtv_soundtracks WHERE id = $1 AND title_id = $2)`,
		soundtrackID, titleID).Scan(&ok)
	return ok, err
}

// soundtrackPaths are the previews titles play, which the cleanup keeps.
func (s *Store) soundtrackPaths(ctx context.Context) (map[string]bool, error) {
	rows, err := s.q.Query(ctx, `SELECT soundtrack_path FROM couchtv_titles WHERE soundtrack_path IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	paths, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(paths))
	for _, p := range paths {
		out[filepath.FromSlash(p)] = true
	}
	return out, nil
}
