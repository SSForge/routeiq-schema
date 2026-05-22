#!/usr/bin/env bash
# Verifies the generated Rust SDK.
# Creates a minimal Cargo project that depends on prost, imports the generated
# source file, and compiles it with `cargo check`.
set -euo pipefail

WORKSPACE="$(cd "$(dirname "$0")/.." && pwd)"
RUST_FILE="$WORKSPACE/out/rust/src/proto/routeiq/v1/telemetry/routeiq.v1.telemetry.rs"
TMP_DIR="$(mktemp -d)"

cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

# Build a minimal Cargo project that includes the generated source.
mkdir -p "$TMP_DIR/src"

cat > "$TMP_DIR/Cargo.toml" << 'EOF'
[package]
name = "routeiq-sdk-verify"
version = "0.1.0"
edition = "2021"

[dependencies]
prost = "0.13"
prost-types = "0.13"
EOF

# lib.rs just includes the generated file — same pattern as prost users.
cat > "$TMP_DIR/src/lib.rs" << RUST
include!("$(realpath "$RUST_FILE")");
RUST

cd "$TMP_DIR"
cargo check --quiet 2>&1

# Also verify key struct names appear in the generated source.
EXPECTED_STRUCTS=(
  "AgentEvent"
  "TaskEvent"
  "StepEvent"
  "RetrievalEvent"
  "InterventionEvent"
  "StateSnapshotEvent"
)
for s in "${EXPECTED_STRUCTS[@]}"; do
  if ! grep -q "pub struct $s" "$RUST_FILE"; then
    echo "ERROR: Rust generated file missing struct: $s"
    exit 1
  fi
done

echo "Rust SDK: OK — cargo check passed, ${#EXPECTED_STRUCTS[@]} key structs verified"
