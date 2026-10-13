package couchtv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"

	"couchverse/internal/db"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/media"
)

// SoundtrackJob finds a title's soundtrack on the web (plan 6.3, "Promo audio from the
// web"): candidates from the catalogs, the confident best one picked, the pick's preview
// downloaded and measured for the promos.
const SoundtrackJob = "couchtv_soundtrack"

// SoundtrackVersion is the finder's version: titles an older one searched are searched
// again at the next start (an admin's pick is kept).
const SoundtrackVersion = 1

// A search that reached no catalog tries again later, a few times.
const (
	soundtrackRetry   = 6 * time.Hour
	soundtrackRetries = 4
)

// SoundtrackPayload is a couchtv_soundtrack job: one title, or none for the backfill.
// Query is an admin's own search, Search the finder's again at the current version; with
// neither, a title the finder searched at this version only has its pick readied.
type SoundtrackPayload struct {
	TitleID string `json:"titleId,omitempty"`
	Query   string `json:"query,omitempty"`
	Search  bool   `json:"search,omitempty"`
	// Retry counts the tries after a search no catalog answered.
	Retry int `json:"retry,omitempty"`
}

// MusicFacts is what TMDB knows of a title's music: its composers and the English names
// it was released under. internal/app wires it to metadata.
type MusicFacts interface {
	MusicFacts(ctx context.Context, kind string, tmdbID int) (composers, names []string, err error)
}

// Soundtracks runs couchtv_soundtrack and serves the candidates' previews to the admin.
type Soundtracks struct {
	Store    *Store
	Jobs     *jobs.Store
	Catalogs []SoundtrackCatalog
	// Music adds TMDB's composers and names to the search; nil leaves them out.
	Music      MusicFacts
	DataDir    string
	FFmpegPath string
}

// Start queues the backfill, which searches every title an older finder (or none) did.
func (s *Soundtracks) Start(ctx context.Context) error {
	_, err := s.Jobs.EnqueueJobOnce(ctx, SoundtrackJob, SoundtrackPayload{}, jobs.EnqueueOpts{Priority: backfillPriority})
	return err
}

// Queue looks at a title a little later: the library calls it when the title's media
// changed, so a new title gets searched.
func (s *Soundtracks) Queue(ctx context.Context, titleID string) error {
	_, err := s.Jobs.EnqueueJobOnce(ctx, SoundtrackJob, SoundtrackPayload{TitleID: titleID},
		jobs.EnqueueOpts{Priority: analyzePriority, RunAt: time.Now().Add(analyzeDelay)})
	return err
}

// Search searches a title again now: the finder's own search, or the admin's query.
func (s *Soundtracks) Search(ctx context.Context, titleID, query string) error {
	_, err := s.Jobs.EnqueueJobOnce(ctx, SoundtrackJob, SoundtrackPayload{TitleID: titleID, Query: query, Search: query == ""},
		jobs.EnqueueOpts{Priority: analyzePriority})
	return err
}

// ready readies a title's pick now (an admin chose it).
func (s *Soundtracks) ready(ctx context.Context, titleID string) error {
	_, err := s.Jobs.EnqueueJobOnce(ctx, SoundtrackJob, SoundtrackPayload{TitleID: titleID}, jobs.EnqueueOpts{Priority: analyzePriority})
	return err
}

// Handle is the couchtv_soundtrack job. A search no catalog answered leaves the title as
// it was and tries again later; it never fails the job.
func (s *Soundtracks) Handle(ctx context.Context, job *jobs.Job, report func(int)) error {
	var p SoundtrackPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return err
	}
	if p.TitleID == "" {
		ids, err := s.Store.StaleSoundtracks(ctx, SoundtrackVersion)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if _, err := s.Jobs.EnqueueJobOnce(ctx, SoundtrackJob, SoundtrackPayload{TitleID: id},
				jobs.EnqueueOpts{Priority: backfillPriority}); err != nil {
				return err
			}
		}
		return nil
	}
	t, err := s.Store.soundtrackSubject(ctx, p.TitleID)
	if errors.Is(err, db.ErrNotFound) {
		return nil
	}
	if err != nil || t.Excluded {
		return err
	}
	if p.Query != "" || p.Search || t.Version < SoundtrackVersion {
		found, err := findSoundtracks(ctx, s.Catalogs, s.subject(ctx, p.TitleID, t), p.Query)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			slog.Warn("couchtv: a music catalog failed", "titleId", p.TitleID, "err", err)
			// the catalog that failed may have had the title's music: what the others
			// found stands only when it is a confident match
			if len(found) == 0 || found[0].Confidence < soundtrackConfident {
				return s.retry(ctx, p)
			}
		}
		if err := s.Store.SaveSoundtracks(ctx, p.TitleID, p.Query, SoundtrackVersion, found); err != nil {
			return err
		}
		if len(found) > 0 {
			slog.Info("couchtv: soundtrack search", "titleId", p.TitleID, "query", p.Query, "candidates", len(found),
				"best", found[0].Name, "confidence", found[0].Confidence)
		}
	}
	report(70)
	return s.readyPick(ctx, p)
}

// subject is the title as the finder searches for it, with TMDB's facts when there are any.
func (s *Soundtracks) subject(ctx context.Context, titleID string, t *soundtrackSubject) SoundtrackTitle {
	st := SoundtrackTitle{Name: t.Name, Year: t.Year, Series: t.Kind == "series"}
	if s.Music == nil || t.TmdbID == 0 {
		return st
	}
	composers, names, err := s.Music.MusicFacts(ctx, t.Kind, t.TmdbID)
	if err != nil {
		slog.Warn("couchtv: no TMDB music facts", "titleId", titleID, "err", err)
		return st
	}
	st.Composers = composers
	for _, n := range names {
		if n != "" && n != t.Name && !slices.Contains(st.Aliases, n) {
			st.Aliases = append(st.Aliases, n)
		}
	}
	return st
}

