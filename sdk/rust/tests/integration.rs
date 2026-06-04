use opentelemetry::Context;
use opentelemetry_sdk::{
    error::OTelSdkResult,
    trace::{SdkTracerProvider, SpanData, SpanProcessor},
};
use routeiq_sdk::{CompleteOpts, RouteIQ, RouteIQOptions, ToolOpts};
use std::sync::{Arc, Mutex};

// ── SpanRecorder ──────────────────────────────────────────────────────────────

struct SpanRecorder {
    spans: Arc<Mutex<Vec<SpanData>>>,
}

impl std::fmt::Debug for SpanRecorder {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "SpanRecorder")
    }
}

impl Clone for SpanRecorder {
    fn clone(&self) -> Self {
        SpanRecorder { spans: self.spans.clone() }
    }
}

impl Default for SpanRecorder {
    fn default() -> Self {
        SpanRecorder { spans: Arc::new(Mutex::new(Vec::new())) }
    }
}

impl SpanProcessor for SpanRecorder {
    fn on_start(&self, _span: &mut opentelemetry_sdk::trace::Span, _cx: &Context) {}

    fn on_end(&self, span: SpanData) {
        self.spans.lock().unwrap().push(span);
    }

    fn force_flush(&self) -> OTelSdkResult {
        Ok(())
    }

    fn shutdown(&self) -> OTelSdkResult {
        Ok(())
    }
}

// ── Test setup ────────────────────────────────────────────────────────────────

fn make_test_riq() -> (RouteIQ, SpanRecorder) {
    let recorder = SpanRecorder::default();
    let provider = SdkTracerProvider::builder()
        .with_span_processor(recorder.clone())
        .build();
    let opts = RouteIQOptions {
        agent_id: "test-agent".to_string(),
        tenant_id: Some("test-tenant".to_string()),
        environment: Some("test".to_string()),
        model: Some("gpt-4o".to_string()),
        agent_version: Some("1.0.0".to_string()),
        ..Default::default()
    };
    (RouteIQ::with_provider(opts, provider), recorder)
}

fn attr_str(span: &SpanData, key: &str) -> String {
    span.attributes
        .iter()
        .find(|kv| kv.key.as_str() == key)
        .map(|kv| kv.value.as_str().to_string())
        .unwrap_or_default()
}

fn attr_i64(span: &SpanData, key: &str) -> i64 {
    span.attributes
        .iter()
        .find(|kv| kv.key.as_str() == key)
        .and_then(|kv| {
            if let opentelemetry::Value::I64(v) = &kv.value { Some(*v) } else { None }
        })
        .unwrap_or(0)
}

fn attr_f64(span: &SpanData, key: &str) -> f64 {
    span.attributes
        .iter()
        .find(|kv| kv.key.as_str() == key)
        .and_then(|kv| {
            if let opentelemetry::Value::F64(v) = &kv.value { Some(*v) } else { None }
        })
        .unwrap_or(0.0)
}

fn has_attr(span: &SpanData, key: &str) -> bool {
    span.attributes.iter().any(|kv| kv.key.as_str() == key)
}

// ── Tests ─────────────────────────────────────────────────────────────────────

#[test]
fn test_task_span_name() {
    let (riq, rec) = make_test_riq();
    riq.task("find Paris").end();
    let spans = rec.spans.lock().unwrap();
    assert!(spans.iter().any(|s| s.name.starts_with("task:")), "expected task: span");
}

#[test]
fn test_task_envelope_attrs() {
    let (riq, rec) = make_test_riq();
    let session_id = riq.session_id.clone();
    let task = riq.task("find Paris");
    let task_id = task.task_id.clone();
    task.end();

    let spans = rec.spans.lock().unwrap();
    let span = spans.iter().find(|s| s.name.starts_with("task:")).unwrap();
    assert_eq!(attr_str(span, "routeiq.agent.id"), "test-agent");
    assert_eq!(attr_str(span, "routeiq.session.id"), session_id);
    assert_eq!(attr_str(span, "routeiq.task.id"), task_id);
    assert_eq!(attr_str(span, "routeiq.task.input_intent"), "find Paris");
    assert_eq!(attr_str(span, "routeiq.version.model.name"), "gpt-4o");
}

