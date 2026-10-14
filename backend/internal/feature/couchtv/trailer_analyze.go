package couchtv

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/media"
)

// TrailerVersion is the scene picker's version: titles an older one looked at are picked again
// (an admin's scenes are kept). A title whose pick is empty is picked again as well once the
// subtitles it read are not the ones it has (Analyzer.subtitlesGained). It is apart from AnalyzeVersion, which measures the loudness of
// whole films.
const TrailerVersion = 1

// What a title's trailer is cut from: a movie's one file, or episodes spread over a series.
const trailerEpisodes = 3

// trailerSources are the files a title's trailer is cut from, with the dialogue and the subtitles
// read of each: a movie's one file, or episodes spread over a series.
func (a *Analyzer) trailerSources(ctx context.Context, t *titleRow, files []titleFile) []pickFile {
	picks := files[:min(1, len(files))]
	if t.TitleKind == "series" {
		picks = spread(files, trailerEpisodes)
	}
	var sources []pickFile
	for _, f := range picks {
		if !f.Readable {
			continue
		}
		src, err := a.Store.pickFile(ctx, a.DataDir, f)
		if err != nil {
			if ctx.Err() == nil {
				slog.Warn("couchtv: no trailer source", "mediaFileId", f.ID, "err", err)
			}
			continue
		}
		sources = append(sources, src)
	}
	return sources
}

// subtitleKey identifies the subtitle files the sources hold: a pick made from other ones (or
// from none) may come out differently.
func subtitleKey(sources []pickFile) string {
	var paths []string
	for _, s := range sources {
		paths = append(paths, s.SubPaths...)
	}
	slices.Sort(paths)
	return strings.Join(paths, ",")
}

// trailer picks the scenes of a title's trailer (D43) from its files: the subtitles and the
// sound of the original-language track. An empty result says there is none to make (no
// subtitles in the dialogue's language that follow the sound, or too few scenes), and is stored
// as the picker's answer, with the key of the subtitles it read.
func (a *Analyzer) trailer(ctx context.Context, t *titleRow, files []titleFile) (scenes []CouchTvTrailerScene, subs string) {
	sources := a.trailerSources(ctx, t, files)
	subs = subtitleKey(sources)
	if len(sources) == 0 {
		return []CouchTvTrailerScene{}, subs
	}
	name, _ := a.Store.titleName(ctx, t.TitleID)
	p := &scenePicker{FFmpeg: a.FFmpegPath, Risk: a.Risk, Keywords: titleKeywords(name)}
	chosen, err := p.pickTitle(ctx, sources)
	if err != nil {
		if ctx.Err() == nil {
			slog.Warn("couchtv: no trailer", "titleId", t.TitleID, "err", err)
		}
		return []CouchTvTrailerScene{}, subs
	}
	if !enoughScenes(chosen) {
		return []CouchTvTrailerScene{}, subs
	}
	scenes = make([]CouchTvTrailerScene, len(chosen))
	for i, c := range chosen {
		scenes[i] = storedScene(c, sources)
	}
	return scenes, subs
}

// trailerScenesMin is the fewest scenes a trailer is made of (the spec's minimum).
const trailerScenesMin = 3

// pickedSecondsMin is the least a picked trailer's scenes last together: under it the template
// would stretch cards and a montage around a few lines (Futurama's loud cartoon sound left four
// scenes of 16 s), so the title keeps its ordinary promos. An admin's own cut is theirs to make.
const pickedSecondsMin = 25.0

// enoughScenes says whether a pick is worth a trailer: enough scenes, long enough together.
func enoughScenes(chosen []sceneCandidate) bool {
	total := 0.0
	for _, c := range chosen {
		total += c.End - c.Start
	}
	return len(chosen) >= trailerScenesMin && total >= pickedSecondsMin
}

// titleKeywords are the words of a title's name that its trailer's lines are likely to say:
// the words of four letters or more that are not articles or numbers.
func titleKeywords(name string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(name, func(r rune) bool { return !unicode.IsLetter(r) }) {
		if len(w) >= 4 && !stopWords[strings.ToLower(w)] && !slices.Contains(out, w) {
			out = append(out, w)
		}
	}
	return out
}

// pickFile reads what the picker needs of a file: where its dialogue is heard and the subtitle
// files in the language it speaks.
func (s *Store) pickFile(ctx context.Context, dataDir string, f titleFile) (pickFile, error) {
	track, err := s.dialogue(ctx, f)
	if err != nil {
		return pickFile{}, err
	}
	paths, err := s.subtitleFiles(ctx, f.ID, track.Lang)
	if err != nil {
		return pickFile{}, err
	}
	subs := make([]string, len(paths))
	for i, p := range paths {
		subs[i] = filepath.Join(dataDir, p)
	}
	return pickFile{ID: f.ID, Path: f.Path, Duration: f.Duration, Audio: track, Subs: subs, SubPaths: paths}, nil
}

