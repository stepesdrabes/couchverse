package library

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"time"

	"couchverse/internal/db"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/media"
)

// MarkersJob finds where a file's intro and credits are (docs/couch-tv-plan.md 3.1).
const MarkersJob = "analyze_markers"

// MarkersVersion is the analyzer's version. Files an older one looked at are analyzed
// again: their auto markers are replaced, admin ones never are.
const MarkersVersion = 1

// fingerprintVersion names the cached fingerprints; bump it when their windows change.
const fingerprintVersion = 1

// What is analyzed: a series' intro within an episode's first 12 minutes and its end
// credits within the last 6; a movie's credits within its last 12.
const (
	headWindow    = 12 * 60.0
	tailWindow    = 6 * 60.0
	creditsWindow = 12 * 60.0
)

// Analysis runs below every other job, the backfill below new files.
const (
	priorityNew      = -5
	priorityBackfill = -10
)

// seasonDelay lets a season's files be fingerprinted (an upload brings many at once)
// before the season is matched.
const seasonDelay = 2 * time.Minute

// AnalyzePayload is an analyze_markers job: one file (MediaFileID), one season of a
// series (TitleID and its season number in Seasons), or neither for the backfill.
type AnalyzePayload struct {
	MediaFileID string `json:"mediaFileId,omitempty"`
	TitleID     string `json:"titleId,omitempty"`
	Seasons     []int  `json:"seasons,omitempty"`
}

var (
	movieKinds  = []string{MarkerCredits, MarkerCreditsMusic}
	seriesKinds = []string{MarkerIntro, MarkerCredits}
)

// MarkerAnalyzer runs the analyze_markers job. A movie file is analyzed on its own; an
// episode file is fingerprinted, and its season matched once its files are.
type MarkerAnalyzer struct {
	Files      *Store
	Jobs       *jobs.Store
	DataDir    string
	FFmpegPath string
	// Analyzed hears about every title whose markers may have changed.
	Analyzed func(ctx context.Context, titleID string) error
}

// Start queues the backfill, which queues every file no analyzer (or an older one)
// looked at; once the library is analyzed it finds nothing.
func (a *MarkerAnalyzer) Start(ctx context.Context) error {
	_, err := a.Jobs.EnqueueJobOnce(ctx, MarkersJob, AnalyzePayload{}, jobs.EnqueueOpts{Priority: priorityBackfill})
	return err
}

// queueFile analyzes a file (again).
func queueFile(ctx context.Context, jb *jobs.Store, fileID string, priority int) error {
	_, err := jb.EnqueueJobOnce(ctx, MarkersJob, AnalyzePayload{MediaFileID: fileID}, jobs.EnqueueOpts{Priority: priority})
	return err
}

// QueueTitle analyzes a title again: a movie's files, every season of a series (from
// the fingerprints at hand).
func (a *MarkerAnalyzer) QueueTitle(ctx context.Context, titleID string) error {
	files, err := a.Files.titleFiles(ctx, titleID)
	if err != nil {
		return err
	}
	seasons := map[int]bool{}
	for _, f := range files {
		switch {
		case f.TitleKind == "movie" && f.Analyzable:
			if err := queueFile(ctx, a.Jobs, f.ID, priorityNew); err != nil {
				return err
			}
		case f.TitleKind == "series" && f.Analyzable && !seasons[f.Season]:
			seasons[f.Season] = true
			if _, err := a.Jobs.EnqueueJobOnce(ctx, MarkersJob, AnalyzePayload{TitleID: titleID, Seasons: []int{f.Season}},
				jobs.EnqueueOpts{Priority: priorityNew}); err != nil {
				return err
			}
		}
	}
	return nil
}

// Handle is the analyze_markers job.
func (a *MarkerAnalyzer) Handle(ctx context.Context, job *jobs.Job, report func(int)) error {
	var p AnalyzePayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return err
	}
	switch {
	case p.MediaFileID != "":
		return a.file(ctx, job, p.MediaFileID)
	case p.TitleID != "" && len(p.Seasons) == 1:
		return a.season(ctx, p.TitleID, p.Seasons[0], report)
	}
	return a.backfill(ctx)
}

func (a *MarkerAnalyzer) backfill(ctx context.Context) error {
	files, err := a.Files.staleMarkerFiles(ctx, MarkersVersion)
	if err != nil {
		return err
	}
	// series need chromaprint; without it they wait for an ffmpeg that has it
	series := media.HasChromaprint(a.FFmpegPath)
	queued := 0
	for _, f := range files {
		if f.TitleKind == "series" && !series {
			continue
		}
		if err := queueFile(ctx, a.Jobs, f.ID, priorityBackfill); err != nil {
			return err
		}
		queued++
	}
	if queued > 0 {
		slog.Info("markers: queued the backfill", "files", queued)
	}
	return nil
}

