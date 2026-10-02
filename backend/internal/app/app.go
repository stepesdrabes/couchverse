// Package app wires the stores, services and background workers into a server. The
// binary runs it; the API tests build the same graph against a test database.
package app

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"

	"couchverse/internal/config"
	"couchverse/internal/feature/analytics"
	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/auth"
	"couchverse/internal/feature/catalog"
	"couchverse/internal/feature/couch"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/library"
	"couchverse/internal/feature/metadata"
	"couchverse/internal/feature/playback"
	"couchverse/internal/feature/ranks"
	"couchverse/internal/feature/subtitles"
	"couchverse/internal/feature/system"
	"couchverse/internal/media"
	"couchverse/internal/server"
	"couchverse/internal/settings"
)

type App struct {
	deps server.Deps
	// stop ends the couch hub's background work
	stop context.CancelFunc
}

// New bootstraps the admin account and managed libraries and constructs every
// store and service. Nothing runs in the background until Start.
func New(ctx context.Context, cfg config.Config, pool *pgxpool.Pool) (*App, error) {
	set := settings.NewStore(pool)
	jobsStore := jobs.NewStore(pool)
	authStore := auth.NewStore(pool)
	catalogStore := catalog.NewStore(pool)
	libraryStore := library.NewStore(pool)
	analyticsStore := analytics.NewStore(pool)
	ranksStore := ranks.NewStore(pool)
	if err := auth.Bootstrap(ctx, authStore, cfg); err != nil {
		return nil, err
	}
	if err := ensureManagedLibraries(ctx, libraryStore, cfg.DataDir); err != nil {
		return nil, err
	}

	artworkService := &artwork.Service{Store: artwork.NewStore(pool), DataDir: cfg.DataDir, FFmpegPath: cfg.FFmpegPath}
	subtitleService := &subtitles.Service{Subs: subtitles.NewStore(pool), Files: libraryStore, DataDir: cfg.DataDir, FFmpegPath: cfg.FFmpegPath}
	sessionManager := &playback.SessionManager{
		Files: libraryStore, Settings: set, DataDir: cfg.DataDir, FFmpegPath: cfg.FFmpegPath, MaxSessions: 3,
	}
	// one shared stream resolver: the playback module serves it and the couch
	// hub reuses its BuildPlayback to assemble follower payloads
	stream := playback.NewStream(subtitleService.Subs, catalogStore, libraryStore, set, jobsStore, cfg.DataDir, sessionManager, cfg.FFmpegPath)

	hubCtx, stop := context.WithCancel(context.Background())
	couchHub := couch.NewHub(hubCtx, couch.Deps{
		Media:     catalogStore,
		Playback:  stream,
		Analytics: analyticsStore,
		Stats:     ranksStore,
		Settings:  set,
		Secure:    cfg.CookieSecure,
	})

	a := &App{
		deps: server.Deps{
			Config:    cfg,
			Pool:      pool,
			Settings:  set,
			Auth:      authStore,
			Catalog:   catalogStore,
			Jobs:      jobsStore,
			Library:   libraryStore,
			System:    system.NewStore(pool),
			Uploads:   &library.Manager{Files: libraryStore, Catalog: catalogStore, Jobs: jobsStore, DataDir: cfg.DataDir},
			Artwork:   artworkService,
			Subtitles: subtitleService,
			Transcode: &playback.JobHandler{Files: libraryStore, Settings: set, Jobs: jobsStore, DataDir: cfg.DataDir, FFmpegPath: cfg.FFmpegPath},
			Sessions:  sessionManager,
			Stream:    stream,
			Analytics: analyticsStore,
			Ranks:     ranksStore,
			Couch:     couchHub,
		},
		stop: stop,
	}
	return a, nil
}

func (a *App) Handler() http.Handler {
	return server.New(a.deps).Handler()
}

// Start launches the job runner and the background maintenance until ctx ends.
func (a *App) Start(ctx context.Context) error {
	d := a.deps
	go playback.DetectEncoders(d.Config.FFmpegPath)

	// transcodes can occupy their full concurrency budget and still leave
	// workers free for quick jobs (probes, scans, metadata) - otherwise a
	// queue of hour-long transcodes starves everything else
	transcodeSlots := media.LoadTranscodeSettings(ctx, d.Settings).MaxConcurrent
	workers := max(d.Config.JobWorkers, transcodeSlots+2)

	runner := jobs.NewRunner(d.Jobs, workers)
	runner.Register("probe", 2, (&library.Prober{Files: d.Library, Catalog: d.Catalog, Settings: d.Settings, Jobs: d.Jobs, FFprobePath: d.Config.FFprobePath}).Handle)
	runner.Register("extract_subtitles", 1, d.Subtitles.HandleExtract)
	runner.Register("fetch_metadata", 2, (&metadata.FetchJob{Catalog: d.Catalog, Settings: d.Settings, Artwork: d.Artwork}).Handle)
	runner.Register("import_episodes", 1, (&metadata.ImportEpisodesJob{Catalog: d.Catalog, Settings: d.Settings, Artwork: d.Artwork}).Handle)
	runner.Register("transcode_hls", transcodeSlots, d.Transcode.Handle)
	runner.Register("cleanup", 1, cleanupHandler(d.Library, d.Auth, d.Jobs, d.Analytics, d.Uploads, d.Config.DataDir))
	if _, err := d.Jobs.EnqueueJobOnce(ctx, "cleanup", struct{}{}, jobs.EnqueueOpts{}); err != nil {
		return err
	}
	go runner.Run(ctx)
	// theme existing libraries: fill in accents for artwork that predates
	// server-side extraction, in the background so startup isn't blocked
	go d.Artwork.BackfillAccents(ctx)
	return nil
}

// Close stops live transcodes and ends every couch session.
func (a *App) Close() {
	a.deps.Sessions.StopAll()
	a.deps.Couch.Shutdown()
	a.stop()
}

// ensureManagedLibraries creates the default upload-target libraries under
// DATA_DIR/media on first start.
func ensureManagedLibraries(ctx context.Context, st *library.Store, dataDir string) error {
	for _, lib := range []struct{ name, kind string }{
		{"Movies", "movies"},
		{"Series", "series"},
	} {
		path := filepath.Join(dataDir, "media", lib.kind)
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if err := st.EnsureLibrary(ctx, lib.name, lib.kind, abs, true); err != nil {
			return err
		}
	}
	return nil
}
