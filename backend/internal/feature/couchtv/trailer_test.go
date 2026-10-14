package couchtv

import (
	"math"
	"math/rand"
	"slices"
	"strings"
	"testing"

	"couchverse/internal/hls"
)

func parseVTT(t *testing.T, text string) cueTrack {
	t.Helper()
	v, err := hls.ParseWebVTT(text)
	if err != nil {
		t.Fatal(err)
	}
	return readCues(v)
}

// Cue cleanup leaves what is said: no markup, sound descriptions, speaker labels or lyrics, and
// no on-screen captions in a file that is not written in capitals.
func TestReadCues(t *testing.T) {
	track := parseVTT(t, `WEBVTT

00:01.000 --> 00:03.000
<i>TONY: A famous man once said,</i>

00:03.100 --> 00:04.000
[GUNSHOTS]

00:04.500 --> 00:06.000
- Who said that?
- (SIGHS) Doesn't matter.

00:07.000 --> 00:09.000
♪ Jingle bells, jingle bells ♪

00:10.000 --> 00:11.500
NEW YORK CITY

00:12.000 --> 00:13.000
{\an8}Don't &amp; won't. <b>Now!</b>

00:14.000 --> 00:15.000
MAN #2 (O.S.): Behind you.

00:16.000 --> 00:17.000
It's fine, really.
`)
	var texts []string
	for _, c := range track.Cues {
		texts = append(texts, c.Text)
	}
	want := []string{"A famous man once said,", "Who said that? Doesn't matter.", "Don't & won't. Now!", "Behind you.", "It's fine, really."}
	if !slices.Equal(texts, want) {
		t.Errorf("cues %q, want %q", texts, want)
	}
	if track.Count != 8 {
		t.Errorf("%d cues counted, want 8", track.Count)
	}
	if len(track.Songs) != 1 || track.Songs[0].From != 7 || track.Songs[0].To != 9 {
		t.Errorf("songs %v, want 7-9 s", track.Songs)
	}
	if got := track.Cues[1]; got.Turns != 1 || got.Words != 5 {
		t.Errorf("a two-speaker cue: %+v", got)
	}

	// a file in capitals throughout is written that way: its lines are speech
	loud := parseVTT(t, "WEBVTT\n\n00:01.000 --> 00:02.000\nHEY, A PHONE BOOTH.\n\n00:03.000 --> 00:04.000\nI CAN CALL MY NEPHEW\n")
	if len(loud.Cues) != 2 {
		t.Errorf("a file in capitals lost cues: %+v", loud.Cues)
	}
}

func cue(start, end float64, text string) dialogueCue {
	return dialogueCue{Start: start, End: end, Text: text, Words: len(strings.Fields(text))}
}

