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
	"couchverse/internal/feature/couchtv"
	"couchverse/internal/feature/downloads"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/library"
	"couchverse/internal/feature/metadata"
	"couchverse/internal/feature/playback"
	"couchverse/internal/feature/ranks"
	"couchverse/internal/feature/subtitles"
	"couchverse/internal/feature/system"
	"couchverse/internal/grant"
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
	if err := system.EnsureServerID(ctx, set); err != nil {
		return nil, err
	}

	secret, err := grant.LoadSecret(ctx, set)
	if err != nil {
		return nil, err
	}
	grants := grant.NewSigner(secret)

	artworkService := &artwork.Service{Store: artwork.NewStore(pool), DataDir: cfg.DataDir, FFmpegPath: cfg.FFmpegPath}
	subtitleService := &subtitles.Service{Subs: subtitles.NewStore(pool), Files: libraryStore, DataDir: cfg.DataDir, FFmpegPath: cfg.FFmpegPath}
	sessionManager := &playback.SessionManager{
		Files: libraryStore, Settings: set, DataDir: cfg.DataDir, FFmpegPath: cfg.FFmpegPath, MaxSessions: 3,
	}
	// one shared stream resolver: the playback module serves it and the couch
	// hub reuses its BuildPlayback to assemble follower payloads
	stream := playback.NewStream(subtitleService.Subs, catalogStore, libraryStore, set, jobsStore, cfg.DataDir, sessionManager, cfg.FFmpegPath, grants)

	// the channel picks its promos from the markers, so a title whose markers change is
	// analyzed again, and a new title gets its soundtrack searched (library cannot import
	// couchtv); the search reads TMDB's composers through metadata
	markers := &library.MarkerAnalyzer{Files: libraryStore, Jobs: jobsStore, DataDir: cfg.DataDir, FFmpegPath: cfg.FFmpegPath}
	couchTvStore := couchtv.NewStore(pool)
	couchTvAnalysis := &couchtv.Analyzer{Store: couchTvStore, Jobs: jobsStore, Markers: markers, FFmpegPath: cfg.FFmpegPath, DataDir: cfg.DataDir}
	couchTvSoundtracks := &couchtv.Soundtracks{Store: couchTvStore, Jobs: jobsStore,
		Catalogs: []couchtv.SoundtrackCatalog{couchtv.NewITunes(), couchtv.NewDeezer()},
		Music:    &metadata.MusicFacts{Settings: set}, DataDir: cfg.DataDir, FFmpegPath: cfg.FFmpegPath}
	markers.Analyzed = func(ctx context.Context, titleID string) error {
		if err := couchTvAnalysis.Queue(ctx, titleID); err != nil {
			return err
		}
		return couchTvSoundtracks.Queue(ctx, titleID)
	}

	hubCtx, stop := context.WithCancel(context.Background())
	couchHub := couch.NewHub(hubCtx, couch.Deps{
		Media:     catalogStore,
		Playback:  stream,
		Analytics: analyticsStore,
		Stats:     ranksStore,
		Settings:  set,
		Secure:    cfg.CookieSecure,
		Grants:    grants,
	})

	// the channel's room maps its viewers' reports onto what aired: progress for the
	// signed in, watch time for everyone and the facts ranks scores the channel by
	couchTvLive := couchtv.NewLive(couchTvStore, cfg.DataDir, cfg.FFmpegPath)
	couchTvHub := couchtv.NewHub(hubCtx, couchtv.HubDeps{
		Lineup:    couchTvLive.Channel,
		Durations: couchTvStore,
		Watch:     analyticsStore,
		Progress:  catalogStore,
		Stats:     ranksStore,
		Settings:  set,
	})
	couchTvScheduler := &couchtv.Scheduler{Store: couchTvStore, Settings: set, Jobs: jobsStore, Changed: couchTvHub.ScheduleChanged}
	// a series whose episodes may play at random airs its strips at random, so changing
	// that queues a plan run for the days not frozen yet (catalog cannot import couchtv)
	catalogStore.RandomPlaybackChanged = couchTvScheduler.Start

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
			Grants:    grants,
			Downloads: downloads.NewStore(pool),
			CouchTv:   couchTvScheduler,

			Markers:            markers,
			CouchTvAnalysis:    couchTvAnalysis,
			CouchTvSoundtracks: couchTvSoundtracks,
			CouchTvLive:        couchTvLive,
			CouchTvPrepare: &couchtv.Preparer{Store: couchTvStore, Files: libraryStore, Jobs: jobsStore, Settings: set,
				DataDir: cfg.DataDir, Changed: couchTvHub.ScheduleChanged},
			CouchTvHub:    couchTvHub,
			CouchTvGuests: couchtv.NewGuests(couchTvStore, set, couchTvHub, cfg.CookieSecure),
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
	go playback.DetectFeatures(d.Config.FFmpegPath)

	// transcodes can occupy their full concurrency budget and still leave
	// workers free for quick jobs (probes, scans, metadata) - otherwise a
	// queue of hour-long transcodes starves everything else
	transcodeSlots := media.LoadTranscodeSettings(ctx, d.Settings).MaxConcurrent
	workers := max(d.Config.JobWorkers, transcodeSlots+2)

	runner := jobs.NewRunner(d.Jobs, workers)
	prober := &library.Prober{Files: d.Library, Catalog: d.Catalog, Settings: d.Settings, Jobs: d.Jobs, FFprobePath: d.Config.FFprobePath}
	runner.Register("probe", 2, prober.Handle)
	runner.Register("reprobe", 1, prober.Reprobe)
	runner.Register("extract_subtitles", 1, d.Subtitles.HandleExtract)
	runner.Register("fetch_metadata", 2, (&metadata.FetchJob{Catalog: d.Catalog, Settings: d.Settings, Artwork: d.Artwork}).Handle)
	runner.Register("import_episodes", 1, (&metadata.ImportEpisodesJob{Catalog: d.Catalog, Settings: d.Settings, Artwork: d.Artwork}).Handle)
	// titles linked to TMDB before logos existed get theirs
	logos := &metadata.LogoBackfill{Catalog: d.Catalog, Settings: d.Settings, Artwork: d.Artwork, Jobs: d.Jobs}
	runner.Register(metadata.LogosJob, 1, logos.Handle)
	if err := logos.Start(ctx); err != nil {
		return err
	}
	runner.Register("transcode_hls", transcodeSlots, d.Transcode.Handle)
	runner.Register(downloads.JobType, 1, (&downloads.Preparer{Store: d.Downloads, Files: d.Library, Subtitles: d.Subtitles,
		Settings: d.Settings, DataDir: d.Config.DataDir, FFmpegPath: d.Config.FFmpegPath}).Handle)
	runner.Register("cleanup", 1, cleanupHandler(d.Library, d.Auth, d.Jobs, d.Analytics, d.Uploads, d.Downloads, d.CouchTv.Store, d.Config.DataDir))
	if _, err := d.Jobs.EnqueueJobOnce(ctx, "cleanup", struct{}{}, jobs.EnqueueOpts{}); err != nil {
		return err
	}
	runner.Register(couchtv.PlanJob, 1, d.CouchTv.Handle)
	if err := d.CouchTv.Start(ctx); err != nil {
		return err
	}
	// media analysis is background work (nice'd, lowest priority); each backfill finds
	// nothing once the library is analyzed
	runner.Register(library.MarkersJob, 1, d.Markers.Handle)
	if err := d.Markers.Start(ctx); err != nil {
		return err
	}
	runner.Register(couchtv.AnalyzeJob, 1, d.CouchTvAnalysis.Handle)
	if err := d.CouchTvAnalysis.Start(ctx); err != nil {
		return err
	}
	// mostly waiting on the music catalogs, so it takes no turn in the transcode budget
	runner.Register(couchtv.SoundtrackJob, 1, d.CouchTvSoundtracks.Handle)
	if err := d.CouchTvSoundtracks.Start(ctx); err != nil {
		return err
	}
	// breaks are composed here and rendered by the sidecar (`couchverse render`), which
	// registers couchtv_render, couchtv_bulletin_render and couchtv_assemble itself
	composer := &couchtv.Composer{Store: d.CouchTv.Store, Settings: d.Settings, Jobs: d.Jobs}
	runner.Register(couchtv.ComposeJob, 1, composer.Handle)
	if err := composer.Start(ctx); err != nil {
		return err
	}
	runner.Register(couchtv.PrepareJob, 1, d.CouchTvPrepare.Handle)
	if err := d.CouchTvPrepare.Start(ctx); err != nil {
		return err
	}
	// the weather and the news for the breaks, a few small requests through the day
	bulletins := &couchtv.Bulletins{Store: d.CouchTv.Store, Settings: d.Settings, Jobs: d.Jobs}
	runner.Register(couchtv.BulletinsJob, 1, bulletins.Handle)
	if err := bulletins.Start(ctx); err != nil {
		return err
	}
	// each of these keeps every core of a small server busy: they take turns within the
	// transcode budget instead of running side by side, the higher priority first
	runner.Share(transcodeSlots, "transcode_hls", downloads.JobType, library.MarkersJob, couchtv.AnalyzeJob)
	// media analysis waits while the render sidecar works: Chromium and its encoder take
	// most of a Pi's memory, and the two together swap
	for _, t := range []string{library.MarkersJob, couchtv.AnalyzeJob} {
		runner.Yield(t, true, couchtv.RenderJob, couchtv.BulletinRenderJob, couchtv.AssembleJob)
	}
	// the slate is made once, before anything has to air it (a failure is logged)
	go func() { _ = d.CouchTvLive.Slate.Ensure(ctx) }()
	// files an older prober read are brought up to date in the background
	if _, err := d.Jobs.EnqueueJobOnce(ctx, "reprobe", struct{}{}, jobs.EnqueueOpts{}); err != nil {
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
