package media

import "testing"

func intPtr(v int) *int { return &v }

func TestParseVideoPath(t *testing.T) {
	tests := []struct {
		path string
		want ParsedVideo
	}{
		{
			"Driftlight/Season 01/Driftlight S01E02 The Long Thaw.mkv",
			ParsedVideo{IsEpisode: true, ShowName: "Driftlight", Season: 1, Episode: 2, Name: "The Long Thaw"},
		},
		{
			"Driftlight (2025)/Season 1/driftlight.s01e10.1080p.web-dl.x264.mkv",
			ParsedVideo{IsEpisode: true, ShowName: "Driftlight", Season: 1, Episode: 10, Year: intPtr(2025)},
		},
		{
			"Show.Name.S02E03.mkv",
			ParsedVideo{IsEpisode: true, ShowName: "Show Name", Season: 2, Episode: 3},
		},
		{
			"Glass Harbor (2025)/Glass Harbor (2025).mkv",
			ParsedVideo{Name: "Glass Harbor", Year: intPtr(2025)},
		},
		{
			"Static Bloom (2024)/static.bloom.2160p.hdr.mkv",
			ParsedVideo{Name: "Static Bloom", Year: intPtr(2024)},
		},
		{
			"Some Random Movie.mp4",
			ParsedVideo{Name: "Some Random Movie"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := ParseVideoPath(tt.path)
			if got.IsEpisode != tt.want.IsEpisode {
				t.Fatalf("IsEpisode = %v, want %v", got.IsEpisode, tt.want.IsEpisode)
			}
			if tt.want.IsEpisode {
				if got.ShowName != tt.want.ShowName || got.Season != tt.want.Season || got.Episode != tt.want.Episode {
					t.Fatalf("got %+v, want %+v", got, tt.want)
				}
				if tt.want.Name != "" && got.Name != tt.want.Name {
					t.Fatalf("episode name = %q, want %q", got.Name, tt.want.Name)
				}
			} else if got.Name != tt.want.Name {
				t.Fatalf("name = %q, want %q", got.Name, tt.want.Name)
			}
			switch {
			case tt.want.Year == nil && got.Year != nil:
				t.Fatalf("year = %d, want nil", *got.Year)
			case tt.want.Year != nil && (got.Year == nil || *got.Year != *tt.want.Year):
				t.Fatalf("year = %v, want %d", got.Year, *tt.want.Year)
			}
		})
	}
}

func TestDirectPlay(t *testing.T) {
	tests := []struct {
		name string
		p    ProbeResult
		want bool
	}{
		{"h264 aac mp4", ProbeResult{HasVideo: true, Container: "mp4", VideoCodec: "h264", AudioCodec: "aac"}, true},
		{"h264 in mkv", ProbeResult{HasVideo: true, Container: "mkv", VideoCodec: "h264", AudioCodec: "aac"}, false},
		{"h264 ac3 mp4", ProbeResult{HasVideo: true, Container: "mp4", VideoCodec: "h264", AudioCodec: "ac3"}, false},
		{"hevc mp4", ProbeResult{HasVideo: true, Container: "mp4", VideoCodec: "hevc", AudioCodec: "aac"}, false},
		{"mp3", ProbeResult{Container: "mp3", AudioCodec: "mp3"}, true},
		{"flac", ProbeResult{Container: "flac", AudioCodec: "flac"}, true},
		{"alac in m4a", ProbeResult{Container: "m4a", AudioCodec: "alac"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DirectPlay(&tt.p); got != tt.want {
				t.Fatalf("DirectPlay = %v, want %v", got, tt.want)
			}
		})
	}
}
