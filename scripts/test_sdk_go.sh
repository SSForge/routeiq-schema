#!/usr/bin/env bash
# Verifies the generated Go SDK compiles cleanly.
# Creates a temporary go.mod, runs go build ./..., then cleans up.
set -euo pipefail

WORKSPACE="$(cd "$(dirname "$0")/.." && pwd)"
GO_OUT="$WORKSPACE/out/go"

cleanup() { rm -f "$GO_OUT/go.mod" "$GO_OUT/go.sum"; }
trap cleanup EXIT

# Inject a go.mod whose module path matches the BSR-generated import paths so
# cross-package imports (e.g. metrics importing telemetry) resolve locally.
cat > "$GO_OUT/go.mod" << 'EOF'
module buf.build/gen/go/ssforge/routeiq/protocolbuffers/go
go 1.23
require (
	google.golang.org/protobuf v1.36.5
	connectrpc.com/connect v1.20.0
)
EOF

cd "$GO_OUT"
go mod tidy -e 2>/dev/null || true  # fetch deps; -e tolerates minor issues

# Build all packages under out/go — catches type errors, missing imports, etc.
go build ./...

# Spot-check: vet flags suspicious constructs beyond plain compile errors.
go vet ./...

echo "Go SDK: OK — go build and go vet passed"
