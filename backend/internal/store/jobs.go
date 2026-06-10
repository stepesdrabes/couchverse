package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/httpx"
)

type Job struct {
	ID          int64           `json:"id"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	Status      string          `json:"status"`
	Priority    int             `json:"priority"`
	RunAt       time.Time       `json:"runAt"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"maxAttempts"`
	Progress    int16           `json:"progress"`
	LastError   *string         `json:"lastError"`
	ClaimedAt   *time.Time      `json:"claimedAt"`
	CreatedAt   time.Time       `json:"createdAt"`
	FinishedAt  *time.Time      `json:"finishedAt"`
}

const jobCols = `id, type, payload, status, priority, run_at, attempts, max_attempts,
	progress, last_error, claimed_at, created_at, finished_at`

func scanJob(row pgx.Row) (*Job, error) {
	var j Job
	err := row.Scan(&j.ID, &j.Type, &j.Payload, &j.Status, &j.Priority, &j.RunAt, &j.Attempts,
		&j.MaxAttempts, &j.Progress, &j.LastError, &j.ClaimedAt, &j.CreatedAt, &j.FinishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &j, nil
}

type EnqueueOpts struct {
	Priority    int
	MaxAttempts int
	RunAt       time.Time // zero = now
}

func (s *Store) EnqueueJob(ctx context.Context, jobType string, payload any, opts EnqueueOpts) (int64, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 3
	}
	if opts.RunAt.IsZero() {
		opts.RunAt = time.Now()
	}
	var id int64
	err = s.pool.QueryRow(ctx,
		`INSERT INTO jobs (type, payload, priority, max_attempts, run_at) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		jobType, body, opts.Priority, opts.MaxAttempts, opts.RunAt).Scan(&id)
	return id, err
}

// EnqueueJobOnce skips enqueueing when an identical pending/running job exists.
func (s *Store) EnqueueJobOnce(ctx context.Context, jobType string, payload any, opts EnqueueOpts) (int64, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	var exists bool
	err = s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM jobs WHERE type = $1 AND payload = $2 AND status IN ('pending', 'running'))`,
		jobType, body).Scan(&exists)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, nil
	}
	return s.EnqueueJob(ctx, jobType, payload, opts)
}

// ClaimJob atomically claims the next runnable job of one of the given types.
func (s *Store) ClaimJob(ctx context.Context, types []string) (*Job, error) {
	if len(types) == 0 {
		return nil, httpx.ErrNotFound
	}
	return scanJob(s.pool.QueryRow(ctx, `
		UPDATE jobs SET status = 'running', claimed_at = now(), attempts = attempts + 1
		WHERE id = (
			SELECT id FROM jobs
			WHERE status = 'pending' AND run_at <= now() AND type = ANY($1)
			ORDER BY priority DESC, id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING `+jobCols, types))
}

func (s *Store) CompleteJob(ctx context.Context, id int64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE jobs SET status = 'done', progress = 100, finished_at = now() WHERE id = $1 AND status = 'running'`, id)
	return err
}

// FailJob retries with exponential backoff until attempts are exhausted.
func (s *Store) FailJob(ctx context.Context, job *Job, jobErr error) error {
	msg := jobErr.Error()
	if job.Attempts >= job.MaxAttempts {
		_, err := s.pool.Exec(ctx,
			`UPDATE jobs SET status = 'failed', last_error = $2, finished_at = now() WHERE id = $1`,
			job.ID, msg)
		return err
	}
	backoff := time.Duration(30) * time.Second << (job.Attempts - 1)
	_, err := s.pool.Exec(ctx,
		`UPDATE jobs SET status = 'pending', last_error = $2, run_at = now() + $3 WHERE id = $1`,
		job.ID, msg, backoff)
	return err
}

func (s *Store) SetJobProgress(ctx context.Context, id int64, pct int) error {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	_, err := s.pool.Exec(ctx, `UPDATE jobs SET progress = $2 WHERE id = $1`, id, pct)
	return err
}

func (s *Store) JobStatus(ctx context.Context, id int64) (string, error) {
	var status string
	err := s.pool.QueryRow(ctx, `SELECT status FROM jobs WHERE id = $1`, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", httpx.ErrNotFound
	}
	return status, err
}

