package couchtv

import (
	"context"
	"time"
)

// CouchTvRenderHealth is how the rendering keeps up with the schedule (plan 4.3).
type CouchTvRenderHealth struct {
	RendererSeenAt *time.Time             `json:"rendererSeenAt" doc:"When the render sidecar last started or finished a job; null before it first ran."`
	Queue          CouchTvRenderQueue     `json:"queue"`
	Breaks         CouchTvBreakReadiness  `json:"breaks"`
	Failures       []CouchTvRenderFailure `json:"failures" doc:"The latest failed clips and breaks, newest first."`
	Cache          CouchTvRenderCache     `json:"cache"`
}

// CouchTvRenderQueue is the render backlog.
type CouchTvRenderQueue struct {
	Clips           int        `json:"clips" doc:"Clips waiting to render or rendering."`
	BacklogSeconds  float64    `json:"backlogSeconds" doc:"Seconds of video those clips make (a promo body not rendered yet counts as 16 s)."`
	FramesPerSecond float64    `json:"framesPerSecond" doc:"Render speed over the last day's clips; 0 before any."`
	EtaSeconds      *float64   `json:"etaSeconds" doc:"How long the backlog takes at that speed; null while the speed is unknown."`
	NeededAt        *time.Time `json:"neededAt" doc:"When the first waiting clip airs; null when none waits."`
	Assemblies      int        `json:"assemblies" doc:"Breaks whose clips are all rendered, waiting to be assembled or assembling."`
}

// CouchTvBreakReadiness counts the breaks of the next 24 hours, as far ahead as breaks are
// composed, by state.
type CouchTvBreakReadiness struct {
	Upcoming    int        `json:"upcoming" doc:"Breaks in the next 24 hours, as far ahead as breaks are composed."`
	Ready       int        `json:"ready" doc:"Assembled, ready to air."`
	Composed    int        `json:"composed" doc:"Composed, waiting for clips or assembly."`
	Uncomposed  int        `json:"uncomposed"`
	Failed      int        `json:"failed" doc:"Could not be composed or assembled; the evergreen break airs instead."`
	AtRisk      int        `json:"atRisk" doc:"Not ready and airing before the backlog would be rendered."`
	NextUnready *time.Time `json:"nextUnready" doc:"When the first break that is not ready airs; null when all are."`
	Evergreen   bool       `json:"evergreen" doc:"An evergreen break is assembled to air where a break is not ready."`
	Standby     bool       `json:"standby" doc:"The standby card is rendered."`
}

// CouchTvRenderFailure is a clip or break that failed.
type CouchTvRenderFailure struct {
	Kind  string    `json:"kind" enum:"clip,break"`
	ID    string    `json:"id" format:"uuid"`
	What  string    `json:"what" doc:"The clip's kind, or the break's (airing, evergreen)."`
	Error string    `json:"error"`
	At    time.Time `json:"at"`
}

// CouchTvRenderCache is the disk the rendered media takes.
type CouchTvRenderCache struct {
	Clips      int   `json:"clips" doc:"Rendered clips kept for reuse."`
	ClipBytes  int64 `json:"clipBytes"`
	Breaks     int   `json:"breaks" doc:"Assembled breaks on disk."`
	BreakBytes int64 `json:"breakBytes"`
}

// bodyEstimate stands in for the length of a promo body not rendered yet.
const bodyEstimate = 16 * time.Second

type renderHealthOutput struct{ Body CouchTvRenderHealth }

// RenderHealth reports how the rendering keeps up with the schedule.
func (h *Admin) RenderHealth(ctx context.Context, _ *struct{}) (*renderHealthOutput, error) {
	health, err := h.scheduler.Store.RenderHealth(ctx, h.scheduler.now())
	if err != nil {
		return nil, err
	}
	if state, ok := loadRendererState(ctx, h.scheduler.Settings); ok {
		health.RendererSeenAt = &state.SeenAt
	}
	return &renderHealthOutput{Body: health}, nil
}

