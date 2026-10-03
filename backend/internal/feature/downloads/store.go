package downloads

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"couchverse/internal/db"
	"couchverse/internal/feature/catalog"
	"couchverse/internal/feature/playback"
)

// Store owns the downloads SQL over the shared pool.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// File is one prepared (or preparing) MP4, shared by every request for the same plan.
type File struct {
	ID          string
	MediaFileID string
	Spec        string
	Plan        Plan
	Status      string
	Progress    int
	Error       string
	SizeBytes   int64
	ReadyAt     *time.Time
	RequestedAt time.Time
}

// Plan is what the prepare_download job makes: the video and audio plan and the
// subtitle tracks muxed in.
type Plan struct {
	Media     playback.DownloadPlan `json:"media"`
	Subtitles []SubtitleRef         `json:"subtitles"`
}

// SubtitleRef is a subtitle track as it was when the download was planned.
type SubtitleRef struct {
	ID     string `json:"id"`
	Lang   string `json:"lang"`
	Label  string `json:"label"`
	Forced bool   `json:"forced"`
}

const fileCols = `id, media_file_id, spec, plan, status, progress, error, size_bytes, ready_at, requested_at`

func scanFile(row pgx.Row) (*File, error) {
	var f File
	var plan []byte
	err := row.Scan(&f.ID, &f.MediaFileID, &f.Spec, &plan, &f.Status, &f.Progress, &f.Error, &f.SizeBytes, &f.ReadyAt, &f.RequestedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &f, json.Unmarshal(plan, &f.Plan)
}

// UpsertFile finds or creates the file for a plan and marks it requested now.
// inserted is true when this call created it.
func (s *Store) UpsertFile(ctx context.Context, mediaFileID, spec string, plan Plan) (f *File, inserted bool, err error) {
	body, err := json.Marshal(plan)
	if err != nil {
		return nil, false, err
	}
	var created bool
	row := s.db.QueryRow(ctx,
		`INSERT INTO download_files (media_file_id, spec, plan) VALUES ($1, $2, $3)
		 ON CONFLICT (media_file_id, spec) DO UPDATE SET requested_at = now()
		 RETURNING `+fileCols+`, xmax = 0`, mediaFileID, spec, body)
	var raw []byte
	f = &File{}
	if err := row.Scan(&f.ID, &f.MediaFileID, &f.Spec, &raw, &f.Status, &f.Progress, &f.Error, &f.SizeBytes, &f.ReadyAt, &f.RequestedAt, &created); err != nil {
		return nil, false, err
	}
	return f, created, json.Unmarshal(raw, &f.Plan)
}

func (s *Store) FileByID(ctx context.Context, id string) (*File, error) {
	return scanFile(s.db.QueryRow(ctx, `SELECT `+fileCols+` FROM download_files WHERE id = $1`, id))
}

// Requeue resets a failed (or vanished) file for another preparation; false when
// it is already queued or preparing.
func (s *Store) Requeue(ctx context.Context, id string) (bool, error) {
	tag, err := s.db.Exec(ctx,
		`UPDATE download_files SET status = 'queued', progress = 0, error = '', size_bytes = 0, ready_at = NULL
		 WHERE id = $1 AND status IN ('failed', 'ready')`, id)
	return tag.RowsAffected() > 0, err
}

func (s *Store) SetPreparing(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, `UPDATE download_files SET status = 'preparing', progress = 0, error = '' WHERE id = $1`, id)
	return err
}

func (s *Store) SetProgress(ctx context.Context, id string, pct int) error {
	_, err := s.db.Exec(ctx, `UPDATE download_files SET progress = $2 WHERE id = $1 AND status = 'preparing'`, id, pct)
	return err
}

func (s *Store) SetReady(ctx context.Context, id string, size int64) error {
	_, err := s.db.Exec(ctx,
		`UPDATE download_files SET status = 'ready', progress = 100, size_bytes = $2, ready_at = now() WHERE id = $1`, id, size)
	return err
}

// SetFailed records why preparation failed; status queued leaves it for a retry.
func (s *Store) SetFailed(ctx context.Context, id, status, message string) error {
	_, err := s.db.Exec(ctx, `UPDATE download_files SET status = $2, progress = 0, error = $3 WHERE id = $1`, id, status, message)
	return err
}

