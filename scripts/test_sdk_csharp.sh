#!/usr/bin/env bash
# Verifies the generated C# SDK.
# Creates a minimal .NET console project, copies the generated .cs files in,
# and runs `dotnet build`.
set -euo pipefail

WORKSPACE="$(cd "$(dirname "$0")/.." && pwd)"
CS_OUT="$WORKSPACE/out/csharp/src/RouteIQ/Proto"
TMP_DIR="$(mktemp -d)"

cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

cd "$TMP_DIR"
dotnet new classlib -n RouteIQSdkVerify --no-restore -o . --framework net8.0 --quiet

# Remove the placeholder Class1.cs that dotnet new creates.
rm -f Class1.cs

# Copy generated .cs files into the project.
cp "$CS_OUT"/*.cs .

# Add Google.Protobuf NuGet package (needed by generated code).
dotnet add package Google.Protobuf --version 3.29.3 --no-restore

dotnet restore --quiet
dotnet build --no-restore --configuration Release --quiet

# Verify key type names appear in the generated C# source.
EVENTS_CS="$CS_OUT/Events.cs"
EXPECTED_TYPES=(
  "AgentEvent"
  "TaskEvent"
  "StepEvent"
  "RetrievalEvent"
  "InterventionEvent"
  "StateSnapshotEvent"
)
for t in "${EXPECTED_TYPES[@]}"; do
  if ! grep -q "class $t" "$EVENTS_CS"; then
    echo "ERROR: C# generated file missing class: $t"
    exit 1
  fi
done

echo "C# SDK: OK — dotnet build passed, ${#EXPECTED_TYPES[@]} key classes verified"
