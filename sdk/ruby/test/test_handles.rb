require "minitest/autorun"
require "securerandom"
require "opentelemetry/sdk"
require_relative "../lib/routeiq/version"
require_relative "../lib/routeiq/handles"

# Minimal stub that acts like a RouteIQ client for testing handles
class TestRiq
  attr_reader :session_id, :tracer

  def initialize(system_id: nil, user_id: nil, slo_success_target: nil, slo_p95_ms_target: nil)
    @session_id         = SecureRandom.uuid
    @agent_id           = "test-agent"
    @tenant_id          = "test-tenant"
    @environment        = "test"
    @model              = "gpt-4o"
    @agent_version      = "1.0.0"
    @system_id          = system_id
    @user_id            = user_id
    @slo_success_target = slo_success_target
    @slo_p95_ms_target  = slo_p95_ms_target

    @exporter = OpenTelemetry::SDK::Trace::Export::InMemorySpanExporter.new
    provider  = OpenTelemetry::SDK::Trace::TracerProvider.new
    provider.add_span_processor(
      OpenTelemetry::SDK::Trace::Export::SimpleSpanProcessor.new(@exporter)
    )
    @tracer = provider.tracer("routeiq.sdk", RouteIQ::VERSION)
  end

  def spans
    @exporter.finished_spans
  end

  def envelope(task = nil, step = nil)
    attrs = {
      "routeiq.agent.id"    => @agent_id,
      "routeiq.tenant.id"   => @tenant_id,
      "routeiq.environment" => @environment,
      "routeiq.session.id"  => @session_id
    }
    attrs["routeiq.system.id"]          = @system_id          if @system_id
    attrs["routeiq.user.id"]            = @user_id            if @user_id
    attrs["routeiq.slo.success_target"] = @slo_success_target if @slo_success_target
    attrs["routeiq.slo.p95_ms_target"]  = @slo_p95_ms_target  if @slo_p95_ms_target
    if task
      attrs["routeiq.task.id"] = task.task_id
      attrs["routeiq.run.id"]  = task.run_id
    end
    attrs["routeiq.step.id"]            = step.step_id if step
    attrs["routeiq.version.model.name"] = @model        if @model
    attrs["routeiq.version.agent"]      = @agent_version
    attrs
  end
end

