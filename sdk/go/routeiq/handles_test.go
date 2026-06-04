package routeiq

import (
	"context"
	"strings"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// ── test helpers ──────────────────────────────────────────────────────────────

type spanRecorder struct {
	mu    sync.Mutex
	spans []sdktrace.ReadOnlySpan
}

func (r *spanRecorder) OnStart(_ context.Context, _ sdktrace.ReadWriteSpan) {}
func (r *spanRecorder) OnEnd(s sdktrace.ReadOnlySpan) {
	r.mu.Lock()
	r.spans = append(r.spans, s)
	r.mu.Unlock()
}
func (r *spanRecorder) Shutdown(_ context.Context) error  { return nil }
func (r *spanRecorder) ForceFlush(_ context.Context) error { return nil }

func makeTestRiq(t *testing.T) (*RouteIQ, *spanRecorder) {
	t.Helper()
	rec := &spanRecorder{}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	riq := newForTest(provider, Options{
		AgentID:      "test-agent",
		TenantID:     "test-tenant",
		Environment:  "test",
		Model:        "gpt-4o",
		AgentVersion: "1.0.0",
	})
	return riq, rec
}

func (r *spanRecorder) all() []sdktrace.ReadOnlySpan {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]sdktrace.ReadOnlySpan, len(r.spans))
	copy(out, r.spans)
	return out
}

func byName(spans []sdktrace.ReadOnlySpan, prefix string) sdktrace.ReadOnlySpan {
	for _, s := range spans {
		if strings.HasPrefix(s.Name(), prefix) {
			return s
		}
	}
	return nil
}

func attrStr(s sdktrace.ReadOnlySpan, key string) string {
	for _, kv := range s.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.Emit()
		}
	}
	return ""
}

func attrInt(s sdktrace.ReadOnlySpan, key attribute.Key) int64 {
	for _, kv := range s.Attributes() {
		if kv.Key == key {
			return kv.Value.AsInt64()
		}
	}
	return 0
}

// ── TaskHandle ────────────────────────────────────────────────────────────────

func TestTaskSpanName(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "find Paris")
	task.End()

	if byName(rec.all(), "task:") == nil {
		t.Fatal("expected span with name starting task:")
	}
}

func TestTaskEnvelopeAttrs(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "find Paris")
	taskID := task.taskID
	task.End()

	span := byName(rec.all(), "task:")
	if span == nil {
		t.Fatal("task span not found")
	}
	if attrStr(span, "routeiq.agent.id") != "test-agent" {
		t.Error("missing agent.id")
	}
	if attrStr(span, "routeiq.session.id") != riq.sessionID {
		t.Error("session_id mismatch")
	}
	if attrStr(span, "routeiq.task.id") != taskID {
		t.Error("task_id mismatch")
	}
	if attrStr(span, "routeiq.task.input_intent") != "find Paris" {
		t.Error("intent mismatch")
	}
	if attrStr(span, "routeiq.version.model.name") != "gpt-4o" {
		t.Error("model mismatch")
	}
}

func TestTaskComplete(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "q")
	task.Complete(WithTokens(100), WithCohort("test"))
	task.End()

	span := byName(rec.all(), "task:")
	if span == nil {
		t.Fatal("task span not found")
	}
	if attrStr(span, "routeiq.task.completion_status") != "1" {
		t.Error("expected success status")
	}
	if attrInt(span, "routeiq.task.total_tokens") != 100 {
		t.Error("tokens mismatch")
	}
	if attrStr(span, "routeiq.task.cohort") != "test" {
		t.Error("cohort mismatch")
	}
}

func TestTaskFail(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "q")
	task.Fail("tool_error")
	task.End()

	span := byName(rec.all(), "task:")
	if span == nil {
		t.Fatal("task span not found")
	}
	if attrStr(span, "routeiq.task.completion_status") != "2" {
		t.Error("expected failure status")
	}
	if attrStr(span, "routeiq.task.failure_category") != "tool_error" {
		t.Error("failure_category mismatch")
	}
}

func TestTaskAutoComplete(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "q")
	task.End()

	span := byName(rec.all(), "task:")
	if span == nil {
		t.Fatal("task span not found")
	}
	if attrStr(span, "routeiq.task.completion_status") != "1" {
		t.Error("expected auto-success on End()")
	}
}

// ── StepHandle ────────────────────────────────────────────────────────────────

func TestStepSpanName(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "q")
	step := task.Step(WithAction("tool_call"))
	step.End()
	task.End()

	if byName(rec.all(), "step:") == nil {
		t.Fatal("expected span with name starting step:")
	}
}

func TestStepCarriesTaskID(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "q")
	step := task.Step()
	stepID := step.stepID
	step.End()
	task.End()

	span := byName(rec.all(), "step:")
	if span == nil {
		t.Fatal("step span not found")
	}
	if attrStr(span, "routeiq.task.id") != task.taskID {
		t.Error("task_id mismatch on step span")
	}
	if attrStr(span, "routeiq.step.id") != stepID {
		t.Error("step_id mismatch")
	}
}

