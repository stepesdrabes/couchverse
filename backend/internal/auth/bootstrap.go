package auth

import (
	"context"
	"fmt"
	"log/slog"

	"couchverse/internal/config"
	"couchverse/internal/store"
)

// Bootstrap creates the master admin account on a fresh database.
func Bootstrap(ctx context.Context, st *store.Store, cfg config.Config) error {
	n, err := st.CountUsers(ctx)
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if n > 0 {
		return nil
	}
	if cfg.AdminUsername == "" || cfg.AdminPassword == "" {
		return fmt.Errorf("fresh database: ADMIN_USERNAME and ADMIN_PASSWORD must be set to create the master admin")
	}
	hash, err := HashPassword(cfg.AdminPassword)
	if err != nil {
		return err
	}
	if _, err := st.CreateUser(ctx, cfg.AdminUsername, cfg.AdminUsername, hash, "admin"); err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	slog.Info("created master admin account", "username", cfg.AdminUsername)
	return nil
}
