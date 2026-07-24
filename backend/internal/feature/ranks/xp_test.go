package ranks

import "testing"

// TestComputeXPSourcesSumToTotal is the one that matters: it fails the moment
// the displayed breakdown drifts from the number the rank is computed off.
func TestComputeXPSourcesSumToTotal(t *testing.T) {
	cases := []XPInputs{
		{},
		{VideoSeconds: 3600},
		{VideoSeconds: 7200, MusicSeconds: 1800, MoviesCompleted: 3},
		{EpisodesCompleted: 40, CouchHosted: 2, CouchJoined: 5},
		{VideoSeconds: 999_999, MusicSeconds: 12_345, MoviesCompleted: 7,
			EpisodesCompleted: 120, CouchHosted: 9, CouchJoined: 11,
			AchievementCount: 6, AchievementXP: 900},
	}
	for i, in := range cases {
		got := ComputeXP(in, defCfg)
		var sum int64
		for _, s := range got.Sources {
			sum += s.XP
			if s.XP <= 0 {
				t.Fatalf("case %d: source %q has %d xp, zero sources must be dropped", i, s.Key, s.XP)
			}
		}
		if sum != got.Total {
			t.Fatalf("case %d: sources sum to %d, total is %d", i, sum, got.Total)
		}
	}
}

func TestComputeXPGolden(t *testing.T) {
	// 120 video minutes, 60 music minutes, 2 movies, 10 episodes, 1 host,
	// 2 joins and 300 achievement xp.
	got := ComputeXP(XPInputs{
		VideoSeconds:      7200,
		MusicSeconds:      3600,
		MoviesCompleted:   2,
		EpisodesCompleted: 10,
		CouchHosted:       1,
		CouchJoined:       2,
		AchievementCount:  3,
		AchievementXP:     300,
	}, defCfg)
	const want = 240 + 60 + 200 + 200 + 50 + 50 + 300
	if got.Total != want {
		t.Fatalf("got %d, want %d", got.Total, want)
	}
	if len(got.Sources) != 7 {
		t.Fatalf("got %d sources, want 7", len(got.Sources))
	}
}

func TestComputeXPZeroInputs(t *testing.T) {
	got := ComputeXP(XPInputs{}, defCfg)
	if got.Total != 0 {
		t.Fatalf("got %d, want 0", got.Total)
	}
	if len(got.Sources) != 0 {
		t.Fatalf("got %d sources, want none", len(got.Sources))
	}
}

// A partial minute is worth nothing, so a 30s sample cannot inflate the total.
func TestComputeXPRoundsDownToWholeMinutes(t *testing.T) {
	if got := ComputeXP(XPInputs{VideoSeconds: 59}, defCfg).Total; got != 0 {
		t.Fatalf("got %d, want 0", got)
	}
	if got := ComputeXP(XPInputs{VideoSeconds: 119}, defCfg).Total; got != 2 {
		t.Fatalf("got %d, want 2", got)
	}
}
