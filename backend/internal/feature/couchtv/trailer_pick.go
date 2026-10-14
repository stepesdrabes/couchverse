package couchtv

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"

	"couchverse/internal/hls"
	"couchverse/internal/media"
)

// The scene picker (D43): cuts a title's trailer from its own scenes. It reads the subtitles of
// the dialogue track's language, which must follow the sound; rates the exchanges the way a
// trailer editor would; places each scene in the pauses around its lines and on cuts of the
// picture; checks the picture; and chooses a handful, spread out and in story order. Only short
// windows are decoded (the sound and the picture around each candidate). A title without
// subtitles that follow its sound has no trailer: what is said in a scene found by listening is
// unknown, and the trailer is not to carry a scene nobody has read.

// dialogueTrack is where a file's dialogue, in the original language, is heard.
type dialogueTrack struct {
	// FileID is the media file whose sound it is (the picture's own, or an alternate file of
	// the same movie or episode), Path its source.
	FileID, Path string
	// Stream is the audio stream's index (ffprobe's); nil takes the file's default track.
	Stream *int
	// Lang is the track's language, BCP 47; und when it says none.
	Lang string
}

// specifier is the ffmpeg stream specifier of the track.
func (d dialogueTrack) specifier() string {
	if d.Stream == nil {
		return ""
	}
	return "0:" + strconv.Itoa(*d.Stream)
}

// pickFile is a media file the picker cuts scenes from.
type pickFile struct {
	// ID is the picture's media file, Path its source.
	ID, Path string
	Duration float64
	Audio    dialogueTrack
	// Subs are subtitle files (WebVTT) in the dialogue's language; SubPaths the same files
	// under DATA_DIR, as the database has them.
	Subs, SubPaths []string
}

// scenePicker picks a trailer's scenes from a title's files.
type scenePicker struct {
	FFmpeg string
	// Risk rates a scene's picture for violence; nil takes none.
	Risk SceneRisk
	// Keywords are the title's own words: a line that says them is a line for its trailer.
	Keywords []string
	// Trace, when set, is told what the picker does.
	Trace func(format string, args ...any)
}

func (p *scenePicker) trace(format string, args ...any) {
	if p.Trace != nil {
		p.Trace(format, args...)
	}
}

// How many candidates are placed and checked (a decode each) per file: a movie's pick comes
// from more than an episode's.
const (
	candidatesMovie   = 24
	candidatesEpisode = 12
	// chunks (exchanges) tried for each scene wanted
	attemptsPerCandidate = 3
	// the quiet between a run and the cues around it that is worth a decode to find a pause in
	minQuiet = 0.4
)

// pickTitle picks the scenes of a title's trailer from its files (a movie's one, or episodes
// spread over a series).
func (p *scenePicker) pickTitle(ctx context.Context, files []pickFile) ([]sceneCandidate, error) {
	var all []sceneCandidate
	for i, f := range files {
		c, err := p.pickFile(ctx, i, f, len(files) == 1)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			p.trace("file %d: %v", i, err)
			continue
		}
		all = append(all, c...)
	}
	return chooseScenes(all, len(files)), nil
}

// pickFile finds the candidate scenes of one file in the first half of its runtime.
func (p *scenePicker) pickFile(ctx context.Context, index int, f pickFile, movie bool) ([]sceneCandidate, error) {
	from, to := f.Duration*shotWindowFrom, f.Duration*shotWindowTo
	limit := candidatesEpisode
	if movie {
		limit = candidatesMovie
	}
	// the fullest track first (the one with the most cues); the next when it does not follow
	// the sound
	var tracks []cueTrack
	for _, path := range f.Subs {
		track, err := readCueFile(path)
		if err != nil {
			p.trace("subtitles %s: %v", path, err)
			continue
		}
		tracks = append(tracks, track)
	}
	if len(tracks) == 0 {
		p.trace("file %d: no subtitles in the dialogue's language, no scenes from it", index)
	}
	slices.SortStableFunc(tracks, func(a, b cueTrack) int { return cmp.Compare(b.Count, a.Count) })
	for _, track := range tracks {
		cands, synced, err := p.fromSubtitles(ctx, index, f, track, from, to, limit)
		if err != nil {
			return nil, err
		}
		if synced {
			return cands, nil
		}
		p.trace("file %d: the subtitles do not follow the sound, no scenes from them", index)
	}
	return nil, nil
}

func readCueFile(path string) (cueTrack, error) {
	text, err := os.ReadFile(path)
	if err != nil {
		return cueTrack{}, err
	}
	v, err := hls.ParseWebVTT(string(text))
	if err != nil {
		return cueTrack{}, err
	}
	return readCues(v), nil
}

