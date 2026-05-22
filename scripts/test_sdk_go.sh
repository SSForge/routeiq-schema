#!/usr/bin/env bash
# Verifies the generated Go SDK compiles cleanly.
# Creates a temporary go.mod, runs go build ./..., then cleans up.
set -euo pipefail

WORKSPACE="$(cd "$(dirname "$0")/.." && pwd)"
GO_OUT="$WORKSPACE/out/go"

cleanup() { rm -f "$GO_OUT/go.mod" "$GO_OUT/go.sum"; }
trap cleanup EXIT

# Inject a minimal go.mod so `go build` can resolve google.golang.org/protobuf.
cat > "$GO_OUT/go.mod" << 'EOF'
module routeiq.dev/sdk-verify
go 1.23
require google.golang.org/protobuf v1.36.5
EOF

cd "$GO_OUT"
go mod tidy -e 2>/dev/null || true  # fetch deps; -e tolerates minor issues

# Build all packages under out/go — catches type errors, missing imports, etc.
go build ./...

# Spot-check: vet flags suspicious constructs beyond plain compile errors.
go vet ./...

echo "Go SDK: OK — go build and go vet passed"
