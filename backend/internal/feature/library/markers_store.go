package library

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/db"
)

// Marker kinds (media_markers.kind): a series episode has an intro and credits (its end
// credits, a range); a movie has credits (where they start) and credits_music (the
// music under them, a range).
const (
	MarkerIntro        = "intro"
	MarkerCredits      = "credits"
	MarkerCreditsMusic = "credits_music"
)

// Marker sources: the analysis writes auto rows, an admin's row overrides them.
const (
	MarkerAuto  = "auto"
	MarkerAdmin = "admin"
)

// MediaMarker is one marker of a media file.
type MediaMarker struct {
	Kind    string `json:"kind" enum:"intro,credits,credits_music"`
	Source  string `json:"source" enum:"auto,admin"`
	StartMs *int   `json:"startMs,omitempty" doc:"Absent on an admin row that says the file has none of this kind."`
	EndMs   *int   `json:"endMs,omitempty" doc:"Where it ends; absent for a movie's credits, which run to the end."`
	None    bool   `json:"none,omitempty" doc:"An admin row saying the file has none of this kind, overriding a wrong detection."`
	Version int    `json:"version" doc:"The analyzer that wrote an auto row; 0 for admin rows."`
}

// MediaMarkers are a media file's markers.
type MediaMarkers struct {
	MediaFileID     string        `json:"mediaFileId" format:"uuid"`
	DurationMs      int           `json:"durationMs"`
	AnalyzedVersion int           `json:"analyzedVersion" doc:"The analyzer that last looked at the file; 0 before the first analysis."`
	Analyzer        int           `json:"analyzer" doc:"The current analyzer's version; a file analyzed by an older one is analyzed again."`
	Markers         []MediaMarker `json:"markers" doc:"Auto and admin rows; an admin row overrides the auto row of its kind."`
}

// MediaMarkers reads a file's markers.
func (s *Store) MediaMarkers(ctx context.Context, fileID string) (*MediaMarkers, error) {
	out := &MediaMarkers{MediaFileID: fileID, Analyzer: MarkersVersion, Markers: []MediaMarker{}}
	var seconds float64
	err := s.db.QueryRow(ctx, `SELECT duration_seconds::float8, markers_version FROM media_files WHERE id = $1`,
		fileID).Scan(&seconds, &out.AnalyzedVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	out.DurationMs = int(seconds * 1000)
	rows, err := s.db.Query(ctx, `SELECT kind, source, start_ms, end_ms, version FROM media_markers
		WHERE media_file_id = $1 ORDER BY kind, source DESC`, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var m MediaMarker
		if err := rows.Scan(&m.Kind, &m.Source, &m.StartMs, &m.EndMs, &m.Version); err != nil {
			return nil, err
		}
		m.None = m.StartMs == nil
		out.Markers = append(out.Markers, m)
	}
	return out, rows.Err()
}

// ReplaceAutoMarkers stores what an analysis found for a file: its auto rows of the
// kinds it looks for are replaced (admin rows stay) and the file is marked as analyzed
// by version.
func (s *Store) ReplaceAutoMarkers(ctx context.Context, fileID string, kinds []string, found []MediaMarker, version int) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM media_markers WHERE media_file_id = $1 AND source = 'auto' AND kind = ANY($2)`,
		fileID, kinds); err != nil {
		return err
	}
	for _, m := range found {
		if _, err := tx.Exec(ctx, `INSERT INTO media_markers (media_file_id, kind, source, start_ms, end_ms, version)
			VALUES ($1, $2, 'auto', $3, $4, $5)`, fileID, m.Kind, m.StartMs, m.EndMs, version); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE media_files SET markers_version = $2 WHERE id = $1`, fileID, version); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetMarkersVersion records that an analyzer looked at a file without storing anything.
func (s *Store) SetMarkersVersion(ctx context.Context, fileID string, version int) error {
	_, err := s.db.Exec(ctx, `UPDATE media_files SET markers_version = $2 WHERE id = $1`, fileID, version)
	return err
}

