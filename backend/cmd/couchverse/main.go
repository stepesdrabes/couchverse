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

	"couchverse/internal/artwork"
	"couchverse/internal/auth"
	"couchverse/internal/config"
	"couchverse/internal/jobs"
	"couchverse/internal/media"
	"couchverse/internal/server"
	"couchverse/internal/store"
	"couchverse/internal/subtitles"
	"couchverse/internal/tmdb"
	"couchverse/internal/transcode"
	"couchverse/internal/upload"
)

// ensureManagedLibraries creates the default upload-target libraries under
// DATA_DIR/media on first start.
func ensureManagedLibraries(ctx context.Context, st *store.Store, dataDir string) error {
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

	pool, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := store.Migrate(pool); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	st := store.New(pool)
	if err := auth.Bootstrap(ctx, st, cfg); err != nil {
		return err
	}
	if err := ensureManagedLibraries(ctx, st, cfg.DataDir); err != nil {
		return err
	}

	uploadManager := &upload.Manager{Store: st, DataDir: cfg.DataDir}
	artworkService := &artwork.Service{Store: st, DataDir: cfg.DataDir, FFmpegPath: cfg.FFmpegPath}
	subtitleService := &subtitles.Service{Store: st, DataDir: cfg.DataDir, FFmpegPath: cfg.FFmpegPath}
	transcodeHandler := &transcode.JobHandler{Store: st, DataDir: cfg.DataDir, FFmpegPath: cfg.FFmpegPath}
	sessionManager := &transcode.SessionManager{
		Store: st, DataDir: cfg.DataDir, FFmpegPath: cfg.FFmpegPath, MaxSessions: 3,
	}
	defer sessionManager.StopAll()

	go transcode.DetectEncoders(cfg.FFmpegPath)

	// transcodes can occupy their full concurrency budget and still leave
	// workers free for quick jobs (probes, scans, metadata) — otherwise a
	// queue of hour-long transcodes starves everything else
	transcodeSlots := transcode.LoadSettings(ctx, st).MaxConcurrent
	workers := cfg.JobWorkers
	if workers < transcodeSlots+2 {
		workers = transcodeSlots + 2
	}

	runner := jobs.NewRunner(st, workers)
	runner.Register("scan_library", 1, (&media.Scanner{Store: st}).Handle)
	runner.Register("probe", 2, (&media.Prober{Store: st, FFprobePath: cfg.FFprobePath, DataDir: cfg.DataDir}).Handle)
	runner.Register("extract_subtitles", 1, subtitleService.HandleExtract)
	runner.Register("fetch_metadata", 2, (&tmdb.FetchJob{Store: st, Artwork: artworkService}).Handle)
	runner.Register("transcode_hls", transcodeSlots, transcodeHandler.Handle)
	runner.Register("cleanup", 1, cleanupHandler(st, uploadManager, cfg.DataDir))
	if _, err := st.EnqueueJobOnce(ctx, "cleanup", struct{}{}, store.EnqueueOpts{}); err != nil {
		return err
	}
	go runner.Run(ctx)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           server.New(cfg, st, uploadManager, artworkService, subtitleService, transcodeHandler, sessionManager).Handler(),
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