// RenderHealth reads the render health as of now.
func (s *Store) RenderHealth(ctx context.Context, now time.Time) (CouchTvRenderHealth, error) {
	h := CouchTvRenderHealth{Failures: []CouchTvRenderFailure{}}
	q := &h.Queue
	var backlogMs int64
	if err := s.q.QueryRow(ctx, `
		SELECT count(*), coalesce(sum(CASE WHEN duration_ms > 0 THEN duration_ms ELSE $1 END), 0), min(needed_at)
		FROM couchtv_clips WHERE state IN ('pending', 'rendering')`, bodyEstimate.Milliseconds()).
		Scan(&q.Clips, &backlogMs, &q.NeededAt); err != nil {
		return h, err
	}
	q.BacklogSeconds = float64(backlogMs) / 1000
	var videoMs, renderMs int64
	if err := s.q.QueryRow(ctx, `
		SELECT coalesce(sum(duration_ms), 0), coalesce(sum(render_ms), 0) FROM couchtv_clips
		WHERE state = 'ready' AND render_ms > 0 AND rendered_at > $1`, now.Add(-24*time.Hour)).Scan(&videoMs, &renderMs); err != nil {
		return h, err
	}
	if renderMs > 0 {
		q.FramesPerSecond = float64(videoMs) * FPS / float64(renderMs)
		eta := q.BacklogSeconds * FPS / q.FramesPerSecond
		q.EtaSeconds = &eta
	}
	if err := s.q.QueryRow(ctx, `
		SELECT count(*) FROM couchtv_breaks b
		WHERE b.state = 'rendering' OR b.state = 'composed' AND NOT EXISTS (
			SELECT 1 FROM jsonb_array_elements(b.recipe->'clips') c
			JOIN couchtv_clips k ON k.id = (c->>'clipId')::uuid WHERE k.state <> 'ready')`).Scan(&q.Assemblies); err != nil {
		return h, err
	}

	b := &h.Breaks
	riskBefore := now
	if q.EtaSeconds != nil {
		riskBefore = now.Add(time.Duration(*q.EtaSeconds * float64(time.Second)))
	}
	if err := s.q.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE b.state = 'ready'),
		       count(*) FILTER (WHERE b.state IN ('composed', 'rendering')), count(*) FILTER (WHERE b.id IS NULL),
		       count(*) FILTER (WHERE b.state = 'failed'),
		       count(*) FILTER (WHERE b.state IS DISTINCT FROM 'ready' AND a.starts_at < $3),
		       min(a.starts_at) FILTER (WHERE b.state IS DISTINCT FROM 'ready')
		FROM couchtv_airings a LEFT JOIN couchtv_breaks b ON b.airing_id = a.id
		WHERE a.kind = 'break' AND a.state <> 'replaced' AND a.starts_at >= $1 AND a.starts_at < $2`,
		now, now.Add(composeAhead), riskBefore).
		Scan(&b.Upcoming, &b.Ready, &b.Composed, &b.Uncomposed, &b.Failed, &b.AtRisk, &b.NextUnready); err != nil {
		return h, err
	}
	if err := s.q.QueryRow(ctx, `SELECT
		EXISTS (SELECT 1 FROM couchtv_breaks WHERE kind = 'evergreen' AND state = 'ready'),
		EXISTS (SELECT 1 FROM couchtv_clips WHERE kind = 'standby' AND state = 'ready')`).Scan(&b.Evergreen, &b.Standby); err != nil {
		return h, err
	}

	rows, err := s.q.Query(ctx, `
		(SELECT 'clip', id, kind, error, updated_at FROM couchtv_clips WHERE state = 'failed')
		UNION ALL
		(SELECT 'break', id, kind, error, updated_at FROM couchtv_breaks WHERE state = 'failed')
		ORDER BY 5 DESC LIMIT 10`)
	if err != nil {
		return h, err
	}
	for rows.Next() {
		var f CouchTvRenderFailure
		if err := rows.Scan(&f.Kind, &f.ID, &f.What, &f.Error, &f.At); err != nil {
			rows.Close()
			return h, err
		}
		h.Failures = append(h.Failures, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return h, err
	}

	c := &h.Cache
	err = s.q.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM couchtv_clips WHERE state = 'ready'),
		(SELECT coalesce(sum(size_bytes), 0) FROM couchtv_clips WHERE state = 'ready'),
		(SELECT count(*) FROM couchtv_breaks WHERE path <> ''),
		(SELECT coalesce(sum(size_bytes), 0) FROM couchtv_breaks WHERE path <> '')`).
		Scan(&c.Clips, &c.ClipBytes, &c.Breaks, &c.BreakBytes)
	return h, err
}
