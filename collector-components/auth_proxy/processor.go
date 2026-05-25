package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	scopeIngestOnly       = "INGEST_ONLY"
	scopeIngestAndControl = "INGEST_AND_CONTROL"
)

// supabaseRow mirrors a row from the api_keys table.
type supabaseRow struct {
	OrgID       string  `json:"organization_id"`
	WorkspaceID *string `json:"workspace_id"`
	Scope       string  `json:"scope"`
	ExpiresAt   *string `json:"expires_at"`
	RevokedAt   *string `json:"revoked_at"`
}

// AuthProcessor validates API keys and injects tenant attributes.
type AuthProcessor struct {
	cache      *Cache
	supaURL    string
	supaKey    string
	httpClient *http.Client
	logRejects bool
}

func NewAuthProcessor(cfg Config) *AuthProcessor {
	return &AuthProcessor{
		cache:      NewCache(cfg.CacheTTL),
		supaURL:    cfg.SupabaseURL,
		supaKey:    cfg.SupabaseKey,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		logRejects: cfg.LogRejects,
	}
}

// Authenticate validates the Bearer token. Returns KeyInfo on success or a gRPC status error.
func (a *AuthProcessor) Authenticate(ctx context.Context, token string) (KeyInfo, error) {
	if token == "" {
		return KeyInfo{}, status.Error(codes.Unauthenticated, "missing Authorization header")
	}
	hash := sha256Hex(token)
	if info, ok := a.cache.Get(hash); ok {
		return info, nil
	}
	info, err := a.supabaseLookup(ctx, hash)
	if err != nil {
		if a.logRejects {
			prefix := token
			if len(prefix) > 12 {
				prefix = prefix[:12]
			}
			log.Printf("REJECT key_prefix=%s reason=%v", prefix, err)
		}
		return KeyInfo{}, err
	}
	a.cache.Set(hash, info)
	return info, nil
}

func (a *AuthProcessor) supabaseLookup(ctx context.Context, hash string) (KeyInfo, error) {
	q := url.Values{}
	q.Set("key_hash", "eq."+hash)
	q.Set("select", "organization_id,workspace_id,scope,expires_at,revoked_at")
	reqURL := a.supaURL + "/rest/v1/api_keys?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return KeyInfo{}, status.Errorf(codes.Internal, "build request: %v", err)
	}
	req.Header.Set("apikey", a.supaKey)
	req.Header.Set("Authorization", "Bearer "+a.supaKey)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return KeyInfo{}, status.Errorf(codes.Unavailable, "supabase unreachable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return KeyInfo{}, status.Errorf(codes.Unavailable, "supabase returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return KeyInfo{}, status.Errorf(codes.Internal, "read supabase response: %v", err)
	}
	var rows []supabaseRow
	if err := json.Unmarshal(body, &rows); err != nil {
		return KeyInfo{}, status.Errorf(codes.Internal, "parse supabase response: %v", err)
	}
	if len(rows) == 0 {
		return KeyInfo{}, status.Error(codes.Unauthenticated, "api key not found")
	}

	row := rows[0]
	if row.RevokedAt != nil {
		return KeyInfo{}, status.Error(codes.Unauthenticated, "api key revoked")
	}
	if row.ExpiresAt != nil {
		exp, parseErr := time.Parse(time.RFC3339, *row.ExpiresAt)
		if parseErr != nil {
			return KeyInfo{}, status.Errorf(codes.Internal, "invalid expires_at format: %v", parseErr)
		}
		if time.Now().After(exp) {
			return KeyInfo{}, status.Error(codes.Unauthenticated, "api key expired")
		}
	}
	if row.Scope != scopeIngestOnly && row.Scope != scopeIngestAndControl {
		return KeyInfo{}, status.Error(codes.PermissionDenied, "scope not allowed for ingest")
	}

	info := KeyInfo{OrgID: row.OrgID, Scope: row.Scope}
	if row.WorkspaceID != nil {
		info.WorkspaceID = *row.WorkspaceID
	}
	return info, nil
}

// InjectAttrs adds routeiq.tenant.id and optionally routeiq.workspace.id to every ResourceSpans entry.
func InjectAttrs(req *tracepb.ExportTraceServiceRequest, info KeyInfo) {
	for _, rs := range req.ResourceSpans {
		if rs.Resource == nil {
			rs.Resource = &resourcepb.Resource{}
		}
		rs.Resource.Attributes = append(rs.Resource.Attributes, kv("routeiq.tenant.id", info.OrgID))
		if info.WorkspaceID != "" {
			rs.Resource.Attributes = append(rs.Resource.Attributes, kv("routeiq.workspace.id", info.WorkspaceID))
		}
	}
}

// InjectAttrsJSON injects tenant attributes directly into a raw OTel JSON body.
// It works with any bytes encoding (hex or base64) since it never touches the bytes fields.
func InjectAttrsJSON(body []byte, info KeyInfo) ([]byte, error) {
	var root map[string]interface{}
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, err
	}
	rsList, _ := root["resourceSpans"].([]interface{})
	for _, rs := range rsList {
		rsMap, ok := rs.(map[string]interface{})
		if !ok {
			continue
		}
		resource, _ := rsMap["resource"].(map[string]interface{})
		if resource == nil {
			resource = map[string]interface{}{}
			rsMap["resource"] = resource
		}
		attrs, _ := resource["attributes"].([]interface{})
		attrs = append(attrs, map[string]interface{}{
			"key":   "routeiq.tenant.id",
			"value": map[string]interface{}{"stringValue": info.OrgID},
		})
		if info.WorkspaceID != "" {
			attrs = append(attrs, map[string]interface{}{
				"key":   "routeiq.workspace.id",
				"value": map[string]interface{}{"stringValue": info.WorkspaceID},
			})
		}
		resource["attributes"] = attrs
	}
	return json.Marshal(root)
}

// ExtractBearer returns the token from an "Authorization: Bearer <token>" header value.
func ExtractBearer(authHeader string) string {
	if after, ok := strings.CutPrefix(authHeader, "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	return ""
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func kv(key, val string) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key:   key,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: val}},
	}
}