#[test]
fn test_task_complete() {
    let (riq, rec) = make_test_riq();
    let mut task = riq.task("q");
    task.complete(CompleteOpts { tokens: 100, cohort: Some("test".to_string()), ..Default::default() });
    task.end();

    let spans = rec.spans.lock().unwrap();
    let span = spans.iter().find(|s| s.name.starts_with("task:")).unwrap();
    assert_eq!(attr_str(span, "routeiq.task.completion_status"), "1");
    assert_eq!(attr_i64(span, "routeiq.task.total_tokens"), 100);
    assert_eq!(attr_str(span, "routeiq.task.cohort"), "test");
}

#[test]
fn test_task_fail() {
    let (riq, rec) = make_test_riq();
    let mut task = riq.task("q");
    task.fail(Some("tool_error"));
    task.end();

    let spans = rec.spans.lock().unwrap();
    let span = spans.iter().find(|s| s.name.starts_with("task:")).unwrap();
    assert_eq!(attr_str(span, "routeiq.task.completion_status"), "2");
    assert_eq!(attr_str(span, "routeiq.task.failure_category"), "tool_error");
}

#[test]
fn test_task_auto_complete() {
    let (riq, rec) = make_test_riq();
    riq.task("q").end();

    let spans = rec.spans.lock().unwrap();
    let span = spans.iter().find(|s| s.name.starts_with("task:")).unwrap();
    assert_eq!(attr_str(span, "routeiq.task.completion_status"), "1");
}

#[test]
fn test_step_span_name() {
    let (riq, rec) = make_test_riq();
    let mut task = riq.task("q");
    task.step(Some("tool_call"), None).end();
    task.end();

    let spans = rec.spans.lock().unwrap();
    assert!(spans.iter().any(|s| s.name.starts_with("step:")), "expected step: span");
}

#[test]
fn test_step_carries_task_id() {
    let (riq, rec) = make_test_riq();
    let mut task = riq.task("q");
    let task_id = task.task_id.clone();
    let step = task.step(None, None);
    let step_id = step.step_id.clone();
    step.end();
    task.end();

    let spans = rec.spans.lock().unwrap();
    let span = spans.iter().find(|s| s.name.starts_with("step:")).unwrap();
    assert_eq!(attr_str(span, "routeiq.task.id"), task_id);
    assert_eq!(attr_str(span, "routeiq.step.id"), step_id);
}

#[test]
fn test_step_index_increments() {
    let (riq, rec) = make_test_riq();
    let mut task = riq.task("q");
    task.step(None, None).end();
    task.step(None, None).end();
    task.end();

    let spans = rec.spans.lock().unwrap();
    let mut indices: Vec<i64> = spans
        .iter()
        .filter(|s| s.name.starts_with("step:"))
        .map(|s| attr_i64(s, "routeiq.step.index"))
        .collect();
    indices.sort();
    assert_eq!(indices, vec![1, 2]);
}

#[test]
fn test_tool_span_name() {
    let (riq, rec) = make_test_riq();
    let mut task = riq.task("q");
    let step = task.step(None, None);
    step.tool("search", ToolOpts::default()).end();
    step.end();
    task.end();

    let spans = rec.spans.lock().unwrap();
    assert!(spans.iter().any(|s| s.name == "tool:search"));
}

#[test]
fn test_tool_success() {
    let (riq, rec) = make_test_riq();
    let mut task = riq.task("q");
    let step = task.step(None, None);
    let mut tool = step.tool("search", ToolOpts::default());
    tool.success(Some(50.0));
    tool.end();
    step.end();
    task.end();

    let spans = rec.spans.lock().unwrap();
    let span = spans.iter().find(|s| s.name == "tool:search").unwrap();
    assert_eq!(attr_str(span, "routeiq.tool.result_status"), "1");
}

#[test]
fn test_tool_fail() {
    let (riq, rec) = make_test_riq();
    let mut task = riq.task("q");
    let step = task.step(None, None);
    let mut tool = step.tool("search", ToolOpts::default());
    tool.fail(Some("TIMEOUT"), None);
    tool.end();
    step.end();
    task.end();

    let spans = rec.spans.lock().unwrap();
    let span = spans.iter().find(|s| s.name == "tool:search").unwrap();
    assert_eq!(attr_str(span, "routeiq.tool.result_status"), "2");
    assert_eq!(attr_str(span, "routeiq.tool.error_code"), "TIMEOUT");
}