// Runs begin where a thought begins and end where one ends, inside an exchange, with room
// to speak and in the window; a song splits an exchange.
func TestTextRuns(t *testing.T) {
	track := cueTrack{Cues: []dialogueCue{
		cue(10, 11.5, "A famous man once said,"),
		cue(11.6, 13, "we create our own demons."),
		cue(13.4, 14.8, "Who said that?"),
		cue(14.9, 16, "Doesn't matter."),
		cue(40, 41, "Hi."),
		cue(60, 61.5, "No way out."),
		cue(61.6, 63, "And that is that."),
	}}
	var got []string
	for _, r := range textRuns(track, 0, 100) {
		got = append(got, r.text())
	}
	want := []string{
		"A famous man once said, we create our own demons.",
		"A famous man once said, we create our own demons. Who said that?",
		"A famous man once said, we create our own demons. Who said that? Doesn't matter.",
		"Who said that? Doesn't matter.",
		"No way out.",
		"No way out. And that is that.",
	}
	// "we create..." carries on a thought, so it opens nothing; "Doesn't matter." is two words;
	// "And that is that." begins a thought (a capital) so it may open a run, but it ends the file
	for _, w := range want[:4] {
		if !slices.Contains(got, w) {
			t.Errorf("no run %q in %q", w, got)
		}
	}
	if slices.Contains(got, "we create our own demons.") || slices.Contains(got, "A famous man once said,") {
		t.Errorf("a run opens or ends mid-thought: %q", got)
	}
	for _, r := range textRuns(track, 0, 100) {
		if r.end()-r.start() > speechMax+1e-9 {
			t.Errorf("a run of %.1f s of speech: %q", r.end()-r.start(), r.text())
		}
	}
	// the window and the quiet around a run
	for _, r := range textRuns(track, 12, 16.5) {
		if r.start() < 12 || r.end() > 16.5 {
			t.Errorf("a run outside the window: %q", r.text())
		}
	}
	for _, r := range textRuns(track, 0, 100) {
		if r.text() == "Who said that? Doesn't matter." && (r.Before < 0.39 || r.Before > 0.41 || r.After < 5) {
			t.Errorf("quiet around %q: %.2f before, %.2f after", r.text(), r.Before, r.After)
		}
	}

	// lyrics split an exchange
	track.Songs = []timeSpan{{12, 13.2}}
	for _, r := range textRuns(track, 0, 100) {
		if overlapsSong(track.Songs, r.start(), r.end()) {
			t.Errorf("a run plays lyrics: %q", r.text())
		}
	}
}

func TestEndCompleteness(t *testing.T) {
	for text, want := range map[string]float64{
		"It's over.": 1, "Is it?": 1, `He said "stop!"`: 1, "Well...": 0.5, "Well…": 0.5, "and then,": 0, "No": 0, "A famous man -": 0,
	} {
		if got := endCompleteness(cue(0, 1, text)); got != want {
			t.Errorf("%q: %v, want %v", text, got, want)
		}
	}
}

func TestExchangeChunks(t *testing.T) {
	var cues []dialogueCue
	for i := range 20 { // a conversation of 60 s without a pause of a second
		cues = append(cues, cue(float64(i)*3, float64(i)*3+2.2+0.1*float64(i%3), "Line."))
	}
	cues = append(cues, cue(100, 101, "Alone."))
	chunks := exchangeChunks(cues, 0, 200)
	if len(chunks) < 3 {
		t.Fatalf("chunks %v, want a long conversation cut and the loner", chunks)
	}
	for _, c := range chunks {
		if c.To-c.From > chunkMax {
			t.Errorf("a chunk of %.1f s: %v", c.To-c.From, c)
		}
	}
	if last := chunks[len(chunks)-1]; last.From != 100 {
		t.Errorf("the last chunk %v", last)
	}
	if inside := exchangeChunks(cues, 30, 70); len(inside) == 0 || inside[0].From < 30 || inside[len(inside)-1].To > 70 {
		t.Errorf("window: %v", inside)
	}
}

