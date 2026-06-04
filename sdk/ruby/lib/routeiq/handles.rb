require "digest"
require "json"
require "opentelemetry/sdk"

module RouteIQ
  COMPLETION_SUCCESS = "1"
  COMPLETION_FAILURE = "2"
  TOOL_SUCCESS = "1"
  TOOL_FAILURE = "2"

  PERMISSION = {
    "READ_ONLY"  => "1",
    "READ_WRITE" => "2",
    "PRIVILEGED" => "3"
  }.freeze

  # ── ToolHandle ──────────────────────────────────────────────────────────────

  class ToolHandle
    def initialize(step, name, args: {}, permission: "READ_ONLY")
      @step       = step
      @name       = name
      @args       = args
      @permission = permission
      @start      = Process.clock_gettime(Process::CLOCK_MONOTONIC)
      @done       = false

      args_hash = Digest::SHA256.hexdigest(JSON.generate(args.sort.to_h))[0, 16]
      perm      = PERMISSION.fetch(permission, "1")
      riq       = step.task.riq

      step.task.record_tool(name)

      @span = riq.tracer.start_span("tool:#{name}", attributes: {
        "routeiq.event.type"             => "7",
        **riq.envelope(step.task, step),
        "routeiq.tool.name"              => name,
        "routeiq.tool.arguments_hash"    => args_hash,
        "routeiq.tool.permission_level"  => perm
      })
    end

    def success(latency_ms: nil, tokens_in: nil, tokens_out: nil)
      finish(TOOL_SUCCESS, latency_ms: latency_ms, tokens_in: tokens_in, tokens_out: tokens_out)
    end

    def fail(error_code: "", latency_ms: nil, retry_count: nil, tokens_in: nil, tokens_out: nil)
      finish(TOOL_FAILURE, error_code: error_code, latency_ms: latency_ms,
             retry_count: retry_count, tokens_in: tokens_in, tokens_out: tokens_out)
    end

    def end_span
      @span.finish unless @span.nil?
    end

    private

    def finish(status, error_code: "", latency_ms: nil, retry_count: nil,
               tokens_in: nil, tokens_out: nil)
      return if @done
      @done = true
      elapsed = (Process.clock_gettime(Process::CLOCK_MONOTONIC) - @start) * 1000
      attrs = {
        "routeiq.tool.result_status" => status,
        "routeiq.tool.latency_ms"    => latency_ms || elapsed
      }
      attrs["routeiq.tool.error_code"]  = error_code  unless error_code.to_s.empty?
      attrs["routeiq.tool.retry_count"] = retry_count  if retry_count
      attrs["routeiq.tool.tokens_in"]   = tokens_in    if tokens_in
      attrs["routeiq.tool.tokens_out"]  = tokens_out   if tokens_out
      @span&.add_attributes(attrs)
    end
  end

  # ── StepHandle ──────────────────────────────────────────────────────────────

  class StepHandle
    attr_reader :step_id, :task

    def initialize(task, action: nil, rationale: nil, model: nil, index: 1)
      @task     = task
      @step_id  = SecureRandom.uuid
      @done     = false
      riq       = task.riq

      attrs = {
        "routeiq.event.type"  => "4",
        **riq.envelope(task, self),
        "routeiq.step.index"  => index
      }
      attrs["routeiq.step.selected_action"]   = action    if action
      attrs["routeiq.step.action_rationale"]  = rationale if rationale
      attrs["routeiq.step.model"]             = model     if model

      @span = riq.tracer.start_span("step:#{@step_id}", attributes: attrs)
    end

    def tool(name, args: {}, permission: "READ_ONLY", &block)
      handle = ToolHandle.new(self, name, args: args, permission: permission)
      if block_given?
        begin
          block.call(handle)
          handle.success unless handle.instance_variable_get(:@done)
        rescue => e
          handle.fail unless handle.instance_variable_get(:@done)
          raise
        ensure
          handle.end_span
        end
      else
        handle
      end
    end

    def guardrail(type, blocked)
      riq = @task.riq
      span = riq.tracer.start_span("guardrail:#{type}", attributes: {
        "routeiq.event.type"       => "9",
        **riq.envelope(@task, self),
        "routeiq.guardrail.type"    => type,
        "routeiq.guardrail.blocked" => blocked.to_s
      })
      span.finish
    end

    def replan(reason)
      @span&.add_attributes(
        "routeiq.replan.triggered" => "true",
        "routeiq.replan.reason"    => reason[0, 256]
      )
    end

    def complete(tokens_in: nil, tokens_out: nil)
      finish(COMPLETION_SUCCESS, tokens_in: tokens_in, tokens_out: tokens_out)
    end

    def fail(category: "")
      finish(COMPLETION_FAILURE, failure_category: category)
    end

    def end_span
      @span.finish unless @span.nil?
    end

    private

    def finish(status, failure_category: "", tokens_in: nil, tokens_out: nil)
      return if @done
      @done = true
      attrs = {"routeiq.step.completion_status" => status}
      attrs["routeiq.step.failure_category"] = failure_category unless failure_category.to_s.empty?
      attrs["routeiq.step.tokens_in"]        = tokens_in        if tokens_in
      attrs["routeiq.step.tokens_out"]       = tokens_out       if tokens_out
      @span&.add_attributes(attrs)
    end
  end

  # ── TaskHandle ──────────────────────────────────────────────────────────────

  class TaskHandle
    attr_reader :task_id, :run_id, :riq

    def initialize(riq, intent, task_type: nil)
      @riq          = riq
      @intent       = intent
      @task_type    = task_type
      @task_id      = SecureRandom.uuid
      @run_id       = SecureRandom.uuid
      @done         = false
      @step_index   = 0
      @tool_sequence = []

      attrs = {
        "routeiq.event.type"        => "1",
        **riq.envelope(self),
        "routeiq.task.input_intent" => intent[0, 256]
      }
      attrs["routeiq.task.type"] = task_type if task_type

      @span = riq.tracer.start_span("task:#{@task_id}", attributes: attrs)
    end

    def record_tool(name)
      @tool_sequence << name
    end

    def max_same_tool_count
      return 0 if @tool_sequence.empty?
      max_count = cur = 1
      (1...@tool_sequence.length).each do |i|
        cur = @tool_sequence[i] == @tool_sequence[i - 1] ? cur + 1 : 1
        max_count = cur if cur > max_count
      end
      max_count
    end

    def step(action: nil, rationale: nil, model: nil, &block)
      @step_index += 1
      handle = StepHandle.new(self, action: action, rationale: rationale,
                              model: model, index: @step_index)
      if block_given?
        begin
          block.call(handle)
          handle.complete unless handle.instance_variable_get(:@done)
        rescue => e
          handle.fail unless handle.instance_variable_get(:@done)
          raise
        ensure
          handle.end_span
        end
      else
        handle
      end
    end

    def escalate(reason: nil, target: nil)
      riq = @riq
      span = riq.tracer.start_span("escalation:#{@task_id}", attributes: {
        "routeiq.event.type"             => "8",
        **riq.envelope(self),
        "routeiq.escalation.triggered"   => "true"
      }.tap do |attrs|
        attrs["routeiq.escalation.reason"] = reason[0, 256] if reason
        attrs["routeiq.escalation.target"] = target          if target
      end)
      span.finish
    end

    def complete(tokens: 0, tokens_in: nil, tokens_out: nil, cost_usd: nil, cohort: nil)
      finish(COMPLETION_SUCCESS, tokens: tokens, tokens_in: tokens_in,
             tokens_out: tokens_out, cost_usd: cost_usd, cohort: cohort)
    end

    def fail(category: "")
      finish(COMPLETION_FAILURE, failure_category: category)
    end

    def end_span
      @span.finish unless @span.nil?
    end

    private

    def finish(status, tokens: 0, tokens_in: nil, tokens_out: nil,
               cost_usd: nil, cohort: nil, failure_category: "")
      return if @done
      @done = true
      attrs = {"routeiq.task.completion_status" => status}
      attrs["routeiq.task.tokens_in"]      = tokens_in   if tokens_in
      attrs["routeiq.task.tokens_out"]     = tokens_out  if tokens_out
      total = tokens > 0 ? tokens : ((tokens_in || 0) + (tokens_out || 0))
      attrs["routeiq.task.total_tokens"]   = total       if total > 0
      attrs["routeiq.task.cost_usd"]       = cost_usd    if cost_usd
      attrs["routeiq.task.cohort"]         = cohort      if cohort
      attrs["routeiq.task.failure_category"] = failure_category unless failure_category.to_s.empty?
      same = max_same_tool_count
      attrs["routeiq.same_tool_count"] = same if same > 1
      @span&.add_attributes(attrs)
    end
  end
end
