package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port          int
	DatabaseURL   string
	DataDir       string
	AdminUsername string
	AdminPassword string
	CookieSecure  bool
	JobWorkers    int
}

func Load() (Config, error) {
	cfg := Config{
		Port:          envInt("PORT", 8080),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		DataDir:       envStr("DATA_DIR", "./data"),
		AdminUsername: os.Getenv("ADMIN_USERNAME"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		CookieSecure:  envBool("COOKIE_SECURE", false),
		JobWorkers:    envInt("JOB_WORKERS", 2),
	}
	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