// audioWindow decodes [from, to) of the dialogue track and runs the detector over it.
func (p *scenePicker) audioWindow(ctx context.Context, f pickFile, from, to float64) (vadAnalysis, error) {
	m := newSpeechMeter()
	if err := media.Samples(ctx, p.FFmpeg, f.Audio.Path, f.Audio.specifier(), from, to-from, m.add); err != nil {
		return vadAnalysis{}, err
	}
	return m.analysis(), nil
}

// exchange is a stretch of dialogue decoded as one, with the runs of it a scene could play.
type exchange struct {
	from, to float64
	runs     []ratedRun
	best     float64
}

// ratedRun is a run of cues with its text score.
type ratedRun struct {
	run textRun
	f   textFeatures
}

// rateExchanges splits a track into exchanges, rates their runs and sorts the exchanges, the
// best first.
func (p *scenePicker) rateExchanges(track cueTrack, from, to float64) (out []*exchange, runs int) {
	for _, span := range exchangeChunks(track.Cues, from, to) {
		out = append(out, &exchange{from: span.From, to: span.To})
	}
	for _, r := range textRuns(track, from, to) {
		for _, c := range out {
			if r.start() >= c.from && r.end() <= c.to {
				rr := ratedRun{r, scoreText(r, p.Keywords)}
				c.runs = append(c.runs, rr)
				c.best = max(c.best, rr.f.Score)
				runs++
				break
			}
		}
	}
	out = slices.DeleteFunc(out, func(c *exchange) bool { return c.best < scoreFloor })
	slices.SortStableFunc(out, func(a, b *exchange) int { return cmp.Compare(b.best, a.best) })
	return out, runs
}

// decodedWindow is the sound around an exchange, analysed.
type decodedWindow struct {
	from, to float64
	a        vadAnalysis
}

// How far the sound is looked at around an exchange, and how far off the subtitles may be.
const (
	windowMargin = 8.0
	maxOffset    = 4.0
	offsetStep   = 0.25
	// exchanges the subtitles' offset is measured on
	syncSample = 40
	// an offset must beat none by this much (the share of its score no offset has to reach)
	zeroHold = 0.85
)