func (a *MarkerAnalyzer) file(ctx context.Context, job *jobs.Job, id string) error {
	f, err := a.Files.markerFile(ctx, id)
	if errors.Is(err, db.ErrNotFound) {
		return nil // deleted meanwhile, or on no title
	}
	if err != nil {
		return err
	}
	if !f.Analyzable {
		return a.Files.SetMarkersVersion(ctx, id, MarkersVersion)
	}
	if f.TitleKind == "series" {
		err = a.fingerprint(ctx, f)
		if errors.Is(err, errNoChromaprint) {
			slog.Warn("markers: ffmpeg has no chromaprint muxer, so series are not analyzed", "mediaFileId", id)
			return nil
		}
		if err == nil {
			_, err = a.Jobs.EnqueueJobOnce(ctx, MarkersJob, AnalyzePayload{TitleID: f.TitleID, Seasons: []int{f.Season}},
				jobs.EnqueueOpts{Priority: job.Priority, RunAt: time.Now().Add(seasonDelay)})
		}
	} else {
		var found []MediaMarker
		if found, err = a.movie(ctx, f); err == nil {
			if err = a.Files.ReplaceAutoMarkers(ctx, id, movieKinds, found, MarkersVersion); err == nil {
				err = a.changed(ctx, f.TitleID)
			}
		}
	}
	if err != nil && ctx.Err() == nil && job.Attempts >= job.MaxAttempts {
		// a file ffmpeg cannot read is not tried again at every start
		if verr := a.Files.SetMarkersVersion(ctx, id, MarkersVersion); verr != nil {
			slog.Warn("markers: give up", "mediaFileId", id, "err", verr)
		}
	}
	return err
}

func (a *MarkerAnalyzer) changed(ctx context.Context, titleID string) error {
	if a.Analyzed == nil || titleID == "" {
		return nil
	}
	return a.Analyzed(ctx, titleID)
}

