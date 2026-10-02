package metadata

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"

	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/catalog"
)

// Logo is one of the logos TMDB has for a title.
type Logo struct {
	FilePath    string  `json:"file_path"`
	Lang        string  `json:"iso_639_1"` // empty for a logo not tied to a language
	VoteAverage float64 `json:"vote_average"`
	VoteCount   int     `json:"vote_count"`
}

// Logos lists a title's logos in langs plus the ones not tied to a language.
// kind is "movie" or "series".
func (c *Client) Logos(ctx context.Context, kind string, tmdbID int, langs []string) ([]Logo, error) {
	endpoint := fmt.Sprintf("/movie/%d/images", tmdbID)
	if kind == "series" {
		endpoint = fmt.Sprintf("/tv/%d/images", tmdbID)
	}
	params := url.Values{}
	params.Set("include_image_language", strings.Join(append(slices.Clone(langs), "null"), ","))
	var raw struct {
		Logos []Logo `json:"logos"`
	}
	if err := c.get(ctx, endpoint, params, &raw); err != nil {
		return nil, err
	}
	return raw.Logos, nil
}

// bestLogo picks a content language's logo: the best-voted one in lang, else
// the best-voted one not tied to a language. Only PNGs qualify; TMDB also
// serves SVG logos, which not every client can draw.
func bestLogo(logos []Logo, lang string) *Logo {
	for _, want := range []string{lang, ""} {
		var best *Logo
		for i := range logos {
			l := &logos[i]
			if l.Lang != want || !strings.EqualFold(path.Ext(l.FilePath), ".png") {
				continue
			}
			if best == nil || l.VoteAverage > best.VoteAverage ||
				(l.VoteAverage == best.VoteAverage && l.VoteCount > best.VoteCount) {
				best = l
			}
		}
		if best != nil {
			return best
		}
	}
	return nil
}

// tmdbLang is the TMDB language of a content language: a legacy title without
// any gets TMDB's default, English.
func tmdbLang(lang string) string {
	if lang == "" {
		return "en"
	}
	return lang
}

// applyLogos stores TMDB's logo for each content language in langs ("" for a
// legacy title) as the title's logo in that language. With replace (metadata
// apply) it also drops the TMDB logos it did not store, so linking the title
// to another TMDB entry leaves none of the old one's behind; uploaded logos are
// only ever overwritten. Without it (episode import) it fills just the
// languages that have no logo yet.
func applyLogos(ctx context.Context, client *Client, art *artwork.Service, title *catalog.Title, tmdbID int, langs []string, replace bool) error {
	existing, err := art.Store.ArtworkFor(ctx, "title", title.ID)
	if err != nil {
		return err
	}
	have := map[string]artwork.Artwork{}
	for _, a := range existing {
		if a.Kind == "logo" {
			lang := ""
			if a.Lang != nil {
				lang = *a.Lang
			}
			have[lang] = a
		}
	}
	wanted := langs
	if !replace {
		wanted = slices.DeleteFunc(slices.Clone(langs), func(lang string) bool {
			_, ok := have[lang]
			return ok
		})
		if len(wanted) == 0 {
			return nil
		}
	}

	query := make([]string, len(wanted))
	for i, lang := range wanted {
		query[i] = tmdbLang(lang)
	}
	logos, err := client.Logos(ctx, title.Kind, tmdbID, query)
	if err != nil {
		return err
	}
	stored := map[string]bool{}
	// languages falling back to the same language-neutral logo share a download
	downloads := map[string][]byte{}
	for _, lang := range wanted {
		logo := bestLogo(logos, tmdbLang(lang))
		if logo == nil {
			continue
		}
		data, ok := downloads[logo.FilePath]
		if !ok {
			if data, err = client.DownloadImage(ctx, logo.FilePath); err != nil {
				return err
			}
			downloads[logo.FilePath] = data
		}
		if _, err := art.SaveBytes(ctx, "title", title.ID, "logo", lang, ".png", data, "tmdb"); err != nil {
			return err
		}
		stored[lang] = true
	}
	if replace {
		for lang, a := range have {
			if !stored[lang] && a.Source == "tmdb" {
				if err := art.Delete(ctx, a.ID); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