// fromSubtitles rates the exchanges of a subtitle track, places the best and checks them
// against the sound, which is decoded once for each exchange. It first measures how far the
// cues are from the sound (subtitles of another release are often a constant few seconds off)
// and shifts them by it; synced is false when they follow the sound at no offset.
func (p *scenePicker) fromSubtitles(ctx context.Context, index int, f pickFile, track cueTrack, from, to float64, limit int) (cands []sceneCandidate, synced bool, err error) {
	chunks, runs := p.rateExchanges(track, from, to)
	p.trace("file %d: %d cues, %d exchanges, %d runs", index, len(track.Cues), len(chunks), runs)

	cache := map[int64]decodedWindow{}
	key := func(start float64) int64 { return int64(math.Round(start * 1000)) }
	window := func(c *exchange, shift float64) (decodedWindow, error) {
		if w, ok := cache[key(c.from-shift)]; ok {
			return w, nil
		}
		w := decodedWindow{from: math.Max(0, c.from-windowMargin), to: math.Min(f.Duration, c.to+windowMargin)}
		var err error
		w.a, err = p.audioWindow(ctx, f, w.from, w.to)
		if err == nil {
			cache[key(c.from-shift)] = w
		}
		return w, err
	}

	var offsets []float64
	for o := -maxOffset; o <= maxOffset+1e-9; o += offsetStep {
		offsets = append(offsets, math.Round(o*100)/100)
	}
	reports := make([]syncReport, len(offsets))
	var wins []pauseWindow
	for _, c := range chunks[:min(syncSample, len(chunks))] {
		w, err := window(c, 0)
		if err != nil {
			return nil, false, err
		}
		around := cuesIn(track.Cues, w.from-maxOffset, w.to+maxOffset)
		for k, o := range offsets {
			reports[k].add(syncOf(w.a, w.from, around, o))
		}
		sw := sceneWindow{Origin: w.from, Audio: w.a}
		wins = append(wins, pauseWindow{w.from, w.to, sw.pauses(w.from, w.to)})
	}
	edgeHits, edgeTotal := edgeSync(offsets, wins, exchangeChunks(track.Cues, 0, f.Duration))
	shift, ok := chooseOffset(offsets, reports, edgeHits, edgeTotal)
	k := slices.Index(offsets, shift)
	p.trace("file %d: the cues are %+.2f s from the sound (ok %v): speech inside %.2f outside %.2f over %d frames; %d of %d exchanges on pauses", index,
		shift, ok, reports[k].Inside(), reports[k].Outside(), reports[k].inN+reports[k].outN, edgeHits[k], edgeTotal[k])
	if !ok {
		return nil, false, nil
	}
	if shift != 0 {
		track = track.shifted(shift)
		chunks, _ = p.rateExchanges(track, from, to)
	}

	for n, c := range chunks {
		if len(cands) >= limit || n >= attemptsPerCandidate*limit {
			break
		}
		dw, err := window(c, shift)
		if err != nil {
			return nil, false, err
		}
		a, wFrom := dw.a, dw.from
		w := sceneWindow{Origin: wFrom, Audio: a}
		// the picture is decoded for a scene the sound allows, once for the cuts it needs
		var scans []*pictureScan
		slices.SortStableFunc(c.runs, func(a, b ratedRun) int { return cmp.Compare(b.f.Score, a.f.Score) })
		placed := 0
		for _, r := range c.runs {
			if placed == 2 || r.f.Score < scoreFloor {
				break
			}
			if r.run.Before < minQuiet || r.run.After < minQuiet ||
				slices.ContainsFunc(cands, func(o sceneCandidate) bool { return r.run.start() < o.End && o.Start < r.run.end() }) {
				continue
			}
			S, E := r.run.start(), r.run.end()
			pl, why := placeScene(S, E, w, track.Songs)
			if why != "" {
				p.trace("  %7.1f %q: no place for a scene, %s", S, r.run.text(), why)
				continue
			}
			clear, over, ok := clarityOf(a, wFrom, pl.Onset, pl.Offset)
			if !ok {
				p.trace("  %7.1f %q: refused, buried in sound (%.1f dB over it)", S, r.run.text(), over)
				continue
			}
			var scan *pictureScan
			if scan, scans, err = p.scanFor(ctx, f, scans, pl.Start, pl.End); err != nil {
				return nil, false, err
			}
			wc := w
			wc.Cuts = scan.Cuts
			if pl, why = placeScene(S, E, wc, track.Songs); why != "" {
				p.trace("  %7.1f %q: no place for a scene on the shots, %s", S, r.run.text(), why)
				continue
			}
			// every cue the scene overlaps is heard in it, those of the run or not
			if word := strongIn(cuesIn(track.Cues, pl.Start, pl.End)); word != "" {
				p.trace("  %7.1f %q: refused, strong language (%s)", S, r.run.text(), word)
				continue
			}
			chk := scan.check(pl.Start, pl.End, p.Risk)
			if chk.Why != "" {
				p.trace("  %7.1f %q: refused, %s", S, r.run.text(), chk.Why)
				continue
			}
			cand := sceneCandidate{File: index, Start: pl.Start, End: pl.End, Source: "subtitles",
				Score:  (0.60*r.f.Score + 0.25*clear + 0.15*chk.Score) * pl.fit() * cutFit(a, wFrom, pl),
				Opener: r.f.Opener, Button: r.f.Button, words: contentWords(r.run.text())}
			for _, cue := range r.run.Cues {
				cand.Lines = append(cand.Lines, lineSpan{From: cue.Start, To: cue.End, Text: cue.Text})
			}
			cand.Lines = refineLines(cand.Lines, w, pl)
			cands = append(cands, cand)
			placed++
			p.trace("  %7.1f %5.1f-%5.1f score %.2f (text %.2f clear %.2f picture %.2f) %q", S, pl.Start, pl.End, cand.Score, r.f.Score, clear, chk.Score, r.run.text())
		}
	}
	return cands, true, nil
}

