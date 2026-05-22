package io.routeiq.sdk;

import io.opentelemetry.sdk.testing.junit5.OpenTelemetryExtension;
import io.opentelemetry.sdk.trace.data.SpanData;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.RegisterExtension;

import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

class HandlesTest {

    @RegisterExtension
    static final OpenTelemetryExtension otelTesting = OpenTelemetryExtension.create();

    private RouteIQ riq;

    @BeforeEach
    void setUp() {
        var provider = otelTesting.getOpenTelemetry().getTracerProvider();
        // Cast to SdkTracerProvider to use the test constructor
        riq = new RouteIQ("test-agent",
                (io.opentelemetry.sdk.trace.SdkTracerProvider) provider);
    }

    private SpanData findSpan(String prefix) {
        return otelTesting.getSpans().stream()
                .filter(s -> s.getName().startsWith(prefix))
                .findFirst()
                .orElseThrow(() -> new AssertionError("No span with prefix: " + prefix));
    }

    private SpanData findSpanNamed(String name) {
        return otelTesting.getSpans().stream()
                .filter(s -> s.getName().equals(name))
                .findFirst()
                .orElseThrow(() -> new AssertionError("No span named: " + name));
    }

    private String attr(SpanData span, String key) {
        var v = span.getAttributes().get(io.opentelemetry.api.common.AttributeKey.stringKey(key));
        return v != null ? v : span.getAttributes().get(
                io.opentelemetry.api.common.AttributeKey.longKey(key)) != null
                ? String.valueOf(span.getAttributes().get(io.opentelemetry.api.common.AttributeKey.longKey(key)))
                : null;
    }

    // ── TaskHandle ────────────────────────────────────────────────────────

    @Test
    void task_spanNameStartsWithTask() {
        try (var task = riq.task("find Paris")) { }
        assertNotNull(findSpan("task:"));
    }

    @Test
    void task_envelopeAttributes() {
        String taskId;
        try (var task = riq.task("find Paris")) {
            taskId = task.taskId;
        }
        var span = findSpan("task:");
        assertEquals("test-agent",    attr(span, "routeiq.agent.id"));
        assertEquals(riq.sessionId,   attr(span, "routeiq.session.id"));
        assertEquals(taskId,          attr(span, "routeiq.task.id"));
        assertEquals("find Paris",    attr(span, "routeiq.task.input_intent"));
        assertEquals("gpt-4o",        attr(span, "routeiq.version.model.name"));
    }

    @Test
    void task_completeSetsSucess() {
        try (var task = riq.task("q")) {
            task.complete(100, null, "test");
        }
        var span = findSpan("task:");
        assertEquals("1",    attr(span, "routeiq.task.completion_status"));
        assertEquals("100",  attr(span, "routeiq.task.total_tokens"));
        assertEquals("test", attr(span, "routeiq.task.cohort"));
    }

    @Test
    void task_failSetsFailure() {
        try (var task = riq.task("q")) {
            task.fail("tool_error");
        }
        var span = findSpan("task:");
        assertEquals("2",          attr(span, "routeiq.task.completion_status"));
        assertEquals("tool_error", attr(span, "routeiq.task.failure_category"));
    }

    @Test
    void task_autoSucceedsOnClose() {
        try (var task = riq.task("q")) { }
        assertEquals("1", attr(findSpan("task:"), "routeiq.task.completion_status"));
    }

    // ── StepHandle ────────────────────────────────────────────────────────

    @Test
    void step_spanNameStartsWithStep() {
        try (var task = riq.task("q")) {
            try (var step = task.step("tool_call", null)) { }
        }
        assertNotNull(findSpan("step:"));
    }

    @Test
    void step_carriesTaskId() {
        String taskId, stepId;
        try (var task = riq.task("q")) {
            taskId = task.taskId;
            try (var step = task.step()) {
                stepId = step.stepId;
            }
        }
        var span = findSpan("step:");
        assertEquals(taskId, attr(span, "routeiq.task.id"));
        assertEquals(stepId, attr(span, "routeiq.step.id"));
    }

    @Test
    void step_indexIncrements() {
        try (var task = riq.task("q")) {
            try (var s = task.step()) { }
            try (var s = task.step()) { }
        }
        var indices = otelTesting.getSpans().stream()
                .filter(s -> s.getName().startsWith("step:"))
                .map(s -> Long.parseLong(attr(s, "routeiq.step.index")))
                .sorted().toList();
        assertEquals(java.util.List.of(1L, 2L), indices);
    }

    // ── ToolHandle ────────────────────────────────────────────────────────

    @Test
    void tool_spanNameIsToolName() {
        try (var task = riq.task("q")) {
            try (var step = task.step()) {
                try (var tool = step.tool("search", Map.of("query", "Paris"), "READ_ONLY")) { }
            }
        }
        assertNotNull(findSpanNamed("tool:search"));
    }

    @Test
    void tool_successSetsStatus1() {
        try (var task = riq.task("q")) {
            try (var step = task.step()) {
                try (var tool = step.tool("search")) {
                    tool.success(50.0);
                }
            }
        }
        var span = findSpanNamed("tool:search");
        assertEquals("1", attr(span, "routeiq.tool.result_status"));
    }

    @Test
    void tool_failSetsStatus2() {
        try (var task = riq.task("q")) {
            try (var step = task.step()) {
                try (var tool = step.tool("search")) {
                    tool.fail("TIMEOUT");
                }
            }
        }
        var span = findSpanNamed("tool:search");
        assertEquals("2",       attr(span, "routeiq.tool.result_status"));
        assertEquals("TIMEOUT", attr(span, "routeiq.tool.error_code"));
    }

    @Test
    void tool_autoSucceedsOnClose() {
        try (var task = riq.task("q")) {
            try (var step = task.step()) {
                try (var tool = step.tool("search")) { }
            }
        }
        assertEquals("1", attr(findSpanNamed("tool:search"), "routeiq.tool.result_status"));
    }

    @Test
    void tool_argsHashIs16Chars() {
        try (var task = riq.task("q")) {
            try (var step = task.step()) {
                try (var tool = step.tool("search", Map.of("query", "Paris"), "READ_ONLY")) { }
            }
        }
        var hash = attr(findSpanNamed("tool:search"), "routeiq.tool.arguments_hash");
        assertNotNull(hash);
        assertEquals(16, hash.length());
    }

    @Test
    void tool_permissionLevelMaps() {
        try (var task = riq.task("q")) {
            try (var step = task.step()) {
                try (var tool = step.tool("write_file", null, "READ_WRITE")) { }
            }
        }
        assertEquals("2", attr(findSpanNamed("tool:write_file"), "routeiq.tool.permission_level"));
    }

    @Test
    void sessionId_consistentAcrossSpans() {
        try (var task = riq.task("q")) {
            try (var step = task.step()) {
                try (var tool = step.tool("search")) { }
            }
        }
        var ids = otelTesting.getSpans().stream()
                .map(s -> attr(s, "routeiq.session.id"))
                .distinct().toList();
        assertEquals(1, ids.size());
        assertEquals(riq.sessionId, ids.get(0));
    }
}