func TestStepIndexIncrements(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "q")
	s1 := task.Step()
	s1.End()
	s2 := task.Step()
	s2.End()
	task.End()

	var indices []int64
	for _, s := range rec.all() {
		if strings.HasPrefix(s.Name(), "step:") {
			indices = append(indices, attrInt(s, "routeiq.step.index"))
		}
	}
	if len(indices) != 2 || indices[0]+indices[1] != 3 {
		t.Errorf("expected step indices {1,2}, got %v", indices)
	}
}

// ── ToolHandle ────────────────────────────────────────────────────────────────

func TestToolSpanName(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "q")
	step := task.Step()
	tool := step.Tool("search", WithArgs(map[string]any{"query": "Paris"}))
	tool.End()
	step.End()
	task.End()

	if byName(rec.all(), "tool:search") == nil {
		t.Fatal("expected span tool:search")
	}
}

func TestToolSuccess(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "q")
	step := task.Step()
	tool := step.Tool("search")
	tool.Success(WithLatencyMs(50.0))
	tool.End()
	step.End()
	task.End()

	span := byName(rec.all(), "tool:search")
	if span == nil {
		t.Fatal("tool span not found")
	}
	if attrStr(span, "routeiq.tool.result_status") != "1" {
		t.Error("expected success status")
	}
	if attrStr(span, "routeiq.tool.latency_ms") == "" {
		t.Error("expected latency_ms to be set")
	}
}

func TestToolFail(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "q")
	step := task.Step()
	tool := step.Tool("search")
	tool.Fail("TIMEOUT")
	tool.End()
	step.End()
	task.End()

	span := byName(rec.all(), "tool:search")
	if span == nil {
		t.Fatal("tool span not found")
	}
	if attrStr(span, "routeiq.tool.result_status") != "2" {
		t.Error("expected failure status")
	}
	if attrStr(span, "routeiq.tool.error_code") != "TIMEOUT" {
		t.Error("error_code mismatch")
	}
}

func TestToolAutoSucceeds(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "q")
	step := task.Step()
	tool := step.Tool("search")
	tool.End()
	step.End()
	task.End()

	span := byName(rec.all(), "tool:search")
	if span == nil {
		t.Fatal("tool span not found")
	}
	if attrStr(span, "routeiq.tool.result_status") != "1" {
		t.Error("expected auto-success on End()")
	}
}

func TestToolArgsHash(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "q")
	step := task.Step()
	tool := step.Tool("search", WithArgs(map[string]any{"query": "Paris"}))
	tool.End()
	step.End()
	task.End()

	span := byName(rec.all(), "tool:search")
	if span == nil {
		t.Fatal("tool span not found")
	}
	h := attrStr(span, "routeiq.tool.arguments_hash")
	if len(h) != 16 {
		t.Errorf("expected 16-char hash, got %q", h)
	}
}

func attrFloat64(s sdktrace.ReadOnlySpan, key string) float64 {
	for _, kv := range s.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsFloat64()
		}
	}
	return 0
}

func TestSessionIDConsistent(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "q")
	step := task.Step()
	tool := step.Tool("search")
	tool.End()
	step.End()
	task.End()

	seen := map[string]bool{}
	for _, s := range rec.all() {
		seen[attrStr(s, "routeiq.session.id")] = true
	}
	if len(seen) != 1 || !seen[riq.sessionID] {
		t.Errorf("session_id inconsistent across spans: %v", seen)
	}
}

// ── v0.3.0 signals ────────────────────────────────────────────────────────────

func TestToolRetryCount(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "q")
	step := task.Step()
	tool := step.Tool("db_query")
	tool.Fail("TIMEOUT", WithRetryCount(3))
	tool.End()
	step.End()
	task.End()

	span := byName(rec.all(), "tool:db_query")
	if span == nil {
		t.Fatal("tool span not found")
	}
	if attrInt(span, "routeiq.tool.retry_count") != 3 {
		t.Errorf("retry_count mismatch, got %d", attrInt(span, "routeiq.tool.retry_count"))
	}
}

func TestToolTokenSplit(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "q")
	step := task.Step()
	tool := step.Tool("llm")
	tool.Success(WithToolTokensIn(100), WithToolTokensOut(200))
	tool.End()
	step.End()
	task.End()

	span := byName(rec.all(), "tool:llm")
	if span == nil {
		t.Fatal("tool span not found")
	}
	if attrInt(span, "routeiq.tool.tokens_in") != 100 {
		t.Error("tokens_in mismatch")
	}
	if attrInt(span, "routeiq.tool.tokens_out") != 200 {
		t.Error("tokens_out mismatch")
	}
}

func TestSameToolCount(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "q")
	for i := 0; i < 4; i++ {
		step := task.Step()
		tool := step.Tool("search")
		tool.End()
		step.End()
	}
	task.Complete()
	task.End()

	span := byName(rec.all(), "task:")
	if span == nil {
		t.Fatal("task span not found")
	}
	if attrInt(span, "routeiq.same_tool_count") != 4 {
		t.Errorf("same_tool_count mismatch, got %d", attrInt(span, "routeiq.same_tool_count"))
	}
}

