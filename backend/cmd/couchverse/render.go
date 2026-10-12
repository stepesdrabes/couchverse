package main

import (
	"context"
	"io/fs"
	"log/slog"
	"os/signal"
	"syscall"

	"couchverse/broadcast"
	"couchverse/internal/config"
	"couchverse/internal/db"
	"couchverse/internal/feature/artwork"
	"couchverse/internal/feature/couchtv"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/settings"
)

// renderRestartEvery is how many jobs one Chromium does before a fresh one replaces it.
const renderRestartEvery = 25

// runRender is `couchverse render`, the Couch TV render sidecar (docs/couch-tv-plan.md
// 6.7): it works the couchtv_render and couchtv_assemble jobs with one headless Chromium,
// one job at a time, never while a transcode runs, at a low CPU and the idle I/O
// priority (its image's entrypoint). It shares the
// app's database and DATA_DIR and serves no API.
func runRender() error {
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
	if err := db.WaitForMigrations(ctx, pool); err != nil {
		return err
	}

	bundle, err := fs.Sub(broadcast.Dist, "dist")
	if err != nil {
		return err
	}
	jobStore := jobs.NewStore(pool)
	r := &couchtv.Renderer{
		Store:        couchtv.NewStore(pool),
		Jobs:         jobStore,
		Settings:     settings.NewStore(pool),
		Artwork:      &artwork.Service{Store: artwork.NewStore(pool), DataDir: cfg.DataDir, FFmpegPath: cfg.FFmpegPath},
		DataDir:      cfg.DataDir,
		FFmpeg:       cfg.FFmpegPath,
		Chromium:     cfg.ChromiumPath,
		Bundle:       bundle,
		RestartEvery: renderRestartEvery,
	}
	if err := r.Start(ctx); err != nil {
		return err
	}
	defer r.Close()

	runner := jobs.NewRunner(jobStore, 1)
	runner.Register(couchtv.RenderJob, 1, r.Render)
	runner.Yield(couchtv.RenderJob, "transcode_hls")
	runner.Register(couchtv.AssembleJob, 1, r.Assemble)
	slog.Info("render: working the couch tv queue")
	runner.Run(ctx)
	return nil
}