#[test]
fn test_tool_auto_success() {
    let (riq, rec) = make_test_riq();
    let mut task = riq.task("q");
    let step = task.step(None, None);
    step.tool("search", ToolOpts::default()).end();
    step.end();
    task.end();

    let spans = rec.spans.lock().unwrap();
    let span = spans.iter().find(|s| s.name == "tool:search").unwrap();
    assert_eq!(attr_str(span, "routeiq.tool.result_status"), "1");
}

#[test]
fn test_tool_args_hash() {
    let (riq, rec) = make_test_riq();
    let mut task = riq.task("q");
    let step = task.step(None, None);
    let opts = ToolOpts {
        args: Some(serde_json::json!({"query": "Paris"})),
        ..Default::default()
    };
    step.tool("search", opts).end();
    step.end();
    task.end();

    let spans = rec.spans.lock().unwrap();
    let span = spans.iter().find(|s| s.name == "tool:search").unwrap();
    let h = attr_str(span, "routeiq.tool.arguments_hash");
    assert_eq!(h.len(), 16, "expected 16-char hex hash, got {h:?}");
}

#[test]
fn test_session_id_consistent() {
    let (riq, rec) = make_test_riq();
    let session_id = riq.session_id.clone();
    let mut task = riq.task("q");
    let step = task.step(None, None);
    step.tool("search", ToolOpts::default()).end();
    step.end();
    task.end();

    let spans = rec.spans.lock().unwrap();
    let ids: std::collections::HashSet<String> = spans
        .iter()
        .map(|s| attr_str(s, "routeiq.session.id"))
        .collect();
    assert_eq!(ids.len(), 1);
    assert!(ids.contains(&session_id));
}

// ── v0.3.0 signals ────────────────────────────────────────────────────────────

#[test]
fn test_tool_retry_count() {
    let (riq, rec) = make_test_riq();
    let mut task = riq.task("q");
    let step = task.step(None, None);
    let mut tool = step.tool("db_query", ToolOpts::default());
    tool.fail_retry(Some("TIMEOUT"), None, 3);
    tool.end();
    step.end();
    task.end();

    let spans = rec.spans.lock().unwrap();
    let span = spans.iter().find(|s| s.name == "tool:db_query").unwrap();
    assert_eq!(attr_i64(span, "routeiq.tool.retry_count"), 3);
}

#[test]
fn test_tool_token_split() {
    let (riq, rec) = make_test_riq();
    let mut task = riq.task("q");
    let step = task.step(None, None);
    let mut tool = step.tool("llm", ToolOpts::default());
    tool.success_tokens(None, 100, 200);
    tool.end();
    step.end();
    task.end();

    let spans = rec.spans.lock().unwrap();
    let span = spans.iter().find(|s| s.name == "tool:llm").unwrap();
    assert_eq!(attr_i64(span, "routeiq.tool.tokens_in"),  100);
    assert_eq!(attr_i64(span, "routeiq.tool.tokens_out"), 200);
}

#[test]
fn test_same_tool_count() {
    let (riq, rec) = make_test_riq();
    let mut task = riq.task("q");
    for _ in 0..4 {
        let step = task.step(None, None);
        step.tool("search", ToolOpts::default()).end();
        step.end();
    }
    task.complete(CompleteOpts::default());
    task.end();

    let spans = rec.spans.lock().unwrap();
    let span = spans.iter().find(|s| s.name.starts_with("task:")).unwrap();
    assert_eq!(attr_i64(span, "routeiq.same_tool_count"), 4);
}

#[test]
fn test_same_tool_count_not_emitted_for_distinct() {
    let (riq, rec) = make_test_riq();
    let mut task = riq.task("q");
    let step1 = task.step(None, None);
    step1.tool("search", ToolOpts::default()).end();
    step1.end();
    let step2 = task.step(None, None);
    step2.tool("write", ToolOpts::default()).end();
    step2.end();
    task.complete(CompleteOpts::default());
    task.end();

    let spans = rec.spans.lock().unwrap();
    let span = spans.iter().find(|s| s.name.starts_with("task:")).unwrap();
    assert!(!has_attr(span, "routeiq.same_tool_count"),
        "same_tool_count should not be emitted for distinct tools");
}