// Wanted reports whether any user still has the file in their downloads.
func (s *Store) Wanted(ctx context.Context, fileID string) (bool, error) {
	var wanted bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM downloads WHERE file_id = $1)`, fileID).Scan(&wanted)
	return wanted, err
}

// AddDownload records the user's request for a file; asking again returns the same row.
func (s *Store) AddDownload(ctx context.Context, userID int64, fileID, titleID string, episodeID *string, quality string) (string, error) {
	var id string
	err := s.db.QueryRow(ctx,
		`INSERT INTO downloads (user_id, file_id, title_id, episode_id, quality) VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_id, file_id) DO UPDATE SET quality = EXCLUDED.quality
		 RETURNING id`, userID, fileID, titleID, episodeID, quality).Scan(&id)
	return id, err
}

func (s *Store) DeleteDownload(ctx context.Context, userID int64, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM downloads WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}

// Expire deletes the files past their retention, failed ones after a day and
// any nobody asks for anymore once no job works on them, returning their ids.
func (s *Store) Expire(ctx context.Context, retention time.Duration) ([]string, error) {
	rows, err := s.db.Query(ctx,
		`DELETE FROM download_files f
		 WHERE (f.status = 'ready' AND GREATEST(f.ready_at, f.requested_at) < now() - make_interval(secs => $1))
			OR (f.status = 'failed' AND f.requested_at < now() - interval '1 day')
			OR (f.status IN ('ready', 'failed') AND NOT EXISTS (SELECT 1 FROM downloads d WHERE d.file_id = f.id))
		 RETURNING f.id`, retention.Seconds())
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// FileIDs lists every file row, for sweeping MP4s left on disk without one.
func (s *Store) FileIDs(ctx context.Context) (map[string]bool, error) {
	rows, err := s.db.Query(ctx, `SELECT id FROM download_files`)
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

// row is one of a user's downloads with what the payload shows about it.
type row struct {
	Download
	file *File
}

const downloadQuery = `
	SELECT d.id, d.quality, d.created_at, d.title_id, t.slug, t.name, t.translations,
		d.episode_id, se.season_number, e.episode_number, e.name, e.translations,
		(SELECT a.id FROM artwork a WHERE a.owner_kind = 'title' AND a.owner_id = t.id::text AND a.kind = 'poster' LIMIT 1),
		COALESCE((SELECT extract(epoch FROM a.created_at)::bigint FROM artwork a WHERE a.owner_kind = 'title' AND a.owner_id = t.id::text AND a.kind = 'poster' LIMIT 1), 0),
		(SELECT a.id FROM artwork a WHERE a.owner_kind = 'title' AND a.owner_id = t.id::text AND a.kind = 'backdrop' LIMIT 1),
		COALESCE((SELECT extract(epoch FROM a.created_at)::bigint FROM artwork a WHERE a.owner_kind = 'title' AND a.owner_id = t.id::text AND a.kind = 'backdrop' LIMIT 1), 0),
		(SELECT a.id FROM artwork a WHERE a.owner_kind = 'episode' AND a.owner_id = e.id::text AND a.kind = 'thumb' LIMIT 1),
		COALESCE((SELECT extract(epoch FROM a.created_at)::bigint FROM artwork a WHERE a.owner_kind = 'episode' AND a.owner_id = e.id::text AND a.kind = 'thumb' LIMIT 1), 0),
		mf.duration_seconds, f.id, f.media_file_id, f.spec, f.plan, f.status, f.progress, f.error, f.size_bytes,
		f.ready_at, f.requested_at
	FROM downloads d
	JOIN download_files f ON f.id = d.file_id
	JOIN media_files mf ON mf.id = f.media_file_id
	JOIN titles t ON t.id = d.title_id
	LEFT JOIN episodes e ON e.id = d.episode_id
	LEFT JOIN seasons se ON se.id = e.season_id`

func scanRow(ctx context.Context, r pgx.Row) (*row, error) {
	var (
		out                    row
		f                      File
		plan, ttr, etr         []byte
		episodeID, episodeName *string
		seasonNum, episodeNum  *int
		posterID, backdropID   *string
		thumbID                *string
		posterVer, backdropVer int64
		thumbVer               int64
	)
	d := &out.Download
	err := r.Scan(&d.ID, &d.Quality, &d.CreatedAt, &d.TitleID, &d.TitleSlug, &d.Title, &ttr,
		&episodeID, &seasonNum, &episodeNum, &episodeName, &etr,
		&posterID, &posterVer, &backdropID, &backdropVer, &thumbID, &thumbVer,
		&d.DurationSeconds,
		&f.ID, &f.MediaFileID, &f.Spec, &plan, &f.Status, &f.Progress, &f.Error, &f.SizeBytes, &f.ReadyAt, &f.RequestedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(plan, &f.Plan); err != nil {
		return nil, err
	}
	catalog.Localize(ctx, ttr, &d.Title, nil)
	d.Kind = "movie"
	if episodeID != nil {
		d.Kind = "episode"
		d.EpisodeID = *episodeID
		if seasonNum != nil && episodeNum != nil {
			d.SeasonNumber, d.EpisodeNumber = *seasonNum, *episodeNum
		}
		if episodeName != nil {
			d.EpisodeName = *episodeName
			catalog.Localize(ctx, etr, &d.EpisodeName, nil)
		}
	}
	d.PosterID, d.PosterVer = posterID, posterVer
	d.BackdropID, d.BackdropVer = backdropID, backdropVer
	d.ThumbID, d.ThumbVer = thumbID, thumbVer
	out.file = &f
	return &out, nil
}

func (s *Store) Download(ctx context.Context, userID int64, id string) (*row, error) {
	return scanRow(ctx, s.db.QueryRow(ctx, downloadQuery+` WHERE d.id = $1 AND d.user_id = $2`, id, userID))
}

func (s *Store) Downloads(ctx context.Context, userID int64) ([]*row, error) {
	rows, err := s.db.Query(ctx, downloadQuery+` WHERE d.user_id = $1 ORDER BY d.created_at DESC, d.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*row
	for rows.Next() {
		r, err := scanRow(ctx, rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
