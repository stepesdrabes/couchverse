package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"couchverse/internal/config"
	"couchverse/internal/db"
	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/auth"
	"couchverse/internal/feature/catalog"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/library"
	"couchverse/internal/feature/metadata"
	"couchverse/internal/feature/music"
	"couchverse/internal/feature/playback"
	"couchverse/internal/feature/subtitles"
	"couchverse/internal/media"
	"couchverse/internal/server"
	"couchverse/internal/settings"
	"couchverse/internal/store"
)

// ensureManagedLibraries creates the default upload-target libraries under
// DATA_DIR/media on first start.
func ensureManagedLibraries(ctx context.Context, st *library.Store, dataDir string) error {
	for _, lib := range []struct{ name, kind string }{
		{"Movies", "movies"},
		{"Series", "series"},
		{"Music", "music"},
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

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.Migrate(pool); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	st := store.New(pool)
	set := settings.NewStore(pool)
	jobsStore := jobs.NewStore(pool)
	authStore := auth.NewStore(pool)
	catalogStore := catalog.NewStore(pool)
	musicStore := music.NewStore(pool)
	libraryStore := library.NewStore(pool)
	if err := auth.Bootstrap(ctx, authStore, cfg); err != nil {
		return err
	}
	if err := ensureManagedLibraries(ctx, libraryStore, cfg.DataDir); err != nil {
		return err
	}

	uploadManager := &library.Manager{Files: libraryStore, Catalog: catalogStore, Jobs: jobsStore, DataDir: cfg.DataDir}
	artworkService := &artwork.Service{Store: artwork.NewStore(pool), DataDir: cfg.DataDir, FFmpegPath: cfg.FFmpegPath}
	subtitleService := &subtitles.Service{Subs: subtitles.NewStore(pool), Files: libraryStore, DataDir: cfg.DataDir, FFmpegPath: cfg.FFmpegPath}
	transcodeHandler := &playback.JobHandler{Files: libraryStore, Settings: set, Jobs: jobsStore, DataDir: cfg.DataDir, FFmpegPath: cfg.FFmpegPath}
	sessionManager := &playback.SessionManager{
		Files: libraryStore, Settings: set, DataDir: cfg.DataDir, FFmpegPath: cfg.FFmpegPath, MaxSessions: 3,
	}
	defer sessionManager.StopAll()

	go playback.DetectEncoders(cfg.FFmpegPath)

	// transcodes can occupy their full concurrency budget and still leave
	// workers free for quick jobs (probes, scans, metadata) — otherwise a
	// queue of hour-long transcodes starves everything else
	transcodeSlots := media.LoadTranscodeSettings(ctx, set).MaxConcurrent
	workers := cfg.JobWorkers
	if workers < transcodeSlots+2 {
		workers = transcodeSlots + 2
	}

	runner := jobs.NewRunner(jobsStore, workers)
	runner.Register("scan_library", 1, (&library.Scanner{Files: libraryStore, Jobs: jobsStore}).Handle)
	runner.Register("probe", 2, (&library.Prober{Files: libraryStore, Catalog: catalogStore, Settings: set, Jobs: jobsStore, Artwork: artworkService.Store, Music: musicStore, FFprobePath: cfg.FFprobePath, DataDir: cfg.DataDir}).Handle)
	runner.Register("extract_subtitles", 1, subtitleService.HandleExtract)
	runner.Register("fetch_metadata", 2, (&metadata.FetchJob{Catalog: catalogStore, Settings: set, Artwork: artworkService}).Handle)
	runner.Register("import_episodes", 1, (&metadata.ImportEpisodesJob{Catalog: catalogStore, Settings: set}).Handle)
	runner.Register("transcode_hls", transcodeSlots, transcodeHandler.Handle)
	runner.Register("cleanup", 1, cleanupHandler(libraryStore, authStore, jobsStore, uploadManager, cfg.DataDir))
	if _, err := jobsStore.EnqueueJobOnce(ctx, "cleanup", struct{}{}, jobs.EnqueueOpts{}); err != nil {
		return err
	}
	go runner.Run(ctx)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           server.New(cfg, st, set, authStore, catalogStore, jobsStore, musicStore, libraryStore, uploadManager, artworkService, subtitleService, transcodeHandler, sessionManager).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", srv.Addr)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return nil
	}
}
