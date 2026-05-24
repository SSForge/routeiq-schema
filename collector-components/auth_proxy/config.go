package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	SupabaseURL  string
	SupabaseKey  string
	UpstreamGRPC string
	UpstreamHTTP string
	CacheTTL     time.Duration
	LogRejects   bool
}

func Load() (Config, error) {
	cfg := Config{
		SupabaseURL:  os.Getenv("SUPABASE_URL"),
		SupabaseKey:  os.Getenv("SUPABASE_KEY"),
		UpstreamGRPC: envOr("UPSTREAM_GRPC", "localhost:14317"),
		UpstreamHTTP: envOr("UPSTREAM_HTTP", "localhost:14318"),
		LogRejects:   envBool("AUTH_LOG_REJECTS", true),
	}
	ttlSecs, err := strconv.Atoi(envOr("CACHE_TTL_SECONDS", "60"))
	if err != nil {
		return cfg, fmt.Errorf("CACHE_TTL_SECONDS: %w", err)
	}
	cfg.CacheTTL = time.Duration(ttlSecs) * time.Second
	if cfg.SupabaseURL == "" {
		return cfg, fmt.Errorf("SUPABASE_URL is required")
	}
	if cfg.SupabaseKey == "" {
		return cfg, fmt.Errorf("SUPABASE_KEY is required")
	}
	return cfg, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
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
