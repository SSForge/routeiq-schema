# routeiq-schema

Protobuf schema, semantic conventions, heuristic detectors, judge rubrics, and SDK generation pipeline for RouteIQ.

## What's here

| Directory | Contents |
|---|---|
| `proto/routeiq/v1/telemetry/` | Agent event envelope and payload types |
| `proto/routeiq/v1/metrics/` | Metric definition shapes (Phase 3) |
| `proto/routeiq/v1/insights/` | SLO, alert, dashboard shapes (Phase 4) |
| `proto/routeiq/v1/control/` | Runtime guardrail, escalation, session RPCs (Phase 4) |
| `proto/routeiq/v1/admin/` | Org, identity, policy, billing (Phase 5) |
| `conventions/telemetry.yaml` | OTel attribute key mappings |
| `detectors/` | Heuristic detector implementations (Python) |
| `rubrics/` | LLM judge prompts (Markdown) |
| `classifiers/` | Semantic classifier configs (YAML) |
| `collector-components/auth_proxy/` | Auth proxy sidecar (Go) — validates API keys, injects tenant attributes |
| `out/` | Generated SDK bindings — committed, language-agnostic |

## Quickstart

```bash
buf dep update          # resolve locked deps
buf lint                # schema style check
buf generate            # regenerate out/
```

## Adding a field

1. Add the field to the relevant `.proto` file in `proto/routeiq/v1/telemetry/`
2. Add its mapping row to `conventions/telemetry.yaml`
3. Open a PR — CI validates both sides of the mapping

## License

Apache 2.0. See [LICENSE](LICENSE).