// A trailer editor's taste: a punchy exchange with stakes beats exposition, filler, small
// talk and crudeness; the title's own name helps; ellipses are less whole than stops.
func TestScoreText(t *testing.T) {
	run := func(cues ...dialogueCue) textRun {
		return textRun{Cues: cues, Complete: endCompleteness(cues[len(cues)-1])}
	}
	punchy := run(cue(0, 1.6, "You're not the Mandarin."), cue(1.7, 3, "Are you? Are you?"))
	exposition := run(cue(0, 4.5, "In 1864, in Sand Creek, Colorado, the US military waited until the friendly Cheyenne braves had all gone hunting."))
	filler := run(cue(0, 1.5, "Uh, well, yeah."), cue(1.6, 3, "Okay, sure."))
	smallTalk := run(cue(0, 2, "Hello, sorry to bother you, please."), cue(2.1, 3.5, "Thank you, goodbye."))
	crude := run(cue(0, 1.5, "What the fuck is that?"), cue(1.6, 3, "I don't know, man."))
	stakes := run(cue(0, 2, "Nobody leaves this world alive."), cue(2.1, 3.6, "Then we fight."))
	plain := run(cue(0, 2, "The table is in the kitchen."), cue(2.1, 3.6, "The chairs are in the hall."))
	trailing := run(cue(0, 2, "Nobody leaves this world alive..."), cue(2.1, 3.6, "Then we fight..."))
	scores := map[string]float64{}
	for name, r := range map[string]textRun{"punchy": punchy, "exposition": exposition, "filler": filler, "smallTalk": smallTalk,
		"crude": crude, "stakes": stakes, "plain": plain, "trailing": trailing} {
		scores[name] = scoreText(r, nil).Score
	}
	for _, c := range []struct{ better, worse string }{
		{"punchy", "exposition"}, {"punchy", "plain"}, {"stakes", "plain"}, {"stakes", "trailing"}, {"punchy", "filler"},
		{"stakes", "smallTalk"}, {"punchy", "crude"}, {"stakes", "exposition"},
	} {
		if scores[c.better] <= scores[c.worse] {
			t.Errorf("%s (%.2f) should score over %s (%.2f)", c.better, scores[c.better], c.worse, scores[c.worse])
		}
	}
	if a, b := scoreText(plain, []string{"kitchen"}).Score, scoreText(plain, nil).Score; a <= b {
		t.Errorf("the title's own word does not help: %.2f against %.2f", a, b)
	}
	for name, s := range scores {
		if s < 0 || s > 1 {
			t.Errorf("%s scores %.2f", name, s)
		}
	}
	// a calm statement opens, a short punchline closes
	opener := scoreText(run(cue(0, 3, "I never thought they would find us here at all.")), nil)
	button := scoreText(run(cue(0, 1.5, "Are you serious?"), cue(1.6, 2.4, "Always.")), nil)
	if opener.Opener < 0.5 || opener.Button != 0 || button.Button < 0.7 || button.Opener != 0 {
		t.Errorf("opener %+v, button %+v", opener, button)
	}
}

// signal builds 16 kHz audio from stretches: speech-like (band-limited noise swelling at a
// syllable rate), a steady chord, or only the background.
type sound struct {
	from, to float64
	kind     string
}

func signal(seconds float64, stretches ...sound) []float32 {
	rng := rand.New(rand.NewSource(7))
	n := int(seconds * vadRate)
	out := make([]float32, n)
	var low float64
	for i := range out {
		t := float64(i) / vadRate
		white := rng.Float64()*2 - 1
		low += 0.3 * (white - low) // a rough low-pass: speech-band noise
		v := 0.002 * white         // the background room
		for _, s := range stretches {
			if t < s.from || t >= s.to {
				continue
			}
			switch s.kind {
			case "speech":
				// 4 syllables a second with a deep envelope, a voice of harmonics on a moving pitch
				env := math.Pow(0.5+0.5*math.Sin(2*math.Pi*4*(t-s.from)-math.Pi/2), 1.5)
				pitch := 120 + 25*math.Sin(2*math.Pi*0.7*t)
				voice := math.Sin(2*math.Pi*pitch*t) + 0.6*math.Sin(2*math.Pi*2*pitch*t) + 0.5*math.Sin(2*math.Pi*3*pitch*t) +
					0.4*math.Sin(2*math.Pi*5*pitch*t)
				v += 0.12 * env * (0.6*voice + 1.5*low)
			case "chord":
				v += 0.08 * (math.Sin(2*math.Pi*440*t) + math.Sin(2*math.Pi*554*t) + math.Sin(2*math.Pi*659*t))
			}
		}
		out[i] = float32(v)
	}
	return out
}

