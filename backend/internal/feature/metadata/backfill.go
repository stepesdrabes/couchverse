package metadata

import (
	"context"
	"fmt"
	"log/slog"

	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/catalog"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/settings"
)

const (
	// LogosJob is the logo backfill's job type.
	LogosJob = "fetch_logos"
	// LogosVersion is the logo backfill's version: raising it (a better pick, say) looks
	// every linked title up once more.
	LogosVersion = 1
	// logosPriority keeps the backfill behind anything an admin asks for.
	logosPriority = -10
	// logosFailures in a row end a run: TMDB is down or refuses the key, and asking about the
	// rest would only add to its load. The job's retries and the next start carry on.
	logosFailures = 3
)

// LogoBackfill is the fetch_logos job, queued at every start: it gives titles linked to TMDB
// the logos they lack in their content languages and English (logoLangs), so a library linked
// before logos existed gets them without an admin applying each title again. It never
// replaces a logo, and leaves a title with one not tied to a language alone, as that one
// shows in every language. Each title is looked up once per LogosVersion, whether TMDB has a
// logo for it or not, one title at a time.
type LogoBackfill struct {
	Catalog  *catalog.Store
	Settings *settings.Store
	Artwork  *artwork.Service
	Jobs     *jobs.Store
}

// Start queues a run, which finds nothing to do once every linked title was looked up.
func (b *LogoBackfill) Start(ctx context.Context) error {
	_, err := b.Jobs.EnqueueJobOnce(ctx, LogosJob, struct{}{}, jobs.EnqueueOpts{Priority: logosPriority})
	return err
}

func (b *LogoBackfill) Handle(ctx context.Context, _ *jobs.Job, report func(int)) error {
	key, err := APIKey(ctx, b.Settings)
	if err != nil {
		return nil // nothing to ask TMDB with; the next start looks again
	}
	titles, err := b.Catalog.LogoBackfillTitles(ctx, LogosVersion)
	if err != nil {
		return err
	}
	client := New(key)
	failed := 0
	for i := range titles {
		t := &titles[i]
		switch err := b.fill(ctx, client, t); {
		case err == nil || unknownToTMDB(err):
			failed = 0
			if err := b.Catalog.SetLogosVersion(ctx, t.ID, LogosVersion); err != nil {
				return err
			}
		case ctx.Err() != nil:
			return ctx.Err()
		default:
			slog.Warn("tmdb logos", "titleId", t.ID, "err", err)
			if failed++; failed == logosFailures {
				return fmt.Errorf("%d titles in a row failed, the rest waits: %w", failed, err)
			}
		}
		report((i + 1) * 100 / len(titles))
	}
	return nil
}

// fill stores the logos a title lacks.
func (b *LogoBackfill) fill(ctx context.Context, client *Client, t *catalog.Title) error {
	have, err := b.Artwork.Store.ArtworkFor(ctx, "title", t.ID)
	if err != nil {
		return err
	}
	for _, a := range have {
		if a.Kind == "logo" && a.Lang == nil {
			return nil
		}
	}
	return applyLogos(ctx, client, b.Artwork, t, *t.TmdbID, logoLangs(t.MetadataLanguages), false)
}