// audioFact is an audio stream of a file as the prober recorded it.
type audioFact struct {
	Index    int
	Lang     string
	Label    string
	Channels int
	Default  bool
}

var auxiliaryAudio = regexp.MustCompile(`(?i)commentary|description|audio desc|descriptive|karaoke`)

// originalLanguage is the language a title's dialogue is taken in: titles do not record
// theirs, and English is what the library's films and series are made in where it has it.
const originalLanguage = "en"

// pickStream chooses a file's stream for its dialogue: one in the original language (the
// default among several, then the most channels, then the first), else the file's default.
// The stream's language is kept as the file says it; the second result says whether it is the
// original's.
func pickStream(streams []audioFact) (audioFact, bool, bool) {
	var english, usable []audioFact
	for _, st := range streams {
		if auxiliaryAudio.MatchString(st.Label) {
			continue
		}
		usable = append(usable, st)
		if media.SameLanguage(st.Lang, originalLanguage) {
			english = append(english, st)
		}
	}
	order := func(list []audioFact) audioFact {
		best := list[0]
		for _, st := range list[1:] {
			switch {
			case st.Default != best.Default:
				if st.Default {
					best = st
				}
			case st.Channels > best.Channels:
				best = st
			}
		}
		return best
	}
	switch {
	case len(english) > 0:
		return order(english), true, true
	case len(usable) > 0:
		return order(usable), false, true
	}
	return audioFact{}, false, false
}

// dialogue finds where a file's dialogue, in the original language, is heard: an English
// stream of the file, else an alternate file of the same movie or episode in English (model B:
// a separate file per language, on the same timeline), else the file's default track.
func (s *Store) dialogue(ctx context.Context, f titleFile) (dialogueTrack, error) {
	rows, err := s.q.Query(ctx, `SELECT stream_index, lang, label, channels, is_default FROM audio_streams
		WHERE media_file_id = $1 ORDER BY stream_index`, f.ID)
	if err != nil {
		return dialogueTrack{}, err
	}
	streams, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (audioFact, error) {
		var a audioFact
		err := row.Scan(&a.Index, &a.Lang, &a.Label, &a.Channels, &a.Default)
		return a, err
	})
	if err != nil {
		return dialogueTrack{}, err
	}
	stream, original, ok := pickStream(streams)
	own := dialogueTrack{FileID: f.ID, Path: f.Path, Lang: "und"}
	if ok {
		own.Stream, own.Lang = &stream.Index, media.BCP47(stream.Lang)
	}
	if original {
		return own, nil
	}
	alt, err := s.alternateAudio(ctx, f.ID, originalLanguage)
	if err != nil || alt == nil {
		return own, err
	}
	return *alt, nil
}

// alternateAudio is the file of the same movie or episode that carries the given language
// (model B), nil when there is none.
func (s *Store) alternateAudio(ctx context.Context, fileID, lang string) (*dialogueTrack, error) {
	rows, err := s.q.Query(ctx, `
		SELECT alt.id, alt.audio_lang, l.path, alt.path
		FROM media_files mf
		JOIN media_files alt ON alt.id <> mf.id AND alt.audio_role = 'audio_alt' AND alt.source_deleted_at IS NULL
		  AND (mf.title_id IS NOT NULL AND alt.title_id = mf.title_id OR mf.episode_id IS NOT NULL AND alt.episode_id = mf.episode_id)
		JOIN libraries l ON l.id = alt.library_id
		WHERE mf.id = $1 ORDER BY alt.id`, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, audioLang, lib, rel string
		if err := rows.Scan(&id, &audioLang, &lib, &rel); err != nil {
			return nil, err
		}
		if media.SameLanguage(audioLang, lang) {
			return &dialogueTrack{FileID: id, Path: filepath.Join(lib, rel), Lang: media.BCP47(audioLang)}, nil
		}
	}
	return nil, rows.Err()
}

// subtitleFiles are the paths (relative to DATA_DIR) of a file's subtitle tracks in a language,
// not the forced ones, which carry the foreign lines only. A language the track does not name
// has none: subtitles must be known to be in the language that is spoken.
func (s *Store) subtitleFiles(ctx context.Context, fileID, lang string) ([]string, error) {
	if media.BCP47(lang) == "und" {
		return nil, nil
	}
	rows, err := s.q.Query(ctx, `SELECT lang, path FROM subtitles WHERE media_file_id = $1 AND NOT forced ORDER BY created_at`, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var subLang, path string
		if err := rows.Scan(&subLang, &path); err != nil {
			return nil, err
		}
		if media.SameLanguage(subLang, lang) {
			out = append(out, path)
		}
	}
	return out, rows.Err()
}

// titleName is the title's English name (the base text when the base language is English).
func (s *Store) titleName(ctx context.Context, titleID string) (string, error) {
	var name string
	err := s.q.QueryRow(ctx, `SELECT `+englishTitle+` FROM titles t WHERE t.id = $1`, titleID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return name, err
}
