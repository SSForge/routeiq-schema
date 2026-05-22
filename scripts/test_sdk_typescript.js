/**
 * Verifies the generated TypeScript SDK across all five namespaces:
 * telemetry, metrics, insights, control, admin.
 */
const fs = require("fs");
const path = require("path");

const TS_OUT = "out/typescript/src/proto/routeiq/v1";

function readFile(relPath) {
  const full = path.join(TS_OUT, relPath);
  if (!fs.existsSync(full)) {
    console.error(`ERROR: file not found: ${full}`);
    process.exit(1);
  }
  return fs.readFileSync(full, "utf8");
}

let errors = [];

function expectSymbols(label, content, symbols) {
  for (const s of symbols) {
    if (!content.includes(s)) {
      errors.push(`${label} missing: ${s}`);
    }
  }
}

// ── Telemetry ────────────────────────────────────────────────────────────────
const eventsJs = readFile("telemetry/events_pb.js");
const eventsDts = readFile("telemetry/events_pb.d.ts");

expectSymbols("telemetry/events_pb.js", eventsJs, [
  "AgentEvent", "TaskEvent", "ToolCallEvent", "StepEvent", "DecisionEvent",
  "RetrievalEvent", "MemoryEvent", "HandoffEvent", "PolicyEvent",
  "InterventionEvent", "RecoveryEvent", "StateSnapshotEvent",
]);
expectSymbols("telemetry/events_pb.d.ts", eventsDts, [
  "taskId", "cacheHit", "completionStatus", "interventionType",
  "recoveryType", "snapshotId",
]);

// ── Metrics ──────────────────────────────────────────────────────────────────
const metricsJs = readFile("metrics/definitions_pb.js");
const metricsDts = readFile("metrics/definitions_pb.d.ts");

expectSymbols("metrics/definitions_pb.js", metricsJs, [
  "MetricDefinition", "Formula", "DeterministicFormula",
  "HeuristicFormula", "SemanticFormula", "FrontierFormula",
]);
expectSymbols("metrics/definitions_pb.d.ts", metricsDts, [
  "MetricDefinition", "Formula",
]);

// ── Insights ─────────────────────────────────────────────────────────────────
const alertsJs = readFile("insights/alerts_pb.js");
const sloJs = readFile("insights/slo_pb.js");

expectSymbols("insights/alerts_pb.js", alertsJs, ["AlertRule", "Condition"]);
expectSymbols("insights/slo_pb.js", sloJs, ["SloTarget", "SloStatus"]);

// ── Control ──────────────────────────────────────────────────────────────────
const guardrailJs = readFile("control/guardrail_pb.js");
const escalationJs = readFile("control/escalation_pb.js");

expectSymbols("control/guardrail_pb.js", guardrailJs, ["GuardrailVerdict"]);
expectSymbols("control/escalation_pb.js", escalationJs, ["EscalateRequest"]);

// Connect stubs exist for control and admin
const guardrailConnect = readFile("control/guardrail_connect.js");
const orgConnect = readFile("admin/organization_connect.js");
expectSymbols("control/guardrail_connect.js", guardrailConnect, ["GuardrailService"]);
expectSymbols("admin/organization_connect.js", orgConnect, ["OrganizationService"]);

// ── Admin ────────────────────────────────────────────────────────────────────
const orgJs = readFile("admin/organization_pb.js");
const identityJs = readFile("admin/identity_pb.js");

expectSymbols("admin/organization_pb.js", orgJs, ["Organization", "Workspace"]);
expectSymbols("admin/identity_pb.js", identityJs, ["User", "ApiKey"]);

if (errors.length > 0) {
  errors.forEach((e) => console.error("ERROR:", e));
  process.exit(1);
}

console.log("TypeScript SDK: OK — telemetry, metrics, insights, control, admin all verified");
