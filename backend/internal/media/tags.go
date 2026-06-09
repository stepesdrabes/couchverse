package media

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/dhowden/tag"
)

type AudioTags struct {
	Title       string
	Artist      string // album artist preferred
	TrackArtist string // set when it differs from the album artist
	Album       string
	Year        int
	Genre       string
	Track       int
	Disc        int
	Picture     []byte // embedded cover art (jpeg/png)
	PictureExt  string
}

var leadingTrackNumRe = regexp.MustCompile(`^(\d{1,3})[\s._-]+`)

// ReadAudioTags reads ID3/FLAC/MP4 tags, falling back to filename heuristics
// so untagged files still land somewhere sensible.
func ReadAudioTags(path string) AudioTags {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	tags := AudioTags{Title: cleanName(base)}
	if m := leadingTrackNumRe.FindStringSubmatch(base); m != nil {
		tags.Track, _ = strconv.Atoi(m[1])
		tags.Title = cleanName(base[len(m[0]):])
	}

	f, err := os.Open(path)
	if err != nil {
		return withTagFallbacks(tags)
	}
	defer f.Close()

	meta, err := tag.ReadFrom(f)
	if err != nil {
		return withTagFallbacks(tags)
	}

	if t := strings.TrimSpace(meta.Title()); t != "" {
		tags.Title = t
	}
	albumArtist := strings.TrimSpace(meta.AlbumArtist())
	artist := strings.TrimSpace(meta.Artist())
	switch {
	case albumArtist != "":
		tags.Artist = albumArtist
		if artist != "" && artist != albumArtist {
			tags.TrackArtist = artist
		}
	case artist != "":
		tags.Artist = artist
	}
	if a := strings.TrimSpace(meta.Album()); a != "" {
		tags.Album = a
	}
	tags.Year = meta.Year()
	tags.Genre = strings.TrimSpace(meta.Genre())
	if n, _ := meta.Track(); n > 0 {
		tags.Track = n
	}
	if d, _ := meta.Disc(); d > 0 {
		tags.Disc = d
	}
	if pic := meta.Picture(); pic != nil && len(pic.Data) > 0 {
		tags.Picture = pic.Data
		tags.PictureExt = pictureExt(pic.MIMEType)
	}
	return withTagFallbacks(tags)
}

func withTagFallbacks(t AudioTags) AudioTags {
	if t.Artist == "" {
		t.Artist = "Unknown Artist"
	}
	if t.Album == "" {
		t.Album = "Unknown Album"
	}
	if t.Disc == 0 {
		t.Disc = 1
	}
	return t
}

func pictureExt(mime string) string {
	if strings.Contains(mime, "png") {
		return ".png"
	}
	return ".jpg"
}
