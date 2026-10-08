package media

import "testing"

func TestRenditionCappedAt(t *testing.T) {
	r := Renditions["1080p"]

	tests := []struct {
		name          string
		sourceBitrate int64
		want          int64
	}{
		{"unknown source bitrate keeps the ladder cap", 0, r.VideoBitrate},
		{"source above the cap keeps the ladder cap", 9_000_000, r.VideoBitrate},
		{"efficient source lowers the cap", 3_500_000, 3_500_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.CappedAt(tt.sourceBitrate).VideoBitrate; got != tt.want {
				t.Errorf("CappedAt(%d).VideoBitrate = %d, want %d", tt.sourceBitrate, got, tt.want)
			}
		})
	}

	if capped := r.CappedAt(1); capped.Height != r.Height || capped.AudioBitrate != r.AudioBitrate {
		t.Error("CappedAt must only touch the video bitrate")
	}
}

func TestClassHeight(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
		want          int
	}{
		{"16:9 is its own height", 1920, 1080, 1080},
		{"a 2.40:1 scope film is as sharp as 1080p", 1920, 800, 1080},
		{"4K scope", 3840, 1600, 2160},
		{"4:3 keeps its height", 1440, 1080, 1080},
		{"no picture", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassHeight(tt.width, tt.height); got != tt.want {
				t.Errorf("ClassHeight(%d, %d) = %d, want %d", tt.width, tt.height, got, tt.want)
			}
		})
	}
}

func TestRenditionFit(t *testing.T) {
	tests := []struct {
		name          string
		rung          string
		width, height int
		wantW, wantH  int
	}{
		{"16:9 fills the box", "720p", 1920, 1080, 1280, 720},
		{"scope keeps the box width", "720p", 1920, 800, 1280, 532},
		{"4:3 keeps the box height", "720p", 1440, 1080, 960, 720},
		{"a picture inside the box is not upscaled", "1080p", 1920, 800, 1920, 800},
		{"4K scope into 1080p", "1080p", 3840, 1600, 1920, 800},
		{"odd sides become even", "1080p", 1919, 799, 1918, 798},
		{"an unknown source keeps the box height", "720p", 0, 0, 0, 720},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, h := Renditions[tt.rung].Fit(tt.width, tt.height)
			if w != tt.wantW || h != tt.wantH {
				t.Errorf("%s.Fit(%d, %d) = %dx%d, want %dx%d", tt.rung, tt.width, tt.height, w, h, tt.wantW, tt.wantH)
			}
		})
	}
}

func TestPrepareRenditionsByClass(t *testing.T) {
	ladder := []string{"480p", "720p", "1080p"}
	names := func(rs []Rendition) []string {
		out := []string{}
		for _, r := range rs {
			out = append(out, r.Name)
		}
		return out
	}
	if got := names(PrepareRenditions(ladder, 1920, 800)); len(got) != 3 {
		t.Errorf("a 1920x800 film gets %v, want every rung up to 1080p", got)
	}
	if got := names(PrepareRenditions(ladder, 1280, 534)); len(got) != 2 || got[1] != "720p" {
		t.Errorf("a 1280x534 film gets %v, want 480p and 720p", got)
	}
	if got := names(PrepareRenditions(ladder, 640, 360)); len(got) != 1 || got[0] != "480p" {
		t.Errorf("a small source gets %v, want only the smallest rung", got)
	}
}
