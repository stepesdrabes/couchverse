// Package db owns the shared pgx pool lifecycle (connect, migrate) and the
// not-found sentinel stores return for missing rows.
package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"couchverse/migrations"
)

// ErrNotFound lets stores signal a missing row without importing pgx everywhere.
var ErrNotFound = errors.New("not found")

// Connect opens a pgx pool, waiting for the database to become reachable
// (compose may start the app before postgres is ready to accept connections).
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err = pool.Ping(pingCtx)
		cancel()
		if err == nil {
			return pool, nil
		}
		if time.Now().After(deadline) {
			pool.Close()
			return nil, fmt.Errorf("database unreachable: %w", err)
		}
		slog.Info("waiting for database", "err", err)
		select {
		case <-ctx.Done():
			pool.Close()
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// Migrate applies all pending goose migrations from the embedded FS.
func Migrate(pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(gooseLogger{})
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, ".")
}

type gooseLogger struct{}

func (gooseLogger) Fatalf(format string, v ...any) {
	slog.Error(fmt.Sprintf("goose: "+format, v...))
}

func (gooseLogger) Printf(format string, v ...any) {
	slog.Info(fmt.Sprintf("goose: "+format, v...))
}