func TestSameToolCountNotEmittedForDistinct(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "q")
	s1 := task.Step()
	s1.Tool("search").End()
	s1.End()
	s2 := task.Step()
	s2.Tool("write").End()
	s2.End()
	task.Complete()
	task.End()

	span := byName(rec.all(), "task:")
	if span == nil {
		t.Fatal("task span not found")
	}
	for _, kv := range span.Attributes() {
		if string(kv.Key) == "routeiq.same_tool_count" {
			t.Error("same_tool_count should not be emitted for distinct tools")
		}
	}
}

func TestEscalation(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "refund")
	task.Escalate("amount_too_large", "human_review")
	task.End()

	span := byName(rec.all(), "escalation:")
	if span == nil {
		t.Fatal("escalation span not found")
	}
	if attrStr(span, "routeiq.escalation.triggered") != "true" {
		t.Error("escalation.triggered mismatch")
	}
	if attrStr(span, "routeiq.escalation.reason") != "amount_too_large" {
		t.Error("escalation.reason mismatch")
	}
	if attrStr(span, "routeiq.escalation.target") != "human_review" {
		t.Error("escalation.target mismatch")
	}
}

func TestGuardrail(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "q")
	step := task.Step()
	step.Guardrail("pii_filter", true)
	step.End()
	task.End()

	span := byName(rec.all(), "guardrail:")
	if span == nil {
		t.Fatal("guardrail span not found")
	}
	if attrStr(span, "routeiq.guardrail.type") != "pii_filter" {
		t.Error("guardrail.type mismatch")
	}
	if attrStr(span, "routeiq.guardrail.blocked") != "true" {
		t.Error("guardrail.blocked mismatch")
	}
}

func TestReplan(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "q")
	step := task.Step(WithAction("search"))
	step.Replan("search_failed_switching_to_cache")
	step.End()
	task.End()

	span := byName(rec.all(), "step:")
	if span == nil {
		t.Fatal("step span not found")
	}
	if attrStr(span, "routeiq.replan.triggered") != "true" {
		t.Error("replan.triggered mismatch")
	}
	if attrStr(span, "routeiq.replan.reason") != "search_failed_switching_to_cache" {
		t.Error("replan.reason mismatch")
	}
}

func TestStepModelOverride(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "q")
	step := task.Step(WithModel("claude-opus-4-5"))
	step.End()
	task.End()

	span := byName(rec.all(), "step:")
	if span == nil {
		t.Fatal("step span not found")
	}
	if attrStr(span, "routeiq.step.model") != "claude-opus-4-5" {
		t.Errorf("step.model mismatch, got %q", attrStr(span, "routeiq.step.model"))
	}
}

func TestTaskTokenSplit(t *testing.T) {
	riq, rec := makeTestRiq(t)
	task := riq.Task(context.Background(), "q")
	task.Complete(WithTokensIn(300), WithTokensOut(700))
	task.End()

	span := byName(rec.all(), "task:")
	if span == nil {
		t.Fatal("task span not found")
	}
	if attrInt(span, "routeiq.task.tokens_in") != 300 {
		t.Error("tokens_in mismatch")
	}
	if attrInt(span, "routeiq.task.tokens_out") != 700 {
		t.Error("tokens_out mismatch")
	}
	if attrInt(span, "routeiq.task.total_tokens") != 1000 {
		t.Error("total_tokens mismatch")
	}
}

func TestSystemIdAndSloTargets(t *testing.T) {
	rec := &spanRecorder{}
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	sloSuccess := 0.95
	sloP95 := 2000.0
	riq := newForTest(provider, Options{
		AgentID:          "test-agent",
		TenantID:         "test-tenant",
		Environment:      "test",
		Model:            "gpt-4o",
		AgentVersion:     "1.0.0",
		SystemID:         "checkout-bot",
		UserID:           "user_42",
		SLOSuccessTarget: &sloSuccess,
		SLOP95MsTarget:   &sloP95,
	})
	task := riq.Task(context.Background(), "q")
	task.End()

	span := byName(rec.all(), "task:")
	if span == nil {
		t.Fatal("task span not found")
	}
	if attrStr(span, "routeiq.system.id") != "checkout-bot" {
		t.Errorf("system.id mismatch, got %q", attrStr(span, "routeiq.system.id"))
	}
	if attrStr(span, "routeiq.user.id") != "user_42" {
		t.Errorf("user.id mismatch, got %q", attrStr(span, "routeiq.user.id"))
	}
	if attrFloat64(span, "routeiq.slo.success_target") != 0.95 {
		t.Errorf("slo.success_target mismatch, got %v", attrFloat64(span, "routeiq.slo.success_target"))
	}
	if attrFloat64(span, "routeiq.slo.p95_ms_target") != 2000.0 {
		t.Errorf("slo.p95_ms_target mismatch, got %v", attrFloat64(span, "routeiq.slo.p95_ms_target"))
	}
}
