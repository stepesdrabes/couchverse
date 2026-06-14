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

// Czech display names for the standard TMDB genres. Genre identity stays the
// English name (used in URLs/filters); only the displayed label is translated.
var genreCS = map[string]string{
	"Action":             "Akční",
	"Adventure":          "Dobrodružný",
	"Animation":          "Animovaný",
	"Comedy":             "Komedie",
	"Crime":              "Krimi",
	"Documentary":        "Dokumentární",
	"Drama":              "Drama",
	"Family":             "Rodinný",
	"Fantasy":            "Fantasy",
	"History":            "Historický",
	"Horror":             "Horor",
	"Music":              "Hudební",
	"Mystery":            "Mysteriózní",
	"Romance":            "Romantický",
	"Science Fiction":    "Sci-Fi",
	"TV Movie":           "TV film",
	"Thriller":           "Thriller",
	"War":                "Válečný",
	"Western":            "Western",
	"Action & Adventure": "Akční a dobrodružný",
	"Kids":               "Dětský",
	"News":               "Zpravodajský",
	"Reality":            "Reality",
	"Sci-Fi & Fantasy":   "Sci-Fi a fantasy",
	"Soap":               "Telenovela",
	"Talk":               "Talk show",
	"War & Politics":     "Válečný a politický",
}

// genreLabel returns the genre's display label for the language, falling back to
// the English name (which stays the stable identity used in URLs and filters).
func genreLabel(name, lang string) string {
	if lang == "cs" {
		if t, ok := genreCS[name]; ok {
			return t
		}
	}
	return name
}
