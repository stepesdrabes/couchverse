package config

import (
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port          int
	DatabaseURL   string
	DataDir       string
	AdminUsername string
	AdminPassword string
	CookieSecure  bool
	JobWorkers    int
	FFmpegPath    string
	FFprobePath   string
	// TrustedProxies are the reverse proxies whose X-Forwarded-For is believed.
	TrustedProxies []netip.Prefix
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
		FFmpegPath:    envStr("FFMPEG_PATH", "ffmpeg"),
		FFprobePath:   envStr("FFPROBE_PATH", "ffprobe"),
	}
	proxies, err := parsePrefixes(os.Getenv("TRUSTED_PROXIES"))
	if err != nil {
		return cfg, fmt.Errorf("TRUSTED_PROXIES: %w", err)
	}
	cfg.TrustedProxies = proxies
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

// parsePrefixes reads a comma-separated list of CIDRs or bare addresses.
func parsePrefixes(v string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, item := range strings.Split(v, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.Contains(item, "/") {
			p, err := netip.ParsePrefix(item)
			if err != nil {
				return nil, err
			}
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(item)
		if err != nil {
			return nil, err
		}
		a = a.Unmap()
		out = append(out, netip.PrefixFrom(a, a.BitLen()))
	}
	return out, nil
}
