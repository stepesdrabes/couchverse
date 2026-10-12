package couchtv

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	"couchverse/internal/db"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/library"
	"couchverse/internal/feature/playback"
	"couchverse/internal/media"
	"couchverse/internal/settings"
)

// PrepareJob is the job that gets the programs about to air ready (plan 6.3).
const PrepareJob = "couchtv_prepare"

// prepareEvery is how often the job runs: transcodes finish meanwhile, and the
// readiness swaps follow them. It prepares the programs within prepareAhead.
const prepareEvery = 10 * time.Minute

// prepareOpts queue a program's package ahead of the other transcodes, since it
// has a time on air, and its trick-play rendition, which the channel never shows,
// as backfill, which renders do not wait for.
var prepareOpts = library.PrepareOpts{Priority: 10, TrickplayPriority: library.PriorityBackfill}

// PreparePayload names a periodic run by its slot, so every run queues the same
// next one; a run asked for by the plan job has none.
type PreparePayload struct {
	At string `json:"at,omitempty"`
}

// Preparer runs couchtv_prepare: for every program airing within prepareAhead it
// queues what its package lacks (library.Prepare), records the files that can
// air (couchtv_media, which makes them ready and their time on air exact), fits
// the planned rows to it, and swaps programs that will not be ready in time.
type Preparer struct {
	Store    *Store
	Files    *library.Store
	Jobs     *jobs.Store
	Settings *settings.Store
	DataDir  string
	// Changed hears that a swap changed the schedule; nil for nobody.
	Changed func()
	// Now is the clock; nil means time.Now (tests set it).
	Now func() time.Time
}

func (p *Preparer) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

// Start queues a run now.
func (p *Preparer) Start(ctx context.Context) error {
	_, err := p.Jobs.EnqueueJobOnce(ctx, PrepareJob, PreparePayload{}, jobs.EnqueueOpts{})
	return err
}

// Handle is the couchtv_prepare job.
func (p *Preparer) Handle(ctx context.Context, _ *jobs.Job, report func(int)) error {
	now := p.now()
	// the next run is queued first, so a failing run cannot end the chain
	next := now.Truncate(prepareEvery).Add(prepareEvery)
	if _, err := p.Jobs.EnqueueJobOnce(ctx, PrepareJob, PreparePayload{At: next.UTC().Format(time.RFC3339)},
		jobs.EnqueueOpts{RunAt: next}); err != nil {
		return err
	}
	programs, err := p.Store.Programs(ctx, now, now.Add(prepareAhead))
	if err != nil {
		return err
	}
	// every file once, with the credits its next airing leaves at
	credits := map[string]time.Duration{}
	var files []string
	for _, a := range programs {
		if _, seen := credits[a.MediaFileID]; a.MediaFileID != "" && !seen {
			credits[a.MediaFileID] = a.CreditsAt
			files = append(files, a.MediaFileID)
		}
	}
	for i, id := range files {
		if err := p.prepare(ctx, id, credits[id]); err != nil {
			return err
		}
		report(80 * (i + 1) / len(files))
	}
	cfg := LoadConfig(ctx, p.Settings)
	swapped := 0
	err = p.Store.Locked(ctx, func(st *Store) error {
		if err := st.FitPrograms(ctx, now, now.Add(prepareAhead)); err != nil {
			return err
		}
		lib, err := st.Library(ctx)
		if err != nil {
			return err
		}
		if swapped, err = swap(ctx, st, cfg, lib, now); err != nil {
			return err
		}
		return st.MarkStates(ctx, now, now.Add(prepareAhead))
	})
	if err == nil && swapped > 0 && p.Changed != nil {
		p.Changed()
	}
	return err
}

// prepare records whether a file can air and queues what it lacks.
func (p *Preparer) prepare(ctx context.Context, id string, credits time.Duration) error {
	mf, err := p.Files.MediaFileByID(ctx, id)
	if errors.Is(err, db.ErrNotFound) {
		return p.Store.ForgetMedia(ctx, id)
	}
	if err != nil {
		return err
	}
	pkg, err := playback.ReadPackage(p.DataDir, id)
	if err != nil {
		pkg = &playback.Package{}
	}
	tracks, err := p.Files.AudioStreamsForFile(ctx, id)
	if err != nil {
		return err
	}
	video, cut := airable(pkg, credits)
	silent := len(tracks) > 0 && len(pkg.Stereo) == 0

	if err := p.queue(ctx, mf, pkg, video, len(tracks) > 0); err != nil {
		return err
	}
	if video == nil || silent || cut <= 0 {
		return p.Store.ForgetMedia(ctx, id)
	}
	return p.Store.SaveMedia(ctx, id, video.Name, milliseconds(cut), credits)
}

// queue asks for what the channel would rather air: the package (the copied
// source and the audio) and, unless the source can air as is, a rung of the
// source's class up to 1080p. A rendition already queued, running or failed is
// left alone.
func (p *Preparer) queue(ctx context.Context, mf *media.MediaFile, pkg *playback.Package, video *playback.PackageRendition, hasAudio bool) error {
	if mf.VideoCodec == "" || mf.SourceDeletedAt != nil {
		return nil
	}
	variants, err := p.Files.VariantsForMediaFile(ctx, mf.ID)
	if err != nil {
		return err
	}
	pending := func(name string) bool { // on its way, or given up on
		return slices.ContainsFunc(variants, func(v library.TranscodeVariant) bool { return v.Name == name && v.Status != "ready" })
	}
	var names []string
	if sourceAirs(mf) && pkg.Source == nil && !pending(media.VariantSource) {
		names = append(names, media.VariantSource)
	}
	// the rung once the source is known not to air (it may have long segments)
	rung := channelRung(mf)
	if (video == nil || video.Name != media.VariantSource) && (!sourceAirs(mf) || pkg.Source != nil) &&
		!slices.ContainsFunc(pkg.Ladder, func(r playback.PackageRendition) bool { return r.Name == rung }) && !pending(rung) {
		names = append(names, rung)
	}
	needAudio := hasAudio && len(pkg.Stereo) == 0 && !pending(media.VariantAudio)
	if needAudio && len(names) == 0 {
		names = append(names, media.VariantSource) // the package job writes the audio
	}
	if len(names) == 0 {
		return nil
	}
	queued, err := library.Prepare(ctx, p.Files, p.Jobs, mf, needAudio, names, prepareOpts)
	if err == nil && len(queued) > 0 {
		slog.Info("couchtv: preparing a program", "mediaFileId", mf.ID, "renditions", queued)
	}
	return err
}

// sourceAirs reports whether the copied source fits the channel's video: 8-bit
// SDR H.264 up to 1080p (its segments are checked once it is copied).
func sourceAirs(mf *media.MediaFile) bool {
	sdr := mf.Video.HDR == "" || mf.Video.HDR == media.HDRNone
	return mf.VideoCodec == "h264" && mf.Video.BitDepth <= 8 && sdr && mf.Width <= 1920 && mf.Height <= 1080
}

// channelRung is the rung the channel airs a source from when the source itself
// cannot: the tallest up to 1080p that is not above the source's class.
func channelRung(mf *media.MediaFile) string {
	class := media.ClassHeight(mf.Width, mf.Height)
	best := ""
	for name, r := range media.Renditions {
		if r.Height <= 1080 && (class <= 0 || r.Height <= class) && (best == "" || r.Height > media.Renditions[best].Height) {
			best = name
		}
	}
	if best == "" {
		best = "480p"
	}
	return best
}