// chooseOffset picks how far the cues are from the sound, if they follow it at all. Two
// measures say it: how many exchanges have their edges on pauses (several times more than at
// most offsets, when the cues are in step at one) and how much likelier speech is inside the
// cues than away from them. Subtitles of another film are as good, or as bad, at every offset,
// and the picker listens instead.
func chooseOffset(offsets []float64, reports []syncReport, hits, total []int) (offset float64, ok bool) {
	// edges: smoothed over neighbouring offsets, against the median
	rates := make([]float64, len(offsets))
	for k := range offsets {
		h, t := 0, 0
		for j := max(0, k-1); j <= min(len(offsets)-1, k+1); j++ {
			h, t = h+hits[j], t+total[j]
		}
		rates[k] = share(h, t)
	}
	best := 0
	for k := range rates {
		if rates[k] > rates[best] {
			best = k
		}
	}
	// cues in step are the likeliest: another offset has to beat none by a clear margin
	zero := slices.Index(offsets, 0)
	if zero >= 0 && rates[zero] >= zeroHold*rates[best] {
		best = zero
	}
	sorted := slices.Clone(rates)
	slices.Sort(sorted)
	if total[best] >= 20 && rates[best] >= 0.2 && rates[best] >= 2.2*sorted[len(sorted)/2] {
		return offsets[best], true
	}
	// speech
	ratios := make([]float64, len(reports))
	best = 0
	for k, r := range reports {
		ratios[k] = r.Ratio()
		if r.Ratio() > reports[best].Ratio() {
			best = k
		}
	}
	if zero >= 0 && reports[zero].Ratio() >= zeroHold*reports[best].Ratio() {
		best = zero
	}
	slices.Sort(ratios)
	r := reports[best]
	ok = r.inN+r.outN >= 500 && r.Inside() >= 0.3 && (r.Ratio() >= 3.5 || r.Ratio() >= 2.2 && r.Ratio() >= 1.5*ratios[len(ratios)/2])
	return offsets[best], ok
}

// scanFor is a scan of the picture that covers [start, end) with a margin for the cuts around
// it: one already made, else a new one around the scene.
func (p *scenePicker) scanFor(ctx context.Context, f pickFile, scans []*pictureScan, start, end float64) (*pictureScan, []*pictureScan, error) {
	for _, s := range scans {
		if n := len(s.frames); n > 0 && s.frames[0].Time <= start-2 && s.frames[n-1].Time >= end+2 {
			return s, scans, nil
		}
	}
	s, err := scanPicture(ctx, p.FFmpeg, f.Path, math.Max(0, start-2.5), end-start+5)
	if err != nil {
		return nil, scans, err
	}
	return s, append(scans, s), nil
}

func cuesIn(cues []dialogueCue, from, to float64) []dialogueCue {
	var out []dialogueCue
	for _, c := range cues {
		if c.End > from && c.Start < to {
			out = append(out, c)
		}
	}
	return out
}

// refineLines fits the lines of a scene to what the sound bears out: inside the stretch the
// speech is heard in, and within a cue to where the detector hears speech. A line that is not
// heard in the scene (a distant shout the cue names) is left out.
func refineLines(lines []lineSpan, w sceneWindow, p scenePlace) []lineSpan {
	var out []lineSpan
	for _, l := range lines {
		from, to := math.Max(l.From, p.Onset), math.Min(l.To, p.Offset)
		if to-from < 0.25 {
			continue
		}
		if t, ok := w.first(w.Audio.Active, from-0.1, to); ok {
			from = math.Max(from, t)
		}
		if t, ok := w.last(w.Audio.Active, from, to+0.1); ok {
			to = math.Min(to, t+0.1)
		}
		if to-from >= 0.25 {
			out = append(out, lineSpan{From: from, To: to, Text: l.Text})
		}
	}
	return out
}

// storedScene turns a chosen candidate into what the title stores.
func storedScene(c sceneCandidate, files []pickFile) CouchTvTrailerScene {
	f := files[c.File]
	s := CouchTvTrailerScene{MediaFileID: f.ID, AudioFileID: f.Audio.FileID, AudioStream: f.Audio.Stream,
		StartMs: int(math.Round(c.Start * 1000)), DurationMs: int(math.Round((c.End-c.Start)*FPS)) * 1000 / FPS,
		Score: math.Round(c.Score*1000) / 1000, Source: c.Source, Lines: []CouchTvTrailerLine{}}
	for _, l := range c.Lines {
		s.Lines = append(s.Lines, CouchTvTrailerLine{FromMs: int(math.Round((l.From - c.Start) * 1000)),
			ToMs: int(math.Round((l.To - c.Start) * 1000)), Text: l.Text})
	}
	return s
}

// describeScenes is a plain-text script of a trailer, for the local run.
func describeScenes(scenes []CouchTvTrailerScene) string {
	var b strings.Builder
	total := 0
	for i, s := range scenes {
		total += s.DurationMs
		fmt.Fprintf(&b, "%d. %s +%.1fs score %.2f (%s)\n", i+1, formatClock(s.StartMs), float64(s.DurationMs)/1000, s.Score, s.Source)
		for _, l := range s.Lines {
			fmt.Fprintf(&b, "     %.2f-%.2f %s\n", float64(l.FromMs)/1000, float64(l.ToMs)/1000, l.Text)
		}
	}
	fmt.Fprintf(&b, "total %.1f s\n", float64(total)/1000)
	return b.String()
}

func formatClock(ms int) string {
	return fmt.Sprintf("%d:%02d:%02d.%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}