// SetAdminMarkers replaces a file's admin markers.
func (s *Store) SetAdminMarkers(ctx context.Context, fileID string, markers []MediaMarker) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM media_markers WHERE media_file_id = $1 AND source = 'admin'`, fileID); err != nil {
		return err
	}
	for _, m := range markers {
		if _, err := tx.Exec(ctx, `INSERT INTO media_markers (media_file_id, kind, source, start_ms, end_ms)
			VALUES ($1, $2, 'admin', $3, $4)`, fileID, m.Kind, m.StartMs, m.EndMs); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// markerFile is a media file as the marker analysis sees it.
type markerFile struct {
	ID       string
	Path     string // on disk
	Duration float64
	// Analyzable: a primary video file whose source is on disk
	Analyzable bool
	TitleID    string
	TitleKind  string // movie or series; empty for a file on neither
	EpisodeID  string
	Season     int
	Episode    int
}

const markerFileSQL = `
	SELECT mf.id, l.path, mf.path, mf.duration_seconds::float8,
	       mf.audio_role = 'primary' AND mf.video_codec <> '' AND mf.source_deleted_at IS NULL AND mf.duration_seconds > 0,
	       coalesce(t.id, st.id)::text, coalesce(t.kind, st.kind, ''), coalesce(e.id::text, ''),
	       coalesce(se.season_number, 0), coalesce(e.episode_number, 0)
	FROM media_files mf
	JOIN libraries l ON l.id = mf.library_id
	LEFT JOIN titles t ON t.id = mf.title_id
	LEFT JOIN episodes e ON e.id = mf.episode_id
	LEFT JOIN seasons se ON se.id = e.season_id
	LEFT JOIN titles st ON st.id = se.title_id`

func (s *Store) markerFiles(ctx context.Context, where string, args ...any) ([]markerFile, error) {
	rows, err := s.db.Query(ctx, markerFileSQL+` WHERE `+where+` ORDER BY se.season_number, e.episode_number, mf.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []markerFile
	for rows.Next() {
		var f markerFile
		var libPath, rel string
		var titleID *string
		if err := rows.Scan(&f.ID, &libPath, &rel, &f.Duration, &f.Analyzable, &titleID, &f.TitleKind, &f.EpisodeID,
			&f.Season, &f.Episode); err != nil {
			return nil, err
		}
		f.Path = filepath.Join(libPath, rel)
		if titleID != nil {
			f.TitleID = *titleID
		}
		if f.TitleKind == "series" && f.EpisodeID == "" {
			f.Analyzable = false
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) markerFile(ctx context.Context, id string) (*markerFile, error) {
	files, err := s.markerFiles(ctx, `mf.id = $1`, id)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 || files[0].TitleKind == "" {
		return nil, db.ErrNotFound
	}
	return &files[0], nil
}

// seasonFiles are the files of a season's episodes, in episode order.
func (s *Store) seasonFiles(ctx context.Context, titleID string, season int) ([]markerFile, error) {
	return s.markerFiles(ctx, `se.title_id = $1 AND se.season_number = $2`, titleID, season)
}

// titleFiles are a title's files: a movie's, or every episode's of a series.
func (s *Store) titleFiles(ctx context.Context, titleID string) ([]markerFile, error) {
	return s.markerFiles(ctx, `(mf.title_id = $1 OR se.title_id = $1)`, titleID)
}

// staleMarkerFiles are the analyzable files an older analyzer (or none) looked at.
func (s *Store) staleMarkerFiles(ctx context.Context, version int) ([]markerFile, error) {
	files, err := s.markerFiles(ctx, `mf.markers_version < $1 AND mf.audio_role = 'primary' AND mf.video_codec <> ''
		AND mf.source_deleted_at IS NULL AND mf.duration_seconds > 0 AND (t.kind = 'movie' OR e.id IS NOT NULL)`, version)
	return files, err
}
