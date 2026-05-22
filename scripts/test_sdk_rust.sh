#!/usr/bin/env bash
# Verifies the generated Rust SDK across all five namespaces.
# Creates a minimal Cargo project that depends on prost, includes all
# generated source files, and compiles with `cargo check`.
set -euo pipefail

WORKSPACE="$(cd "$(dirname "$0")/.." && pwd)"
RUST_BASE="$WORKSPACE/out/rust/src/proto/routeiq/v1"
TMP_DIR="$(mktemp -d)"

cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

TELEMETRY_RS="$RUST_BASE/telemetry/routeiq.v1.telemetry.rs"
METRICS_RS="$RUST_BASE/metrics/routeiq.v1.metrics.rs"
INSIGHTS_RS="$RUST_BASE/insights/routeiq.v1.insights.rs"
CONTROL_RS="$RUST_BASE/control/routeiq.v1.control.rs"
ADMIN_RS="$RUST_BASE/admin/routeiq.v1.admin.rs"

for f in "$TELEMETRY_RS" "$METRICS_RS" "$INSIGHTS_RS" "$CONTROL_RS" "$ADMIN_RS"; do
  if [[ ! -f "$f" ]]; then
    echo "ERROR: generated Rust file not found: $f"
    exit 1
  fi
done

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

# Include all five generated files — same pattern as prost users.
cat > "$TMP_DIR/src/lib.rs" << RUST
include!("$(realpath "$TELEMETRY_RS")");
include!("$(realpath "$METRICS_RS")");
include!("$(realpath "$INSIGHTS_RS")");
include!("$(realpath "$CONTROL_RS")");
include!("$(realpath "$ADMIN_RS")");
RUST

cd "$TMP_DIR"
cargo check --quiet 2>&1

# Verify key struct names in each generated file.
declare -A CHECKS
CHECKS["$TELEMETRY_RS"]="AgentEvent TaskEvent StepEvent RetrievalEvent StateSnapshotEvent"
CHECKS["$METRICS_RS"]="MetricDefinition Formula DeterministicFormula HeuristicFormula FrontierFormula"
CHECKS["$INSIGHTS_RS"]="AlertRule Condition SloTarget"
CHECKS["$CONTROL_RS"]="CheckGuardrailRequest GuardrailVerdict EscalateRequest"
CHECKS["$ADMIN_RS"]="Organization User ApiKey"

for file in "${!CHECKS[@]}"; do
  for s in ${CHECKS[$file]}; do
    if ! grep -q "pub struct $s" "$file"; then
      echo "ERROR: $(basename "$file") missing struct: $s"
      exit 1
    fi
  done
done

echo "Rust SDK: OK — cargo check passed, all 5 namespace files verified"
