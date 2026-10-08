package media

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"couchverse/internal/settings"
)

type Rendition struct {
	Name string
	// Width x Height is the 16:9 box a rung's picture is fitted into
	Width        int
	Height       int
	VideoBitrate int64 // bits/s cap
	AudioBitrate int64
}

var Renditions = map[string]Rendition{
	"1080p": {Name: "1080p", Width: 1920, Height: 1080, VideoBitrate: 6_000_000, AudioBitrate: 192_000},
	"720p":  {Name: "720p", Width: 1280, Height: 720, VideoBitrate: 3_000_000, AudioBitrate: 128_000},
	"480p":  {Name: "480p", Width: 854, Height: 480, VideoBitrate: 1_200_000, AudioBitrate: 96_000},
}

// ClassHeight is the height of the 16:9 picture as sharp as a width x height
// one: a 1920x800 scope film is 1080p though it is only 800 lines tall.
func ClassHeight(width, height int) int {
	return max(height, width*9/16)
}

// Fit is the size of a width x height picture scaled down into the rung's box,
// never up, with even sides for 4:2:0. An unknown source keeps the box height
// and leaves the width (0) to its aspect.
func (r Rendition) Fit(width, height int) (int, int) {
	if width <= 0 || height <= 0 {
		return 0, r.Height
	}
	switch {
	case width <= r.Width && height <= r.Height:
	case width*r.Height > height*r.Width: // wider than the box
		width, height = r.Width, height*r.Width/width
	default:
		width, height = width*r.Height/height, r.Height
	}
	return width &^ 1, height &^ 1
}

// CappedAt bounds the target video bitrate at the source's overall bitrate,
// so a transcode never outweighs the file it came from (an efficient HEVC
// source would otherwise ride the much higher h264 ladder cap).
func (r Rendition) CappedAt(sourceBitrate int64) Rendition {
	if sourceBitrate > 0 && sourceBitrate < r.VideoBitrate {
		r.VideoBitrate = sourceBitrate
	}
	return r
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

// PrepareRenditions picks the ladder entries that make sense for a source -
// none above its class, so never upscaling, and always at least the smallest.
func PrepareRenditions(ladder []string, sourceWidth, sourceHeight int) []Rendition {
	class := ClassHeight(sourceWidth, sourceHeight)
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
		if class <= 0 || r.Height <= class {
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
