package server_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"couchverse/internal/feature/ranks"
)

// TestRanksCouchTv records a member's Couch TV as the room does and reads it back as
// ranks: XP for the channel's minutes, each family of achievements from what aired,
// and the badges gone with the flag while the XP stays.
func TestRanksCouchTv(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	st := ranks.NewStore(env.pool)
	const nora = 2
	prague, _ := time.LoadLocation("Europe/Prague")
	// days from Monday 5 October 2026, on the channel's clock
	at := func(day, hour, minute int) time.Time {
		return time.Date(2026, 10, 5+day, hour, minute, 0, 0, prague)
	}
	airing := func(starts time.Time, length time.Duration, fill, titleID, state string) string {
		t.Helper()
		var id string
		if err := env.pool.QueryRow(ctx, `INSERT INTO couchtv_airings (starts_at, ends_at, kind, title_id, fill, state)
			VALUES ($1, $2, 'program', $3, $4, $5) RETURNING id::text`,
			starts, starts.Add(length), titleID, fill, state).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	var tvSeconds int64
	// watch reports a stretch minute by minute, as the room's accruals add up
	watch := func(airingID, kind string, from time.Time, d time.Duration) {
		t.Helper()
		for m := time.Duration(0); m < d; m += time.Minute {
			if err := st.RecordTvViewing(ctx, nora, airingID, kind, from.Add(m), 60); err != nil {
				t.Fatal(err)
			}
			if kind == "program" || kind == "break" || kind == "bulletin" {
				tvSeconds += 60
			}
		}
	}

	// prime time: four nights of 40 minutes of the 20:15 movie and one of 20 (seen
	// at 30), a premiere watched whole and one watched for an hour; a matinee is
	// no prime time, however long
	for d := range 5 {
		minutes := 40
		if d == 4 {
			minutes = 20
		}
		id := airing(at(d, 20, 15), 110*time.Minute, "movie", movieID, "aired")
		watch(id, "program", at(d, 20, 15), time.Duration(minutes)*time.Minute)
	}
	premiere := airing(at(5, 20, 15), 110*time.Minute, "premiere", movieID, "aired")
	watch(premiere, "program", at(5, 20, 15), 100*time.Minute)
	halfPremiere := airing(at(12, 20, 15), 110*time.Minute, "premiere", movieID, "aired")
	watch(halfPremiere, "program", at(12, 20, 15), 60*time.Minute)
	matinee := airing(at(0, 12, 0), 110*time.Minute, "matinee", movieID, "aired")
	watch(matinee, "program", at(0, 12, 0), 110*time.Minute)

	// a strip seen on two days, missed on the third (five minutes of 22) and seen
	// on the next five
	for d := range 8 {
		id := airing(at(d, 18, 0), 22*time.Minute, "strip", seriesID, "aired")
		minutes := 12
		if d == 2 {
			minutes = 5
		}
		watch(id, "program", at(d, 18, 0), time.Duration(minutes)*time.Minute)
	}

	// after two at night: twenty minutes on Tuesday; ten on Wednesday, then the
	// slate, which is no television
	night := airing(at(1, 2, 0), time.Hour, "reruns", seriesID, "aired")
	watch(night, "program", at(1, 2, 30), 20*time.Minute)
	late := airing(at(2, 3, 0), time.Hour, "reruns", seriesID, "aired")
	watch(late, "program", at(2, 3, 0), 10*time.Minute)
	watch(late, "slate", at(2, 3, 10), 30*time.Minute)

	// mornings: twenty minutes on five days, ten on a sixth
	for d := range 6 {
		minutes := 20
		if d == 5 {
			minutes = 10
		}
		id := airing(at(d, 6, 0), 3*time.Hour, "reruns", seriesID, "aired")
		watch(id, "program", at(d, 6, 30), time.Duration(minutes)*time.Minute)
	}

	// a break between programs counts as channel time, the bulletin in its slot too
	var brk string
	if err := env.pool.QueryRow(ctx, `INSERT INTO couchtv_airings (starts_at, ends_at, kind, state)
		VALUES ($1, $2, 'break', 'aired') RETURNING id::text`, at(0, 22, 5), at(0, 22, 10)).Scan(&brk); err != nil {
		t.Fatal(err)
	}
	watch(brk, "break", at(0, 22, 5), 4*time.Minute)
	watch(brk, "bulletin", at(0, 22, 9), time.Minute)

	// the first weekend's marathon watched whole (an episode an admin took off
	// aside); the next one only on its Saturday, while Sunday's is still to come
	for _, d := range []int{5, 6} {
		for _, start := range []time.Time{at(d, 13, 0), at(d, 13, 50)} {
			id := airing(start, 45*time.Minute, "marathon", seriesID, "aired")
			watch(id, "program", start, 40*time.Minute)
		}
	}
	airing(at(6, 14, 40), 45*time.Minute, "marathon", seriesID, "replaced")
	for _, start := range []time.Time{at(12, 13, 0), at(12, 13, 50)} {
		id := airing(start, 45*time.Minute, "marathon", seriesID, "aired")
		watch(id, "program", start, 45*time.Minute)
	}
	airing(at(13, 13, 0), 45*time.Minute, "marathon", seriesID, "planned")

	for _, n := range []int{60, 45} {
		if err := st.RecordTvReactions(ctx, nora, n); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range []int{3, 5, 4} {
		if err := st.RecordTvAudience(ctx, nora, n); err != nil {
			t.Fatal(err)
		}
	}

	snap, err := st.SnapshotFor(ctx, nora)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name      string
		got, want int64
	}{
		{"seconds", snap.TvSeconds, tvSeconds},
		{"prime-time nights", snap.TvPrimeNights, 6},
		{"premieres whole", snap.TvPremieres, 1},
		{"strip run", snap.TvStripRun, 5},
		{"late nights", snap.TvLateNights, 1},
		{"mornings", snap.TvMornings, 5},
		{"marathons", snap.TvMarathons, 1},
		{"reactions", snap.TvReactions, 105},
		{"audience", snap.TvAudienceMax, 5},
	} {
		if c.got != c.want {
			t.Errorf("%s: %d, want %d", c.name, c.got, c.want)
		}
	}

	// the profile with the channel on: a Couch TV line in the XP and every badge
	setFeatures(t, env, true)
	var profile ranks.UserProfile
	env.getJSON(t, memberName, "/me/stats", &profile)
	tvLine := func(p ranks.UserProfile) *ranks.XPSource {
		for _, s := range p.XP.Sources {
			if s.Key == "tv" {
				return &s
			}
		}
		return nil
	}
	if line := tvLine(profile); line == nil || line.Units != tvSeconds/60 || line.XP != tvSeconds/60*ranks.DefaultConfig().Rates.TvMinute {
		t.Errorf("the Couch TV XP is %+v, want %d minutes", line, tvSeconds/60)
	}
	unlocked := map[string]bool{}
	for _, a := range profile.Achievements {
		if a.Category == "tv" {
			unlocked[a.Code] = a.Unlocked
		}
	}
	want := map[string]bool{
		"tv_prime_time_5": true, "tv_night_owl": true, "tv_morning_5": true, "tv_full_house": true,
		"tv_reactions_100": true, "tv_strip_5": true, "tv_marathon": true, "tv_premiere": true,
	}
	for code, u := range want {
		if got, ok := unlocked[code]; !ok || got != u {
			t.Errorf("%s: listed %v, unlocked %v", code, ok, got)
		}
	}
	status, body, _ := env.request(t, env.clients[memberName], "POST", "/me/achievements/check", nil)
	if status != http.StatusOK {
		t.Fatalf("check: %d %s", status, body)
	}
	var persisted int
	if err := env.pool.QueryRow(ctx, `SELECT count(*) FROM user_achievements WHERE user_id = $1 AND code LIKE 'tv\_%'`,
		nora).Scan(&persisted); err != nil || persisted != len(want) {
		t.Errorf("the check persisted %d Couch TV badges (%v), want %d", persisted, err, len(want))
	}

	// with the channel off its badges are absent, not locked, and nobody is demoted
	setFeatures(t, env, false)
	env.getJSON(t, memberName, "/me/stats", &profile)
	for _, a := range profile.Achievements {
		if a.Category == "tv" {
			t.Errorf("%s is listed with the channel off", a.Code)
		}
	}
	if tvLine(profile) == nil {
		t.Error("the Couch TV XP went with the flag")
	}
}
