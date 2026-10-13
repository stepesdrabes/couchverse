package couchtv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/db"
	"couchverse/internal/feature/library"
)

// CouchTvTitle is a title's channel facts: whether it airs, and what its promos are
// made of.
type CouchTvTitle struct {
	TitleID              string                 `json:"titleId" format:"uuid"`
	TitleKind            string                 `json:"titleKind" enum:"movie,series"`
	Excluded             bool                   `json:"excluded" doc:"Never on Couch TV: the planner leaves the title out."`
	PromoAudio           *CouchTvPromoAudio     `json:"promoAudio,omitempty" doc:"The promo soundtrack excerpt; absent when none was found."`
	Shots                []CouchTvShot          `json:"shots" doc:"Footage for promos, in the order they air."`
	ShotsSource          string                 `json:"shotsSource,omitempty" enum:"auto,admin" doc:"Who chose the shots; absent before the analysis."`
	Mood                 *string                `json:"mood,omitempty" enum:"cinematic,grit,bright" doc:"The promo's mood; absent lets the composer choose."`
	LoudnessLufs         *float64               `json:"loudnessLufs" doc:"The program's integrated loudness (EBU R128); null until measured."`
	AnalyzedVersion      int                    `json:"analyzedVersion" doc:"The analysis that measured the title; 0 before the first."`
	AnalyzedAt           *time.Time             `json:"analyzedAt" doc:"When it ran; null before the first."`
	Analyzing            bool                   `json:"analyzing" doc:"An analysis of the title (its markers or its promo picks) is queued or running."`
	PromoBed             string                 `json:"promoBed" enum:"none,excerpt,soundtrack" doc:"What the promos play under them now: the soundtrack from the web, the excerpt of the title's own audio (promoAudio), or nothing."`
	Soundtrack           *CouchTvSoundtrackPick `json:"soundtrack,omitempty" doc:"The soundtrack from the web picked for the promos; absent when none is."`
	Soundtracks          []CouchTvSoundtrack    `json:"soundtracks" doc:"What the last soundtrack search found, best first (the pick among them)."`
	SoundtrackQuery      string                 `json:"soundtrackQuery,omitempty" doc:"The admin's own query of the last search; absent for the finder's."`
	SoundtrackSearchedAt *time.Time             `json:"soundtrackSearchedAt" doc:"When the soundtrack was last searched; null before the first search."`
	SearchingSoundtrack  bool                   `json:"searchingSoundtrack" doc:"A soundtrack search, or the download of the picked track, is queued or running."`
}

// CouchTvSoundtrack is a track from a music catalog that may be the title's own music:
// its official 30-second preview can play under the promos.
type CouchTvSoundtrack struct {
	ID         string  `json:"id" format:"uuid"`
	Provider   string  `json:"provider" enum:"itunes,deezer" doc:"The catalog the preview comes from."`
	Track      string  `json:"track"`
	Artist     string  `json:"artist"`
	Album      string  `json:"album"`
	Year       int     `json:"year,omitempty" doc:"The album's year; absent when the catalog does not say."`
	DurationMs int     `json:"durationMs" doc:"The whole track's length; the preview is 30 s of it."`
	Confidence float64 `json:"confidence" minimum:"0" maximum:"1" doc:"How sure the finder is that this is the title's own music and suits a promo; from 0.7 it picks one itself."`
}

// CouchTvSoundtrackPick is the soundtrack from the web a title's promos play.
type CouchTvSoundtrackPick struct {
	SoundtrackID string `json:"soundtrackId" format:"uuid" doc:"One of the title's soundtracks."`
	Source       string `json:"source" enum:"auto,admin" doc:"auto: the finder's confident match; admin: the admin's choice, which searches keep."`
	Ready        bool   `json:"ready" doc:"The preview is downloaded and measured; until then the promos play the excerpt."`
	StartMs      int    `json:"startMs" doc:"Where the promos' stretch of the preview starts; 0 until ready."`
	DurationMs   int    `json:"durationMs" doc:"How long it is; 0 until ready."`
}

// What a title's promos play under them.
const (
	BedNone       = "none"
	BedExcerpt    = "excerpt"
	BedSoundtrack = "soundtrack"
)

