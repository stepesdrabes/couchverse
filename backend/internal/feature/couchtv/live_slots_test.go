package couchtv

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"couchverse/internal/feature/playback"
	"couchverse/internal/hls"
)

// slotChannel airs, from 18:00:
//
//	harbor   0-20     its copied source
//	break    20-33    assembled around a news slot: 6 s, the slot's 4 s fallback, 3 s
//	harbor   33-53    again
func slotChannel() (*Channel, *fakeStore) {
	c, st := testChannel()
	files := c.media.(fakeMedia)
	section := func(seconds float64) *breakMedia {
		return &breakMedia{Video: ptr(rendition("source", rung, seconds)), Audio: ptr(rendition("audio-1-aac", stereo("en", true), seconds+0.3))}
	}
	files.breaks["breaks/b9/0"] = section(6)
	files.breaks["breaks/b9/1"] = section(4)
	files.breaks["breaks/b9/2"] = section(3)
	st.rows = []LiveAiring{
		programRow(1, 101, at0, 20*time.Second, 0),
		breakRow(2, at0.Add(20*time.Second), at0.Add(33*time.Second)),
		programRow(3, 101, at0.Add(33*time.Second), 20*time.Second, 0),
	}
	st.breaks = map[string]string{uuidOf(2): "breaks/b9"}
	st.recipes = map[string]Recipe{uuidOf(2): {Frames: 325, Slots: []RecipeSlot{{Kind: BulletinNews, At: 150, Frames: 100, Loudness: -19}}}}
	// two rendered news bulletins of the slot's length, each with every slot loudness
	files.clips = map[string]*playback.Package{}
	for _, key := range []string{"n1", "n2"} {
		pkg := &playback.Package{Source: ptr(rendition("source", rung, 4))}
		for i, l := range slotLoudnesses() {
			pkg.Stereo = append(pkg.Stereo, rendition(slotAudio(l), stereo("en", i == 0), 4.02))
		}
		files.clips["couchtv/clips/"+key] = pkg
	}
	c.media = files
	return c, st
}

// newsClip is a rendered news bulletin (n1 or n2) of length frames, valid until until.
func newsClip(key string, frames int, until time.Time) fakeBulletin {
	return fakeBulletin{kind: BulletinNews, until: until,
		clip: &Clip{ID: "clip-" + key, Kind: ClipNews, State: ClipReady, Frames: frames, Path: "couchtv/clips/" + key}}
}

