#!/usr/bin/env bash
# Verifies the generated C# SDK across all five namespaces.
# Creates a minimal .NET project, copies all generated .cs files, and builds.
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

# Copy all generated .cs files into the project.
cp "$CS_OUT"/*.cs .

# Add Google.Protobuf NuGet package (needed by generated code).
dotnet add package Google.Protobuf --version 3.29.3 --no-restore

dotnet restore --quiet
dotnet build --no-restore --configuration Release --quiet

# Verify key class names appear in each namespace's generated source.
declare -A CHECKS
CHECKS["$CS_OUT/Events.cs"]="AgentEvent TaskEvent StepEvent RetrievalEvent StateSnapshotEvent"
CHECKS["$CS_OUT/Definitions.cs"]="MetricDefinition Formula"
CHECKS["$CS_OUT/Alerts.cs"]="AlertRule Condition"
CHECKS["$CS_OUT/Guardrail.cs"]="CheckGuardrailRequest GuardrailVerdict"
CHECKS["$CS_OUT/Organization.cs"]="Organization Workspace"
CHECKS["$CS_OUT/Identity.cs"]="User ApiKey"

for file in "${!CHECKS[@]}"; do
  for t in ${CHECKS[$file]}; do
    if ! grep -q "class $t" "$file"; then
      echo "ERROR: $(basename "$file") missing class: $t"
      exit 1
    fi
  done
done

echo "C# SDK: OK — dotnet build passed, all 5 namespace files verified"
