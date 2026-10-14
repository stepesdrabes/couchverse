package couchtv

import (
	"slices"
	"testing"
)

func adminScene(file string, start int, lines ...string) CouchTvTrailerScene {
	s := CouchTvTrailerScene{MediaFileID: file, AudioFileID: file, StartMs: start, DurationMs: 5000, Score: 0.7, Source: "subtitles",
		Lines: []CouchTvTrailerLine{}}
	for i, text := range lines {
		s.Lines = append(s.Lines, CouchTvTrailerLine{FromMs: 500 + 1500*i, ToMs: 1800 + 1500*i, Text: text})
	}
	return s
}

func starts(scenes []CouchTvTrailerScene) []int {
	out := []int{}
	for _, s := range scenes {
		out = append(out, s.StartMs)
	}
	return out
}

// An admin's pick keeps the scenes it names, in the order the trailer plays them, whatever
// order it names them in; a scene is its file and its start, so two files may share a start.
func TestKeepScenes(t *testing.T) {
	const a, b = "00000000-0000-4000-8000-000000000a01", "00000000-0000-4000-8000-000000000a02"
	current := []CouchTvTrailerScene{adminScene(a, 60000), adminScene(a, 300000), adminScene(b, 60000), adminScene(b, 900000)}
	ref := func(file string, start int) CouchTvTrailerSceneRef {
		return CouchTvTrailerSceneRef{MediaFileID: file, StartMs: start}
	}

	for _, c := range []struct {
		name string
		keep []CouchTvTrailerSceneRef
		want []int
		bad  bool
	}{
		{name: "some", keep: []CouchTvTrailerSceneRef{ref(a, 300000), ref(b, 900000)}, want: []int{300000, 900000}},
		{name: "in the trailer's order", keep: []CouchTvTrailerSceneRef{ref(b, 900000), ref(a, 60000)}, want: []int{60000, 900000}},
		{name: "the same start in another file", keep: []CouchTvTrailerSceneRef{ref(b, 60000)}, want: []int{60000}},
		{name: "none", keep: nil, want: []int{}},
		{name: "twice is once", keep: []CouchTvTrailerSceneRef{ref(a, 60000), ref(a, 60000)}, want: []int{60000}},
		{name: "all", keep: []CouchTvTrailerSceneRef{ref(a, 60000), ref(a, 300000), ref(b, 60000), ref(b, 900000)}, want: []int{60000, 300000, 60000, 900000}},
		{name: "a start that is no scene", keep: []CouchTvTrailerSceneRef{ref(a, 60040)}, bad: true},
		{name: "a file that is no scene's", keep: []CouchTvTrailerSceneRef{ref(a, 60000), ref("00000000-0000-4000-8000-000000000a03", 60000)}, bad: true},
	} {
		got, err := keepScenes(current, c.keep)
		switch {
		case c.bad && err == nil:
			t.Errorf("%s: kept %v, want an error", c.name, starts(got))
		case !c.bad && err != nil:
			t.Errorf("%s: %v", c.name, err)
		case !c.bad && !slices.Equal(starts(got), c.want):
			t.Errorf("%s: kept %v, want %v", c.name, starts(got), c.want)
		case !c.bad && got == nil:
			t.Errorf("%s: kept nil, which hands the trailer back instead of picking none", c.name)
		}
	}

	// the kept scenes are the stored ones whole, sound and all
	got, _ := keepScenes(current, []CouchTvTrailerSceneRef{ref(b, 900000)})
	if len(got) != 1 || got[0].AudioFileID != b || got[0].Score != 0.7 {
		t.Errorf("a kept scene lost what it was stored with: %+v", got)
	}
}

// The admin sees a scene by its picture, when it plays and what is said in it; a scene found
// by listening has no text to show.
func TestTrailerCuts(t *testing.T) {
	const file = "00000000-0000-4000-8000-000000000a01"
	heard := adminScene(file, 900000)
	heard.Source = "speech"
	heard.Lines = []CouchTvTrailerLine{{FromMs: 400, ToMs: 2000}}
	stream := 1
	read := adminScene(file, 60000, "Who are you?", "Nobody leaves this world alive.")
	read.AudioStream = &stream

	cuts := trailerCuts([]CouchTvTrailerScene{read, heard})
	if len(cuts) != 2 {
		t.Fatalf("%d cuts", len(cuts))
	}
	if c := cuts[0]; c.MediaFileID != file || c.StartMs != 60000 || c.DurationMs != 5000 || c.Source != "subtitles" ||
		!slices.Equal(c.Lines, []string{"Who are you?", "Nobody leaves this world alive."}) {
		t.Errorf("a scene read from the subtitles: %+v", c)
	}
	if c := cuts[1]; c.Source != "speech" || c.Lines == nil || len(c.Lines) != 0 {
		t.Errorf("a scene found by listening: %+v (its lines must be an empty list, not null)", c)
	}
	if got := trailerCuts(nil); got == nil || len(got) != 0 {
		t.Errorf("no scenes: %v (must be an empty list, not null)", got)
	}
}

func TestSameScenes(t *testing.T) {
	const file = "00000000-0000-4000-8000-000000000a01"
	a := []CouchTvTrailerScene{adminScene(file, 60000), adminScene(file, 300000)}
	for _, c := range []struct {
		name string
		b    []CouchTvTrailerScene
		want bool
	}{
		{"the same", []CouchTvTrailerScene{adminScene(file, 60000), adminScene(file, 300000)}, true},
		{"fewer", a[:1], false},
		{"another order", []CouchTvTrailerScene{a[1], a[0]}, false},
		{"another start", []CouchTvTrailerScene{a[0], adminScene(file, 300040)}, false},
		{"none", nil, false},
	} {
		if got := sameScenes(a, c.b); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	if !sameScenes(nil, []CouchTvTrailerScene{}) {
		t.Error("no scenes and an empty list are the same trailer")
	}
}
