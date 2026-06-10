package media

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"couchverse/internal/settings"
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

type TranscodeSettings struct {
	HWAccel       string   `json:"hwAccel"`       // auto | none | encoder name
	Ladder        []string `json:"ladder"`        // rendition names for full transcodes
	Preset        string   `json:"preset"`        // libx264 preset
	MaxConcurrent int      `json:"maxConcurrent"` // concurrent transcode jobs
	JITEnabled    *bool    `json:"jitEnabled"`    // nil = auto (on when hw encoder exists)
	AutoPrepare   *bool    `json:"autoPrepare"`   // nil = on: queue transcodes for unplayable files at probe time
	// delete the original file once every requested variant is ready (default off)
	DeleteSourceAfterTranscode *bool `json:"deleteSourceAfterTranscode"`
}

func (s TranscodeSettings) DeleteSourceEnabled() bool {
	return s.DeleteSourceAfterTranscode != nil && *s.DeleteSourceAfterTranscode
}

func (s TranscodeSettings) AutoPrepareEnabled() bool {
	return s.AutoPrepare == nil || *s.AutoPrepare
}

// HWEncoderCandidates lists candidate h264 encoders in preference order per
// platform; detection (in the playback feature) probes them with a test encode.
var HWEncoderCandidates = []string{
	"h264_videotoolbox", // macOS
	"h264_nvenc",        // NVIDIA
	"h264_qsv",          // Intel QuickSync
	"h264_vaapi",        // generic VA-API (Intel/AMD)
	"h264_v4l2m2m",      // Raspberry Pi 4
}

var validPresets = map[string]bool{
	"": true, "ultrafast": true, "superfast": true, "veryfast": true,
	"faster": true, "fast": true, "medium": true, "slow": true,
}

// Validate rejects settings the transcoder would silently ignore.
func (s TranscodeSettings) Validate() error {
	for _, name := range s.Ladder {
		if _, ok := Renditions[name]; !ok {
			return fmt.Errorf("unknown rendition %q", name)
		}
	}
	if !validPresets[s.Preset] {
		return fmt.Errorf("unknown preset %q", s.Preset)
	}
	switch s.HWAccel {
	case "", "auto", "none":
	default:
		if !slices.Contains(HWEncoderCandidates, s.HWAccel) {
			return fmt.Errorf("unknown encoder %q", s.HWAccel)
		}
	}
	if s.MaxConcurrent < 0 || s.MaxConcurrent > 8 {
		return fmt.Errorf("maxConcurrent must be between 1 and 8")
	}
	return nil
}

// PrepareRenditions picks the ladder entries that make sense for a source
// height - never upscaling, and always returning at least the smallest one.
func PrepareRenditions(ladder []string, sourceHeight int) []Rendition {
	picked := []Rendition{}
	var smallest *Rendition
	for _, name := range ladder {
		r, ok := Renditions[name]
		if !ok {
			continue
		}
		if smallest == nil || r.Height < smallest.Height {
			rc := r
			smallest = &rc
		}
		if sourceHeight <= 0 || r.Height <= sourceHeight {
			picked = append(picked, r)
		}
	}
	if len(picked) == 0 && smallest != nil {
		picked = append(picked, *smallest)
	}
	return picked
}

// LoadTranscodeSettings reads transcode preferences with Pi-friendly defaults.
func LoadTranscodeSettings(ctx context.Context, st *settings.Store) TranscodeSettings {
	s := TranscodeSettings{
		HWAccel:       "auto",
		Ladder:        []string{"720p"},
		Preset:        "veryfast",
		MaxConcurrent: 1,
	}
	raw, err := st.Get(ctx, "transcode")
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