// The detector finds speech that swells the way syllables do, with its edges to a fraction of
// a second, and takes a steady chord, loud as it is, for nothing.
func TestSpeechDetector(t *testing.T) {
	a := analyzeSpeech(signal(30,
		sound{3, 5.5, "speech"}, sound{6.3, 8, "speech"}, sound{12, 16, "chord"}, sound{20, 24, "speech"}, sound{21, 23, "chord"}))
	utts := a.utterances(vadMinPause)
	// the chord's start and end are steps, which may read as a swell for half a second; what holds
	// is not speech
	for _, u := range utts {
		if u.From >= 11.5 && u.To <= 17 && (u.To-u.From > 0.7 || u.From > 12.7 && u.To < 15.3) {
			t.Errorf("a steady chord was taken for speech: %v", utts)
		}
	}
	near := func(u utterance, from, to float64) bool {
		return math.Abs(u.From-from) < 0.35 && math.Abs(u.To-to) < 0.35
	}
	var found [3]bool
	for _, u := range utts {
		switch {
		case near(u, 3, 5.5):
			found[0] = true
		case near(u, 6.3, 8):
			found[1] = true
		case u.From > 19 && u.To < 25:
			found[2] = true
		}
	}
	// the first two are separated by a pause of 0.8 s: two utterances (a pause of 0.28 s or
	// more bounds one); the third is speech over a chord
	if !found[0] || !found[1] || !found[2] {
		t.Errorf("utterances %v, want speech at 3-5.5, 6.3-8 and about 20-24", utts)
	}
	// pauses are quiet in between and long enough
	w := sceneWindow{Origin: 0, Audio: a}
	got := w.pauses(1, 10)
	var between bool
	for _, p := range got {
		if p.From < 5.8 && p.To > 6.0 && p.To-p.From >= minPause {
			between = true
		}
	}
	if !between {
		t.Errorf("no pause found between the first two utterances: %v", got)
	}
	if share := a.speechIn(3.2, 5.3); share < 0.8 {
		t.Errorf("speech share %.2f inside the first utterance", share)
	}
	if share := a.speechIn(12.8, 15.3); share > 0.02 {
		t.Errorf("speech share %.2f over the chord", share)
	}
}

func TestMeterIsStreaming(t *testing.T) {
	x := signal(10, sound{2, 6, "speech"})
	whole := analyzeSpeech(x)
	m := newSpeechMeter()
	for i := 0; i < len(x); i += 7001 { // chunks that are not whole frames
		m.add(x[i:min(len(x), i+7001)])
	}
	streamed := m.analysis()
	if len(streamed.Speech) != len(whole.Speech) || !slices.Equal(streamed.Speech, whole.Speech) {
		t.Errorf("streamed %d frames, whole %d, equal %v", len(streamed.Speech), len(whole.Speech), slices.Equal(streamed.Speech, whole.Speech))
	}
}

// window builds the sound of a scene's surroundings from stretches of speech (strict), loud
// frames and quiet ones: everything outside the stretches is quiet.
func window(seconds float64, speech ...timeSpan) sceneWindow {
	n := int(seconds / frameSec)
	a := vadAnalysis{Speech: make([]bool, n), Active: make([]bool, n), Quiet: make([]bool, n),
		Level: make([]float64, n), Floor: make([]float64, n), Clarity: make([]float64, n)}
	for i := range n {
		a.Quiet[i] = true
		t := float64(i) * frameSec
		for _, s := range speech {
			if t >= s.From && t < s.To {
				a.Speech[i], a.Active[i], a.Quiet[i] = true, true, false
				a.Level[i], a.Clarity[i] = -20, 0.8
			}
		}
		if !a.Speech[i] {
			a.Level[i] = -50
		}
		a.Floor[i] = -50
	}
	return sceneWindow{Audio: a}
}

