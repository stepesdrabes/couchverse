package couchtv

import (
	"math"
	"regexp"
	"strings"
	"unicode"
)

// How a trailer editor would read a run of lines (D43): short, punchy, whole lines, questions
// and exclamations, a back-and-forth between two voices, a name or a stake; not exposition,
// not mumbling, not small talk, not a line that needs the scene before it. The rules are plain
// features of the text, and a run's score is 0 to 1.

var (
	// what is at stake in a story, and what it feels like: the first count more
	stakeWords = wordSet(`kill killed die dies died dead death murder war world destroy end enemy truth secret monster evil
		blood danger escape hunt fight save power mission chosen destiny never nobody everything nothing`)
	softWords = wordSet(`love trust fear home family remember promise believe lie lost gone free forever help alone afraid
		wrong trouble`)
	crudeRe  = regexp.MustCompile(`(?i)\b(fuck\w*|shit\w*|bitch\w*|asshole|cunt|dick|pussy|nigg\w+|whore|slut)\b`)
	fillerRe = regexp.MustCompile(`(?i)^(uh|um|er|erm|hmm|huh|well|so|yeah|yes|no|okay|ok|hey|hi|hello|oh|ah)\b[,.!? ]`)
	// a line that carries on the one before it, or answers a question it does not repeat
	carryOnRe = regexp.MustCompile(`(?i)^(and|but|so|because|or|then|which|anyway|besides|that's why)\b`)
	// small talk: manners and logistics
	smallTalkRe = regexp.MustCompile(`(?i)\b(sorry|thanks|thank you|please|hello|goodbye|bye|excuse me|mind if|how are you|nice to meet|see you|good morning|good night|wait up)\b`)
	nameRe      = regexp.MustCompile(`[^.?!"“]\s+([A-Z][a-z]{2,})`)
	// how a story starts, in a voice that sets the scene
	narrationRe = regexp.MustCompile(`^(I |I'|My |This is|There |Once |When |In |It was|It's been|Some )`)
	digitRe     = regexp.MustCompile(`\d`)
)

func wordSet(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

// textFeatures are the parts of a run's text score, kept for tuning and the admin.
type textFeatures struct {
	Shape, Brevity, Content float64
	// Opener and Button say how well the run opens a trailer or ends one on a punchline.
	Opener, Button float64
	Score          float64
}

// scoreText rates what a run says. Its parts: the shape of its speech (2.5 to 5.2 s is the
// sweet spot), its brevity (a trailer line is a dozen words; a speech is not), and its
// content (punctuation that punches, a back-and-forth, stakes, a name, the title's own name,
// minus filler, small talk, crudeness and detail). keywords are the title's own words (its
// name): a line that says them is a line for its trailer.
func scoreText(r textRun, keywords []string) textFeatures {
	text := r.text()
	words := r.words()
	speech := r.end() - r.start()
	var f textFeatures

	switch {
	case speech < 1.6:
		f.Shape = 0.5
	case speech < 2.4:
		f.Shape = 0.5 + (speech-1.6)/0.8*0.5
	case speech <= 5.2:
		f.Shape = 1
	default:
		f.Shape = 1 - (speech-5.2)/1.2*0.3
	}

	longest := 0
	for _, c := range r.Cues {
		longest = max(longest, c.Words)
	}
	switch {
	case words <= 14:
		f.Brevity = 1
	case words <= 22:
		f.Brevity = 1 - float64(words-14)*0.04
	default:
		f.Brevity = max(0.35, 0.68-float64(words-22)*0.04)
	}
	if longest > 14 {
		f.Brevity *= 0.7
	}

	content := 0.25
	if strings.Contains(text, "?") {
		content += 0.15
	}
	if strings.Contains(text, "!") {
		content += 0.15
	}
	if len(r.Cues) >= 2 || r.Cues[0].Turns > 0 {
		content += 0.15
		if len(r.Cues) >= 3 && len(r.Cues) <= 4 {
			content += 0.05
		}
	}
	strong, soft := 0, 0
	seen := map[string]bool{}
	lower := strings.ToLower(text)
	for _, w := range strings.FieldsFunc(lower, func(r rune) bool { return !unicode.IsLetter(r) && r != '\'' }) {
		switch {
		case seen[w]:
		case stakeWords[w]:
			strong++
			seen[w] = true
		case softWords[w]:
			soft++
			seen[w] = true
		}
	}
	content += min(0.36, 0.18*float64(strong)) + min(0.12, 0.06*float64(soft))
	content += min(0.12, 0.06*float64(len(nameRe.FindAllStringSubmatch(text, -1))))
	for _, k := range keywords {
		if k != "" && strings.Contains(lower, strings.ToLower(k)) {
			content += 0.2
			break
		}
	}
	if carryOnRe.MatchString(text) {
		content -= 0.25
	}
	if words <= 6 && fillerRe.MatchString(text) {
		content -= 0.25
	}
	if words < 4 && !strings.ContainsAny(text, "?!") {
		content -= 0.5
	}
	content -= min(0.3, 0.15*float64(len(smallTalkRe.FindAllString(text, -1))))
	if crudeRe.MatchString(text) {
		content -= 0.3
	}
	if digitRe.MatchString(text) {
		content -= 0.1
	}
	f.Content = math.Max(0, math.Min(1, content))

	complete := 1.0
	if r.Complete < 1 {
		complete = 0.6
	}
	f.Score = f.Shape * f.Brevity * f.Content * complete

	// the structure of a trailer: a calm line to open on, a punchline to end on
	last := r.Cues[len(r.Cues)-1]
	if speech <= 3.6 && words <= 9 {
		f.Button = 0.4
		if strings.Contains(last.Text, "!") || len(r.Cues) >= 2 {
			f.Button += 0.3
		}
		if last.Words <= 6 {
			f.Button += 0.3
		}
	}
	if !strings.ContainsAny(text, "!?") && words >= 5 && words <= 18 {
		f.Opener = 0.5
		if narrationRe.MatchString(text) {
			f.Opener += 0.5
		}
	}
	return f
}
