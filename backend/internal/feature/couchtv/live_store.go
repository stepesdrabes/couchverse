package couchtv

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/db"
	"couchverse/internal/feature/catalog"
	"couchverse/internal/hls"
	"couchverse/internal/media"
)

const liveCols = airingCols + `, origin, feed`

func scanLive(rows pgx.Rows) ([]LiveAiring, error) {
	defer rows.Close()
	var out []LiveAiring
	for rows.Next() {
		var a LiveAiring
		var credits *int
		var origin, feed []byte
		if err := rows.Scan(&a.ID, &a.StartsAt, &a.EndsAt, &a.Kind, &a.TitleID, &a.EpisodeID, &a.MediaFileID,
			&a.Slot, &a.Fill, &a.Source, &a.Locked, &a.State, &credits, &origin, &feed); err != nil {
			return nil, err
		}
		if credits != nil {
			a.CreditsAt = time.Duration(*credits) * time.Millisecond
		}
		if origin != nil && feed != nil {
			a.Origin, a.Feed = &hls.LiveOrigin{}, &Feed{}
			if err := json.Unmarshal(origin, a.Origin); err != nil {
				return nil, err
			}
			if err := json.Unmarshal(feed, a.Feed); err != nil {
				return nil, err
			}
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// LiveAirings lists the airings overlapping (from, to] in order, after the last
// one that ended by from.
func (s *Store) LiveAirings(ctx context.Context, from, to time.Time) ([]LiveAiring, error) {
	rows, err := s.q.Query(ctx, `
		(SELECT `+liveCols+` FROM couchtv_airings WHERE state <> 'replaced' AND ends_at <= $1
		 ORDER BY ends_at DESC, starts_at DESC LIMIT 1)
		UNION ALL
		(SELECT `+liveCols+` FROM couchtv_airings WHERE state <> 'replaced' AND ends_at > $1 AND starts_at <= $2)
		ORDER BY 2, 4 DESC`, from, to)
	if err != nil {
		return nil, err
	}
	return scanLive(rows)
}

// PinAiring stores what an airing airs and its numbering as it goes on air,
// unless it went on air already; the stored airing wins.
func (s *Store) PinAiring(ctx context.Context, id string, origin hls.LiveOrigin, feed Feed) (LiveAiring, error) {
	o, err := json.Marshal(origin)
	if err != nil {
		return LiveAiring{}, err
	}
	f, err := json.Marshal(feed)
	if err != nil {
		return LiveAiring{}, err
	}
	if _, err := s.q.Exec(ctx, `UPDATE couchtv_airings SET origin = $2, feed = $3, state = 'aired'
		WHERE id = $1 AND origin IS NULL`, id, string(o), string(f)); err != nil {
		return LiveAiring{}, err
	}
	a, err := s.PinnedAiring(ctx, id)
	if err != nil {
		return LiveAiring{}, err
	}
	return *a, nil
}

// PinnedAiring reads an airing that went on air.
func (s *Store) PinnedAiring(ctx context.Context, id string) (*LiveAiring, error) {
	rows, err := s.q.Query(ctx, `SELECT `+liveCols+` FROM couchtv_airings WHERE id = $1 AND origin IS NOT NULL AND feed IS NOT NULL`, id)
	if err != nil {
		return nil, err
	}
	out, err := scanLive(rows)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, db.ErrNotFound
	}
	return &out[0], nil
}

// FileSubtitles are a media file's subtitle tracks, oldest first.
func (s *Store) FileSubtitles(ctx context.Context, fileID string) ([]liveSubtitle, error) {
	rows, err := s.q.Query(ctx, `SELECT id, lang, forced FROM subtitles WHERE media_file_id = $1 ORDER BY created_at, id`, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []liveSubtitle
	for rows.Next() {
		var sub liveSubtitle
		if err := rows.Scan(&sub.ID, &sub.Lang, &sub.Forced); err != nil {
			return nil, err
		}
		sub.Lang = media.BCP47(sub.Lang)
		out = append(out, sub)
	}
	return out, rows.Err()
}

// SubtitlePath is where a subtitle track's WebVTT file lives, under the data directory.
func (s *Store) SubtitlePath(ctx context.Context, id string) (string, error) {
	var path string
	err := s.q.QueryRow(ctx, `SELECT path FROM subtitles WHERE id = $1`, id).Scan(&path)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", db.ErrNotFound
	}
	return path, err
}

// Listings are the airings overlapping [from, to) as viewers see them, with
// their titles in the request's display language; programs only unless breaks.
func (s *Store) Listings(ctx context.Context, from, to time.Time, breaks bool) ([]CouchTvListing, error) {
	rows, err := s.q.Query(ctx, `
		SELECT a.id, a.kind, a.starts_at, a.ends_at, coalesce(a.fill, ''),
		       coalesce(t.id::text, ''), coalesce(t.slug, ''), coalesce(t.kind, ''), coalesce(t.name, ''),
		       coalesce(t.overview, ''), t.translations,
		       coalesce(e.id::text, ''), coalesce(se.season_number, 0), coalesce(e.episode_number, 0),
		       coalesce(e.name, ''), coalesce(e.overview, ''), e.translations,
		       coalesce(poster.id::text, ''), coalesce(poster.ver, 0), coalesce(backdrop.id::text, ''), coalesce(backdrop.ver, 0)
		FROM couchtv_airings a
		LEFT JOIN titles t ON t.id = a.title_id
		LEFT JOIN episodes e ON e.id = a.episode_id
		LEFT JOIN seasons se ON se.id = e.season_id
		LEFT JOIN LATERAL (SELECT ar.id, extract(epoch FROM ar.created_at)::bigint AS ver FROM artwork ar
			WHERE ar.owner_kind = 'title' AND ar.owner_id = t.id::text AND ar.kind = 'poster' LIMIT 1) poster ON true
		LEFT JOIN LATERAL (SELECT ar.id, extract(epoch FROM ar.created_at)::bigint AS ver FROM artwork ar
			WHERE ar.owner_kind = 'title' AND ar.owner_id = t.id::text AND ar.kind = 'backdrop' LIMIT 1) backdrop ON true
		WHERE a.state <> 'replaced' AND a.starts_at < $2 AND a.ends_at > $1 AND ($3 OR a.kind = 'program')
		ORDER BY a.starts_at, a.kind DESC`, from, to, breaks)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CouchTvListing{}
	for rows.Next() {
		var l CouchTvListing
		var titleTr, episodeTr []byte
		var titleOverview, episodeOverview string
		if err := rows.Scan(&l.ID, &l.Kind, &l.StartsAt, &l.EndsAt, &l.Fill,
			&l.TitleID, &l.TitleSlug, &l.TitleKind, &l.TitleName, &titleOverview, &titleTr,
			&l.EpisodeID, &l.SeasonNumber, &l.EpisodeNumber, &l.EpisodeName, &episodeOverview, &episodeTr,
			&l.PosterID, &l.PosterVer, &l.BackdropID, &l.BackdropVer); err != nil {
			return nil, err
		}
		catalog.Localize(ctx, titleTr, &l.TitleName, &titleOverview)
		catalog.Localize(ctx, episodeTr, &l.EpisodeName, &episodeOverview)
		l.Overview = titleOverview
		if l.EpisodeID != "" && episodeOverview != "" {
			l.Overview = episodeOverview
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