func TestPlaceScene(t *testing.T) {
	// two lines at 10-12 and 12.6-14.2, with pauses around: a scene around both
	w := window(30, timeSpan{10, 12}, timeSpan{12.6, 14.2})
	p, why := placeScene(10, 14.2, w, nil)
	if why != "" {
		t.Fatal(why)
	}
	if math.Abs(p.Start-9.55) > 0.05 || math.Abs(p.End-14.8) > 0.05 {
		t.Errorf("scene %.2f-%.2f, want 9.55 and 14.8 (a lead of 0.45 s and a tail of 0.6 s)", p.Start, p.End)
	}
	if p.Start*FPS != math.Floor(p.Start*FPS) || p.End*FPS != math.Floor(p.End*FPS) {
		t.Errorf("not whole frames: %v %v", p.Start, p.End)
	}

	// a cut near the start opens the scene on its shot; one right after the last line ends it
	w.Cuts = []float64{9.2, 14.55, 20}
	p, why = placeScene(10, 14.2, w, nil)
	if why != "" {
		t.Fatal(why)
	}
	if math.Abs(p.Start-9.2) > 0.05 || math.Abs(p.End-14.55) > 0.05 {
		t.Errorf("scene %.2f-%.2f on cuts, want 9.2 and 14.55", p.Start, p.End)
	}

	// a cut in the first second moves the start to it
	w.Cuts = []float64{9.9}
	if p, why = placeScene(10, 14.2, w, nil); why != "" || math.Abs(p.Start-9.9) > 0.05 {
		t.Errorf("a cut in the first second: %.2f %q, want a start at 9.9", p.Start, why)
	}
	w.Cuts = nil

	// short lines grow to three seconds, into the pause after them
	w = window(30, timeSpan{10, 11.2})
	if p, why = placeScene(10, 11.2, w, nil); why != "" || p.End-p.Start < 3-1e-9 {
		t.Errorf("a short line: %.2f-%.2f %q, want 3 s at least", p.Start, p.End, why)
	}

	// no pause before: the line before runs right into it
	w = window(30, timeSpan{8, 9.9}, timeSpan{10, 12})
	if _, why = placeScene(10, 12, w, nil); why == "" {
		t.Error("a scene was placed with no pause before it")
	}
	// no pause after
	w = window(30, timeSpan{10, 12}, timeSpan{12.1, 13})
	if _, why = placeScene(10, 12, w, nil); why == "" {
		t.Error("a scene was placed with no pause after it")
	}
	// nothing is heard where the subtitles say
	w = window(30, timeSpan{20, 22})
	if _, why = placeScene(10, 12, w, nil); why == "" {
		t.Error("a scene was placed where nothing is heard")
	}
	// lyrics
	w = window(30, timeSpan{10, 12})
	if _, why = placeScene(10, 12, w, []timeSpan{{12.4, 13}}); why != "lyrics" {
		t.Errorf("lyrics next to the scene: %q", why)
	}
	// too much speech for 8 s
	w = window(30, timeSpan{10, 18.5})
	if _, why = placeScene(10, 18.5, w, nil); why == "" {
		t.Error("a scene of 9 s was placed")
	}
	// the sound must reach 3.5 s around
	w = window(30, timeSpan{1, 3})
	if _, why = placeScene(1, 3, w, nil); why == "" {
		t.Error("a scene was placed with no sound before it")
	}
}

func TestCutFit(t *testing.T) {
	pl := scenePlace{Start: 9.55, End: 12.7, Onset: 10, Offset: 12.1}
	w := window(30, timeSpan{10, 12})
	if f := cutFit(w.Audio, 0, pl); f != 1 {
		t.Errorf("cuts in the quiet: %.2f", f)
	}
	// a noise that goes on through the cuts, as loud as the speech
	for i := range w.Audio.Level {
		w.Audio.Level[i] = -20
	}
	if f := cutFit(w.Audio, 0, pl); math.Abs(f-0.7) > 1e-9 {
		t.Errorf("cuts in the noise: %.2f, want 0.7", f)
	}
	// 6 dB under the speech is half way
	for i := range w.Audio.Level {
		if !w.Audio.Speech[i] {
			w.Audio.Level[i] = -26
		}
	}
	if f := cutFit(w.Audio, 0, pl); math.Abs(f-0.85) > 1e-9 {
		t.Errorf("cuts 6 dB under the speech: %.2f, want 0.85", f)
	}
}