// The slot airs the newest bulletin still valid when the slot ends, in its place and at its
// loudness; else its fallback.
func TestLiveBulletinFillsTheSlot(t *testing.T) {
	slotEnd := at0.Add(30 * time.Second)
	for _, c := range []struct {
		name      string
		bulletins []fakeBulletin
		recipe    func(*Recipe)
		want      string // the slot's part: its kind and path
	}{
		{"no bulletin: the fallback", nil, nil, "break breaks/b9/1"},
		{"a valid bulletin", []fakeBulletin{newsClip("n1", 100, slotEnd.Add(time.Hour))}, nil, "bulletin couchtv/clips/n1"},
		{"the newest valid one", []fakeBulletin{newsClip("n1", 100, slotEnd.Add(time.Hour)), newsClip("n2", 100, slotEnd.Add(2*time.Hour))},
			nil, "bulletin couchtv/clips/n2"},
		{"never past its validity", []fakeBulletin{newsClip("n1", 100, slotEnd.Add(time.Hour)), newsClip("n2", 100, slotEnd.Add(-time.Second))},
			nil, "bulletin couchtv/clips/n1"},
		{"an expired one: the fallback", []fakeBulletin{newsClip("n1", 100, slotEnd.Add(-time.Second))}, nil, "break breaks/b9/1"},
		{"a weather slot takes no news", []fakeBulletin{newsClip("n1", 100, slotEnd.Add(time.Hour))},
			func(r *Recipe) { r.Slots[0].Kind = BulletinWeather }, "break breaks/b9/1"},
		{"a loudness without a rendition: the fallback", []fakeBulletin{newsClip("n1", 100, slotEnd.Add(time.Hour))},
			func(r *Recipe) { r.Slots[0].Loudness = -12 }, "break breaks/b9/1"},
	} {
		ch, st := slotChannel()
		st.bulletins = c.bulletins
		if c.recipe != nil {
			r := st.recipes[uuidOf(2)]
			r.Slots = slices.Clone(r.Slots)
			c.recipe(&r)
			st.recipes[uuidOf(2)] = r
		}
		parts, err := ch.Lineup(context.Background(), at0, at0.Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		if got := parts[2].Kind + " " + parts[2].Path; got != c.want || parts[2].End.Sub(parts[2].Start) != 4*time.Second {
			t.Errorf("%s: the slot airs %s for %s, want %s", c.name, got, parts[2].End.Sub(parts[2].Start), c.want)
		}
	}

	// on air: the bulletin's own renditions at the slot's loudness, under the slot's part
	ch, st := slotChannel()
	st.bulletins = []fakeBulletin{newsClip("n1", 100, slotEnd.Add(time.Hour))}
	b := "../airings/" + uuidOf(2) + "/parts/1/"
	video := playlist(t, ch, videoKey, at0.Add(31*time.Second))
	audio := playlist(t, ch, audioKey("en"), at0.Add(31*time.Second))
	if !slices.ContainsFunc(video.Segments, func(s hls.Segment) bool { return s.URI == b+"video/seg_00000.m4s" }) ||
		!slices.ContainsFunc(audio.Segments, func(s hls.Segment) bool { return s.URI == b+"audio-en/seg_00000.m4s" }) {
		t.Errorf("the slot's playlists:\n%s\n%s", video, audio)
	}
	pinned, _ := st.PinnedAiring(context.Background(), uuidOf(2))
	part, _ := pinned.Feed.mediaPart(1)
	if part.Kind != PartBulletin || part.File != "clip-n1" || part.Video != "source" || part.Audio["cs"] != "audio-l19-aac" {
		t.Errorf("the pinned slot %+v", part)
	}
	// once on air the slot never changes, whatever arrives later
	st.bulletins = append(st.bulletins, newsClip("n2", 100, slotEnd.Add(2*time.Hour)))
	if again := playlist(t, ch, videoKey, at0.Add(31*time.Second)); again.String() != video.String() {
		t.Errorf("a newer bulletin changed what aired:\n%s\n%s", again, video)
	}

	// a break cut into its slot airs the fallback's start
	ch, st = slotChannel()
	st.bulletins = []fakeBulletin{newsClip("n1", 100, slotEnd.Add(time.Hour))}
	st.rows[1].EndsAt = at0.Add(28 * time.Second)
	st.rows[2].StartsAt = st.rows[1].EndsAt
	parts, err := ch.Lineup(context.Background(), at0, at0.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if parts[2].Kind != PartBreak || parts[2].End.Sub(parts[2].Start) != 2*time.Second {
		t.Errorf("a cut slot airs %+v", parts[2])
	}
}

// A break with a bulletin slot airs in sections, each an item of its own (a discontinuity and
// a program date at every edge), its parts after the first reached under parts/<n>/.
func TestLiveBreakSections(t *testing.T) {
	c, st := slotChannel()
	parts, err := c.Lineup(context.Background(), at0, at0.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	type span struct {
		kind, path string
		start, end float64
	}
	var got []span
	for _, p := range parts {
		got = append(got, span{p.Kind, p.Path, p.Start.Sub(at0).Seconds(), p.End.Sub(at0).Seconds()})
	}
	want := []span{{PartProgram, "", 0, 20}, {PartBreak, "breaks/b9/0", 20, 26}, {PartBreak, "breaks/b9/1", 26, 30},
		{PartBreak, "breaks/b9/2", 30, 33}, {PartProgram, "", 33, 53}}
	if !slices.Equal(got, want) {
		t.Fatalf("lineup\n got %v\nwant %v", got, want)
	}

	b := "../airings/" + uuidOf(2) + "/"
	video := playlist(t, c, videoKey, at0.Add(31*time.Second))
	var uris []string
	for _, s := range video.Segments {
		if strings.Contains(s.URI, uuidOf(2)) {
			uris = append(uris, s.URI)
			if !s.Discontinuity || s.ProgramDateTime.IsZero() {
				t.Errorf("%s starts a section without a discontinuity and a date", s.URI)
			}
		}
	}
	if wantURIs := []string{b + "video/seg_00000.m4s", b + "parts/1/video/seg_00000.m4s", b + "parts/2/video/seg_00000.m4s"}; !slices.Equal(uris, wantURIs) {
		t.Errorf("the break's video\n got %v\nwant %v", uris, wantURIs)
	}
	// each section's audio is cut where its video is
	audio := playlist(t, c, audioKey("cs"), at0.Add(31*time.Second))
	for _, s := range audio.Segments {
		if strings.Contains(s.URI, uuidOf(2)) && !strings.HasSuffix(s.URI, "?keep=6.000") && !strings.HasSuffix(s.URI, "?keep=4.000") &&
			!strings.HasSuffix(s.URI, "?keep=3.000") {
			t.Errorf("an overrunning section's audio is not trimmed: %s", s.URI)
		}
	}
	pinned, _ := st.PinnedAiring(context.Background(), uuidOf(2))
	if part, ok := pinned.Feed.mediaPart(1); !ok || part.Path != "breaks/b9/1" {
		t.Errorf("the second media part is %+v", part)
	}
	if _, ok := pinned.Feed.mediaPart(3); ok {
		t.Error("a fourth media part")
	}

	// numbering holds across the sections and a restart
	seen := numbering{}
	var texts []string
	for at := at0; at.Before(at0.Add(60 * time.Second)); at = at.Add(700 * time.Millisecond) {
		for _, key := range liveKeys() {
			m := playlist(t, c, key, at)
			seen.check(t, key, m)
			if key == videoKey {
				texts = append(texts, m.String())
			}
		}
	}
	restarted := &Channel{store: st, media: c.media}
	i := 0
	for at := at0; at.Before(at0.Add(60 * time.Second)); at = at.Add(700 * time.Millisecond) {
		if got := playlist(t, restarted, videoKey, at).String(); got != texts[i] {
			t.Fatalf("after a restart at %s:\n%s\nbefore:\n%s", at.Format("15:04:05.000"), got, texts[i])
		}
		i++
	}
}

// A break with slots fits its span as any break does: after a filler when its sections are
// shorter, cut where the span ends when they are longer.
func TestLiveBreakSectionsFit(t *testing.T) {
	for _, c := range []struct {
		name  string
		end   time.Duration // the break's end after 18:00:20
		parts []string
	}{
		{"a longer span", 15 * time.Second, []string{"slate 2", "break 6", "break 4", "break 3"}},
		{"a span shorter by under half a second", 12800 * time.Millisecond, []string{"break 6", "break 4", "break 2.8"}},
		{"cut in the slot", 8 * time.Second, []string{"break 6", "break 2"}},
	} {
		ch, st := slotChannel()
		st.rows[1].EndsAt = at0.Add(20*time.Second + c.end)
		st.rows[2].StartsAt = st.rows[1].EndsAt
		parts, err := ch.Lineup(context.Background(), at0, at0.Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, p := range parts {
			if p.AiringID == uuidOf(2) {
				got = append(got, fmt.Sprintf("%s %g", p.Kind, p.End.Sub(p.Start).Seconds()))
			}
		}
		if !slices.Equal(got, c.parts) {
			t.Errorf("%s: %v, want %v", c.name, got, c.parts)
		}
	}
}

// A section that cannot be read airs the slate for its time; the break is not ready without it.
func TestLiveBreakSectionMissing(t *testing.T) {
	c, _ := slotChannel()
	delete(c.media.(fakeMedia).breaks, "breaks/b9/1")
	parts, err := c.Lineup(context.Background(), at0, at0.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	// the evergreen break (20 s) cut to the 13 s gap
	if parts[1].Kind != PartBreak || parts[1].Path != "breaks/e1" || parts[1].End.Sub(parts[1].Start) != 13*time.Second {
		t.Errorf("a break missing a section airs %+v", parts[1])
	}
}
