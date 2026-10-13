package couchtv

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
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
	return c, st
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
