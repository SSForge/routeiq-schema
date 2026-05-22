#!/usr/bin/env bash
# Verifies the generated Java SDK.
# Compiles the generated .java files against the protobuf runtime jar.
# Downloads protobuf-java if not present.
set -euo pipefail

WORKSPACE="$(cd "$(dirname "$0")/.." && pwd)"
JAVA_OUT="$WORKSPACE/out/java/src/main/java"
LIB_DIR="$WORKSPACE/.sdk-test-libs"
PROTO_JAR="$LIB_DIR/protobuf-java-4.29.3.jar"

mkdir -p "$LIB_DIR"

if [[ ! -f "$PROTO_JAR" ]]; then
  echo "Downloading protobuf-java runtime..."
  curl -fsSL \
    "https://repo1.maven.org/maven2/com/google/protobuf/protobuf-java/4.29.3/protobuf-java-4.29.3.jar" \
    -o "$PROTO_JAR"
fi

JAVA_FILES=$(find "$JAVA_OUT" -name "*.java")
FILE_COUNT=$(echo "$JAVA_FILES" | wc -l | tr -d ' ')

echo "Compiling $FILE_COUNT Java source files..."
javac -cp "$PROTO_JAR" -d "$LIB_DIR/java-classes" $JAVA_FILES

# Verify key classes are in the compiled output.
EXPECTED_CLASSES=(
  "build/buf/gen/ssforge/routeiq/routeiq/v1/telemetry/EventsProto.class"
  "build/buf/gen/ssforge/routeiq/routeiq/v1/telemetry/EntitiesProto.class"
)
for cls in "${EXPECTED_CLASSES[@]}"; do
  if [[ ! -f "$LIB_DIR/java-classes/$cls" ]]; then
    echo "ERROR: compiled class not found: $cls"
    exit 1
  fi
done

echo "Java SDK: OK — $FILE_COUNT files compiled, key classes verified"