class HandlesTest < Minitest::Test
  def setup
    @riq = TestRiq.new
  end

  def spans_by_prefix(prefix)
    @riq.spans.select { |s| s.name.start_with?(prefix) }
  end

  def attr(span, key)
    span.attributes[key]
  end

  # ── TaskHandle ────────────────────────────────────────────────────────────

  def test_task_span_name_starts_with_task
    task = RouteIQ::TaskHandle.new(@riq, "find Paris")
    task.end_span
    assert spans_by_prefix("task:").any?, "expected span starting with task:"
  end

  def test_task_envelope_attrs
    task = RouteIQ::TaskHandle.new(@riq, "find Paris")
    task_id = task.task_id
    task.end_span

    span = spans_by_prefix("task:").first
    assert_equal "test-agent",        attr(span, "routeiq.agent.id")
    assert_equal @riq.session_id,     attr(span, "routeiq.session.id")
    assert_equal task_id,             attr(span, "routeiq.task.id")
    assert_equal "find Paris",        attr(span, "routeiq.task.input_intent")
    assert_equal "gpt-4o",            attr(span, "routeiq.version.model.name")
  end

  def test_task_complete_sets_success
    task = RouteIQ::TaskHandle.new(@riq, "q")
    task.complete(tokens: 100, cost_usd: 0.001, cohort: "test")
    task.end_span

    span = spans_by_prefix("task:").first
    assert_equal "1",     attr(span, "routeiq.task.completion_status")
    assert_equal 100,     attr(span, "routeiq.task.total_tokens")
    assert_equal 0.001,   attr(span, "routeiq.task.cost_usd")
    assert_equal "test",  attr(span, "routeiq.task.cohort")
  end

  def test_task_fail_sets_failure
    task = RouteIQ::TaskHandle.new(@riq, "q")
    task.fail(category: "tool_error")
    task.end_span

    span = spans_by_prefix("task:").first
    assert_equal "2",          attr(span, "routeiq.task.completion_status")
    assert_equal "tool_error", attr(span, "routeiq.task.failure_category")
  end

  def test_task_auto_succeeds_with_block
    RouteIQ::TaskHandle.new(@riq, "q").tap do |task|
      # simulate block-style: complete + end_span
      task.complete
      task.end_span
    end

    span = spans_by_prefix("task:").first
    assert_equal "1", attr(span, "routeiq.task.completion_status")
  end

  # ── StepHandle ────────────────────────────────────────────────────────────

  def test_step_span_name_starts_with_step
    task = RouteIQ::TaskHandle.new(@riq, "q")
    step = RouteIQ::StepHandle.new(task, action: "tool_call")
    step.end_span
    task.end_span

    assert spans_by_prefix("step:").any?, "expected span starting with step:"
  end

  def test_step_carries_task_id
    task = RouteIQ::TaskHandle.new(@riq, "q")
    step = RouteIQ::StepHandle.new(task)
    step_id = step.step_id
    step.end_span
    task.end_span

    span = spans_by_prefix("step:").first
    assert_equal task.task_id, attr(span, "routeiq.task.id")
    assert_equal step_id,      attr(span, "routeiq.step.id")
  end

  def test_step_index_increments
    task = RouteIQ::TaskHandle.new(@riq, "q")
    s1 = RouteIQ::StepHandle.new(task, index: 1)
    s1.end_span
    s2 = RouteIQ::StepHandle.new(task, index: 2)
    s2.end_span
    task.end_span

    indices = spans_by_prefix("step:").map { |s| attr(s, "routeiq.step.index") }.sort
    assert_equal [1, 2], indices
  end

  # ── ToolHandle ────────────────────────────────────────────────────────────

  def test_tool_span_name
    task = RouteIQ::TaskHandle.new(@riq, "q")
    step = RouteIQ::StepHandle.new(task)
    tool = RouteIQ::ToolHandle.new(step, "search", args: {"query" => "Paris"})
    tool.success
    tool.end_span
    step.end_span
    task.end_span

    assert @riq.spans.any? { |s| s.name == "tool:search" }, "expected tool:search span"
  end

  def test_tool_success
    task = RouteIQ::TaskHandle.new(@riq, "q")
    step = RouteIQ::StepHandle.new(task)
    tool = RouteIQ::ToolHandle.new(step, "search")
    tool.success(latency_ms: 50.0)
    tool.end_span
    step.end_span
    task.end_span

    span = @riq.spans.find { |s| s.name == "tool:search" }
    assert_equal "1",   attr(span, "routeiq.tool.result_status")
    assert_equal 50.0,  attr(span, "routeiq.tool.latency_ms")
  end

  def test_tool_fail
    task = RouteIQ::TaskHandle.new(@riq, "q")
    step = RouteIQ::StepHandle.new(task)
    tool = RouteIQ::ToolHandle.new(step, "search")
    tool.fail(error_code: "TIMEOUT")
    tool.end_span
    step.end_span
    task.end_span

    span = @riq.spans.find { |s| s.name == "tool:search" }
    assert_equal "2",       attr(span, "routeiq.tool.result_status")
    assert_equal "TIMEOUT", attr(span, "routeiq.tool.error_code")
  end

  def test_tool_arguments_hash_length
    task = RouteIQ::TaskHandle.new(@riq, "q")
    step = RouteIQ::StepHandle.new(task)
    tool = RouteIQ::ToolHandle.new(step, "search", args: {"query" => "Paris"})
    tool.success
    tool.end_span
    step.end_span
    task.end_span

    span = @riq.spans.find { |s| s.name == "tool:search" }
    assert_equal 16, attr(span, "routeiq.tool.arguments_hash").length
  end

  def test_tool_permission_level
    task = RouteIQ::TaskHandle.new(@riq, "q")
    step = RouteIQ::StepHandle.new(task)
    tool = RouteIQ::ToolHandle.new(step, "write_file", permission: "READ_WRITE")
    tool.success
    tool.end_span
    step.end_span
    task.end_span

    span = @riq.spans.find { |s| s.name == "tool:write_file" }
    assert_equal "2", attr(span, "routeiq.tool.permission_level")
  end

  def test_session_id_consistent_across_spans
    task = RouteIQ::TaskHandle.new(@riq, "q")
    step = RouteIQ::StepHandle.new(task)
    tool = RouteIQ::ToolHandle.new(step, "search")
    tool.success
    tool.end_span
    step.end_span
    task.end_span

    session_ids = @riq.spans.map { |s| attr(s, "routeiq.session.id") }.uniq
    assert_equal 1, session_ids.length
    assert_equal @riq.session_id, session_ids.first
  end

  # ── v0.3.0 signals ──────────────────────────────────────────────────────────

  def test_tool_retry_count
    task = RouteIQ::TaskHandle.new(@riq, "q")
    step = RouteIQ::StepHandle.new(task)
    tool = RouteIQ::ToolHandle.new(step, "db_query")
    tool.fail(error_code: "TIMEOUT", retry_count: 3)
    tool.end_span
    step.end_span
    task.end_span

    span = @riq.spans.find { |s| s.name == "tool:db_query" }
    assert_equal 3, attr(span, "routeiq.tool.retry_count")
  end

  def test_tool_token_split
    task = RouteIQ::TaskHandle.new(@riq, "q")
    step = RouteIQ::StepHandle.new(task)
    tool = RouteIQ::ToolHandle.new(step, "llm")
    tool.success(tokens_in: 100, tokens_out: 200)
    tool.end_span
    step.end_span
    task.end_span

    span = @riq.spans.find { |s| s.name == "tool:llm" }
    assert_equal 100, attr(span, "routeiq.tool.tokens_in")
    assert_equal 200, attr(span, "routeiq.tool.tokens_out")
  end

  def test_same_tool_count
    task = RouteIQ::TaskHandle.new(@riq, "q")
    4.times do |i|
      step = RouteIQ::StepHandle.new(task, index: i + 1)
      tool = RouteIQ::ToolHandle.new(step, "search")
      tool.success
      tool.end_span
      step.complete
      step.end_span
    end
    task.complete
    task.end_span

    span = spans_by_prefix("task:").first
    assert_equal 4, attr(span, "routeiq.same_tool_count")
  end

  def test_same_tool_count_not_emitted_for_distinct
    task = RouteIQ::TaskHandle.new(@riq, "q")
    s1 = RouteIQ::StepHandle.new(task, index: 1)
    RouteIQ::ToolHandle.new(s1, "search").tap { |t| t.success; t.end_span }
    s1.complete; s1.end_span
    s2 = RouteIQ::StepHandle.new(task, index: 2)
    RouteIQ::ToolHandle.new(s2, "write").tap { |t| t.success; t.end_span }
    s2.complete; s2.end_span
    task.complete
    task.end_span

    span = spans_by_prefix("task:").first
    assert_nil attr(span, "routeiq.same_tool_count")
  end

  def test_escalation
    task = RouteIQ::TaskHandle.new(@riq, "refund")
    task.escalate(reason: "amount_too_large", target: "human_review")
    task.end_span

    span = @riq.spans.find { |s| s.name.start_with?("escalation:") }
    assert_equal "true",             attr(span, "routeiq.escalation.triggered")
    assert_equal "amount_too_large", attr(span, "routeiq.escalation.reason")
    assert_equal "human_review",     attr(span, "routeiq.escalation.target")
  end

  def test_guardrail
    task = RouteIQ::TaskHandle.new(@riq, "q")
    step = RouteIQ::StepHandle.new(task)
    step.guardrail("pii_filter", true)
    step.end_span
    task.end_span

    span = @riq.spans.find { |s| s.name.start_with?("guardrail:") }
    assert_equal "pii_filter", attr(span, "routeiq.guardrail.type")
    assert_equal "true",       attr(span, "routeiq.guardrail.blocked")
  end

  def test_replan
    task = RouteIQ::TaskHandle.new(@riq, "q")
    step = RouteIQ::StepHandle.new(task, action: "search")
    step.replan("search_failed_switching_to_cache")
    step.end_span
    task.end_span

    span = spans_by_prefix("step:").first
    assert_equal "true",                             attr(span, "routeiq.replan.triggered")
    assert_equal "search_failed_switching_to_cache", attr(span, "routeiq.replan.reason")
  end

  def test_step_model_override
    task = RouteIQ::TaskHandle.new(@riq, "q")
    step = RouteIQ::StepHandle.new(task, model: "claude-opus-4-5")
    step.end_span
    task.end_span

    span = spans_by_prefix("step:").first
    assert_equal "claude-opus-4-5", attr(span, "routeiq.step.model")
  end

  def test_task_token_split_auto_sums
    task = RouteIQ::TaskHandle.new(@riq, "q")
    task.complete(tokens_in: 300, tokens_out: 700)
    task.end_span

    span = spans_by_prefix("task:").first
    assert_equal 300,  attr(span, "routeiq.task.tokens_in")
    assert_equal 700,  attr(span, "routeiq.task.tokens_out")
    assert_equal 1000, attr(span, "routeiq.task.total_tokens")
  end

  def test_system_id_and_slo_targets
    riq = TestRiq.new(system_id: "checkout-bot", user_id: "user_42",
                      slo_success_target: 0.95, slo_p95_ms_target: 2000.0)
    task = RouteIQ::TaskHandle.new(riq, "q")
    task.end_span

    span = riq.spans.find { |s| s.name.start_with?("task:") }
    assert_equal "checkout-bot", attr(span, "routeiq.system.id")
    assert_equal "user_42",      attr(span, "routeiq.user.id")
    assert_in_delta 0.95,   attr(span, "routeiq.slo.success_target"), 0.001
    assert_in_delta 2000.0, attr(span, "routeiq.slo.p95_ms_target"),  0.001
  end
end
