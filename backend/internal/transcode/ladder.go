package transcode

import (
	"context"
	"encoding/json"

	"couchverse/internal/store"
)

type Rendition struct {
	Name         string
	Height       int
	VideoBitrate int64 // bits/s cap
	AudioBitrate int64
}

var Renditions = map[string]Rendition{
	"1080p": {Name: "1080p", Height: 1080, VideoBitrate: 6_000_000, AudioBitrate: 192_000},
	"720p":  {Name: "720p", Height: 720, VideoBitrate: 3_000_000, AudioBitrate: 128_000},
	"480p":  {Name: "480p", Height: 480, VideoBitrate: 1_200_000, AudioBitrate: 96_000},
}

type Settings struct {
	HWAccel       string   `json:"hwAccel"`       // auto | none | encoder name
	Ladder        []string `json:"ladder"`        // rendition names for full transcodes
	Preset        string   `json:"preset"`        // libx264 preset
	MaxConcurrent int      `json:"maxConcurrent"` // concurrent transcode jobs
	JITEnabled    *bool    `json:"jitEnabled"`    // nil = auto (on when hw encoder exists)
}

// LoadSettings reads transcode preferences with Pi-friendly defaults.
func LoadSettings(ctx context.Context, st *store.Store) Settings {
	s := Settings{
		HWAccel:       "auto",
		Ladder:        []string{"720p"},
		Preset:        "veryfast",
		MaxConcurrent: 1,
	}
	raw, err := st.Setting(ctx, "transcode")
	if err == nil && raw != nil {
		_ = json.Unmarshal(raw, &s)
	}
	if len(s.Ladder) == 0 {
		s.Ladder = []string{"720p"}
	}
	if s.MaxConcurrent < 1 {
		s.MaxConcurrent = 1
	}
	if s.Preset == "" {
		s.Preset = "veryfast"
	}
	return s
}
