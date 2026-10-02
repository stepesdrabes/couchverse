package system

import (
	"context"
	"encoding/json"
	"os"

	"github.com/google/uuid"

	"couchverse/internal/settings"
	"couchverse/internal/version"
)

// ServerInfo identifies a Couchverse server to clients: the add-server flow
// checks it (an HTML fallback page can never pass as a server), and clients
// compare APILevel with the minimum they need.
type ServerInfo struct {
	ID       string `json:"id" format:"uuid" doc:"Stable for the server's lifetime; clients key their accounts by it."`
	Name     string `json:"name"`
	Version  string `json:"version" doc:"The release the server was built from."`
	APILevel int    `json:"apiLevel" doc:"Raised whenever clients need new server behaviour."`
	Accent   string `json:"accent" doc:"The site accent colour as a CSS hex value."`
}

type serverInfoOutput struct{ Body ServerInfo }

func (h *Theme) Server(ctx context.Context, _ *struct{}) (*serverInfoOutput, error) {
	id, err := readString(ctx, h.settings, keyServerID)
	if err != nil {
		return nil, err
	}
	name, err := readString(ctx, h.settings, keyServerName)
	if err != nil {
		return nil, err
	}
	if name == "" {
		name, _ = os.Hostname()
	}
	return &serverInfoOutput{Body: ServerInfo{
		ID:       id,
		Name:     name,
		Version:  version.Version,
		APILevel: version.APILevel,
		Accent:   h.accent(ctx),
	}}, nil
}

// EnsureServerID gives the server its identity on first boot.
func EnsureServerID(ctx context.Context, set *settings.Store) error {
	id, err := readString(ctx, set, keyServerID)
	if err != nil || id != "" {
		return err
	}
	raw, err := json.Marshal(uuid.NewString())
	if err != nil {
		return err
	}
	return set.Set(ctx, keyServerID, raw)
}

// readString reads a string setting, "" when unset.
func readString(ctx context.Context, set *settings.Store, key string) (string, error) {
	raw, err := set.Get(ctx, key)
	if err != nil || raw == nil {
		return "", err
	}
	// an unreadable value reads as unset, like every other setting
	var s string
	_ = json.Unmarshal(raw, &s)
	return s, nil
}