func TestRefineLines(t *testing.T) {
	w := window(30, timeSpan{10.2, 12}, timeSpan{12.6, 14})
	p := scenePlace{Start: 9.6, End: 14.6, Onset: 10.0, Offset: 14.2}
	got := refineLines([]lineSpan{{9.9, 12.3, "One."}, {12.5, 14.4, "Two."}, {20, 22, "Elsewhere."}, {10.0, 10.1, "Blip."}}, w, p)
	if len(got) != 2 {
		t.Fatalf("lines %+v, want two", got)
	}
	if got[0].From < p.Onset || math.Abs(got[0].From-10.2) > 0.05 || got[0].To > 12.15 {
		t.Errorf("first line %+v, want it to start with the sound at 10.2 and end with it", got[0])
	}
	if got[1].To > 14.15 {
		t.Errorf("second line %+v, want it to end with the sound at 14", got[1])
	}
}

func TestChooseOffset(t *testing.T) {
	var offsets []float64
	for o := -maxOffset; o <= maxOffset+1e-9; o += offsetStep {
		offsets = append(offsets, o)
	}
	n := len(offsets)
	at := func(o float64) int { return slices.Index(offsets, o) }
	filled := func(v int) []int {
		out := make([]int, n)
		for k := range out {
			out[k] = v
		}
		return out
	}
	total := filled(40)
	flat := make([]syncReport, n)
	for k := range flat {
		flat[k] = syncReport{in: 100, inN: 300, out: 900, outN: 1500}
	}

	// edges on pauses at +2 s, the rate falling away to either side as it does in a real track
	peaked := filled(7)
	for o, hits := range map[float64]int{1.25: 9, 1.5: 11, 1.75: 14, 2: 19, 2.25: 17, 2.5: 12, 2.75: 9} {
		peaked[at(o)] = hits
	}
	if o, ok := chooseOffset(offsets, flat, peaked, total); !ok || o != 2 {
		t.Errorf("edges peak at +2: offset %v ok %v", o, ok)
	}
	// the speech says it: a ratio of 4 at 0, flat elsewhere
	speech := slices.Clone(flat)
	speech[at(0)] = syncReport{in: 250, inN: 300, out: 150, outN: 1500}
	if o, ok := chooseOffset(offsets, speech, filled(7), total); !ok || o != 0 {
		t.Errorf("speech peaks at 0: offset %v ok %v", o, ok)
	}
	// subtitles of another film: nothing stands out
	if _, ok := chooseOffset(offsets, flat, filled(7), total); ok {
		t.Error("flat sync was taken for in step")
	}
	// too little to say
	few := filled(0)
	few[at(0)] = 3
	if _, ok := chooseOffset(offsets, flat, few, filled(5)); ok {
		t.Error("five exchanges were taken for proof")
	}
}

func TestSyncOf(t *testing.T) {
	w := window(20, timeSpan{5, 7}, timeSpan{10, 12})
	cues := []dialogueCue{cue(5, 7, "One."), cue(10, 12, "Two.")}
	in := syncOf(w.Audio, 0, cues, 0)
	if in.Inside() < 0.99 || in.Outside() != 0 {
		t.Errorf("cues in step: %+v", in)
	}
	off := syncOf(w.Audio, 0, cues, 3)
	if off.Inside() != 0 || off.Outside() < 0.2 {
		t.Errorf("cues 3 s late: %+v", off)
	}
}

func cand(file int, start, length, score float64, text string) sceneCandidate {
	return sceneCandidate{File: file, Start: start, End: start + length, Score: score, words: contentWords(text)}
}

var distinctLines = []string{"harbour lantern whisper", "copper mountain engine", "velvet thunder garden", "silver horizon compass",
	"marble river thought", "crimson window ladder", "granite forest melody", "amber desert harvest", "ivory castle orchard",
	"cobalt island blanket", "scarlet meadow trumpet", "jasmine canyon bicycle"}

