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
