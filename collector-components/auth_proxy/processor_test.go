package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracesdatapb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func supabaseOK(row supabaseRow) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]supabaseRow{row})
	}))
}

func supabaseEmpty() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]supabaseRow{})
	}))
}

func supabaseDown() *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	return srv
}

func procWith(srv *httptest.Server) *AuthProcessor {
	cfg := Config{
		SupabaseURL: srv.URL,
		SupabaseKey: "test-key",
		CacheTTL:    time.Minute,
		LogRejects:  false,
	}
	return NewAuthProcessor(cfg)
}

func TestAuthenticate_EmptyToken(t *testing.T) {
	srv := supabaseEmpty()
	defer srv.Close()
	proc := procWith(srv)

	_, err := proc.Authenticate(context.Background(), "")
	if st, _ := status.FromError(err); st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestAuthenticate_ValidKey(t *testing.T) {
	ws := "ws-456"
	srv := supabaseOK(supabaseRow{
		OrgID:       "org-123",
		WorkspaceID: &ws,
		Scope:       "INGEST_ONLY",
	})
	defer srv.Close()
	proc := procWith(srv)

	info, err := proc.Authenticate(context.Background(), "riq_live_abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.OrgID != "org-123" {
		t.Errorf("OrgID = %q, want org-123", info.OrgID)
	}
	if info.WorkspaceID != "ws-456" {
		t.Errorf("WorkspaceID = %q, want ws-456", info.WorkspaceID)
	}
}

func TestAuthenticate_CacheHit(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]supabaseRow{{OrgID: "org-1", Scope: "INGEST_ONLY"}})
	}))
	defer srv.Close()
	proc := procWith(srv)

	proc.Authenticate(context.Background(), "riq_live_token")
	proc.Authenticate(context.Background(), "riq_live_token")

	if callCount != 1 {
		t.Errorf("Supabase called %d times, want 1 (second call should use cache)", callCount)
	}
}

func TestAuthenticate_KeyNotFound(t *testing.T) {
	srv := supabaseEmpty()
	defer srv.Close()
	proc := procWith(srv)

	_, err := proc.Authenticate(context.Background(), "riq_live_unknown")
	if st, _ := status.FromError(err); st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated for missing key, got %v", err)
	}
}

func TestAuthenticate_RevokedKey(t *testing.T) {
	revokedAt := "2026-01-01T00:00:00Z"
	srv := supabaseOK(supabaseRow{
		OrgID:     "org-1",
		Scope:     "INGEST_ONLY",
		RevokedAt: &revokedAt,
	})
	defer srv.Close()
	proc := procWith(srv)

	_, err := proc.Authenticate(context.Background(), "riq_live_revoked")
	if st, _ := status.FromError(err); st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated for revoked key, got %v", err)
	}
}

func TestAuthenticate_ExpiredKey(t *testing.T) {
	exp := "2025-01-01T00:00:00Z"
	srv := supabaseOK(supabaseRow{
		OrgID:     "org-1",
		Scope:     "INGEST_ONLY",
		ExpiresAt: &exp,
	})
	defer srv.Close()
	proc := procWith(srv)

	_, err := proc.Authenticate(context.Background(), "riq_live_expired")
	if st, _ := status.FromError(err); st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated for expired key, got %v", err)
	}
}

func TestAuthenticate_AdminScope(t *testing.T) {
	srv := supabaseOK(supabaseRow{OrgID: "org-1", Scope: "ADMIN_SCOPE"})
	defer srv.Close()
	proc := procWith(srv)

	_, err := proc.Authenticate(context.Background(), "riq_live_admin")
	if st, _ := status.FromError(err); st.Code() != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for ADMIN_SCOPE, got %v", err)
	}
}

func TestAuthenticate_SupabaseUnreachable(t *testing.T) {
	srv := supabaseDown()
	defer srv.Close()
	proc := procWith(srv)

	_, err := proc.Authenticate(context.Background(), "riq_live_x")
	if st, _ := status.FromError(err); st.Code() != codes.Unavailable {
		t.Fatalf("expected Unavailable when supabase returns 500, got %v", err)
	}
}

func TestInjectAttrs_WithWorkspace(t *testing.T) {
	req := &tracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracesdatapb.ResourceSpans{
			{Resource: &resourcepb.Resource{}},
		},
	}
	InjectAttrs(req, KeyInfo{OrgID: "org-1", WorkspaceID: "ws-1"})

	attrs := req.ResourceSpans[0].Resource.Attributes
	if !hasAttr(attrs, "routeiq.tenant.id", "org-1") {
		t.Error("missing routeiq.tenant.id")
	}
	if !hasAttr(attrs, "routeiq.workspace.id", "ws-1") {
		t.Error("missing routeiq.workspace.id")
	}
}

func TestInjectAttrs_NoWorkspace(t *testing.T) {
	req := &tracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracesdatapb.ResourceSpans{
			{Resource: &resourcepb.Resource{}},
		},
	}
	InjectAttrs(req, KeyInfo{OrgID: "org-2"})

	attrs := req.ResourceSpans[0].Resource.Attributes
	if !hasAttr(attrs, "routeiq.tenant.id", "org-2") {
		t.Error("missing routeiq.tenant.id")
	}
	for _, a := range attrs {
		if a.Key == "routeiq.workspace.id" {
			t.Error("routeiq.workspace.id should not be set when WorkspaceID is empty")
		}
	}
}

func TestInjectAttrs_NilResource(t *testing.T) {
	req := &tracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracesdatapb.ResourceSpans{
			{Resource: nil},
		},
	}
	InjectAttrs(req, KeyInfo{OrgID: "org-3"})

	if req.ResourceSpans[0].Resource == nil {
		t.Error("Resource should be initialized by InjectAttrs")
	}
}

func hasAttr(attrs []*commonpb.KeyValue, key, val string) bool {
	for _, a := range attrs {
		if a.Key == key {
			sv, ok := a.Value.Value.(*commonpb.AnyValue_StringValue)
			return ok && sv.StringValue == val
		}
	}
	return false
}
