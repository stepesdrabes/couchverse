package catalog

import (
	"context"
	"encoding/json"

	"couchverse/internal/httpx"
)

// localize overwrites name/overview with the translation for the request's
// display language (httpx.LangFrom). It is a no-op when no language is set
// (admin reads, background jobs) or the translation is absent - the base
// columns are the fallback. Pass nil for a field that has no translation.
func localize(ctx context.Context, translations []byte, name, overview *string) {
	lang := httpx.LangFrom(ctx)
	if lang == "" || len(translations) == 0 {
		return
	}
	var m map[string]struct {
		Name     string `json:"name"`
		Overview string `json:"overview"`
	}
	if json.Unmarshal(translations, &m) != nil {
		return
	}
	tr, ok := m[lang]
	if !ok {
		return
	}
	if name != nil && tr.Name != "" {
		*name = tr.Name
	}
	if overview != nil && tr.Overview != "" {
		*overview = tr.Overview
	}
}
