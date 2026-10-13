package ranks

import (
	"context"
	"time"
)

// The parts of an airing the channel reports (couchtv's PartProgram, PartBreak
// and PartBulletin, a bulletin airing in a break's slot). Its fillers, the
// standby card and the slate, are not television and earn nothing.
const (
	tvProgram  = "program"
	tvBreak    = "break"
	tvBulletin = "bulletin"
)

// What counts on Couch TV. A program is seen at half its time on air, at most
// tvSeenAtMost (eleven minutes of a sitcom, half an hour of a movie), and
// watched whole at tvWholePercent of it, which leaves room for tuning in late
// and a moment away. A time-of-day window counts on a day with tvWindowMinutes
// in it.
const (
	tvSeenAtMost    = 30 * time.Minute
	tvWholePercent  = 80
	tvWindowMinutes = 15
)

// The three methods below satisfy couchtv.TvStatsRecorder; the room calls them
// from its 20 s accrual, never from a socket's read path. Like the couch
// counters they stay live while the rankings flag is off.

// RecordTvViewing adds a stretch a member watched of one part of an airing. at
// is on the channel's clock, so its day and hour are the channel's.
func (s *Store) RecordTvViewing(ctx context.Context, userID int64, airingID, kind string, at time.Time, seconds int) error {
	if seconds <= 0 || (kind != tvProgram && kind != tvBreak && kind != tvBulletin) {
		return nil
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO user_tv_viewing (user_id, airing_id, day, hour, seconds) VALUES ($1, $2, $3::date, $4, $5)
		 ON CONFLICT (user_id, airing_id, day, hour)
		 DO UPDATE SET seconds = user_tv_viewing.seconds + EXCLUDED.seconds`,
		userID, airingID, at.Format(time.DateOnly), at.Hour(), seconds)
	return err
}

func (s *Store) RecordTvReactions(ctx context.Context, userID int64, count int) error {
	return s.addCounter(ctx, userID, counterTvReactions, int64(count))
}

func (s *Store) RecordTvAudience(ctx context.Context, userID int64, viewers int) error {
	return s.maxCounter(ctx, userID, counterTvAudience, int64(viewers))
}

// tvFacts fills the Couch TV part of a snapshot. What an airing was (its fill,
// its series, its time on air) is read from couchtv_airings; the day of an
// airing is the one the member started watching it on.
//
// A strip's run is gaps-and-islands per series, as in the watch streaks. A
// weekend marathon is every marathon episode the channel airs without a two-day
// gap between them, planned ones included, so a weekend only completes once its
// last episode was watched; it is the template's whole weekend block, however
// many episodes an admin gives it.
func (s *Store) tvFacts(ctx context.Context, userID int64, snap *Snapshot) error {
	return s.db.QueryRow(ctx,
		`WITH mine AS (
			SELECT airing_id, day, hour, seconds FROM user_tv_viewing WHERE user_id = $1
		), days AS (
			SELECT day,
				sum(seconds) FILTER (WHERE hour BETWEEN 2 AND 5) AS late,
				sum(seconds) FILTER (WHERE hour BETWEEN 6 AND 8) AS morning
			FROM mine GROUP BY day
		), programs AS (
			SELECT a.id, a.fill, a.title_id, min(m.day) AS day, sum(m.seconds) AS seconds,
				extract(epoch FROM a.ends_at - a.starts_at) AS length
			FROM mine m JOIN couchtv_airings a ON a.id = m.airing_id AND a.kind = 'program'
			GROUP BY a.id
		), seen AS (
			SELECT fill, title_id, day FROM programs WHERE seconds >= least(length / 2, $2)
		), whole AS (
			SELECT id, fill FROM programs WHERE seconds * 100 >= length * $3
		), runs AS (
			SELECT count(*) AS len FROM (
				SELECT title_id, day - (row_number() OVER (PARTITION BY title_id ORDER BY day))::int AS anchor
				FROM (SELECT DISTINCT title_id, day FROM seen WHERE fill = 'strip') d
			) g GROUP BY title_id, anchor
		), marathon AS (
			SELECT id, sum(gap) OVER (ORDER BY starts_at) AS weekend FROM (
				SELECT id, starts_at,
					CASE WHEN starts_at - lag(ends_at) OVER (ORDER BY starts_at) < interval '2 days' THEN 0 ELSE 1 END AS gap
				FROM couchtv_airings WHERE kind = 'program' AND fill = 'marathon' AND state <> 'replaced'
			) g
		)
		SELECT
			COALESCE((SELECT sum(seconds) FROM mine), 0)::bigint,
			(SELECT count(DISTINCT day) FROM seen WHERE fill IN ('movie', 'premiere', 'saga')),
			(SELECT count(*) FROM days WHERE late >= $4),
			(SELECT count(*) FROM days WHERE morning >= $4),
			COALESCE((SELECT max(len) FROM runs), 0),
			(SELECT count(*) FROM (
				SELECT m.weekend FROM marathon m LEFT JOIN whole w ON w.id = m.id
				GROUP BY m.weekend HAVING count(w.id) = count(*)) done),
			(SELECT count(*) FROM whole WHERE fill = 'premiere')`,
		userID, int64(tvSeenAtMost.Seconds()), tvWholePercent, tvWindowMinutes*60).Scan(
		&snap.TvSeconds, &snap.TvPrimeNights, &snap.TvLateNights, &snap.TvMornings,
		&snap.TvStripRun, &snap.TvMarathons, &snap.TvPremieres)
}

// TvSeconds sums every member's Couch TV seconds, for the leaderboard's XP.
func (s *Store) TvSeconds(ctx context.Context) (map[int64]int64, error) {
	rows, err := s.db.Query(ctx,
		`SELECT user_id, sum(seconds)::bigint FROM user_tv_viewing GROUP BY user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanScores(rows)
}
