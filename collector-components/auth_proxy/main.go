package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	tracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// ── gRPC handler ──────────────────────────────────────────────────────────────

type grpcHandler struct {
	tracepb.UnimplementedTraceServiceServer
	proc     *AuthProcessor
	upstream tracepb.TraceServiceClient
}

func (h *grpcHandler) Export(ctx context.Context, req *tracepb.ExportTraceServiceRequest) (*tracepb.ExportTraceServiceResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	vals := md.Get("authorization")
	var raw string
	if len(vals) > 0 {
		raw = vals[0]
	}
	token := ExtractBearer(raw)

	info, err := h.proc.Authenticate(ctx, token)
	if err != nil {
		return nil, err
	}
	InjectAttrs(req, info)
	resp, err := h.upstream.Export(ctx, req)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "upstream: %v", err)
	}
	return resp, nil
}

// ── HTTP handler ──────────────────────────────────────────────────────────────

type httpHandler struct {
	proc        *AuthProcessor
	upstreamURL string
	client      *http.Client
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.URL.Path != "/v1/traces" {
		http.NotFound(w, r)
		return
	}

	token := ExtractBearer(r.Header.Get("Authorization"))
	info, err := h.proc.Authenticate(r.Context(), token)
	if err != nil {
		st, _ := status.FromError(err)
		switch st.Code() {
		case codes.Unauthenticated:
			http.Error(w, st.Message(), http.StatusUnauthorized)
		case codes.PermissionDenied:
			http.Error(w, st.Message(), http.StatusForbidden)
		case codes.Unavailable:
			http.Error(w, st.Message(), http.StatusServiceUnavailable)
		default:
			http.Error(w, st.Message(), http.StatusInternalServerError)
		}
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	isJSON := strings.Contains(r.Header.Get("Content-Type"), "application/json")

	var upBody []byte
	var upContentType string

	if isJSON {
		out, err := InjectAttrsJSON(body, info)
		if err != nil {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}
		upBody = out
		upContentType = "application/json"
	} else {
		var req tracepb.ExportTraceServiceRequest
		if err := proto.Unmarshal(body, &req); err != nil {
			http.Error(w, "invalid protobuf body", http.StatusBadRequest)
			return
		}
		InjectAttrs(&req, info)
		out, err := proto.Marshal(&req)
		if err != nil {
			http.Error(w, "internal marshal error", http.StatusInternalServerError)
			return
		}
		upBody = out
		upContentType = "application/x-protobuf"
	}

	upResp, err := h.client.Post(h.upstreamURL+"/v1/traces", upContentType, bytes.NewReader(upBody))
	if err != nil {
		http.Error(w, "upstream collector unreachable", http.StatusServiceUnavailable)
		return
	}
	defer upResp.Body.Close()

	respBody, _ := io.ReadAll(upResp.Body)
	w.Header().Set("Content-Type", upResp.Header.Get("Content-Type"))
	w.WriteHeader(upResp.StatusCode)
	w.Write(respBody)
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	cfg, err := Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	proc := NewAuthProcessor(cfg)

	upConn, err := grpc.NewClient(cfg.UpstreamGRPC,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial upstream gRPC: %v", err)
	}
	defer upConn.Close()
	upClient := tracepb.NewTraceServiceClient(upConn)

	grpcSrv := grpc.NewServer()
	tracepb.RegisterTraceServiceServer(grpcSrv, &grpcHandler{proc: proc, upstream: upClient})

	grpcLis, err := net.Listen("tcp", ":4317")
	if err != nil {
		log.Fatalf("listen :4317: %v", err)
	}

	httpSrv := &http.Server{
		Addr: ":4318",
		Handler: &httpHandler{
			proc:        proc,
			upstreamURL: "http://" + cfg.UpstreamHTTP,
			client:      &http.Client{},
		},
	}

	go func() {
		log.Println("gRPC auth proxy listening on :4317")
		if err := grpcSrv.Serve(grpcLis); err != nil {
			log.Fatalf("gRPC serve: %v", err)
		}
	}()

	go func() {
		log.Println("HTTP auth proxy listening on :4318")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP serve: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	<-sig
	log.Println("shutting down auth proxy")
	grpcSrv.GracefulStop()
	httpSrv.Shutdown(context.Background())
}