func TestChooseScenes(t *testing.T) {
	var cands []sceneCandidate
	// a movie: good scenes a few minutes apart, a near twin of the best, and one a minute from it
	for i := range 12 {
		cands = append(cands, cand(0, 300+float64(i)*300, 5, 0.9-0.03*float64(i), distinctLines[i]))
	}
	cands = append(cands, cand(0, 330, 5, 0.89, "close by"))
	cands = append(cands, sceneCandidate{File: 0, Start: 900, End: 905, Score: 0.88, words: contentWords("different thing entirely")})
	cands = append(cands, cand(0, 5000, 5, 0.1, "too weak"))
	got := chooseScenes(cands, 1)
	if len(got) < 6 || len(got) > scenesMax {
		t.Fatalf("%d scenes", len(got))
	}
	total := 0.0
	for i, c := range got {
		total += c.length()
		if c.Score < scoreFloor {
			t.Errorf("a weak scene: %+v", c)
		}
		if i > 0 {
			if c.Start < got[i-1].Start {
				t.Errorf("not in story order: %v then %v", got[i-1].Start, c.Start)
			}
			if c.Start-got[i-1].End < sceneSpacing {
				t.Errorf("scenes %.0f and %.0f are too close", got[i-1].Start, c.Start)
			}
		}
	}
	if total > trailerSeconds {
		t.Errorf("%.0f s of scenes", total)
	}

	// the same joke twice is told once
	twin := []sceneCandidate{cand(0, 100, 5, 0.9, "this is iron patriot now"), cand(0, 1000, 5, 0.85, "iron patriot now suddenly"),
		cand(0, 2000, 5, 0.6, "something else altogether"), cand(0, 3000, 5, 0.6, "another thing again")}
	if got = chooseScenes(twin, 1); len(got) != 3 {
		t.Errorf("%d scenes, want the joke once and the two others", len(got))
	}

	// filler goes, down to six scenes: a short trailer of strong scenes beats a long one
	var mixed []sceneCandidate
	for i, score := range []float64{0.8, 0.75, 0.7, 0.7, 0.3, 0.3, 0.3} {
		mixed = append(mixed, cand(0, 100+float64(i)*400, 8, score, distinctLines[i]))
	}
	if got = chooseScenes(mixed, 1); len(got) != 6 {
		t.Errorf("%d scenes, want the four strong ones and two of the weak", len(got))
	}
	for _, c := range mixed[:4] {
		if !slices.ContainsFunc(got, func(o sceneCandidate) bool { return o.Start == c.Start }) {
			t.Errorf("a strong scene at %.0f was dropped", c.Start)
		}
	}

	// but not below 36 s of scenes: short ones of 5 s stay
	var brief []sceneCandidate
	for i, score := range []float64{0.8, 0.75, 0.7, 0.7, 0.3, 0.3, 0.3} {
		brief = append(brief, cand(0, 100+float64(i)*400, 5, score, distinctLines[i]))
	}
	if got = chooseScenes(brief, 1); len(got) != 7 {
		t.Errorf("%d scenes, want all seven (35 s are not to be cut)", len(got))
	}

	// a series: at most three from an episode, an episode's scenes may be close, and the
	// episodes come in order
	var series []sceneCandidate
	for ep := 2; ep >= 0; ep-- {
		for i := range 5 {
			series = append(series, cand(ep, 100+float64(i)*200, 5, 0.8-0.05*float64(i), "ep "+strings.Repeat("y", ep*10+i)))
		}
	}
	got = chooseScenes(series, 3)
	per := map[int]int{}
	for i, c := range got {
		per[c.File]++
		if i > 0 && c.File < got[i-1].File {
			t.Errorf("episodes out of order: %d then %d", got[i-1].File, c.File)
		}
	}
	for ep, n := range per {
		if n > perEpisodeMax {
			t.Errorf("%d scenes from episode %d", n, ep)
		}
	}

	// a calm opener and a punchline are put first and last when they are good scenes
	structure := []sceneCandidate{
		{File: 0, Start: 100, End: 105, Score: 0.60, words: contentWords("alpha beta gamma delta")},
		{File: 0, Start: 400, End: 405, Score: 0.62, words: contentWords("epsilon zeta theta iota")},
		{File: 0, Start: 700, End: 705, Score: 0.63, words: contentWords("kappa lambda sigma omega")},
		{File: 0, Start: 1000, End: 1005, Score: 0.64, words: contentWords("rhodium osmium iridium platinum")},
		{File: 0, Start: 1300, End: 1305, Score: 0.61, words: contentWords("carbon silicon boron nitrogen"), Opener: 1},
		{File: 0, Start: 1600, End: 1605, Score: 0.62, words: contentWords("oxygen sulphur selenium tellurium"), Button: 1},
	}
	// the opener is late in the title and the button early, so story order puts them elsewhere:
	// the choice is in the set, not the order
	got = chooseScenes(structure, 1)
	if len(got) != 6 {
		t.Errorf("%d scenes of 6", len(got))
	}
}

