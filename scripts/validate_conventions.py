"""
Validates conventions/telemetry.yaml:
- Every entry has proto_field_path, otel_attribute_key, type.
- otel_attribute_key starts with "routeiq.".
- No duplicate otel_attribute_key values.
- No duplicate proto_field_path values.
"""
import sys
import yaml

with open("conventions/telemetry.yaml") as f:
    data = yaml.safe_load(f)

conventions = data.get("conventions", [])
if not conventions:
    print("ERROR: conventions list is empty")
    sys.exit(1)

otel_keys = []
field_paths = []
errors = []

for i, c in enumerate(conventions):
    row = f"row {i + 1} ({c.get('proto_field_path', '?')})"

    for required_key in ("proto_field_path", "otel_attribute_key", "type"):
        if not c.get(required_key):
            errors.append(f"{row}: missing required key '{required_key}'")

    key = c.get("otel_attribute_key", "")
    if key and not key.startswith("routeiq."):
        errors.append(f"{row}: otel_attribute_key must start with 'routeiq.' — got '{key}'")

    if key in otel_keys:
        errors.append(f"{row}: duplicate otel_attribute_key '{key}'")
    otel_keys.append(key)

    path = c.get("proto_field_path", "")
    if path in field_paths:
        errors.append(f"{row}: duplicate proto_field_path '{path}'")
    field_paths.append(path)

if errors:
    for e in errors:
        print(f"ERROR: {e}")
    sys.exit(1)

print(f"OK: {len(conventions)} conventions validated")
