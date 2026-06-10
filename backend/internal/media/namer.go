package media

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

type ParsedVideo struct {
	IsEpisode bool
	ShowName  string
	Season    int
	Episode   int
	Name      string // movie name, or best-effort episode title
	Year      *int
}

var (
	episodeRe = regexp.MustCompile(`(?i)\bS(\d{1,2})[\s._-]*E(\d{1,3})\b`)
	yearRe    = regexp.MustCompile(`^(.*?)[\s.]*\((\d{4})\)`)
	junkRe    = regexp.MustCompile(`(?i)[\s._-]*(1080p|2160p|720p|480p|4k|uhd|bluray|web-?dl|webrip|hdtv|x264|x265|h\.?264|h\.?265|hevc|aac|dts|remux|hdr10?|dv)\b.*$`)
)

// ParseVideoPath infers what a video file is from its library-relative path.
// Series: "Show Name/Season 01/Show Name S01E02 episode.mkv"
// Movie:  "Movie Name (2023)/Movie Name (2023).mkv"
func ParseVideoPath(relPath string) ParsedVideo {
	relPath = strings.ReplaceAll(relPath, "\\", "/")
	// Treat underscores as separators. They are word characters to regexp's
	// \b, so "S01E01_Title" would otherwise hide the SxxExx token. Replacing
	// '_' with a space is length-preserving, so substring indices stay valid.
	relPath = strings.ReplaceAll(relPath, "_", " ")
	dir, file := path.Split(relPath)
	base := strings.TrimSuffix(file, path.Ext(file))

	if m := episodeRe.FindStringSubmatchIndex(base); m != nil {
		season, _ := strconv.Atoi(base[m[2]:m[3]])
		episode, _ := strconv.Atoi(base[m[4]:m[5]])

		raw := topLevelDir(dir)
		if raw == "" {
			raw = base[:m[0]]
		}
		show, year := splitNameYear(raw)

		return ParsedVideo{
			IsEpisode: true,
			ShowName:  show,
			Season:    season,
			Episode:   episode,
			Name:      cleanName(base[m[1]:]),
			Year:      year,
		}
	}

	name, year := splitNameYear(base)
	if year == nil {
		// fall back to the folder name: "Movie (2024)/movie.2160p.mkv"
		if raw := topLevelDir(dir); raw != "" {
			if parentName, parentYear := splitNameYear(raw); parentYear != nil {
				name, year = parentName, parentYear
			}
		}
	}
	return ParsedVideo{Name: name, Year: year}
}

func topLevelDir(dir string) string {
	dir = strings.Trim(dir, "/")
	if dir == "" {
		return ""
	}
	return strings.Split(dir, "/")[0]
}

func splitNameYear(s string) (string, *int) {
	if m := yearRe.FindStringSubmatch(s); m != nil {
		if y, err := strconv.Atoi(m[2]); err == nil {
			return cleanName(m[1]), &y
		}
	}
	return cleanName(s), nil
}

func cleanName(s string) string {
	s = junkRe.ReplaceAllString(s, "")
	s = strings.NewReplacer(".", " ", "_", " ").Replace(s)
	s = strings.Trim(s, " -")
	return strings.Join(strings.Fields(s), " ")
}
