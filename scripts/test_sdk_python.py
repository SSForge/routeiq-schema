"""
Verifies the generated Python SDK:
- All 11 AgentEvent payload message types are importable and instantiable.
- Key fields on each type are accessible (proves codegen produced the right fields).
- Enum values resolve correctly (CompletionStatus.SUCCESS, etc.)

Requires: protobuf>=7.0 (Python 3.12+). BSR's protocolbuffers/python plugin
generates gencode version 7.x; the runtime must be >= gencode version.
"""
import sys

if sys.version_info < (3, 12):
    print(f"SKIP: Python SDK test requires Python 3.12+ (running {sys.version}). Passes in CI.")
    sys.exit(0)

sys.path.insert(0, "out/python/src")

from routeiq._proto.routeiq.v1.telemetry import events_pb2, entities_pb2

# ── Envelope ──────────────────────────────────────────────────────────────
env = events_pb2.AgentEvent()
env.task_id = "task-001"
env.agent_id = "agent-001"
env.tenant_id = "acme"
env.environment = "prod"
assert env.task_id == "task-001"

# ── TaskEvent ─────────────────────────────────────────────────────────────
task = events_pb2.TaskEvent()
task.completion_status = entities_pb2.SUCCESS
task.total_cost_usd = 0.042
task.total_tokens = 1024
task.human_escalated = False
assert task.completion_status == entities_pb2.SUCCESS
env.task.CopyFrom(task)

# ── ToolCallEvent ─────────────────────────────────────────────────────────
tool = events_pb2.ToolCallEvent()
tool.tool_name = "bash"
tool.result_status = entities_pb2.TOOL_SUCCESS
tool.latency_ms = 120.5
assert tool.tool_name == "bash"

# ── StepEvent ─────────────────────────────────────────────────────────────
step = events_pb2.StepEvent()
step.selected_action = "call_tool"
step.completion_status = entities_pb2.SUCCESS
step.step_index = 3
step.retry_count = 0
assert step.step_index == 3

# ── DecisionEvent ─────────────────────────────────────────────────────────
decision = events_pb2.DecisionEvent()
decision.decision_type = "tool_selection"
decision.chosen_option = "bash"
decision.confidence = 0.92
assert decision.confidence == 0.92

# ── RetrievalEvent (cache_hit is the worked-example field) ────────────────
retrieval = events_pb2.RetrievalEvent()
retrieval.retrieval_type = "vector"
retrieval.results_count = 5
retrieval.top_score = 0.87
retrieval.cache_hit = True
assert retrieval.cache_hit is True

# ── MemoryEvent ───────────────────────────────────────────────────────────
memory = events_pb2.MemoryEvent()
memory.memory_type = "short_term"
memory.operation = "write"
memory.entries_affected = 1
assert memory.operation == "write"

# ── HandoffEvent ──────────────────────────────────────────────────────────
handoff = events_pb2.HandoffEvent()
handoff.from_agent_id = "agent-a"
handoff.to_agent_id = "agent-b"
handoff.status = entities_pb2.SUCCESS
assert handoff.to_agent_id == "agent-b"

# ── PolicyEvent ───────────────────────────────────────────────────────────
policy = events_pb2.PolicyEvent()
policy.policy_id = "pol-001"
policy.verdict = "allow"
policy.rules_triggered.append("rule-pii")
assert policy.verdict == "allow"
assert len(policy.rules_triggered) == 1

# ── InterventionEvent ─────────────────────────────────────────────────────
intervention = events_pb2.InterventionEvent()
intervention.intervention_type = "circuit_break"
intervention.automated = True
assert intervention.automated is True

# ── RecoveryEvent ─────────────────────────────────────────────────────────
recovery = events_pb2.RecoveryEvent()
recovery.recovery_type = "retry"
recovery.succeeded = True
recovery.attempt_number = 2
assert recovery.attempt_number == 2

# ── StateSnapshotEvent ────────────────────────────────────────────────────
snapshot = events_pb2.StateSnapshotEvent()
snapshot.snapshot_type = "checkpoint"
snapshot.state_size_bytes = 4096
snapshot.snapshot_id = "snap-001"
assert snapshot.state_size_bytes == 4096

print("Python SDK: OK — all 11 AgentEvent payload types verified")
