package io.routeiq.sdk;

import io.opentelemetry.api.trace.Span;

import java.util.ArrayList;
import java.util.List;
import java.util.UUID;

public final class TaskHandle implements AutoCloseable {

    final String taskId = UUID.randomUUID().toString();
    final String runId  = UUID.randomUUID().toString();

    private final RouteIQ riq;
    private final Span span;
    private boolean done = false;
    private int stepIndex = 0;
    private final List<String> toolSequence = new ArrayList<>();

    TaskHandle(RouteIQ riq, String intent, String taskType) {
        this.riq  = riq;
        this.span = riq.tracer.spanBuilder("task:" + taskId).startSpan();

        try (var scope = span.makeCurrent()) {
            riq.envelope(this, null).forEach((k, v) -> span.setAttribute(k, v.toString()));
            span.setAttribute("routeiq.event.type",        "1");
            span.setAttribute("routeiq.task.input_intent", intent.length() <= 256 ? intent : intent.substring(0, 256));
            if (taskType != null) span.setAttribute("routeiq.task.type", taskType);
        }
    }

    public StepHandle step() {
        return new StepHandle(riq, this, null, null, null, ++stepIndex);
    }

    public StepHandle step(String action, String rationale) {
        return new StepHandle(riq, this, action, rationale, null, ++stepIndex);
    }

    public StepHandle step(String action, String rationale, String model) {
        return new StepHandle(riq, this, action, rationale, model, ++stepIndex);
    }

    public void escalate(String reason, String target) {
        var escSpan = riq.tracer.spanBuilder("escalation:" + taskId).startSpan();
        try (var scope = escSpan.makeCurrent()) {
            riq.envelope(this, null).forEach((k, v) -> escSpan.setAttribute(k, v.toString()));
            escSpan.setAttribute("routeiq.event.type", "8");
            escSpan.setAttribute("routeiq.escalation.triggered", "true");
            if (reason != null) escSpan.setAttribute("routeiq.escalation.reason",
                reason.length() <= 256 ? reason : reason.substring(0, 256));
            if (target != null) escSpan.setAttribute("routeiq.escalation.target", target);
        } finally {
            escSpan.end();
        }
    }

    void recordTool(String name) {
        toolSequence.add(name);
    }

    private int maxSameToolCount() {
        if (toolSequence.isEmpty()) return 0;
        int maxCount = 1, cur = 1;
        for (int i = 1; i < toolSequence.size(); i++) {
            cur = toolSequence.get(i).equals(toolSequence.get(i - 1)) ? cur + 1 : 1;
            if (cur > maxCount) maxCount = cur;
        }
        return maxCount;
    }

    public void complete() { finish("1", 0, 0, 0, null, null, null); }

    public void complete(int tokens, Double costUsd, String cohort) {
        finish("1", tokens, 0, 0, costUsd, cohort, null);
    }

    public void complete(int tokensIn, int tokensOut) {
        finish("1", 0, tokensIn, tokensOut, null, null, null);
    }

    public void complete(int tokensIn, int tokensOut, Double costUsd, String cohort) {
        finish("1", 0, tokensIn, tokensOut, costUsd, cohort, null);
    }

    public void fail() { fail(null); }

    public void fail(String category) {
        finish("2", 0, 0, 0, null, null, category);
    }

    private void finish(String status, int tokens, int tokensIn, int tokensOut,
                        Double costUsd, String cohort, String failureCat) {
        if (done) return;
        done = true;
        span.setAttribute("routeiq.task.completion_status", status);
        if (tokensIn > 0)      span.setAttribute("routeiq.task.tokens_in",  (long) tokensIn);
        if (tokensOut > 0)     span.setAttribute("routeiq.task.tokens_out", (long) tokensOut);
        int total = tokens > 0 ? tokens : tokensIn + tokensOut;
        if (total > 0)         span.setAttribute("routeiq.task.total_tokens",    (long) total);
        if (costUsd != null)   span.setAttribute("routeiq.task.cost_usd",         costUsd);
        if (cohort != null)    span.setAttribute("routeiq.task.cohort",            cohort);
        if (failureCat != null) span.setAttribute("routeiq.task.failure_category", failureCat);
        int same = maxSameToolCount();
        if (same > 1)          span.setAttribute("routeiq.same_tool_count",       (long) same);
    }

    @Override
    public void close() {
        if (!done) complete();
        span.end();
    }
}
