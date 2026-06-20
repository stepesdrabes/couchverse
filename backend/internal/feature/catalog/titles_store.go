package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"couchverse/internal/db"
	"couchverse/internal/httpx"
	"couchverse/internal/slug"
)

type Title struct {
	ID                  string          `json:"id"`
	Slug                string          `json:"slug"`
	Kind                string          `json:"kind"`
	Name                string          `json:"name"`
	SortName            string          `json:"sortName"`
	Overview            string          `json:"overview"`
	Year                *int            `json:"year"`
	ReleaseDate         *time.Time      `json:"releaseDate"`
	ContentRating       string          `json:"contentRating"`
	RuntimeMinutes      *int            `json:"runtimeMinutes"`
	Status              string          `json:"status"`
	TmdbID              *int            `json:"tmdbId"`
	AddedAt             time.Time       `json:"addedAt"`
	UpdatedAt           time.Time       `json:"updatedAt"`
	Genres              []string        `json:"genres"`
	GenreLabels         []string        `json:"genreLabels"`
	MetadataLanguages   []string        `json:"metadataLanguages"`
	AllowRandomPlayback bool            `json:"allowRandomPlayback"`
	Translations        json.RawMessage `json:"-"`
}

const titleCols = `id, slug, kind, name, sort_name, overview, year, release_date, content_rating,
	runtime_minutes, status, tmdb_id, added_at, updated_at, translations, metadata_languages, allow_random_playback`

func scanTitle(ctx context.Context, row pgx.Row) (*Title, error) {
	var t Title
	err := row.Scan(&t.ID, &t.Slug, &t.Kind, &t.Name, &t.SortName, &t.Overview, &t.Year, &t.ReleaseDate,
		&t.ContentRating, &t.RuntimeMinutes, &t.Status, &t.TmdbID, &t.AddedAt, &t.UpdatedAt,
		&t.Translations, &t.MetadataLanguages, &t.AllowRandomPlayback)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.Genres = []string{}
	t.GenreLabels = []string{}
	if t.MetadataLanguages == nil {
		t.MetadataLanguages = []string{}
	}
	localize(ctx, t.Translations, &t.Name, &t.Overview)
	return &t, nil
}

