package catalog

import (
	"context"
	"fmt"
	"time"

	"couchverse/internal/media"
)

// Public catalog queries - published content only, shaped for the user app.

type CardItem struct {
	TitleID        string  `json:"titleId"`
	Slug           string  `json:"slug"`
	Kind           string  `json:"kind"`
	Name           string  `json:"name"`
	Year           *int    `json:"year"`
	PosterID       *string `json:"posterId"`
	PosterVer      int64   `json:"posterVer,omitempty"`
	PosterAccent   string  `json:"posterAccent,omitempty"`
	BackdropID     *string `json:"backdropId"`
	BackdropVer    int64   `json:"backdropVer,omitempty"`
	BackdropAccent string  `json:"backdropAccent,omitempty"`
}

type ContinueItem struct {
	CardItem
	EpisodeID    *string   `json:"episodeId"`
	EpisodeLabel string    `json:"episodeLabel"` // "S1 E3 · Pilot"
	PlaybackKind string    `json:"playbackKind"` // movie | episode
	PlaybackID   string    `json:"playbackId"`
	Position     int       `json:"positionSeconds"`
	Duration     int       `json:"durationSeconds"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type HomeRow struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
	Items any    `json:"items"`
}

// Each card carries its poster/backdrop id plus a version token (artwork
// updated time, unix seconds - an immutable cache-busting key; 0 when absent)
// and the server-extracted accent colour. The lateral joins fetch all three in
// one lookup per artwork kind.
const cardSelect = `
	SELECT t.id, t.slug, t.kind, t.name, t.year,
		poster.id, COALESCE(poster.ver, 0), COALESCE(poster.accent, ''),
		backdrop.id, COALESCE(backdrop.ver, 0), COALESCE(backdrop.accent, '')
	FROM titles t
	LEFT JOIN LATERAL (
		SELECT a.id, extract(epoch FROM a.created_at)::bigint AS ver, a.accent FROM artwork a
		WHERE a.owner_kind = 'title' AND a.owner_id = t.id::text AND a.kind = 'poster' LIMIT 1
	) poster ON true
	LEFT JOIN LATERAL (
		SELECT a.id, extract(epoch FROM a.created_at)::bigint AS ver, a.accent FROM artwork a
		WHERE a.owner_kind = 'title' AND a.owner_id = t.id::text AND a.kind = 'backdrop' LIMIT 1
	) backdrop ON true`

func (s *Store) scanCards(ctx context.Context, query string, args ...any) ([]CardItem, error) {
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []CardItem{}
	for rows.Next() {
		var c CardItem
		if err := rows.Scan(&c.TitleID, &c.Slug, &c.Kind, &c.Name, &c.Year,
			&c.PosterID, &c.PosterVer, &c.PosterAccent,
			&c.BackdropID, &c.BackdropVer, &c.BackdropAccent); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (s *Store) RecentlyAdded(ctx context.Context, limit int) ([]CardItem, error) {
	return s.scanCards(ctx, cardSelect+`
		WHERE t.status = 'published'
		ORDER BY t.added_at DESC LIMIT $1`, limit)
}

func (s *Store) TitlesByGenre(ctx context.Context, genreID int64, limit int) ([]CardItem, error) {
	return s.scanCards(ctx, cardSelect+`
		JOIN title_genres tg ON tg.title_id = t.id
		WHERE t.status = 'published' AND tg.genre_id = $1
		ORDER BY t.added_at DESC LIMIT $2`, genreID, limit)
}

// FeaturedTitles picks the hero carousel: the most recently published titles.
func (s *Store) FeaturedTitles(ctx context.Context, limit int) ([]Title, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+titleCols+` FROM titles WHERE status = 'published'
		 ORDER BY added_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	titles := []Title{}
	for rows.Next() {
		t, err := scanTitle(rows)
		if err != nil {
			return nil, err
		}
		titles = append(titles, *t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range titles {
		if err := s.loadTitleGenres(ctx, &titles[i]); err != nil {
			return nil, err
		}
	}
	return titles, nil
}

func (s *Store) HomeRowConfigs(ctx context.Context) ([]struct {
	Kind    string
	Label   string
	GenreID *int64
}, error) {
	rows, err := s.db.Query(ctx,
		`SELECT kind, label, genre_id FROM home_rows WHERE enabled ORDER BY position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []struct {
		Kind    string
		Label   string
		GenreID *int64
	}
	for rows.Next() {
		var r struct {
			Kind    string
			Label   string
			GenreID *int64
		}
		if err := rows.Scan(&r.Kind, &r.Label, &r.GenreID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type BrowseFilter struct {
	Kind     string
	Genre    string
	Query    string
	Sort     string // added | name | year
	Page     int
	PageSize int
}

var browseSorts = map[string]string{
	"":      "t.added_at DESC",
	"added": "t.added_at DESC",
	"name":  "t.sort_name ASC, t.name ASC",
	"year":  "t.year DESC NULLS LAST",
}

func (s *Store) BrowseTitles(ctx context.Context, f BrowseFilter) ([]CardItem, int, error) {
	orderBy, ok := browseSorts[f.Sort]
	if !ok {
		return nil, 0, fmt.Errorf("invalid sort %q", f.Sort)
	}
	if f.PageSize <= 0 || f.PageSize > 100 {
		f.PageSize = 48
	}
	if f.Page < 1 {
		f.Page = 1
	}

	where := `WHERE t.status = 'published'
		AND ($1 = '' OR t.kind = $1)
		AND ($2 = '' OR t.name ILIKE '%' || $2 || '%')
		AND ($3 = '' OR EXISTS (
			SELECT 1 FROM title_genres tg JOIN genres g ON g.id = tg.genre_id
			WHERE tg.title_id = t.id AND lower(g.name) = lower($3)))`

	var total int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM titles t `+where,
		f.Kind, f.Query, f.Genre).Scan(&total); err != nil {
		return nil, 0, err
	}

	items, err := s.scanCards(ctx, cardSelect+` `+where+`
		ORDER BY `+orderBy+` LIMIT $4 OFFSET $5`,
		f.Kind, f.Query, f.Genre, f.PageSize, (f.Page-1)*f.PageSize)
	return items, total, err
}

type SearchResults struct {
	Titles  []CardItem  `json:"titles"`
	Artists []SearchHit `json:"artists"`
	Albums  []SearchHit `json:"albums"`
	Tracks  []SearchHit `json:"tracks"`
}

type SearchHit struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Subtitle string `json:"subtitle"`
}

func (s *Store) Search(ctx context.Context, q string, limit int, includeMusic bool) (*SearchResults, error) {
	res := &SearchResults{Titles: []CardItem{}, Artists: []SearchHit{}, Albums: []SearchHit{}, Tracks: []SearchHit{}}
	if q == "" {
		return res, nil
	}

	var err error
	res.Titles, err = s.scanCards(ctx, cardSelect+`
		WHERE t.status = 'published' AND t.name ILIKE '%' || $1 || '%'
		ORDER BY similarity(t.name, $1) DESC LIMIT $2`, q, limit)
	if err != nil {
		return nil, err
	}
	if !includeMusic {
		return res, nil
	}

	collect := func(query string) ([]SearchHit, error) {
		rows, err := s.db.Query(ctx, query, q, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		hits := []SearchHit{}
		for rows.Next() {
			var h SearchHit
			if err := rows.Scan(&h.ID, &h.Name, &h.Subtitle); err != nil {
				return nil, err
			}
			hits = append(hits, h)
		}
		return hits, rows.Err()
	}

	if res.Artists, err = collect(
		`SELECT id, name, '' FROM artists WHERE name ILIKE '%' || $1 || '%'
		 ORDER BY similarity(name, $1) DESC LIMIT $2`); err != nil {
		return nil, err
	}
	if res.Albums, err = collect(
		`SELECT al.id, al.name, ar.name FROM albums al JOIN artists ar ON ar.id = al.artist_id
		 WHERE al.status = 'published' AND al.name ILIKE '%' || $1 || '%'
		 ORDER BY similarity(al.name, $1) DESC LIMIT $2`); err != nil {
		return nil, err
	}
	if res.Tracks, err = collect(
		`SELECT t.id, t.name, ar.name || ' · ' || al.name
		 FROM tracks t JOIN albums al ON al.id = t.album_id JOIN artists ar ON ar.id = al.artist_id
		 WHERE al.status = 'published' AND t.name ILIKE '%' || $1 || '%'
		 ORDER BY similarity(t.name, $1) DESC LIMIT $2`); err != nil {
		return nil, err
	}
	return res, nil
}

// PrimaryMediaFileForTitle returns the playable file for a movie title.
func (s *Store) PrimaryMediaFileForTitle(ctx context.Context, titleID string) (*media.MediaFile, error) {
	return media.ScanMediaFile(s.db.QueryRow(ctx,
		`SELECT `+media.MediaFileCols+` FROM media_files
		 WHERE title_id = $1 ORDER BY height DESC, id LIMIT 1`, titleID))
}

func (s *Store) PrimaryMediaFileForEpisode(ctx context.Context, episodeID string) (*media.MediaFile, error) {
	return media.ScanMediaFile(s.db.QueryRow(ctx,
		`SELECT `+media.MediaFileCols+` FROM media_files
		 WHERE episode_id = $1 ORDER BY height DESC, id LIMIT 1`, episodeID))
}

type EpisodeRef struct {
	EpisodeID     string `json:"episodeId"`
	SeasonNumber  int    `json:"seasonNumber"`
	EpisodeNumber int    `json:"episodeNumber"`
	Name          string `json:"name"`
	TitleID       string `json:"titleId"`
	TitleName     string `json:"titleName"`
	TitleSlug     string `json:"titleSlug"`
}

func (s *Store) EpisodeRef(ctx context.Context, episodeID string) (*EpisodeRef, error) {
	var ref EpisodeRef
	err := s.db.QueryRow(ctx,
		`SELECT e.id, se.season_number, e.episode_number, e.name, t.id, t.name, t.slug
		 FROM episodes e
		 JOIN seasons se ON se.id = e.season_id
		 JOIN titles t ON t.id = se.title_id
		 WHERE e.id = $1`, episodeID).
		Scan(&ref.EpisodeID, &ref.SeasonNumber, &ref.EpisodeNumber, &ref.Name, &ref.TitleID, &ref.TitleName, &ref.TitleSlug)
	if err != nil {
		return nil, err
	}
	return &ref, nil
}

type SeriesEpisode struct {
	EpisodeID     string  `json:"episodeId"`
	SeasonNumber  int     `json:"seasonNumber"`
	EpisodeNumber int     `json:"episodeNumber"`
	Name          string  `json:"name"`
	ThumbID       *string `json:"thumbId"`
	ThumbVer      int64   `json:"thumbVer,omitempty"`
}

// PlayableEpisodes lists a series' episodes that have a media file, for the
// in-player episode switcher (with each episode's still thumbnail).
func (s *Store) PlayableEpisodes(ctx context.Context, titleID string) ([]SeriesEpisode, error) {
	rows, err := s.db.Query(ctx,
		`SELECT e.id, se.season_number, e.episode_number, e.name,
			(SELECT a.id FROM artwork a WHERE a.owner_kind = 'episode' AND a.owner_id = e.id::text AND a.kind = 'thumb'),
			COALESCE((SELECT extract(epoch FROM a.created_at)::bigint FROM artwork a WHERE a.owner_kind = 'episode' AND a.owner_id = e.id::text AND a.kind = 'thumb'), 0)
		 FROM episodes e
		 JOIN seasons se ON se.id = e.season_id
		 WHERE se.title_id = $1
			AND EXISTS (SELECT 1 FROM media_files mf WHERE mf.episode_id = e.id)
		 ORDER BY se.season_number, e.episode_number`, titleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SeriesEpisode{}
	for rows.Next() {
		var e SeriesEpisode
		if err := rows.Scan(&e.EpisodeID, &e.SeasonNumber, &e.EpisodeNumber, &e.Name, &e.ThumbID, &e.ThumbVer); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// NextEpisode finds the episode that follows (same season, then next season).
func (s *Store) NextEpisode(ctx context.Context, episodeID string) (*EpisodeRef, error) {
	var ref EpisodeRef
	err := s.db.QueryRow(ctx,
		`WITH cur AS (
			SELECT e.id, e.episode_number, se.season_number, se.title_id
			FROM episodes e JOIN seasons se ON se.id = e.season_id
			WHERE e.id = $1
		)
		SELECT e.id, se.season_number, e.episode_number, e.name, t.id, t.name, t.slug
		FROM episodes e
		JOIN seasons se ON se.id = e.season_id
		JOIN titles t ON t.id = se.title_id, cur
		WHERE se.title_id = cur.title_id
			AND (se.season_number, e.episode_number) > (cur.season_number, cur.episode_number)
		ORDER BY se.season_number, e.episode_number
		LIMIT 1`, episodeID).
		Scan(&ref.EpisodeID, &ref.SeasonNumber, &ref.EpisodeNumber, &ref.Name, &ref.TitleID, &ref.TitleName, &ref.TitleSlug)
	if err != nil {
		return nil, nil // no next episode is not an error
	}
	return &ref, nil
}

// MediaFileIDsForTitles powers the bulk re-scan action.
func (s *Store) MediaFileIDsForTitles(ctx context.Context, titleIDs []string) ([]string, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id FROM media_files
		 WHERE title_id = ANY($1::uuid[])
			OR episode_id IN (
				SELECT e.id FROM episodes e
				JOIN seasons se ON se.id = e.season_id
				WHERE se.title_id = ANY($1::uuid[]))`, titleIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SubtitlesForMediaFiles bulk-loads subtitles keyed by media file id.
func (s *Store) SubtitlesForMediaFiles(ctx context.Context, mediaFileIDs []string) (map[string][]media.Subtitle, error) {
	out := map[string][]media.Subtitle{}
	if len(mediaFileIDs) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx,
		`SELECT `+media.SubtitleCols+` FROM subtitles
		 WHERE media_file_id = ANY($1::uuid[]) ORDER BY lang, id`, mediaFileIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		sub, err := media.ScanSubtitle(rows)
		if err != nil {
			return nil, err
		}
		out[sub.MediaFileID] = append(out[sub.MediaFileID], *sub)
	}
	return out, rows.Err()
}

func (s *Store) MediaFilesForTitle(ctx context.Context, titleID string) ([]media.MediaFile, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+media.MediaFileCols+` FROM media_files
		 WHERE title_id = $1
			OR episode_id IN (SELECT e.id FROM episodes e JOIN seasons se ON se.id = e.season_id WHERE se.title_id = $1)
		 ORDER BY path`, titleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	files := []media.MediaFile{}
	for rows.Next() {
		m, err := media.ScanMediaFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, *m)
	}
	return files, rows.Err()
}