// promoBed is what a title's promos play (D25, plan 6.3): an admin's excerpt of the
// title's own audio, else the soundtrack from the web (the admin's choice or the
// finder's confident match) once its preview is ready, else the analysis' excerpt.
func promoBed(excerpt bool, excerptSource string, soundtrackReady bool) string {
	switch {
	case excerpt && excerptSource == SourceAdmin:
		return BedExcerpt
	case soundtrackReady:
		return BedSoundtrack
	case excerpt:
		return BedExcerpt
	}
	return BedNone
}

// CouchTvPromoAudio is the stretch of a media file a title's promos play under.
type CouchTvPromoAudio struct {
	MediaFileID string `json:"mediaFileId" format:"uuid"`
	StartMs     int    `json:"startMs"`
	DurationMs  int    `json:"durationMs"`
	Source      string `json:"source" enum:"auto,admin" doc:"auto: the analysis' pick; admin: the admin's, which the analysis keeps."`
}

// CouchTvShot is a short shot of a title's footage for its promos.
type CouchTvShot struct {
	MediaFileID string  `json:"mediaFileId" format:"uuid"`
	StartMs     int     `json:"startMs"`
	DurationMs  int     `json:"durationMs" doc:"Whole frames at 25 fps, 1 to 3 seconds."`
	Score       float64 `json:"score" doc:"How well it suits a promo (motion and contrast), 0 to 1; 1 for an admin's pick."`
}

// titleRow is a couchtv_titles row with its title's kind.
type titleRow struct {
	CouchTvTitle
	promoSource string
}

// Title reads a title's channel facts; a title without a row yet has the defaults.
func (s *Store) Title(ctx context.Context, titleID string) (*titleRow, error) {
	var r titleRow
	var shots []byte
	var fileID, promoSource, shotsSource, soundtrackID, soundtrackSource *string
	var start, duration, soundtrackStart, soundtrackDuration *int
	var soundtrackReady bool
	err := s.q.QueryRow(ctx, `
		SELECT t.id, t.kind, coalesce(ct.excluded, false), ct.promo_audio_file_id::text, ct.promo_audio_start_ms,
		       ct.promo_audio_duration_ms, ct.promo_audio_source, coalesce(ct.shots, '[]'), ct.shots_source, ct.mood,
		       ct.loudness_lufs, coalesce(ct.analyzed_version, 0), ct.analyzed_at,
		       EXISTS (SELECT 1 FROM jobs j WHERE j.status IN ('pending', 'running') AND j.type IN ('`+AnalyzeJob+`', '`+library.MarkersJob+`')
		               AND (j.payload->>'titleId' = t.id::text OR j.payload->>'mediaFileId' IN (
		                   SELECT mf.id::text FROM media_files mf
		                   LEFT JOIN episodes e ON e.id = mf.episode_id
		                   LEFT JOIN seasons se ON se.id = e.season_id
		                   WHERE mf.title_id = t.id OR se.title_id = t.id))),
		       ct.soundtrack_id::text, ct.soundtrack_source, ct.soundtrack_path IS NOT NULL, ct.soundtrack_start_ms,
		       ct.soundtrack_duration_ms, coalesce(ct.soundtrack_query, ''), ct.soundtrack_searched_at,
		       EXISTS (SELECT 1 FROM jobs j WHERE j.status IN ('pending', 'running') AND j.type = '`+SoundtrackJob+`'
		               AND j.payload->>'titleId' = t.id::text AND j.run_at <= now())
		FROM titles t
		LEFT JOIN couchtv_titles ct ON ct.title_id = t.id
		WHERE t.id = $1`, titleID).Scan(&r.TitleID, &r.TitleKind, &r.Excluded, &fileID, &start, &duration, &promoSource,
		&shots, &shotsSource, &r.Mood, &r.LoudnessLufs, &r.AnalyzedVersion, &r.AnalyzedAt, &r.Analyzing,
		&soundtrackID, &soundtrackSource, &soundtrackReady, &soundtrackStart, &soundtrackDuration, &r.SoundtrackQuery,
		&r.SoundtrackSearchedAt, &r.SearchingSoundtrack)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if promoSource != nil {
		r.promoSource = *promoSource
	}
	if fileID != nil && start != nil && duration != nil {
		r.PromoAudio = &CouchTvPromoAudio{MediaFileID: *fileID, StartMs: *start, DurationMs: *duration, Source: r.promoSource}
	}
	if shotsSource != nil {
		r.ShotsSource = *shotsSource
	}
	r.Shots = []CouchTvShot{}
	if err := json.Unmarshal(shots, &r.Shots); err != nil {
		return nil, err
	}
	if soundtrackID != nil && soundtrackSource != nil {
		r.Soundtrack = &CouchTvSoundtrackPick{SoundtrackID: *soundtrackID, Source: *soundtrackSource, Ready: soundtrackReady}
		if soundtrackReady && soundtrackStart != nil && soundtrackDuration != nil {
			r.Soundtrack.StartMs, r.Soundtrack.DurationMs = *soundtrackStart, *soundtrackDuration
		}
	}
	r.PromoBed = promoBed(r.PromoAudio != nil, r.promoSource, r.Soundtrack != nil && r.Soundtrack.Ready)
	if r.Soundtracks, err = s.soundtracks(ctx, titleID); err != nil {
		return nil, err
	}
	return &r, nil
}