// retry queues the title again later after a search no catalog answered.
func (s *Soundtracks) retry(ctx context.Context, p SoundtrackPayload) error {
	if p.Retry >= soundtrackRetries {
		return nil
	}
	p.Retry++
	_, err := s.Jobs.EnqueueJobOnce(ctx, SoundtrackJob, p,
		jobs.EnqueueOpts{Priority: backfillPriority, RunAt: time.Now().Add(soundtrackRetry)})
	return err
}

// errUnplayable is a preview that cannot be a promo's bed.
var errUnplayable = errors.New("the preview is not usable as a promo bed")

// readyPick downloads and measures the title's pick. An admin may choose again while it
// runs, so it looks again until the pick it readied is still the pick.
func (s *Soundtracks) readyPick(ctx context.Context, p SoundtrackPayload) error {
	for range 3 {
		pick, err := s.Store.soundtrackPick(ctx, p.TitleID)
		if err != nil || pick == nil || pick.Ready {
			return err
		}
		path, err := s.download(ctx, pick.storedSoundtrack)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("couchtv: no soundtrack preview", "titleId", p.TitleID, "soundtrack", pick.ID, "err", err)
			return s.retry(ctx, SoundtrackPayload{TitleID: p.TitleID, Retry: p.Retry})
		}
		start, length, err := s.excerpt(ctx, filepath.Join(s.DataDir, path))
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("couchtv: a soundtrack preview cannot play", "titleId", p.TitleID, "soundtrack", pick.ID, "err", err)
			if err := s.Store.dropSoundtrack(ctx, p.TitleID, pick.ID); err != nil {
				return err
			}
			continue
		}
		ok, err := s.Store.soundtrackReady(ctx, p.TitleID, pick.ID, path, start, length)
		if err != nil || ok {
			return err
		}
	}
	return nil
}

// previewFile is where a catalog track's preview is kept, relative to DATA_DIR. The
// render sidecar extracts its beds from it (mediaserver.go).
func previewFile(catalog, trackID string) string {
	return filepath.Join("couchtv", "soundtracks", catalog+"-"+trackID+previewExt(catalog))
}

// download fetches a candidate's preview once and answers its path under DATA_DIR.
func (s *Soundtracks) download(ctx context.Context, t storedSoundtrack) (string, error) {
	rel := previewFile(t.Provider, t.TrackID)
	path := filepath.Join(s.DataDir, rel)
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		now := time.Now()
		return rel, os.Chtimes(path, now, now)
	}
	var catalog SoundtrackCatalog
	for _, c := range s.Catalogs {
		if c.Name() == t.Provider {
			catalog = c
		}
	}
	if catalog == nil {
		return "", fmt.Errorf("no catalog %q", t.Provider)
	}
	body, err := catalog.Preview(ctx, t.track())
	if err != nil {
		return "", err
	}
	defer body.Close()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".preview-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	// the render sidecar reads it too
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return "", err
	}
	n, err := io.Copy(tmp, body)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	switch {
	case err != nil:
		return "", err
	case n == 0:
		return "", errors.New("the preview is empty")
	}
	return rel, os.Rename(tmp.Name(), path)
}

// Preview is a candidate's preview on disk, downloaded first if need be, and its type.
func (s *Soundtracks) Preview(ctx context.Context, soundtrackID string) (path, contentType string, err error) {
	t, err := s.Store.soundtrack(ctx, soundtrackID)
	if err != nil {
		return "", "", err
	}
	rel, err := s.download(ctx, *t)
	if err != nil {
		return "", "", err
	}
	contentType = "audio/mp4"
	if t.Provider == CatalogDeezer {
		contentType = "audio/mpeg"
	}
	return filepath.Join(s.DataDir, rel), contentType, nil
}

// excerpt measures a preview and picks what promos play of it.
func (s *Soundtracks) excerpt(ctx context.Context, path string) (start, length time.Duration, err error) {
	l, err := media.MeasureLoudness(ctx, s.FFmpegPath, path, 0, 0)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %w", errUnplayable, err)
	}
	start, length, ok := previewExcerpt(l.Momentary)
	if !ok {
		return 0, 0, errUnplayable
	}
	return start, length, nil
}

// A preview bed is the preview without its fade in and out, at most bedMax (the longest
// promo, a full one with its end card, runs about 23 s) and at least bedMin, under which
// most promos would run out of music.
const (
	bedMax = 30 * time.Second
	bedMin = 12 * time.Second
	// fadeBelow is how far under the preview's typical loudness a fade lies
	fadeBelow = 10.0
)

// previewExcerpt is the stretch of a preview a promo plays, from its momentary loudness
// (one value per media.LoudnessStep): from where the music is up to where it fades.
func previewExcerpt(momentary []float64) (start, length time.Duration, ok bool) {
	var heard []float64
	for _, v := range momentary {
		if v > -70 {
			heard = append(heard, v)
		}
	}
	if len(heard) == 0 {
		return 0, 0, false
	}
	slices.Sort(heard)
	floor := heard[len(heard)/2] - fadeBelow
	first, last := -1, -1
	for i, v := range momentary {
		if v >= floor {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	step := time.Duration(media.LoudnessStep * float64(time.Second))
	// a momentary value covers the 400 ms before it
	start = max(0, time.Duration(first+1)*step-400*time.Millisecond).Truncate(100 * time.Millisecond)
	end := time.Duration(last+1) * step
	length = min(end-start, bedMax)
	return start, length, length >= bedMin
}