func TestPickStream(t *testing.T) {
	en := func(i int, lang string, ch int, def bool) audioFact {
		return audioFact{Index: i, Lang: lang, Channels: ch, Default: def}
	}
	for name, c := range map[string]struct {
		streams  []audioFact
		index    int
		original bool
		ok       bool
	}{
		"english default beside czech":  {[]audioFact{en(1, "eng", 2, true), en(2, "ces", 2, false)}, 1, true, true},
		"czech default beside english":  {[]audioFact{en(1, "cs", 2, true), en(2, "en", 6, false)}, 2, true, true},
		"two english, the default wins": {[]audioFact{en(1, "en", 6, false), en(2, "en", 2, true)}, 2, true, true},
		"two english, more channels":    {[]audioFact{en(1, "en", 2, false), en(2, "en", 6, false)}, 2, true, true},
		"one untagged track":            {[]audioFact{en(1, "und", 2, true)}, 1, false, true},
		"czech only":                    {[]audioFact{en(1, "cze", 2, true), en(2, "und", 2, false)}, 1, false, true},
		"commentary is not dialogue": {[]audioFact{{Index: 1, Lang: "en", Channels: 2, Default: true, Label: "Director's Commentary"},
			en(2, "cs", 2, false)}, 2, false, true},
		"no streams": {nil, 0, false, false},
	} {
		got, original, ok := pickStream(c.streams)
		if ok != c.ok || original != c.original || (ok && got.Index != c.index) {
			t.Errorf("%s: stream %d original %v ok %v, want %d %v %v", name, got.Index, original, ok, c.index, c.original, c.ok)
		}
	}
}

func TestStoredScene(t *testing.T) {
	stream := 2
	files := []pickFile{{ID: "picture", Audio: dialogueTrack{FileID: "audio", Stream: &stream}}}
	s := storedScene(sceneCandidate{File: 0, Start: 100.04, End: 105.64, Score: 0.7234, Source: "subtitles",
		Lines: []lineSpan{{100.5, 102.26, "Hello there."}, {103, 105, "General."}}}, files)
	if s.MediaFileID != "picture" || s.AudioFileID != "audio" || s.AudioStream == nil || *s.AudioStream != 2 {
		t.Errorf("files: %+v", s)
	}
	if s.StartMs != 100040 || s.DurationMs != 5600 || s.DurationMs%40 != 0 {
		t.Errorf("start %d length %d, want 100040 and 5600", s.StartMs, s.DurationMs)
	}
	if len(s.Lines) != 2 || s.Lines[0].FromMs != 460 || s.Lines[0].ToMs != 2220 || s.Lines[0].Text != "Hello there." || s.Score != 0.723 {
		t.Errorf("lines %+v score %v", s.Lines, s.Score)
	}
}

func TestTitleKeywords(t *testing.T) {
	got := titleKeywords("Harry Potter and the Deathly Hallows - Part 2")
	want := []string{"Harry", "Potter", "Deathly", "Hallows", "Part"}
	if !slices.Equal(got, want) {
		t.Errorf("keywords %q, want %q", got, want)
	}
}
