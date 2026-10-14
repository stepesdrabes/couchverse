package couchtv

import "testing"

func TestStrongWord(t *testing.T) {
	for _, line := range []string{
		"What the fuck is that?",
		"WHAT THE FUCK",
		"You give me the fuckin' creeps.",
		"Get your motherfucking hands off him.",
		"Mothafucka!",
		"Fuckers.",
		"Oh, f**k.",
		"F*ck this.",
		"This is bullshit.",
		"Holy shit!",
		"You're full of sh*t.",
		"Shitty day.",
		"Don't be such a cunt.",
		"What a c*nt.",
		"Cocksucker.",
		"Little twat.",
		"Nice tits.",
		"Don't call me that, nigger.",
		"My niggas.",
		"Stupid faggot.",
		"Don't be a fag.",
		"That's retarded.",
		"You retard.",
		"Dirty kike.",
		"Go home, you spic.",
		"Chinks and gooks.",
		"Filthy wetback.",
		"Damn it, you mofo.",
	} {
		if strongWord(line) == "" {
			t.Errorf("%q passes", line)
		}
	}
	for _, line := range []string{
		"You can't be serious.",
		"Sit down and assist the assistant.",
		"Pass the classic mass.",
		"Scunthorpe and Penistone are in England.",
		"The shiitake are in the kitchen.",
		"Dick Grayson is a boy.",
		"What the hell is going on?",
		"Damn it, Jarvis.",
		"In what, ass-kissing? Politics.",
		"He's a bastard, and a prick.",
		"You bitch.",
		"Hancock, Hitchcock and a cocktail.",
		"A raccoon in the attic.",
		"Mind the pakistani cook.",
		"Tennessee, Jarvis?",
		"I'm a piping hot mess.",
	} {
		if w := strongWord(line); w != "" {
			t.Errorf("%q is refused for %q", line, w)
		}
	}
	// the first strong word is the one reported
	if w := strongIn([]dialogueCue{{Text: "Hello there."}, {Text: "You absolute fucker."}, {Text: "Shit."}}); w != "fucker" {
		t.Errorf("strongIn reports %q", w)
	}
	if w := strongIn([]dialogueCue{{Text: "Hello there."}}); w != "" {
		t.Errorf("strongIn reports %q of a clean line", w)
	}
}

// A run with strong language scores nothing and says so; a mild word only costs it, and a run
// that says neither is better than both.
func TestScoreTextLanguage(t *testing.T) {
	run := func(text string) textRun {
		c := cue(0, 2.4, text)
		return textRun{Cues: []dialogueCue{c}, Complete: endCompleteness(c)}
	}
	clean := scoreText(run("Nobody leaves this world alive!"), nil)
	mild := scoreText(run("Nobody leaves this damn world alive!"), nil)
	strong := scoreText(run("Nobody leaves this fucking world alive!"), nil)
	if clean.Strong || mild.Strong || !strong.Strong {
		t.Errorf("strong: clean %v, mild %v, strong %v", clean.Strong, mild.Strong, strong.Strong)
	}
	if strong.Score != 0 || mild.Score <= 0 || mild.Score >= clean.Score {
		t.Errorf("scores: clean %.2f, mild %.2f, strong %.2f", clean.Score, mild.Score, strong.Score)
	}
}