func (s *Store) TitleByID(ctx context.Context, id string) (*Title, error) {
	t, err := scanTitle(ctx, s.db.QueryRow(ctx, `SELECT `+titleCols+` FROM titles WHERE id = $1`, id))
	if err != nil {
		return nil, err
	}
	if err := s.loadTitleGenres(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Store) TitleBySlug(ctx context.Context, slug string) (*Title, error) {
	t, err := scanTitle(ctx, s.db.QueryRow(ctx, `SELECT `+titleCols+` FROM titles WHERE slug = $1`, slug))
	if err != nil {
		return nil, err
	}
	if err := s.loadTitleGenres(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Store) loadTitleGenres(ctx context.Context, t *Title) error {
	rows, err := s.db.Query(ctx,
		`SELECT g.name FROM genres g JOIN title_genres tg ON tg.genre_id = g.id
		 WHERE tg.title_id = $1 ORDER BY g.name`, t.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	lang := httpx.LangFrom(ctx)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		t.Genres = append(t.Genres, name)
		t.GenreLabels = append(t.GenreLabels, genreLabel(name, lang))
	}
	return rows.Err()
}

// uniqueSlug returns base or the first free base-N suffix.
func (s *Store) uniqueSlug(ctx context.Context, base string) (string, error) {
	rows, err := s.db.Query(ctx,
		`SELECT slug FROM titles WHERE slug = $1 OR slug LIKE $1 || '-%'`, base)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	taken := map[string]bool{}
	for rows.Next() {
		var sl string
		if err := rows.Scan(&sl); err != nil {
			return "", err
		}
		taken[sl] = true
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if !taken[base] {
		return base, nil
	}
	for n := 2; ; n++ {
		candidate := base + "-" + strconv.Itoa(n)
		if !taken[candidate] {
			return candidate, nil
		}
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

type TitleInput struct {
	Kind              string   `json:"kind"`
	Name              string   `json:"name"`
	Overview          string   `json:"overview"`
	Year              *int     `json:"year"`
	ContentRating     string   `json:"contentRating"`
	RuntimeMinutes    *int     `json:"runtimeMinutes"`
	Genres            []string `json:"genres"`
	MetadataLanguages []string `json:"metadataLanguages"`
}

func (s *Store) CreateTitle(ctx context.Context, in TitleInput) (*Title, error) {
	langs := in.MetadataLanguages
	if langs == nil {
		langs = []string{}
	}
	var t *Title
	for attempt := 0; ; attempt++ {
		sl, err := s.uniqueSlug(ctx, slug.Make(in.Name, in.Year))
		if err != nil {
			return nil, err
		}
		t, err = scanTitle(ctx, s.db.QueryRow(ctx,
			`INSERT INTO titles (kind, name, slug, sort_name, overview, year, content_rating, runtime_minutes, metadata_languages)
			 VALUES ($1, $2, $3, $2, $4, $5, $6, $7, $8)
			 RETURNING `+titleCols,
			in.Kind, in.Name, sl, in.Overview, in.Year, in.ContentRating, in.RuntimeMinutes, langs))
		if err == nil {
			break
		}
		if attempt == 0 && isUniqueViolation(err) {
			continue // slug raced another insert; recompute once
		}
		return nil, err
	}
	if len(in.Genres) > 0 {
		if err := s.SetTitleGenres(ctx, t.ID, in.Genres); err != nil {
			return nil, err
		}
		t.Genres = in.Genres
	}
	return t, nil
}

type TitleUpdate struct {
	Name                *string   `json:"name"`
	SortName            *string   `json:"sortName"`
	Overview            *string   `json:"overview"`
	Year                *int      `json:"year"`
	ContentRating       *string   `json:"contentRating"`
	RuntimeMinutes      *int      `json:"runtimeMinutes"`
	Status              *string   `json:"status"`
	TmdbID              *int      `json:"tmdbId"`
	Genres              *[]string `json:"genres"`
	MetadataLanguages   *[]string `json:"metadataLanguages"`
	AllowRandomPlayback *bool     `json:"allowRandomPlayback"`
}

func (s *Store) UpdateTitle(ctx context.Context, id string, up TitleUpdate) (*Title, error) {
	var mlangs any
	if up.MetadataLanguages != nil {
		mlangs = *up.MetadataLanguages
	}
	t, err := scanTitle(ctx, s.db.QueryRow(ctx,
		`UPDATE titles SET
			name = COALESCE($2, name),
			sort_name = COALESCE($3, sort_name),
			overview = COALESCE($4, overview),
			year = COALESCE($5, year),
			content_rating = COALESCE($6, content_rating),
			runtime_minutes = COALESCE($7, runtime_minutes),
			status = COALESCE($8, status),
			tmdb_id = COALESCE($9, tmdb_id),
			metadata_languages = COALESCE($10, metadata_languages),
			allow_random_playback = COALESCE($11, allow_random_playback),
			updated_at = now()
		 WHERE id = $1
		 RETURNING `+titleCols,
		id, up.Name, up.SortName, up.Overview, up.Year, up.ContentRating,
		up.RuntimeMinutes, up.Status, up.TmdbID, mlangs, up.AllowRandomPlayback))
	if err != nil {
		return nil, err
	}
	if up.Genres != nil {
		if err := s.SetTitleGenres(ctx, id, *up.Genres); err != nil {
			return nil, err
		}
	}
	if err := s.loadTitleGenres(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// SetTitleTranslation merges one language's TMDB text into the title's
// translations JSONB, leaving the base (default-language) columns untouched.
func (s *Store) SetTitleTranslation(ctx context.Context, id, lang, name, overview, tagline string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE titles SET translations = translations || jsonb_build_object($2::text,
			jsonb_build_object('name', $3::text, 'overview', $4::text, 'tagline', $5::text)),
			updated_at = now()
		 WHERE id = $1`, id, lang, name, overview, tagline)
	return err
}

// SetTitleTranslationText merges a manually-edited name/overview into one
// language's translation subtree, preserving other keys (e.g. a TMDB tagline).
func (s *Store) SetTitleTranslationText(ctx context.Context, id, lang, name, overview string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE titles SET translations = translations || jsonb_build_object($2::text,
			coalesce(translations->$2, '{}'::jsonb) || jsonb_build_object('name', $3::text, 'overview', $4::text)),
			updated_at = now()
		 WHERE id = $1`, id, lang, name, overview)
	return err
}

// ErrLastLanguage is returned when removing a title's only content language;
// a title must keep at least one.
var ErrLastLanguage = errors.New("cannot remove the last content language")

// RemoveContentLanguage drops one content language from a title: it deletes the
// language's translations from the title and every season/episode and removes it
// from metadata_languages. When the removed language was the base (first) one,
// the next language is promoted into the base columns so the title keeps a
// coherent default. The matching audio files and subtitles on disk are removed by
// the client through their own endpoints; this owns only the catalog side.
func (s *Store) RemoveContentLanguage(ctx context.Context, titleID, lang string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var langs []string
	err = tx.QueryRow(ctx,
		`SELECT metadata_languages FROM titles WHERE id = $1 FOR UPDATE`, titleID).Scan(&langs)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.ErrNotFound
	}
	if err != nil {
		return err
	}

	member := false
	for _, l := range langs {
		if l == lang {
			member = true
			break
		}
	}
	if !member {
		return db.ErrNotFound
	}
	if len(langs) <= 1 {
		return ErrLastLanguage
	}

	if langs[0] == lang {
		// removing the base language: promote the next one into the base columns,
		// then drop both subtrees (the base never had its own translation entry).
		promote := langs[1]
		if _, err := tx.Exec(ctx,
			`UPDATE titles SET
				name = COALESCE(NULLIF(translations->$2->>'name', ''), name),
				overview = COALESCE(translations->$2->>'overview', overview),
				translations = translations - $2 - $3,
				metadata_languages = array_remove(metadata_languages, $3),
				updated_at = now()
			 WHERE id = $1`, titleID, promote, lang); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE seasons SET
				name = COALESCE(NULLIF(translations->$2->>'name', ''), name),
				overview = COALESCE(translations->$2->>'overview', overview),
				translations = translations - $2 - $3
			 WHERE title_id = $1`, titleID, promote, lang); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE episodes e SET
				name = COALESCE(NULLIF(e.translations->$2->>'name', ''), e.name),
				overview = COALESCE(e.translations->$2->>'overview', e.overview),
				translations = e.translations - $2 - $3
			 FROM seasons s WHERE e.season_id = s.id AND s.title_id = $1`, titleID, promote, lang); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec(ctx,
			`UPDATE titles SET translations = translations - $2,
				metadata_languages = array_remove(metadata_languages, $2), updated_at = now()
			 WHERE id = $1`, titleID, lang); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE seasons SET translations = translations - $2 WHERE title_id = $1`, titleID, lang); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE episodes e SET translations = e.translations - $2
			 FROM seasons s WHERE e.season_id = s.id AND s.title_id = $1`, titleID, lang); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// RegenerateTitleSlug rebuilds the slug from name+year, keeping it unique.
func (s *Store) RegenerateTitleSlug(ctx context.Context, id string, name string, year *int) (string, error) {
	sl, err := s.uniqueSlug(ctx, slug.Make(name, year))
	if err != nil {
		return "", err
	}
	_, err = s.db.Exec(ctx,
		`UPDATE titles SET slug = $2, updated_at = now() WHERE id = $1`, id, sl)
	return sl, err
}

func (s *Store) SetTitleReleaseDate(ctx context.Context, id string, date string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE titles SET release_date = $2::date, updated_at = now() WHERE id = $1`, id, date)
	return err
}

// TitleBackdrop returns the title's backdrop artwork id, version token (unix
// seconds) and accent colour for the player banner, or zero values when absent.
func (s *Store) TitleBackdrop(ctx context.Context, titleID string) (id *string, ver int64, accent string, err error) {
	var aid string
	var created time.Time
	err = s.db.QueryRow(ctx,
		`SELECT id, created_at, accent FROM artwork WHERE owner_kind = 'title' AND owner_id = $1 AND kind = 'backdrop' LIMIT 1`,
		titleID).Scan(&aid, &created, &accent)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, "", nil
	}
	if err != nil {
		return nil, 0, "", err
	}
	return &aid, created.Unix(), accent, nil
}

func (s *Store) DeleteTitle(ctx context.Context, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM titles WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}

func (s *Store) SetTitlesStatus(ctx context.Context, ids []string, status string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE titles SET status = $2, updated_at = now() WHERE id = ANY($1::uuid[])`, ids, status)
	return err
}

func (s *Store) DeleteTitles(ctx context.Context, ids []string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM titles WHERE id = ANY($1::uuid[])`, ids)
	return err
}

// SetTitleGenres replaces a title's genres, creating unknown genre names.
func (s *Store) SetTitleGenres(ctx context.Context, titleID string, names []string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM title_genres WHERE title_id = $1`, titleID); err != nil {
		return err
	}
	for _, name := range names {
		var genreID int64
		err := tx.QueryRow(ctx,
			`INSERT INTO genres (name) VALUES ($1)
			 ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
			 RETURNING id`, name).Scan(&genreID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO title_genres (title_id, genre_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			titleID, genreID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

type Genre struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`  // English - stable identity for URLs/filters
	Label string `json:"label"` // resolved display label
}

func (s *Store) ListGenres(ctx context.Context) ([]Genre, error) {
	rows, err := s.db.Query(ctx, `SELECT id, name FROM genres ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	lang := httpx.LangFrom(ctx)
	genres := []Genre{}
	for rows.Next() {
		var g Genre
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return nil, err
		}
		g.Label = genreLabel(g.Name, lang)
		genres = append(genres, g)
	}
	return genres, rows.Err()
}

// FindOrCreateTitle matches scanner-discovered files to existing titles by
// case-insensitive name (and year when known), creating a draft otherwise.
func (s *Store) FindOrCreateTitle(ctx context.Context, kind, name string, year *int) (*Title, error) {
	t, err := scanTitle(ctx, s.db.QueryRow(ctx,
		`SELECT `+titleCols+` FROM titles
		 WHERE kind = $1 AND lower(name) = lower($2)
			AND ($3::int IS NULL OR year IS NULL OR year = $3)
		 ORDER BY (year = $3) DESC NULLS LAST
		 LIMIT 1`, kind, name, year))
	if err == nil {
		if loadErr := s.loadTitleGenres(ctx, t); loadErr != nil {
			return nil, loadErr
		}
		return t, nil
	}
	if !errors.Is(err, db.ErrNotFound) {
		return nil, err
	}
	for attempt := 0; ; attempt++ {
		sl, slugErr := s.uniqueSlug(ctx, slug.Make(name, year))
		if slugErr != nil {
			return nil, slugErr
		}
		t, err = scanTitle(ctx, s.db.QueryRow(ctx,
			`INSERT INTO titles (kind, name, slug, sort_name, year) VALUES ($1, $2, $3, $2, $4)
			 RETURNING `+titleCols, kind, name, sl, year))
		if err == nil {
			return t, nil
		}
		if attempt == 0 && isUniqueViolation(err) {
			continue
		}
		return nil, err
	}
}

func (s *Store) FindOrCreateSeason(ctx context.Context, titleID string, seasonNumber int) (string, error) {
	var id string
	err := s.db.QueryRow(ctx,
		`INSERT INTO seasons (title_id, season_number, name)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (title_id, season_number) DO UPDATE SET title_id = EXCLUDED.title_id
		 RETURNING id`, titleID, seasonNumber, fmt.Sprintf("Season %d", seasonNumber)).Scan(&id)
	return id, err
}

func (s *Store) FindOrCreateEpisode(ctx context.Context, seasonID string, episodeNumber int, name string) (string, error) {
	var id string
	err := s.db.QueryRow(ctx,
		`INSERT INTO episodes (season_id, episode_number, name)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (season_id, episode_number) DO UPDATE
			SET name = CASE WHEN episodes.name = '' THEN EXCLUDED.name ELSE episodes.name END
		 RETURNING id`, seasonID, episodeNumber, name).Scan(&id)
	return id, err
}

type LibraryFilter struct {
	Kind     string // movie | series | "" (all)
	Status   string
	Query    string
	Sort     string // added | name | year | size
	Page     int
	PageSize int
}

type LibraryRow struct {
	ID              string    `json:"id"`
	Slug            string    `json:"slug"`
	Kind            string    `json:"kind"`
	Name            string    `json:"name"`
	Year            *int      `json:"year"`
	Status          string    `json:"status"`
	SeasonCount     int       `json:"seasonCount"`
	EpisodeCount    int       `json:"episodeCount"`
	SizeBytes       int64     `json:"sizeBytes"`
	TranscodedBytes int64     `json:"transcodedBytes"`
	MaxHeight       int       `json:"maxHeight"`
	HDR             bool      `json:"hdr"`
	PosterID        *string   `json:"posterId"`
	BackdropID      *string   `json:"backdropId"`
	NeedsPrepare    bool      `json:"needsPrepare"`
	AddedAt         time.Time `json:"addedAt"`
}

var librarySorts = map[string]string{
	"":      "t.added_at DESC",
	"added": "t.added_at DESC",
	"name":  "t.sort_name ASC, t.name ASC",
	"year":  "t.year DESC NULLS LAST",
	"size":  "size_bytes DESC",
}

// ListLibrary powers the admin library table: titles with rolled-up file
// stats (total size, max resolution, HDR) and season/episode counts.
func (s *Store) ListLibrary(ctx context.Context, f LibraryFilter) ([]LibraryRow, int, error) {
	orderBy, ok := librarySorts[f.Sort]
	if !ok {
		return nil, 0, fmt.Errorf("invalid sort %q", f.Sort)
	}
	if f.PageSize <= 0 || f.PageSize > 200 {
		f.PageSize = 50
	}
	if f.Page < 1 {
		f.Page = 1
	}

	where := `WHERE ($1 = '' OR t.kind = $1)
		AND ($2 = '' OR t.status = $2)
		AND ($3 = '' OR t.name ILIKE '%' || $3 || '%')`

	var total int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM titles t `+where,
		f.Kind, f.Status, f.Query).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(ctx, `
		SELECT t.id, t.slug, t.kind, t.name, t.year,
			CASE WHEN EXISTS (
				SELECT 1 FROM transcode_variants tv
				WHERE tv.status IN ('queued', 'processing')
					AND tv.media_file_id IN (
						SELECT mf3.id FROM media_files mf3
						WHERE mf3.title_id = t.id OR mf3.episode_id IN (
							SELECT e3.id FROM episodes e3
							JOIN seasons s3 ON s3.id = e3.season_id
							WHERE s3.title_id = t.id))
			) THEN 'processing' ELSE t.status END AS status,
			t.added_at,
			count(DISTINCT se.id) AS season_count,
			count(DISTINCT e.id) AS episode_count,
			COALESCE(sum(mf.size_bytes), 0) AS size_bytes,
			(SELECT COALESCE(sum(tv3.size_bytes), 0)
			 FROM transcode_variants tv3
			 JOIN media_files mf4 ON mf4.id = tv3.media_file_id
			 WHERE tv3.status = 'ready' AND (mf4.title_id = t.id OR mf4.episode_id IN (
				SELECT e4.id FROM episodes e4
				JOIN seasons s4 ON s4.id = e4.season_id
				WHERE s4.title_id = t.id))
			) AS transcoded_bytes,
			COALESCE(max(mf.height), 0) AS max_height,
			COALESCE(bool_or(mf.video_range <> 'sdr'), false) AS hdr,
			(SELECT a.id FROM artwork a WHERE a.owner_kind = 'title' AND a.owner_id = t.id::text AND a.kind = 'poster') AS poster_id,
			(SELECT a.id FROM artwork a WHERE a.owner_kind = 'title' AND a.owner_id = t.id::text AND a.kind = 'backdrop') AS backdrop_id,
			COALESCE(bool_or(NOT mf.direct_play AND mf.scanned_at IS NOT NULL AND NOT EXISTS (
				SELECT 1 FROM transcode_variants tv2
				WHERE tv2.media_file_id = mf.id AND tv2.status IN ('queued', 'processing', 'ready')
			)), false) AS needs_prepare
		FROM titles t
		LEFT JOIN seasons se ON se.title_id = t.id
		LEFT JOIN episodes e ON e.season_id = se.id
		LEFT JOIN media_files mf ON mf.title_id = t.id OR mf.episode_id = e.id
		`+where+`
		GROUP BY t.id
		ORDER BY `+orderBy+`
		LIMIT $4 OFFSET $5`,
		f.Kind, f.Status, f.Query, f.PageSize, (f.Page-1)*f.PageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []LibraryRow{}
	for rows.Next() {
		var r LibraryRow
		if err := rows.Scan(&r.ID, &r.Slug, &r.Kind, &r.Name, &r.Year, &r.Status, &r.AddedAt,
			&r.SeasonCount, &r.EpisodeCount, &r.SizeBytes, &r.TranscodedBytes, &r.MaxHeight, &r.HDR,
			&r.PosterID, &r.BackdropID, &r.NeedsPrepare); err != nil {
			return nil, 0, err
		}
		items = append(items, r)
	}
	return items, total, rows.Err()
}

// StorageSegment is one bar of a title's storage breakdown: an episode (series)
// or a media file (movie), split into source and ready-transcode bytes.
type StorageSegment struct {
	Label           string `json:"label"`
	SourceBytes     int64  `json:"sourceBytes"`
	TranscodedBytes int64  `json:"transcodedBytes"`
}

// TitleStorageBreakdown is where a title's disk usage goes, plus the totals.
type TitleStorageBreakdown struct {
	Items           []StorageSegment `json:"items"`
	SourceBytes     int64            `json:"sourceBytes"`
	TranscodedBytes int64            `json:"transcodedBytes"`
}

// TitleStorage breaks a title's disk usage down per episode (series) or per media
// file (movie), each split into source size and ready-transcode size. Transcoded
// bytes use a per-file subquery so a file's multiple variants never fan out and
// double-count its source (same shape as ListLibrary). Rows with nothing on disk
// are skipped.
func (s *Store) TitleStorage(ctx context.Context, titleID, kind string) (*TitleStorageBreakdown, error) {
	out := &TitleStorageBreakdown{Items: []StorageSegment{}}

	var rows pgx.Rows
	var err error
	if kind == "series" {
		rows, err = s.db.Query(ctx, `
			SELECT s.season_number, e.episode_number,
				COALESCE(sum(mf.size_bytes), 0) AS source_bytes,
				COALESCE(sum((SELECT COALESCE(sum(tv.size_bytes), 0) FROM transcode_variants tv
					WHERE tv.media_file_id = mf.id AND tv.status = 'ready')), 0) AS transcoded_bytes
			FROM episodes e
			JOIN seasons s ON s.id = e.season_id
			LEFT JOIN media_files mf ON mf.episode_id = e.id
			WHERE s.title_id = $1
			GROUP BY e.id, s.season_number, e.episode_number
			ORDER BY s.season_number, e.episode_number`, titleID)
	} else {
		rows, err = s.db.Query(ctx, `
			SELECT mf.path,
				mf.size_bytes AS source_bytes,
				(SELECT COALESCE(sum(tv.size_bytes), 0) FROM transcode_variants tv
					WHERE tv.media_file_id = mf.id AND tv.status = 'ready') AS transcoded_bytes
			FROM media_files mf
			WHERE mf.title_id = $1
			ORDER BY mf.audio_role, mf.created_at`, titleID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var seg StorageSegment
		if kind == "series" {
			var season, episode int
			if err := rows.Scan(&season, &episode, &seg.SourceBytes, &seg.TranscodedBytes); err != nil {
				return nil, err
			}
			seg.Label = fmt.Sprintf("S%dE%d", season, episode)
		} else {
			var path string
			if err := rows.Scan(&path, &seg.SourceBytes, &seg.TranscodedBytes); err != nil {
				return nil, err
			}
			seg.Label = filepath.Base(path)
		}
		if seg.SourceBytes == 0 && seg.TranscodedBytes == 0 {
			continue
		}
		out.Items = append(out.Items, seg)
		out.SourceBytes += seg.SourceBytes
		out.TranscodedBytes += seg.TranscodedBytes
	}
	return out, rows.Err()
}