// FileChanged tells whoever listens that a file's markers changed (an admin's edit).
func (a *MarkerAnalyzer) FileChanged(ctx context.Context, fileID string) error {
	f, err := a.Files.markerFile(ctx, fileID)
	if errors.Is(err, db.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return a.changed(ctx, f.TitleID)
}

// movie finds a movie's credits in its last minutes: the crawl from 2 fps frames, the
// music from the loudness.
func (a *MarkerAnalyzer) movie(ctx context.Context, f *markerFile) ([]MediaMarker, error) {
	from := max(0, f.Duration-creditsWindow)
	var looks []look
	var prev media.Frame
	err := media.Frames(ctx, a.FFmpegPath, f.Path, media.FrameOptions{
		From: from, FPS: creditsFPS, Width: creditsW, Height: creditsH, Chroma: true,
	}, func(fr media.Frame) error {
		if fr.Index == 0 {
			looks = append(looks, lookAt(nil, fr))
		} else {
			looks = append(looks, lookAt(&prev, fr))
		}
		prev = fr
		return nil
	})
	if err != nil {
		return nil, err
	}
	var music media.Stretch
	hasMusic := false
	loud, err := media.MeasureLoudness(ctx, a.FFmpegPath, f.Path, from, 0)
	switch {
	case err == nil:
		music, hasMusic = creditsMusic(loud)
	case ctx.Err() != nil:
		return nil, ctx.Err()
	default:
		slog.Warn("markers: no loudness", "mediaFileId", f.ID, "err", err) // a file without sound
	}
	span := f.Duration - from
	var found []MediaMarker
	if hasMusic {
		found = append(found, MediaMarker{Kind: MarkerCreditsMusic, StartMs: ms(from + music.Start), EndMs: ms(from + music.End)})
	}
	if at, ok := creditsAt(creditsLike(looks), music, hasMusic, span); ok {
		found = append(found, MediaMarker{Kind: MarkerCredits, StartMs: ms(from + at)})
	}
	return found, nil
}

func ms(seconds float64) *int {
	v := int(math.Round(seconds * 1000))
	return &v
}

func (a *MarkerAnalyzer) fingerprintPath(id, part string) string {
	return filepath.Join(a.DataDir, "cache", "fingerprints", id, fmt.Sprintf("%s-v%d.fp", part, fingerprintVersion))
}

// windows are where a file's head and tail fingerprints start and how long they are.
func windows(duration float64) (headFrom, headLen, tailFrom, tailLen float64) {
	return 0, min(headWindow, duration), max(0, duration-tailWindow), min(tailWindow, duration)
}

// fingerprint takes a file's head and tail fingerprints and caches them for its season.
func (a *MarkerAnalyzer) fingerprint(ctx context.Context, f *markerFile) error {
	if !media.HasChromaprint(a.FFmpegPath) {
		return errNoChromaprint
	}
	headFrom, headLen, tailFrom, tailLen := windows(f.Duration)
	for _, part := range []struct {
		name     string
		from, to float64
	}{{"head", headFrom, headLen}, {"tail", tailFrom, tailLen}} {
		fp, err := media.Fingerprint(ctx, a.FFmpegPath, f.Path, part.from, part.to)
		if err != nil {
			return err
		}
		if err := writeFingerprint(a.fingerprintPath(f.ID, part.name), fp); err != nil {
			return err
		}
	}
	return nil
}

func writeFingerprint(path string, fp []uint32) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	buf := make([]byte, 4*len(fp))
	for i, v := range fp {
		binary.LittleEndian.PutUint32(buf[4*i:], v)
	}
	tmp := path + ".part"
	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readFingerprint(path string) ([]uint32, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fp := make([]uint32, len(raw)/4)
	for i := range fp {
		fp[i] = binary.LittleEndian.Uint32(raw[4*i:])
	}
	return fp, nil
}

// season matches a season's episodes with each other: every file gets the intro and
// end credits it shares with its neighbours, or none without a consensus.
func (a *MarkerAnalyzer) season(ctx context.Context, titleID string, number int, report func(int)) error {
	files, err := a.Files.seasonFiles(ctx, titleID, number)
	if err != nil {
		return err
	}
	var usable []markerFile
	var heads, tails [][]uint32
	var headOffsets, tailOffsets []float64
	var episodes []string
	for i := range files {
		f := &files[i]
		head, tail, err := a.cachedFingerprints(ctx, f)
		switch {
		case ctx.Err() != nil:
			return ctx.Err()
		case errors.Is(err, errNoChromaprint):
			// left for an ffmpeg that can (the version stays old)
			continue
		case err != nil:
			if !errors.Is(err, errNotAnalyzable) {
				slog.Warn("markers: no fingerprint", "mediaFileId", f.ID, "err", err)
			}
			if err := a.Files.SetMarkersVersion(ctx, f.ID, MarkersVersion); err != nil {
				return err
			}
			continue
		}
		_, _, tailFrom, _ := windows(f.Duration)
		usable = append(usable, *f)
		heads, tails = append(heads, head), append(tails, tail)
		headOffsets, tailOffsets = append(headOffsets, 0), append(tailOffsets, tailFrom)
		episodes = append(episodes, f.EpisodeID)
		report(min(40, 40*(i+1)/len(files)))
	}
	intros := sharedSegments(heads, headOffsets, episodes)
	report(70)
	outros := sharedSegments(tails, tailOffsets, episodes)
	report(95)
	found := 0
	for i, f := range usable {
		var markers []MediaMarker
		if s := intros[i]; s != nil {
			markers = append(markers, MediaMarker{Kind: MarkerIntro, StartMs: ms(s.start), EndMs: ms(s.end)})
		}
		if s := outros[i]; s != nil {
			markers = append(markers, MediaMarker{Kind: MarkerCredits, StartMs: ms(s.start), EndMs: ms(s.end)})
		}
		if len(markers) > 0 {
			found++
		}
		if err := a.Files.ReplaceAutoMarkers(ctx, f.ID, seriesKinds, markers, MarkersVersion); err != nil {
			return err
		}
	}
	slog.Info("markers: season matched", "titleId", titleID, "season", number, "files", len(usable), "withMarkers", found)
	return a.changed(ctx, titleID)
}

// cachedFingerprints reads a file's fingerprints, taking them first if they are not
// cached (yet) and the file can be read.
func (a *MarkerAnalyzer) cachedFingerprints(ctx context.Context, f *markerFile) (head, tail []uint32, err error) {
	read := func() error {
		if head, err = readFingerprint(a.fingerprintPath(f.ID, "head")); err != nil {
			return err
		}
		tail, err = readFingerprint(a.fingerprintPath(f.ID, "tail"))
		return err
	}
	if read() == nil {
		return head, tail, nil
	}
	if !f.Analyzable {
		return nil, nil, errNotAnalyzable
	}
	if err := a.fingerprint(ctx, f); err != nil {
		return nil, nil, err
	}
	return head, tail, read()
}

var (
	errNotAnalyzable = errors.New("not an analyzable file")
	errNoChromaprint = errors.New("ffmpeg has no chromaprint muxer")
)
