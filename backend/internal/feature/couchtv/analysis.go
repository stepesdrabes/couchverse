package couchtv

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"couchverse/internal/db"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/library"
	"couchverse/internal/media"
)

// AnalyzeJob measures a title for the channel (plan 6.3): its promo soundtrack excerpt,
// footage shots and loudness.
const AnalyzeJob = "couchtv_analyze"

// AnalyzeVersion is the analysis' version: titles an older one measured are measured
// again (an admin's picks are kept).
const AnalyzeVersion = 1

// Analysis runs below every other job, the backfill lowest.
const (
	analyzePriority  = -5
	backfillPriority = -10
)

// analyzeDelay gathers the markers changes of a title (one per season) into one run.
const analyzeDelay = 5 * time.Minute

// loudnessSamples is how many episodes a series' loudness is measured on.
const loudnessSamples = 4

// AnalyzePayload is a couchtv_analyze job: one title (Force measures it again even at
// the current version), or neither for the backfill.
type AnalyzePayload struct {
	TitleID string `json:"titleId,omitempty"`
	Force   bool   `json:"force,omitempty"`
}

// Analyzer runs couchtv_analyze. The promo pick reads the title's markers, so it runs
// again whenever they change; the measurements (shots, loudness) once per version.
type Analyzer struct {
	Store      *Store
	Jobs       *jobs.Store
	Markers    *library.MarkerAnalyzer
	FFmpegPath string
}

// Start queues the backfill, which queues every title an older analysis (or none)
// measured.
func (a *Analyzer) Start(ctx context.Context) error {
	_, err := a.Jobs.EnqueueJobOnce(ctx, AnalyzeJob, AnalyzePayload{}, jobs.EnqueueOpts{Priority: backfillPriority})
	return err
}

// Queue picks a title's promo again a little later; library calls it whenever the
// title's markers changed.
func (a *Analyzer) Queue(ctx context.Context, titleID string) error {
	_, err := a.Jobs.EnqueueJobOnce(ctx, AnalyzeJob, AnalyzePayload{TitleID: titleID},
		jobs.EnqueueOpts{Priority: analyzePriority, RunAt: time.Now().Add(analyzeDelay)})
	return err
}

// Reanalyze runs the whole analysis of a title again: its markers, then its
// measurements and picks.
func (a *Analyzer) Reanalyze(ctx context.Context, titleID string) error {
	if err := a.Markers.QueueTitle(ctx, titleID); err != nil {
		return err
	}
	return a.force(ctx, titleID)
}

func (a *Analyzer) force(ctx context.Context, titleID string) error {
	_, err := a.Jobs.EnqueueJobOnce(ctx, AnalyzeJob, AnalyzePayload{TitleID: titleID, Force: true},
		jobs.EnqueueOpts{Priority: analyzePriority})
	return err
}

// Handle is the couchtv_analyze job.
func (a *Analyzer) Handle(ctx context.Context, job *jobs.Job, report func(int)) error {
	var p AnalyzePayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return err
	}
	if p.TitleID == "" {
		ids, err := a.Store.StaleTitles(ctx, AnalyzeVersion)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if _, err := a.Jobs.EnqueueJobOnce(ctx, AnalyzeJob, AnalyzePayload{TitleID: id},
				jobs.EnqueueOpts{Priority: backfillPriority}); err != nil {
				return err
			}
		}
		return nil
	}
	return a.analyze(ctx, p.TitleID, p.Force, report)
}

func (a *Analyzer) analyze(ctx context.Context, titleID string, force bool, report func(int)) error {
	t, err := a.Store.Title(ctx, titleID)
	if errors.Is(err, db.ErrNotFound) {
		return nil
	}
	if err != nil || t.Excluded {
		return err
	}
	files, err := a.Store.TitleFiles(ctx, titleID)
	if err != nil || len(files) == 0 {
		return err
	}
	res := analysis{version: AnalyzeVersion}
	var profile *media.Loudness
	if force || t.AnalyzedVersion < AnalyzeVersion {
		res.measured = true
		if t.TitleKind == "movie" {
			profile, res.loudness = a.movieLoudness(ctx, files[0])
		} else {
			res.loudness = a.seriesLoudness(ctx, files)
		}
		report(30)
		if t.ShotsSource != SourceAdmin {
			res.shots, res.shotsDone = a.shots(ctx, t.TitleKind, files), true
		}
		report(80)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if t.promoSource != SourceAdmin {
		promo, err := a.promo(ctx, t, files, profile)
		switch {
		case ctx.Err() != nil:
			return ctx.Err()
		case err != nil:
			slog.Warn("couchtv: no promo audio", "titleId", titleID, "err", err)
		default:
			res.promo, res.promoDone = promo, true
		}
	}
	return a.Store.SaveAnalysis(ctx, titleID, res)
}

// movieLoudness measures a whole film: its integrated loudness, and the profile the
// promo pick may use.
func (a *Analyzer) movieLoudness(ctx context.Context, f titleFile) (*media.Loudness, *float64) {
	if !f.Readable {
		return nil, nil
	}
	l, err := media.MeasureLoudness(ctx, a.FFmpegPath, f.Path, 0, 0)
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("couchtv: no loudness", "mediaFileId", f.ID, "err", err)
		}
		return nil, nil
	}
	return l, &l.Integrated
}

// seriesLoudness measures a few episodes spread over a series and averages them: a full
// pass over every episode would take hours on the Pi for a number the break mix only
// needs roughly (it is clamped to -23..-16 LUFS).
func (a *Analyzer) seriesLoudness(ctx context.Context, files []titleFile) *float64 {
	var lufs, weights []float64
	for _, f := range spread(files, loudnessSamples) {
		if !f.Readable {
			continue
		}
		l, err := media.MeasureLoudness(ctx, a.FFmpegPath, f.Path, 0, 0)
		if err != nil {
			if ctx.Err() == nil {
				slog.Warn("couchtv: no loudness", "mediaFileId", f.ID, "err", err)
			}
			continue
		}
		lufs, weights = append(lufs, l.Integrated), append(weights, f.Duration)
	}
	if len(lufs) == 0 {
		return nil
	}
	v := media.CombineLoudness(lufs, weights)
	return &v
}
