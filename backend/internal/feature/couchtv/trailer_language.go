package couchtv

import "regexp"

// The language of a trailer's lines (D43). A trailer plays its scenes' lines as the film says
// them, on a channel anyone may have on in the room, so a scene whose lines hold strong
// profanity or a slur is never taken, whatever else it has going for it; milder words only count
// against it. The words are matched whole (so "Scunthorpe" and "assistant" are safe), whatever
// the case, with their inflections, with the letters hidden by asterisks as some subtitles hide
// them, and across the apostrophes of speech ("fuckin'").
var (
	strongRe = regexp.MustCompile(`(?i)\b(?:` +
		// the f-word, the s-word and the c-word, in all their forms
		`\w*fuck\w*|f[*#@%]+[ck]\w*|mofo|\w*shit\w*|sh?[*#@%]+t|cunt\w*|c[*#@%]+n?t|cock[- ]?suck\w*|twat\w*|tits|` +
		// slurs: racial, against gay and trans people, against the disabled
		`nigg(?:er|a)\w*|n[*#@%]+(?:er|a)\w*|fag|fags|faggot\w*|trann(?:y|ies)|retard(?:ed|s)?|kikes?|spics?|spicks?|` +
		`chinks?|gooks?|wetbacks?|pakis?|towelheads?|ragheads?|coons?|dykes?|beaners?|japs?|midgets?|spazz?` +
		`)\b`)

	// mildRe is what a promo airs but would rather not: curses, insults and crude words.
	mildRe = regexp.MustCompile(`(?i)\b(?:ass|asses|asshole\w*|bastards?|bitch\w*|damn\w*|goddamn\w*|crap\w*|piss\w*|dicks?|dickhead\w*|` +
		`pricks?|pussy|pussies|whore\w*|slut\w*|bollocks|arse\w*|wank\w*)\b`)
)

// strongWord is the first strong profanity or slur in text, empty when it has none.
func strongWord(text string) string { return strongRe.FindString(text) }

// strongIn is the first strong word of the cues, empty when they have none.
func strongIn(cues []dialogueCue) string {
	for _, c := range cues {
		if w := strongWord(c.Text); w != "" {
			return w
		}
	}
	return ""
}
