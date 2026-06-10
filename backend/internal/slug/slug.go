// Package slug builds URL-safe identifiers from content names.
package slug

import (
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const maxLen = 80

var deaccent = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// Make builds a slug like "the-dark-knight-2008" from a name and optional year.
func Make(name string, year *int) string {
	s, _, err := transform.String(deaccent, name)
	if err != nil {
		s = name
	}
	var b strings.Builder
	lastDash := true // suppress leading dash
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > maxLen {
		out = strings.Trim(out[:maxLen], "-")
	}
	if out == "" {
		out = "title"
	}
	if year != nil && *year > 0 {
		out += "-" + strconv.Itoa(*year)
	}
	return out
}
