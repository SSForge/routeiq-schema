package main

import (
	"os"
	"testing"
	"time"
)

func TestLoad_MissingSupabaseURL(t *testing.T) {
	os.Unsetenv("SUPABASE_URL")
	os.Setenv("SUPABASE_KEY", "key")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing SUPABASE_URL")
	}
}

func TestLoad_MissingSupabaseKey(t *testing.T) {
	os.Setenv("SUPABASE_URL", "https://x.supabase.co")
	os.Unsetenv("SUPABASE_KEY")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing SUPABASE_KEY")
	}
}

func TestLoad_Defaults(t *testing.T) {
	os.Setenv("SUPABASE_URL", "https://x.supabase.co")
	os.Setenv("SUPABASE_KEY", "key")
	os.Unsetenv("UPSTREAM_GRPC")
	os.Unsetenv("UPSTREAM_HTTP")
	os.Unsetenv("CACHE_TTL_SECONDS")
	os.Unsetenv("AUTH_LOG_REJECTS")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.UpstreamGRPC != "localhost:14317" {
		t.Errorf("UpstreamGRPC = %q, want localhost:14317", cfg.UpstreamGRPC)
	}
	if cfg.UpstreamHTTP != "localhost:14318" {
		t.Errorf("UpstreamHTTP = %q, want localhost:14318", cfg.UpstreamHTTP)
	}
	if cfg.CacheTTL != 60*time.Second {
		t.Errorf("CacheTTL = %v, want 60s", cfg.CacheTTL)
	}
	if !cfg.LogRejects {
		t.Error("LogRejects should default to true")
	}
}
