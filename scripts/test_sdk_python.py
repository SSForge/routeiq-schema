"""
Verifies the generated Python SDK across all five namespaces:
telemetry, metrics, insights, control, admin.

Requires: protobuf>=7.0 (Python 3.12+). BSR's protocolbuffers/python plugin
generates gencode version 7.x; the runtime must be >= gencode version.
"""
import sys

if sys.version_info < (3, 12):
    print(f"SKIP: Python SDK test requires Python 3.12+ (running {sys.version}). Passes in CI.")
    sys.exit(0)

# BSR Python layout: packages live under out/python/src/routeiq/_proto/
# but generated files import from routeiq.v1.* (without the _proto prefix).
# Adding the _proto dir to sys.path makes those imports resolve.
sys.path.insert(0, "out/python/src")
sys.path.insert(0, "out/python/src/routeiq/_proto")

# ── Telemetry ─────────────────────────────────────────────────────────────────
from routeiq._proto.routeiq.v1.telemetry import events_pb2, entities_pb2

env = events_pb2.AgentEvent()
env.task_id = "task-001"
env.agent_id = "agent-001"
env.tenant_id = "acme"
env.environment = "prod"
assert env.task_id == "task-001"

task = events_pb2.TaskEvent()
task.completion_status = entities_pb2.SUCCESS
task.total_cost_usd = 0.042
task.total_tokens = 1024
task.human_escalated = False
assert task.completion_status == entities_pb2.SUCCESS
env.task.CopyFrom(task)

tool = events_pb2.ToolCallEvent()
tool.tool_name = "bash"
tool.result_status = entities_pb2.TOOL_SUCCESS
tool.latency_ms = 120.5
assert tool.tool_name == "bash"

step = events_pb2.StepEvent()
step.selected_action = "call_tool"
step.completion_status = entities_pb2.SUCCESS
step.step_index = 3
step.retry_count = 0
assert step.step_index == 3

decision = events_pb2.DecisionEvent()
decision.decision_type = "tool_selection"
decision.chosen_option = "bash"
decision.confidence = 0.92
assert decision.confidence == 0.92

retrieval = events_pb2.RetrievalEvent()
retrieval.retrieval_type = "vector"
retrieval.results_count = 5
retrieval.top_score = 0.87
retrieval.cache_hit = True
assert retrieval.cache_hit is True

memory = events_pb2.MemoryEvent()
memory.memory_type = "short_term"
memory.operation = "write"
memory.entries_affected = 1
assert memory.operation == "write"

handoff = events_pb2.HandoffEvent()
handoff.from_agent_id = "agent-a"
handoff.to_agent_id = "agent-b"
handoff.status = entities_pb2.SUCCESS
assert handoff.to_agent_id == "agent-b"

policy = events_pb2.PolicyEvent()
policy.policy_id = "pol-001"
policy.verdict = "allow"
policy.rules_triggered.append("rule-pii")
assert policy.verdict == "allow"

intervention = events_pb2.InterventionEvent()
intervention.intervention_type = "circuit_break"
intervention.automated = True
assert intervention.automated is True

recovery = events_pb2.RecoveryEvent()
recovery.recovery_type = "retry"
recovery.succeeded = True
recovery.attempt_number = 2
assert recovery.attempt_number == 2

snapshot = events_pb2.StateSnapshotEvent()
snapshot.snapshot_type = "checkpoint"
snapshot.state_size_bytes = 4096
snapshot.snapshot_id = "snap-001"
assert snapshot.state_size_bytes == 4096

print("  telemetry: OK")

# ── Metrics ───────────────────────────────────────────────────────────────────
from routeiq._proto.routeiq.v1.metrics import definitions_pb2 as metrics_pb2

m = metrics_pb2.MetricDefinition()
m.id = "p95_latency"
m.display_name = "P95 Latency"
m.layer = metrics_pb2.LAYER_0_RUNTIME
m.evidence_class = metrics_pb2.OTEL_DIRECT
m.unit = "ms"
m.default_aggregation = metrics_pb2.P95
assert m.id == "p95_latency"
assert m.default_aggregation == metrics_pb2.P95

f = metrics_pb2.Formula()
det = metrics_pb2.DeterministicFormula()
det.expression = "p95(latency_ms)"
f.deterministic.CopyFrom(det)
assert f.WhichOneof("kind") == "deterministic"

heur = metrics_pb2.HeuristicFormula()
heur.detector_id = "loop_detector_v1"
heur.parameters["window_steps"] = "5"
assert heur.detector_id == "loop_detector_v1"

frontier = metrics_pb2.FrontierFormula()
frontier.judge_model = "claude-opus-4-7"
frontier.rubric_id = "outcome_correctness_v1"
assert frontier.rubric_id == "outcome_correctness_v1"

print("  metrics: OK")

# ── Insights ──────────────────────────────────────────────────────────────────
from routeiq._proto.routeiq.v1.insights import alerts_pb2, slo_pb2

rule = alerts_pb2.AlertRule()
rule.id = "high-loop-rate"
rule.metric_id = "loop_rate"
rule.severity = alerts_pb2.CRITICAL
rule.enabled = True
rule.actions.append(alerts_pb2.CIRCUIT_BREAK)
assert rule.severity == alerts_pb2.CRITICAL

cond = alerts_pb2.Condition()
cond.op = alerts_pb2.GT
cond.threshold = 0.1
cond.window = "5m"
cond.min_samples = 100
rule.condition.CopyFrom(cond)
assert rule.condition.threshold == 0.1

slo = slo_pb2.SloTarget()
slo.metric_id = "task_success_rate"
slo.target = 0.95
slo.window = "30d"
assert slo.target == 0.95

print("  insights: OK")

# ── Control ───────────────────────────────────────────────────────────────────
from routeiq._proto.routeiq.v1.control import guardrail_pb2, escalation_pb2

req = guardrail_pb2.CheckGuardrailRequest()
req.session_id = "sess-001"
req.proposed_action = "delete_file"
req.permission_level = entities_pb2.PRIVILEGED
req.risk_context = "irreversible action"
assert req.session_id == "sess-001"

resp = guardrail_pb2.CheckGuardrailResponse()
resp.verdict = guardrail_pb2.BLOCK
resp.reason = "high-risk action blocked"
assert resp.verdict == guardrail_pb2.BLOCK

esc = escalation_pb2.EscalateRequest()
esc.session_id = "sess-001"
esc.reason = "task stuck for 10 steps"
esc.urgency = escalation_pb2.HIGH
assert esc.urgency == escalation_pb2.HIGH

print("  control: OK")

# ── Admin ─────────────────────────────────────────────────────────────────────
from routeiq._proto.routeiq.v1.admin import organization_pb2, identity_pb2

org = organization_pb2.Organization()
org.id = "org-001"
org.name = "Acme Corp"
org.tier = organization_pb2.TEAM
assert org.name == "Acme Corp"

ws = organization_pb2.Workspace()
ws.id = "ws-001"
ws.organization_id = "org-001"
ws.name = "prod"
assert ws.name == "prod"

user = identity_pb2.User()
user.id = "user-001"
user.email = "alice@acme.com"
user.display_name = "Alice"
assert user.email == "alice@acme.com"

key = identity_pb2.ApiKey()
key.id = "key-001"
key.organization_id = "org-001"
key.scope = identity_pb2.INGEST_ONLY
assert key.scope == identity_pb2.INGEST_ONLY

print("  admin: OK")

print("Python SDK: OK — telemetry, metrics, insights, control, admin all verified")