#[test]
fn test_escalation() {
    let (riq, rec) = make_test_riq();
    let mut task = riq.task("refund");
    task.escalate(Some("amount_too_large"), Some("human_review"));
    task.end();

    let spans = rec.spans.lock().unwrap();
    let span = spans.iter().find(|s| s.name.starts_with("escalation:")).unwrap();
    assert_eq!(attr_str(span, "routeiq.escalation.triggered"), "true");
    assert_eq!(attr_str(span, "routeiq.escalation.reason"),    "amount_too_large");
    assert_eq!(attr_str(span, "routeiq.escalation.target"),    "human_review");
}

#[test]
fn test_guardrail() {
    let (riq, rec) = make_test_riq();
    let mut task = riq.task("q");
    let mut step = task.step(None, None);
    step.guardrail("pii_filter", true);
    step.end();
    task.end();

    let spans = rec.spans.lock().unwrap();
    let span = spans.iter().find(|s| s.name.starts_with("guardrail:")).unwrap();
    assert_eq!(attr_str(span, "routeiq.guardrail.type"),    "pii_filter");
    assert_eq!(attr_str(span, "routeiq.guardrail.blocked"), "true");
}

#[test]
fn test_replan() {
    let (riq, rec) = make_test_riq();
    let mut task = riq.task("q");
    let mut step = task.step(Some("search"), None);
    step.replan("search_failed_switching_to_cache");
    step.end();
    task.end();

    let spans = rec.spans.lock().unwrap();
    let span = spans.iter().find(|s| s.name.starts_with("step:")).unwrap();
    assert_eq!(attr_str(span, "routeiq.replan.triggered"), "true");
    assert_eq!(attr_str(span, "routeiq.replan.reason"),    "search_failed_switching_to_cache");
}

#[test]
fn test_step_model_override() {
    let (riq, rec) = make_test_riq();
    let mut task = riq.task("q");
    task.step_model(None, None, Some("claude-opus-4-5")).end();
    task.end();

    let spans = rec.spans.lock().unwrap();
    let span = spans.iter().find(|s| s.name.starts_with("step:")).unwrap();
    assert_eq!(attr_str(span, "routeiq.step.model"), "claude-opus-4-5");
}

#[test]
fn test_task_token_split() {
    let (riq, rec) = make_test_riq();
    let mut task = riq.task("q");
    task.complete(CompleteOpts { tokens_in: 300, tokens_out: 700, ..Default::default() });
    task.end();

    let spans = rec.spans.lock().unwrap();
    let span = spans.iter().find(|s| s.name.starts_with("task:")).unwrap();
    assert_eq!(attr_i64(span, "routeiq.task.tokens_in"),    300);
    assert_eq!(attr_i64(span, "routeiq.task.tokens_out"),   700);
    assert_eq!(attr_i64(span, "routeiq.task.total_tokens"), 1000);
}

#[test]
fn test_system_id_and_slo_targets() {
    let recorder = SpanRecorder::default();
    let provider = SdkTracerProvider::builder()
        .with_span_processor(recorder.clone())
        .build();
    let opts = RouteIQOptions {
        agent_id:           "test-agent".to_string(),
        tenant_id:          Some("test-tenant".to_string()),
        environment:        Some("test".to_string()),
        model:              Some("gpt-4o".to_string()),
        agent_version:      Some("1.0.0".to_string()),
        system_id:          Some("checkout-bot".to_string()),
        user_id:            Some("user_42".to_string()),
        slo_success_target: Some(0.95),
        slo_p95_ms_target:  Some(2000.0),
        ..Default::default()
    };
    let riq = RouteIQ::with_provider(opts, provider);
    riq.task("q").end();

    let spans = recorder.spans.lock().unwrap();
    let span = spans.iter().find(|s| s.name.starts_with("task:")).unwrap();
    assert_eq!(attr_str(span, "routeiq.system.id"), "checkout-bot");
    assert_eq!(attr_str(span, "routeiq.user.id"),   "user_42");
    assert!((attr_f64(span, "routeiq.slo.success_target") - 0.95).abs() < 1e-9);
    assert!((attr_f64(span, "routeiq.slo.p95_ms_target") - 2000.0).abs() < 1e-9);
}
