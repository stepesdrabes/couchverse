package main

import (
	"context"
	"errors"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	"couchverse/internal/config"
	"couchverse/internal/db"
	"couchverse/internal/feature/couchtv"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/settings"
)

// runCouchTvRegenerate is `couchverse couchtv-regenerate YYYY-MM-DD`: the admin schedule's
// Regenerate for an operator on the server. It replans that broadcast day and the days
// after it on the app's database; the app's prepare and compose jobs take it from there.
func runCouchTvRegenerate(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: couchverse couchtv-regenerate YYYY-MM-DD")
	}
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
	s := &couchtv.Scheduler{Store: couchtv.NewStore(pool), Settings: settings.NewStore(pool), Jobs: jobs.NewStore(pool)}
	day, _, err := s.RegenerateDay(ctx, args[0])
	if err != nil {
		return err
	}
	slog.Info("couchtv: regenerated", "from", day.Format(time.DateOnly))
	return nil
}
