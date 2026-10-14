package couchtv

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"

	"couchverse/internal/db"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/httpx"
	"couchverse/internal/media"
)

// Titles serves the per-title channel operations: what a title's promos are made of,
// whether it airs, its analysis, the promo audio picker's waveform and the soundtracks
// found on the web.
type Titles struct {
	analyzer    *Analyzer
	soundtracks *Soundtracks
}

type titleInput struct {
	ID string `path:"id" format:"uuid"`
}

type titleOutput struct{ Body CouchTvTitle }

// Get reads a title's channel facts.
func (h *Titles) Get(ctx context.Context, in *titleInput) (*titleOutput, error) {
	t, err := h.analyzer.Store.Title(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	return &titleOutput{Body: t.CouchTvTitle}, nil
}

// CouchTvPromoAudioPick is an admin's promo soundtrack excerpt.
type CouchTvPromoAudioPick struct {
	MediaFileID string `json:"mediaFileId" format:"uuid" doc:"A file of the title (its own or one of its episodes')."`
	StartMs     int    `json:"startMs" minimum:"0"`
	DurationMs  int    `json:"durationMs" minimum:"1000" maximum:"120000"`
}

// CouchTvShotPick is an admin's footage shot.
type CouchTvShotPick struct {
	MediaFileID string `json:"mediaFileId" format:"uuid" doc:"A file of the title (its own or one of its episodes')."`
	StartMs     int    `json:"startMs" minimum:"0"`
	DurationMs  int    `json:"durationMs" minimum:"1000" maximum:"3000" doc:"Rounded down to whole frames at 25 fps."`
}

// CouchTvTitleUpdate is an admin's edit of a title's channel facts. A part left out
// goes back to the analysis (or the soundtrack finder), which picks it again.
type CouchTvTitleUpdate struct {
	Excluded     bool                   `json:"excluded" doc:"Never on Couch TV."`
	Mood         *string                `json:"mood,omitempty" required:"false" enum:"cinematic,grit,bright" doc:"Absent lets the composer choose."`
	PromoAudio   *CouchTvPromoAudioPick `json:"promoAudio,omitempty" required:"false" doc:"An excerpt of the title's own audio for the promos, over any soundtrack; absent hands the promo audio back to the analysis."`
	SoundtrackID *string                `json:"soundtrackId,omitempty" required:"false" format:"uuid" doc:"One of the title's soundtracks for the promos (not with promoAudio); absent hands the choice back to the finder."`
	Shots        []CouchTvShotPick      `json:"shots,omitempty" required:"false" maxItems:"6" doc:"In the order they air; absent or empty hands the shots back to the analysis."`
}

type updateTitleInput struct {
	ID   string `path:"id" format:"uuid"`
	Body CouchTvTitleUpdate
}

// Update stores an admin's edit. Leaving the channel or coming back replans it; a part
// handed back to the analysis is picked again by a fresh one.
func (h *Titles) Update(ctx context.Context, in *updateTitleInput) (*titleOutput, error) {
	st := h.analyzer.Store
	before, err := st.Title(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	b := in.Body
	edit := titleEdit{excluded: b.Excluded, mood: b.Mood, shots: []CouchTvShot{}, soundtrack: b.SoundtrackID}
	if b.PromoAudio != nil && b.SoundtrackID != nil {
		return nil, httpx.BadRequestError("promos play one bed: promoAudio or soundtrackId, not both")
	}
	if p := b.PromoAudio; p != nil {
		if err := h.checkRange(ctx, in.ID, p.MediaFileID, p.StartMs, p.DurationMs); err != nil {
			return nil, err
		}
		edit.promo = &CouchTvPromoAudio{MediaFileID: p.MediaFileID, StartMs: p.StartMs, DurationMs: p.DurationMs, Source: SourceAdmin}
	}
	if id := b.SoundtrackID; id != nil {
		ok, err := st.SoundtrackOfTitle(ctx, *id, in.ID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, httpx.BadRequestError(fmt.Sprintf("soundtrack %s is not the title's", *id))
		}
	}
	for _, s := range b.Shots {
		frames := s.DurationMs / 40
		if err := h.checkRange(ctx, in.ID, s.MediaFileID, s.StartMs, frames*40); err != nil {
			return nil, err
		}
		edit.shots = append(edit.shots, CouchTvShot{MediaFileID: s.MediaFileID, StartMs: s.StartMs, DurationMs: frames * 40, Score: 1})
	}
	if err := st.UpdateTitle(ctx, in.ID, edit); err != nil {
		return nil, err
	}
	if before.Excluded != b.Excluded {
		if _, err := h.analyzer.Jobs.EnqueueJobOnce(ctx, PlanJob, PlanPayload{}, jobs.EnqueueOpts{}); err != nil {
			return nil, err
		}
	}
	handedBack := before.promoSource == SourceAdmin && edit.promo == nil ||
		before.ShotsSource == SourceAdmin && len(edit.shots) == 0
	if !b.Excluded {
		var err error
		switch {
		case handedBack || before.Excluded && before.AnalyzedVersion < AnalyzeVersion:
			err = h.analyzer.force(ctx, in.ID)
		case before.Excluded && (before.shotsVersion < ShotsVersion || before.trailerVersion < TrailerVersion):
			err = h.analyzer.repick(ctx, in.ID)
		}
		if err != nil {
			return nil, err
		}
	}
	// a new pick plays once its preview is downloaded
	if pick, err := st.soundtrackPick(ctx, in.ID); err != nil {
		return nil, err
	} else if !b.Excluded && pick != nil && !pick.Ready {
		if err := h.soundtracks.ready(ctx, in.ID); err != nil {
			return nil, err
		}
	}
	return h.Get(ctx, &titleInput{ID: in.ID})
}

// CouchTvSoundtrackSearch asks the finder to search a title's soundtrack again.
type CouchTvSoundtrackSearch struct {
	Query string `json:"query,omitempty" required:"false" maxLength:"200" doc:"The admin's own search (\"simpsons danny elfman\"); empty searches as the finder does. Its candidates only offer more to choose from: the pick stays."`
}

type searchSoundtrackInput struct {
	ID   string `path:"id" format:"uuid"`
	Body CouchTvSoundtrackSearch
}

// SearchSoundtrack queues a soundtrack search of the title.
func (h *Titles) SearchSoundtrack(ctx context.Context, in *searchSoundtrackInput) (*titleOutput, error) {
	t, err := h.analyzer.Store.Title(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if t.Excluded {
		return nil, httpx.Fail(http.StatusConflict, "excluded", "the title is not on Couch TV")
	}
	if err := h.soundtracks.Search(ctx, in.ID, in.Body.Query); err != nil {
		return nil, err
	}
	return h.Get(ctx, &titleInput{ID: in.ID})
}

// SoundtrackPreview plays a candidate's preview to the admin from the server's copy, so
// the browser never reaches the catalog's servers itself.
func (h *Titles) SoundtrackPreview(w http.ResponseWriter, r *http.Request) {
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	path, contentType, err := h.soundtracks.Preview(r.Context(), id)
	switch {
	case errors.Is(err, db.ErrNotFound):
		httpx.NotFound(w)
		return
	case err != nil:
		slog.Warn("couchtv: no soundtrack preview", "soundtrack", id, "err", err)
		httpx.Error(w, http.StatusBadGateway, "soundtrack_unavailable", "the music catalog did not deliver the preview")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	httpx.ServeFile(w, r, path)
}

// checkRange rejects a stretch that is not within one of the title's files.
func (h *Titles) checkRange(ctx context.Context, titleID, fileID string, startMs, durationMs int) error {
	seconds, ok, err := h.analyzer.Store.FileOfTitle(ctx, fileID, titleID)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.BadRequestError(fmt.Sprintf("media file %s is not the title's", fileID))
	}
	if float64(startMs+durationMs) > seconds*1000 {
		return httpx.BadRequestError(fmt.Sprintf("the stretch runs past the end of media file %s", fileID))
	}
	return nil
}

// Analyze runs the title's analysis again: its markers, then its promo picks and
// measurements (an admin's picks are kept).
func (h *Titles) Analyze(ctx context.Context, in *titleInput) (*titleOutput, error) {
	t, err := h.analyzer.Store.Title(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if t.Excluded {
		return nil, httpx.Fail(http.StatusConflict, "excluded", "the title is not on Couch TV")
	}
	if err := h.analyzer.Reanalyze(ctx, in.ID); err != nil {
		return nil, err
	}
	return h.Get(ctx, in)
}

// The waveform's reach: one request decodes at most maxWaveformMs of audio.
const (
	maxWaveformMs     = 15 * 60 * 1000
	defaultWaveformMs = 5 * 60 * 1000
)

type waveformInput struct {
	ID     string `path:"id" format:"uuid"`
	FromMs int    `query:"fromMs" minimum:"0" doc:"Where the stretch starts."`
	ToMs   int    `query:"toMs" minimum:"0" doc:"Where it ends; 5 minutes on (or the file's end) when omitted, at most 15 minutes after fromMs."`
	Points int    `query:"points" minimum:"10" maximum:"2000" default:"300" doc:"How many values to return (fewer for a stretch under 100 ms a value)."`
}

// CouchTvWaveform is a stretch of a file's loudness for the promo audio picker.
type CouchTvWaveform struct {
	MediaFileID string    `json:"mediaFileId" format:"uuid"`
	FromMs      int       `json:"fromMs"`
	ToMs        int       `json:"toMs"`
	StepMs      int       `json:"stepMs" doc:"The span each value covers."`
	Loudness    []float64 `json:"loudness" doc:"The loudest momentary loudness (LUFS, -70 to 0) of each step from fromMs."`
}

type waveformOutput struct{ Body CouchTvWaveform }

// Waveform measures a stretch of a file's first audio track.
func (h *Titles) Waveform(ctx context.Context, in *waveformInput) (*waveformOutput, error) {
	f, err := h.analyzer.Store.SourceFile(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if !f.Readable {
		return nil, httpx.Fail(http.StatusNotFound, "source_deleted", "the original file was removed after transcoding")
	}
	if _, err := os.Stat(f.Path); err != nil {
		return nil, httpx.Fail(http.StatusNotFound, "file_missing", "media file is missing on disk")
	}
	if !f.HasAudio {
		return nil, httpx.Fail(http.StatusConflict, "no_audio", "the file has no sound")
	}
	duration := int(f.Duration * 1000)
	from, to := in.FromMs, in.ToMs
	if to == 0 {
		to = min(from+defaultWaveformMs, duration)
	}
	if to <= from || to-from > maxWaveformMs || from >= duration {
		return nil, httpx.BadRequestError("toMs must be after fromMs, by at most 15 minutes, within the file")
	}
	l, err := media.MeasureLoudness(ctx, h.analyzer.FFmpegPath, f.Path, float64(from)/1000, float64(to-from)/1000)
	if err != nil {
		return nil, err
	}
	step := max((to-from)/max(in.Points, 1), int(media.LoudnessStep*1000))
	return &waveformOutput{Body: waveform(in.ID, from, to, step, l.Momentary)}, nil
}

// waveform buckets momentary loudness (one value per LoudnessStep) into steps of
// stepMs: the loudest of each, floored at -70 and rounded to 0.1 LU.
func waveform(fileID string, from, to, stepMs int, momentary []float64) CouchTvWaveform {
	n := (to - from + stepMs - 1) / stepMs
	per := float64(stepMs) / (media.LoudnessStep * 1000)
	out := CouchTvWaveform{MediaFileID: fileID, FromMs: from, ToMs: to, StepMs: stepMs, Loudness: make([]float64, n)}
	for i := range out.Loudness {
		peak := -70.0
		for j := int(float64(i) * per); j < min(int(float64(i+1)*per), len(momentary)); j++ {
			peak = max(peak, momentary[j])
		}
		out.Loudness[i] = math.Round(min(peak, 0)*10) / 10
	}
	return out
}
