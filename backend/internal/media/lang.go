package media

import (
	"strings"

	"golang.org/x/text/language"
)

// BCP47 turns a stream's ISO 639-2 tag (eng, cze) into the BCP 47 form HLS and
// the clients use (en, cs); unknown tags become und.
func BCP47(lang string) string {
	if lang == "" {
		return "und"
	}
	tag, err := language.Parse(lang)
	if err != nil {
		return "und"
	}
	base, conf := tag.Base()
	if conf != language.Exact {
		return "und"
	}
	return strings.ToLower(base.String())
}

// ISO639 is the three-letter code an MP4 language field takes; und when unknown.
func ISO639(lang string) string {
	if b := BCP47(lang); b != "und" {
		base, _ := language.MustParse(b).Base()
		return base.ISO3()
	}
	return "und"
}

// SameLanguage reports whether two tags name the same known language, whichever
// form they are written in.
func SameLanguage(a, b string) bool {
	return BCP47(a) != "und" && BCP47(a) == BCP47(b)
}
