/**
 * Verifies the generated TypeScript SDK:
 * - All 12 message types are exported from events_pb.js.
 * - All 12 type declarations are in events_pb.d.ts.
 * - Key fields appear in the declaration file (proves field codegen was correct).
 */
const fs = require("fs");

const jsPath = "out/typescript/src/proto/routeiq/v1/telemetry/events_pb.js";
const dtsPath = "out/typescript/src/proto/routeiq/v1/telemetry/events_pb.d.ts";

const js = fs.readFileSync(jsPath, "utf8");
const dts = fs.readFileSync(dtsPath, "utf8");

const MESSAGE_TYPES = [
  "AgentEvent",
  "VersionLineage",
  "TaskEvent",
  "ToolCallEvent",
  "StepEvent",
  "DecisionEvent",
  "RetrievalEvent",
  "MemoryEvent",
  "HandoffEvent",
  "PolicyEvent",
  "InterventionEvent",
  "RecoveryEvent",
  "StateSnapshotEvent",
];

const KEY_FIELDS = [
  "taskId",      // AgentEvent.task_id  (camelCase in TS)
  "cacheHit",    // RetrievalEvent.cache_hit — the worked-example field
  "completionStatus",
  "interventionType",
  "recoverType", // will catch if field name drifted (should be recoveryType)
  "snapshotId",
];

// Actually check recoveryType not recoverType:
const CORRECT_FIELDS = [
  "taskId",
  "cacheHit",
  "completionStatus",
  "interventionType",
  "recoveryType",
  "snapshotId",
];

let errors = [];

// Check all message types present in JS
MESSAGE_TYPES.forEach((t) => {
  if (!js.includes(t)) {
    errors.push(`JS missing type: ${t}`);
  }
});

// Check all message types present in .d.ts
MESSAGE_TYPES.forEach((t) => {
  if (!dts.includes(t)) {
    errors.push(`.d.ts missing type: ${t}`);
  }
});

// Check key camelCase field names appear in .d.ts
CORRECT_FIELDS.forEach((f) => {
  if (!dts.includes(f)) {
    errors.push(`.d.ts missing field: ${f}`);
  }
});

if (errors.length > 0) {
  errors.forEach((e) => console.error("ERROR:", e));
  process.exit(1);
}

console.log(
  `TypeScript SDK: OK — ${MESSAGE_TYPES.length} message types and ${CORRECT_FIELDS.length} key fields verified`
);
