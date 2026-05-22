# Verifies the generated Ruby SDK across all five namespaces:
# telemetry, metrics, insights, control, admin.

$LOAD_PATH.unshift("out/ruby/lib")

require "google/protobuf"

# ── Telemetry ─────────────────────────────────────────────────────────────────
load "out/ruby/lib/routeiq/proto/routeiq/v1/telemetry/entities_pb.rb"
load "out/ruby/lib/routeiq/proto/routeiq/v1/telemetry/events_pb.rb"

telemetry_classes = %w[
  Routeiq::V1::Telemetry::AgentEvent
  Routeiq::V1::Telemetry::TaskEvent
  Routeiq::V1::Telemetry::ToolCallEvent
  Routeiq::V1::Telemetry::StepEvent
  Routeiq::V1::Telemetry::DecisionEvent
  Routeiq::V1::Telemetry::RetrievalEvent
  Routeiq::V1::Telemetry::MemoryEvent
  Routeiq::V1::Telemetry::HandoffEvent
  Routeiq::V1::Telemetry::PolicyEvent
  Routeiq::V1::Telemetry::InterventionEvent
  Routeiq::V1::Telemetry::RecoveryEvent
  Routeiq::V1::Telemetry::StateSnapshotEvent
]

errors = []

telemetry_classes.each do |klass_name|
  begin
    klass = Object.const_get(klass_name)
    klass.new
  rescue NameError, NoMethodError => e
    errors << "#{klass_name}: #{e.message}"
  end
end

r = Routeiq::V1::Telemetry::RetrievalEvent.new(cache_hit: true)
errors << "RetrievalEvent.cache_hit field not working" unless r.cache_hit == true

puts "  telemetry: OK (#{telemetry_classes.size} classes)"

# ── Metrics ───────────────────────────────────────────────────────────────────
load "out/ruby/lib/routeiq/proto/routeiq/v1/metrics/definitions_pb.rb"

begin
  m = Routeiq::V1::Metrics::MetricDefinition.new(id: "loop_rate", unit: "percent")
  errors << "MetricDefinition.id wrong" unless m.id == "loop_rate"

  f = Routeiq::V1::Metrics::Formula.new
  det = Routeiq::V1::Metrics::DeterministicFormula.new(expression: "count(*)")
  f.deterministic = det
  errors << "Formula.deterministic wrong" unless f.deterministic.expression == "count(*)"

  heur = Routeiq::V1::Metrics::HeuristicFormula.new(detector_id: "loop_detector_v1")
  errors << "HeuristicFormula.detector_id wrong" unless heur.detector_id == "loop_detector_v1"
rescue => e
  errors << "metrics: #{e.message}"
end

puts "  metrics: OK"

# ── Insights ──────────────────────────────────────────────────────────────────
load "out/ruby/lib/routeiq/proto/routeiq/v1/insights/alerts_pb.rb"
load "out/ruby/lib/routeiq/proto/routeiq/v1/insights/slo_pb.rb"

begin
  rule = Routeiq::V1::Insights::AlertRule.new(id: "high-loop-rate", metric_id: "loop_rate", enabled: true)
  errors << "AlertRule.metric_id wrong" unless rule.metric_id == "loop_rate"

  slo = Routeiq::V1::Insights::SloTarget.new(metric_id: "task_success_rate", target: 0.95)
  errors << "SloTarget.target wrong" unless slo.target == 0.95
rescue => e
  errors << "insights: #{e.message}"
end

puts "  insights: OK"

# ── Control ───────────────────────────────────────────────────────────────────
load "out/ruby/lib/routeiq/proto/routeiq/v1/control/guardrail_pb.rb"
load "out/ruby/lib/routeiq/proto/routeiq/v1/control/escalation_pb.rb"

begin
  req = Routeiq::V1::Control::CheckGuardrailRequest.new(
    session_id: "sess-001",
    proposed_action: "delete_file"
  )
  errors << "CheckGuardrailRequest.session_id wrong" unless req.session_id == "sess-001"

  esc = Routeiq::V1::Control::EscalateRequest.new(
    session_id: "sess-001",
    reason: "task stuck"
  )
  errors << "EscalateRequest.reason wrong" unless esc.reason == "task stuck"
rescue => e
  errors << "control: #{e.message}"
end

puts "  control: OK"

# ── Admin ─────────────────────────────────────────────────────────────────────
load "out/ruby/lib/routeiq/proto/routeiq/v1/admin/organization_pb.rb"
load "out/ruby/lib/routeiq/proto/routeiq/v1/admin/identity_pb.rb"

begin
  org = Routeiq::V1::Admin::Organization.new(id: "org-001", name: "Acme Corp")
  errors << "Organization.name wrong" unless org.name == "Acme Corp"

  user = Routeiq::V1::Admin::User.new(id: "user-001", email: "alice@acme.com")
  errors << "User.email wrong" unless user.email == "alice@acme.com"

  key = Routeiq::V1::Admin::ApiKey.new(id: "key-001", organization_id: "org-001")
  errors << "ApiKey.id wrong" unless key.id == "key-001"
rescue => e
  errors << "admin: #{e.message}"
end

puts "  admin: OK"

if errors.any?
  errors.each { |e| puts "ERROR: #{e}" }
  exit 1
end

puts "Ruby SDK: OK — telemetry, metrics, insights, control, admin all verified"
