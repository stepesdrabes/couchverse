package system

import (
	"context"
	"encoding/json"

	"couchverse/internal/flags"
	"couchverse/internal/httpx"
	"couchverse/internal/media"
	"couchverse/internal/settings"
)

// The settings keys this editor owns. Other features keep their own keys
// (ranks writes "ranks" through its validated config endpoint).
const (
	keyTMDB       = "tmdb.api_key"
	keyTranscode  = "transcode"
	keyFeatures   = "features"
	keyHome       = "home"
	keyAppearance = "appearance"
	keyServerName = "server.name"
	// keyServerID is the server's identity, set once at first boot; it is not
	// editable, or clients would mistake the server for a new one
	keyServerID = "server.id"
)

// ServerSettings are the admin-editable server settings. Every key is stored
// on its own: a key is absent until first saved, and an update replaces just
// the keys it sends, each as a whole.
type ServerSettings struct {
	ServerName *string                  `json:"server.name,omitempty" minLength:"1" maxLength:"60" doc:"The name clients show for this server; the host name when unset."`
	TMDBAPIKey *string                  `json:"tmdb.api_key,omitempty" doc:"TMDB v3 API key used for metadata search and fetches."`
	Transcode  *media.TranscodeSettings `json:"transcode,omitempty"`
	Features   *FeatureFlags            `json:"features,omitempty"`
	Home       *HomeSettings            `json:"home,omitempty"`
	Appearance *AppearanceSettings      `json:"appearance,omitempty"`
}

// FeatureFlags are the admin toggles for optional features (flags.Flags with
// documentation; a flag missing from the stored value counts as enabled).
type FeatureFlags struct {
	CouchEnabled     bool `json:"couchEnabled" doc:"Couch sessions (synced watch parties)."`
	RankingsEnabled  bool `json:"rankingsEnabled" doc:"Ranks, achievements, public profiles and leaderboards."`
	DownloadsEnabled bool `json:"downloadsEnabled" doc:"Downloads for offline viewing in the native apps (prepared MP4s use server disk and CPU)."`
}

type HomeSettings struct {
	FeaturedCount int `json:"featuredCount" doc:"Titles the home hero cycles through, read as 1-10 (3 when unset)."`
}

type AppearanceSettings struct {
	Accent string `json:"accent" doc:"Accent colour as a CSS hex value such as #e50914."`
}

type AdminSettings struct {
	settings *settings.Store
}

func NewAdminSettings(set *settings.Store) *AdminSettings {
	return &AdminSettings{settings: set}
}

type settingsOutput struct{ Body ServerSettings }

func (h *AdminSettings) Get(ctx context.Context, _ *struct{}) (*settingsOutput, error) {
	stored, err := h.settings.All(ctx)
	if err != nil {
		return nil, err
	}
	out := ServerSettings{
		ServerName: decodeSetting[string](stored, keyServerName),
		TMDBAPIKey: decodeSetting[string](stored, keyTMDB),
		Transcode:  decodeSetting[media.TranscodeSettings](stored, keyTranscode),
		Home:       decodeSetting[HomeSettings](stored, keyHome),
		Appearance: decodeSetting[AppearanceSettings](stored, keyAppearance),
	}
	if out.Transcode != nil && out.Transcode.Ladder == nil {
		out.Transcode.Ladder = []string{}
	}
	if _, ok := stored[keyFeatures]; ok {
		// read the way the guards read it: values saved before a flag existed
		// lack it, and a missing flag means enabled
		f := FeatureFlags(flags.Load(ctx, h.settings))
		out.Features = &f
	}
	return &settingsOutput{Body: out}, nil
}

// decodeSetting reads one stored key, nil when it is unset, null or
// unreadable (the server's readers fall back to their defaults for all three).
func decodeSetting[T any](stored map[string]json.RawMessage, key string) *T {
	raw, ok := stored[key]
	if !ok || string(raw) == "null" {
		return nil
	}
	var v T
	if json.Unmarshal(raw, &v) != nil {
		return nil
	}
	return &v
}

type updateSettingsInput struct {
	Body ServerSettings
}

// Put stores the posted keys, leaving the others untouched.
func (h *AdminSettings) Put(ctx context.Context, in *updateSettingsInput) (*settingsOutput, error) {
	// the transcoder silently ignores values it doesn't know - reject them here
	if ts := in.Body.Transcode; ts != nil {
		if err := ts.Validate(); err != nil {
			return nil, httpx.BadRequestError(err.Error())
		}
	}
	// omitempty drops the keys that were not sent
	body, err := json.Marshal(in.Body)
	if err != nil {
		return nil, err
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(body, &values); err != nil {
		return nil, err
	}
	for key, value := range values {
		if err := h.settings.Set(ctx, key, value); err != nil {
			return nil, err
		}
	}
	return h.Get(ctx, nil)
}
