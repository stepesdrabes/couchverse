package couchtv

import (
	"context"
	"encoding/json"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
)

// program is a program airing with what the breaks show of its episode.
type program struct {
	Airing
	Season      int
	Number      int
	EpisodeName string
	Runtime     int
}

// Cutscenes are English (D13): a title's or episode's base text when the title's base
// language is English, else its English translation where it has one.
const (
	englishTitle   = `CASE WHEN coalesce(t.metadata_languages[1], 'en') = 'en' THEN t.name ELSE coalesce(nullif(t.translations->'en'->>'name', ''), t.name) END`
	englishTagline = `CASE WHEN coalesce(t.metadata_languages[1], 'en') = 'en' THEN t.tagline ELSE coalesce(t.translations->'en'->>'tagline', '') END`
	englishEpisode = `CASE WHEN coalesce(t.metadata_languages[1], 'en') = 'en' THEN e.name ELSE coalesce(nullif(e.translations->'en'->>'name', ''), e.name) END`
)

// programs are the programs overlapping [from, to), in order.
func (s *Store) programs(ctx context.Context, from, to time.Time) ([]program, error) {
	rows, err := s.q.Query(ctx, `
		SELECT a.id, a.starts_at, a.ends_at, a.title_id, coalesce(a.episode_id::text, ''), coalesce(a.media_file_id::text, ''),
		       coalesce(a.fill, ''), a.slot, coalesce(a.credits_at_ms, 0),
		       coalesce(se.season_number, 0), coalesce(e.episode_number, 0), coalesce(`+englishEpisode+`, ''),
		       coalesce(e.runtime_minutes, 0)
		FROM couchtv_airings a
		JOIN titles t ON t.id = a.title_id
		LEFT JOIN episodes e ON e.id = a.episode_id
		LEFT JOIN seasons se ON se.id = e.season_id
		WHERE a.kind = 'program' AND a.state <> 'replaced' AND a.ends_at > $1 AND a.starts_at < $2
		ORDER BY a.starts_at`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []program
	for rows.Next() {
		var p program
		var credits int
		if err := rows.Scan(&p.ID, &p.StartsAt, &p.EndsAt, &p.TitleID, &p.EpisodeID, &p.MediaFileID, &p.Fill, &p.Slot, &credits,
			&p.Season, &p.Number, &p.EpisodeName, &p.Runtime); err != nil {
			return nil, err
		}
		p.Kind, p.CreditsAt = KindProgram, time.Duration(credits)*time.Millisecond
		out = append(out, p)
	}
	return out, rows.Err()
}

// uncomposed are the breaks starting in [from, to) without a composition; one that failed
// is tried again after retryAfter.
func (s *Store) uncomposed(ctx context.Context, from, to time.Time) ([]Airing, error) {
	return s.airings(ctx, `kind = 'break' AND starts_at >= $1 AND starts_at < $2 AND NOT EXISTS (
		SELECT 1 FROM couchtv_breaks b WHERE b.airing_id = couchtv_airings.id
		AND (b.state <> 'failed' OR b.updated_at > $3))`, from, to, from.Add(-retryAfter))
}

// unusedClips are the clips waiting to be rendered that no composition uses, aside from
// promo bodies (requested for breaks yet to be composed) and the standby card (which no
// break holds).
func (s *Store) unusedClips(ctx context.Context) ([]string, error) {
	rows, err := s.q.Query(ctx, `
		SELECT c.id FROM couchtv_clips c
		WHERE c.state = 'pending' AND c.kind NOT IN ('promo_body', 'standby') AND NOT EXISTS (
			SELECT 1 FROM couchtv_breaks b
			CROSS JOIN LATERAL jsonb_array_elements(CASE WHEN jsonb_typeof(b.recipe->'clips') = 'array'
				THEN b.recipe->'clips' ELSE '[]' END) rc
			WHERE b.state <> 'failed' AND rc->>'clipId' = c.id::text)`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// retryAfter is how long a break that failed waits before it is composed again.
const retryAfter = 6 * time.Hour

// outdated are the breaks starting in [from, to), none on air yet, that use a clip rendered
// from another version of its template than the sidecar's: composed again, they request
// the clips of the template as it is now. (A clip not rendered yet renders from the current
// template anyway.)
func (s *Store) outdated(ctx context.Context, from, to time.Time, versions map[string]int) ([]Airing, error) {
	current, err := json.Marshal(versions)
	if err != nil {
		return nil, err
	}
	return s.airings(ctx, `kind = 'break' AND starts_at >= $1 AND starts_at < $2 AND feed IS NULL AND EXISTS (
		SELECT 1 FROM couchtv_breaks b
		CROSS JOIN LATERAL jsonb_array_elements(CASE WHEN jsonb_typeof(b.recipe->'clips') = 'array'
			THEN b.recipe->'clips' ELSE '[]' END) rc
		JOIN couchtv_clips c ON c.id::text = rc->>'clipId'
		WHERE b.airing_id = couchtv_airings.id AND b.state <> 'failed' AND c.state = 'ready'
		AND c.version <> coalesce(($3::jsonb ->> (c.spec->>'template'))::int, c.version))`, from, to, current)
}

var hexColour = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// titleFacts reads what the breaks show of the given titles.
func (s *Store) titleFacts(ctx context.Context, ids []string) (map[string]titleFacts, error) {
	genres, err := s.genres(ctx)
	if err != nil {
		return nil, err
	}
	art := func(kind string) string {
		return `LEFT JOIN LATERAL (SELECT id, (extract(epoch FROM created_at) * 1000)::bigint AS version, accent FROM artwork
			WHERE owner_kind = 'title' AND owner_id = t.id::text AND kind = '` + kind + `'
			ORDER BY lang IS NOT NULL, lang IS DISTINCT FROM 'en' LIMIT 1) ` + kind + ` ON true`
	}
	rows, err := s.q.Query(ctx, `
		SELECT t.id, `+englishTitle+`, coalesce(t.year, 0), coalesce(t.runtime_minutes, 0),
		       `+englishTagline+`,
		       coalesce(backdrop.id::text, ''), coalesce(backdrop.version, 0), coalesce(backdrop.accent, ''),
		       coalesce(poster.id::text, ''), coalesce(poster.version, 0), coalesce(poster.accent, ''),
		       coalesce(ct.mood, ''), coalesce(ct.shots, '[]'), coalesce(ct.promo_audio_file_id::text, ''),
		       coalesce(ct.promo_audio_start_ms, 0), coalesce(ct.promo_audio_duration_ms, 0), coalesce(ct.loudness_lufs, 0)
		FROM titles t
		`+art("backdrop")+`
		`+art("poster")+`
		LEFT JOIN couchtv_titles ct ON ct.title_id = t.id
		WHERE t.id = ANY($1::uuid[])`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]titleFacts{}
	for rows.Next() {
		var id, backdrop, bdAccent, poster, poAccent, promoFile string
		var bdVersion, poVersion int64
		var shots []byte
		var promoStart, promoLength int
		var f titleFacts
		if err := rows.Scan(&id, &f.Name, &f.Year, &f.Runtime, &f.Tagline, &backdrop, &bdVersion, &bdAccent,
			&poster, &poVersion, &poAccent, &f.Mood, &shots, &promoFile, &promoStart, &promoLength, &f.Loudness); err != nil {
			return nil, err
		}
		f.Genres = genres[id]
		if backdrop != "" {
			f.Backdrop = ArtworkURL(backdrop, bdVersion, "")
		}
		if poster != "" {
			f.Poster = ArtworkURL(poster, poVersion, "w780")
		}
		if hexColour.MatchString(bdAccent) {
			f.Accents.Backdrop = bdAccent
		}
		if hexColour.MatchString(poAccent) {
			f.Accents.Poster = poAccent
		}
		f.Shots = parseShots(shots)
		if promoFile != "" && promoLength > 0 {
			length := time.Duration(promoLength) * time.Millisecond
			f.Promo = &Bed{URL: AudioURL(promoFile, time.Duration(promoStart)*time.Millisecond, length), Out: length.Seconds()}
		}
		out[id] = f
	}
	return out, rows.Err()
}

// promotions are the titles promoted by the breaks composed in [from, to), by start.
func (s *Store) promotions(ctx context.Context, from, to time.Time) ([]promotion, error) {
	rows, err := s.q.Query(ctx, `
		SELECT a.starts_at, coalesce(b.recipe->'promos', '[]') FROM couchtv_breaks b JOIN couchtv_airings a ON a.id = b.airing_id
		WHERE a.starts_at >= $1 AND a.starts_at < $2 AND b.state <> 'failed' ORDER BY a.starts_at`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []promotion
	for rows.Next() {
		var p promotion
		var raw []byte
		if err := rows.Scan(&p.at, &raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &p.titles); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// evergreenComposed reports whether an evergreen break with this recipe key is composed or
// assembled already.
func (s *Store) evergreenComposed(ctx context.Context, key string) (bool, error) {
	var exists bool
	err := s.q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM couchtv_breaks
		WHERE kind = 'evergreen' AND recipe->>'key' = $1 AND state <> 'failed')`, key).Scan(&exists)
	return exists, err
}