func (s *Store) CancelJob(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE jobs SET status = 'cancelled', finished_at = now()
		 WHERE id = $1 AND status IN ('pending', 'running')`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.ErrNotFound
	}
	return nil
}

func (s *Store) RetryJob(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE jobs SET status = 'pending', attempts = 0, progress = 0, last_error = NULL,
			run_at = now(), finished_at = NULL
		 WHERE id = $1 AND status IN ('failed', 'cancelled')`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.ErrNotFound
	}
	return nil
}

// ResetRunningJobs requeues jobs orphaned by a previous process crash.
// Single-instance deployment makes this safe at startup.
func (s *Store) ResetRunningJobs(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE jobs SET status = 'pending', run_at = now() WHERE status = 'running'`)
	return tag.RowsAffected(), err
}

func (s *Store) ListJobs(ctx context.Context, status string, limit int) ([]Job, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx,
		`SELECT `+jobCols+` FROM jobs
		 WHERE ($1 = '' OR status = $1)
		 ORDER BY id DESC LIMIT $2`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := []Job{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *j)
	}
	return jobs, rows.Err()
}

func (s *Store) PendingJobCount(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM jobs WHERE status IN ('pending', 'running')`).Scan(&n)
	return n, err
}

// TranscodeProgress reports the max progress of pending transcode jobs for a
// media file (drives the "Preparing…" player state).
func (s *Store) TranscodeProgress(ctx context.Context, mediaFileID string) (int, error) {
	var progress int
	err := s.pool.QueryRow(ctx,
		`SELECT COALESCE(max(progress), 0) FROM jobs
		 WHERE type = 'transcode_hls' AND status IN ('pending', 'running')
			AND (payload->>'mediaFileId')::uuid = $1`, mediaFileID).Scan(&progress)
	return progress, err
}

// ActiveTranscode links a pending/running transcode job to the content it
// belongs to, for inline progress on the admin library pages.
type ActiveTranscode struct {
	JobID       int64   `json:"jobId"`
	MediaFileID string  `json:"mediaFileId"`
	TitleID     *string `json:"titleId"`
	EpisodeID   *string `json:"episodeId"`
	Variant     string  `json:"variant"`
	Status      string  `json:"status"`
	Progress    int     `json:"progress"`
}

func (s *Store) ActiveTranscodes(ctx context.Context) ([]ActiveTranscode, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT j.id, mf.id, COALESCE(mf.title_id, se.title_id), mf.episode_id,
			COALESCE(j.payload->>'variant', ''), j.status, j.progress
		 FROM jobs j
		 JOIN media_files mf ON mf.id = (j.payload->>'mediaFileId')::uuid
		 LEFT JOIN episodes e ON e.id = mf.episode_id
		 LEFT JOIN seasons se ON se.id = e.season_id
		 WHERE j.type = 'transcode_hls' AND j.status IN ('pending', 'running')
		 ORDER BY j.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ActiveTranscode{}
	for rows.Next() {
		var t ActiveTranscode
		if err := rows.Scan(&t.JobID, &t.MediaFileID, &t.TitleID, &t.EpisodeID,
			&t.Variant, &t.Status, &t.Progress); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// HasOtherPendingJobsForMediaFile reports whether any pending/running job other
// than excludeJobID still references the media file — sibling transcodes or a
// subtitle extraction that still needs to read the source.
func (s *Store) HasOtherPendingJobsForMediaFile(ctx context.Context, mediaFileID string, excludeJobID int64) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM jobs
		 WHERE status IN ('pending', 'running') AND id <> $2
			AND (payload->>'mediaFileId')::uuid = $1)`, mediaFileID, excludeJobID).Scan(&exists)
	return exists, err
}

// DeleteOldJobs prunes finished jobs to keep the table small.
func (s *Store) DeleteOldJobs(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM jobs WHERE status IN ('done', 'failed', 'cancelled') AND finished_at < $1`,
		time.Now().Add(-olderThan))
	if err != nil {
		return 0, fmt.Errorf("prune jobs: %w", err)
	}
	return tag.RowsAffected(), nil
}