// analysis is what couchtv_analyze stores: a nil part was not measured this time.
type analysis struct {
	loudness *float64
	shots    []CouchTvShot
	promo    *CouchTvPromoAudio
	// measured: the loudness and shots stage ran (the title is analyzed at this version)
	measured  bool
	shotsDone bool
	promoDone bool
	version   int
}

// SaveAnalysis stores an analysis without touching what an admin chose.
func (s *Store) SaveAnalysis(ctx context.Context, titleID string, a analysis) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO couchtv_titles (title_id) VALUES ($1) ON CONFLICT DO NOTHING`, titleID); err != nil {
		return err
	}
	if a.measured {
		if _, err := tx.Exec(ctx, `UPDATE couchtv_titles SET loudness_lufs = $2, analyzed_version = $3, analyzed_at = now(),
			updated_at = now() WHERE title_id = $1`, titleID, a.loudness, a.version); err != nil {
			return err
		}
	}
	if a.shotsDone {
		shots, err := json.Marshal(a.shots)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE couchtv_titles SET shots = $2, shots_source = 'auto', updated_at = now()
			WHERE title_id = $1 AND shots_source IS DISTINCT FROM 'admin'`, titleID, shots); err != nil {
			return err
		}
	}
	if a.promoDone {
		var fileID *string
		var start, duration *int
		source := (*string)(nil)
		if p := a.promo; p != nil {
			fileID, start, duration, source = &p.MediaFileID, &p.StartMs, &p.DurationMs, ptr(SourceAuto)
		}
		if _, err := tx.Exec(ctx, `UPDATE couchtv_titles SET promo_audio_file_id = $2, promo_audio_start_ms = $3,
			promo_audio_duration_ms = $4, promo_audio_source = $5, updated_at = now()
			WHERE title_id = $1 AND promo_audio_source IS DISTINCT FROM 'admin'`,
			titleID, fileID, start, duration, source); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func ptr[T any](v T) *T { return &v }

// titleEdit is an admin's edit of a title's channel facts; a nil promo or shots hands
// that part back to the analysis, a nil soundtrack to the finder.
type titleEdit struct {
	excluded   bool
	mood       *string
	promo      *CouchTvPromoAudio
	shots      []CouchTvShot
	soundtrack *string
}

// UpdateTitle stores an admin's edit.
func (s *Store) UpdateTitle(ctx context.Context, titleID string, e titleEdit) error {
	shots, err := json.Marshal(e.shots)
	if err != nil {
		return err
	}
	var fileID, promoSource, shotsSource *string
	var start, duration *int
	if p := e.promo; p != nil {
		fileID, start, duration, promoSource = &p.MediaFileID, &p.StartMs, &p.DurationMs, ptr(SourceAdmin)
	}
	if len(e.shots) > 0 {
		shotsSource = ptr(SourceAdmin)
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// handing a part back clears it until the analysis picks again
	_, err = tx.Exec(ctx, `
		INSERT INTO couchtv_titles (title_id, excluded, mood, promo_audio_file_id, promo_audio_start_ms,
		                            promo_audio_duration_ms, promo_audio_source, shots, shots_source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, coalesce($8::jsonb, '[]'), $9)
		ON CONFLICT (title_id) DO UPDATE SET excluded = $2, mood = $3,
			promo_audio_file_id = CASE WHEN $7::text IS NOT NULL OR couchtv_titles.promo_audio_source = 'admin' THEN $4 ELSE couchtv_titles.promo_audio_file_id END,
			promo_audio_start_ms = CASE WHEN $7::text IS NOT NULL OR couchtv_titles.promo_audio_source = 'admin' THEN $5 ELSE couchtv_titles.promo_audio_start_ms END,
			promo_audio_duration_ms = CASE WHEN $7::text IS NOT NULL OR couchtv_titles.promo_audio_source = 'admin' THEN $6 ELSE couchtv_titles.promo_audio_duration_ms END,
			promo_audio_source = CASE WHEN $7::text IS NOT NULL OR couchtv_titles.promo_audio_source = 'admin' THEN $7 ELSE couchtv_titles.promo_audio_source END,
			shots = CASE WHEN $9::text IS NOT NULL OR couchtv_titles.shots_source = 'admin' THEN coalesce($8::jsonb, '[]') ELSE couchtv_titles.shots END,
			shots_source = CASE WHEN $9::text IS NOT NULL OR couchtv_titles.shots_source = 'admin' THEN $9 ELSE couchtv_titles.shots_source END,
			updated_at = now()`,
		titleID, e.excluded, e.mood, fileID, start, duration, promoSource, nullJSON(shots, len(e.shots) > 0), shotsSource)
	if err != nil {
		return err
	}
	if e.soundtrack != nil {
		// choosing the pick again keeps its downloaded preview
		_, err = tx.Exec(ctx, `UPDATE couchtv_titles SET soundtrack_source = 'admin', soundtrack_id = $2,
			soundtrack_path = CASE WHEN soundtrack_id = $2 THEN soundtrack_path END,
			soundtrack_start_ms = CASE WHEN soundtrack_id = $2 THEN soundtrack_start_ms END,
			soundtrack_duration_ms = CASE WHEN soundtrack_id = $2 THEN soundtrack_duration_ms END,
			updated_at = now() WHERE title_id = $1`, titleID, *e.soundtrack)
	} else {
		// back to the finder: its confident best, which may be the same track
		if _, err = tx.Exec(ctx, `UPDATE couchtv_titles SET soundtrack_source = 'auto'
			WHERE title_id = $1 AND soundtrack_source = 'admin'`, titleID); err == nil {
			_, err = tx.Exec(ctx, pickSoundtrackSQL, titleID, soundtrackConfident)
		}
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func nullJSON(b []byte, set bool) *string {
	if !set {
		return nil
	}
	s := string(b)
	return &s
}

// titleFile is a media file of a title as the analysis reads it.
type titleFile struct {
	ID       string
	Path     string
	Duration float64
	// Readable: the source is on disk (by the database's account)
	Readable bool
	Season   int // 0 for a movie
	Episode  int
}

// TitleFiles are the files a title airs: a movie's primary file, or the primary file of
// every episode of a series in seasons 1 and up, in broadcast order.
func (s *Store) TitleFiles(ctx context.Context, titleID string) ([]titleFile, error) {
	rows, err := s.q.Query(ctx, `
		SELECT mf.id, l.path, mf.path, mf.duration_seconds::float8, mf.source_deleted_at IS NULL AND mf.video_codec <> '', 0, 0
		FROM titles t
		JOIN LATERAL (`+fmt.Sprintf(primaryFile, "title_id = t.id")+`) mf ON true
		JOIN libraries l ON l.id = mf.library_id
		WHERE t.id = $1 AND t.kind = 'movie' AND mf.duration_seconds > 0
		UNION ALL
		SELECT mf.id, l.path, mf.path, mf.duration_seconds::float8, mf.source_deleted_at IS NULL AND mf.video_codec <> '',
		       s.season_number, e.episode_number
		FROM seasons s
		JOIN episodes e ON e.season_id = s.id
		JOIN LATERAL (`+fmt.Sprintf(primaryFile, "episode_id = e.id")+`) mf ON true
		JOIN libraries l ON l.id = mf.library_id
		WHERE s.title_id = $1 AND s.season_number > 0 AND mf.duration_seconds > 0
		ORDER BY 6, 7`, titleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []titleFile
	for rows.Next() {
		var f titleFile
		var libPath, rel string
		if err := rows.Scan(&f.ID, &libPath, &rel, &f.Duration, &f.Readable, &f.Season, &f.Episode); err != nil {
			return nil, err
		}
		f.Path = filepath.Join(libPath, rel)
		out = append(out, f)
	}
	return out, rows.Err()
}

// marker is a file's effective marker of a kind: the admin's over the analysis'.
type marker struct {
	FileID     string
	Season     int
	Start, End int // ms
}

// Markers are the effective markers of a kind on a title's files (a movie's, or its
// episodes' in seasons 1 and up) that have a range; an admin's "none" hides the kind.
func (s *Store) Markers(ctx context.Context, titleID, kind string) ([]marker, error) {
	rows, err := s.q.Query(ctx, `
		SELECT file_id, season, start_ms, end_ms FROM (
			SELECT DISTINCT ON (mm.media_file_id) mm.media_file_id::text AS file_id, coalesce(se.season_number, 0) AS season,
			       mm.start_ms, mm.end_ms
			FROM media_markers mm
			JOIN media_files mf ON mf.id = mm.media_file_id
			LEFT JOIN episodes e ON e.id = mf.episode_id
			LEFT JOIN seasons se ON se.id = e.season_id
			WHERE mm.kind = $2 AND (mf.title_id = $1 OR se.title_id = $1 AND se.season_number > 0)
			ORDER BY mm.media_file_id, mm.source = 'admin' DESC
		) m WHERE start_ms IS NOT NULL AND end_ms IS NOT NULL
		ORDER BY season, file_id`, titleID, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []marker
	for rows.Next() {
		var m marker
		if err := rows.Scan(&m.FileID, &m.Season, &m.Start, &m.End); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SeasonSizes counts a series' episode files per season (seasons 1 and up).
func (s *Store) SeasonSizes(ctx context.Context, titleID string) (map[int]int, error) {
	rows, err := s.q.Query(ctx, `
		SELECT se.season_number, count(DISTINCT e.id) FROM seasons se
		JOIN episodes e ON e.season_id = se.id
		JOIN media_files mf ON mf.episode_id = e.id
		WHERE se.title_id = $1 AND se.season_number > 0
		GROUP BY se.season_number`, titleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]int{}
	for rows.Next() {
		var season, n int
		if err := rows.Scan(&season, &n); err != nil {
			return nil, err
		}
		out[season] = n
	}
	return out, rows.Err()
}

// FileOfTitle reports whether a media file is a title's (its own, or an episode's).
func (s *Store) FileOfTitle(ctx context.Context, fileID, titleID string) (duration float64, ok bool, err error) {
	err = s.q.QueryRow(ctx, `
		SELECT mf.duration_seconds::float8 FROM media_files mf
		LEFT JOIN episodes e ON e.id = mf.episode_id
		LEFT JOIN seasons se ON se.id = e.season_id
		WHERE mf.id = $1 AND (mf.title_id = $2 OR se.title_id = $2)`, fileID, titleID).Scan(&duration)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	return duration, err == nil, err
}

// sourceFile is a media file's source for a measurement.
type sourceFile struct {
	Path     string
	Duration float64
	Readable bool
	HasAudio bool
}

// SourceFile reads where a media file's source is.
func (s *Store) SourceFile(ctx context.Context, fileID string) (*sourceFile, error) {
	var f sourceFile
	var libPath, rel string
	err := s.q.QueryRow(ctx, `
		SELECT l.path, mf.path, mf.duration_seconds::float8, mf.source_deleted_at IS NULL, mf.audio_codec <> ''
		FROM media_files mf JOIN libraries l ON l.id = mf.library_id WHERE mf.id = $1`, fileID).
		Scan(&libPath, &rel, &f.Duration, &f.Readable, &f.HasAudio)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	f.Path = filepath.Join(libPath, rel)
	return &f, nil
}

// StaleTitles are the published titles with media that an older analysis (or none)
// measured, leaving out excluded ones.
func (s *Store) StaleTitles(ctx context.Context, version int) ([]string, error) {
	rows, err := s.q.Query(ctx, `
		SELECT t.id FROM titles t
		LEFT JOIN couchtv_titles ct ON ct.title_id = t.id
		WHERE t.status = 'published' AND NOT coalesce(ct.excluded, false) AND coalesce(ct.analyzed_version, 0) < $1
		  AND EXISTS (SELECT 1 FROM media_files mf
		              LEFT JOIN episodes e ON e.id = mf.episode_id
		              LEFT JOIN seasons se ON se.id = e.season_id
		              WHERE (mf.title_id = t.id OR se.title_id = t.id) AND mf.video_codec <> '' AND mf.duration_seconds > 0)
		ORDER BY t.id`, version)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
