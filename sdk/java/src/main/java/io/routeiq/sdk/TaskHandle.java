package io.routeiq.sdk;

import io.opentelemetry.api.trace.Span;
import io.opentelemetry.api.trace.StatusCode;

import java.util.UUID;

public final class TaskHandle implements AutoCloseable {

    final String taskId = UUID.randomUUID().toString();
    final String runId  = UUID.randomUUID().toString();

    private final RouteIQ riq;
    private final Span span;
    private boolean done = false;
    private int stepIndex = 0;

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
        return new StepHandle(riq, this, null, null, ++stepIndex);
    }

    public StepHandle step(String action, String rationale) {
        return new StepHandle(riq, this, action, rationale, ++stepIndex);
    }

    public void complete() { complete(0, null, null); }

    public void complete(int tokens, Double costUsd, String cohort) {
        finish("1", tokens, costUsd, cohort, null);
    }

    public void fail() { fail(null); }

    public void fail(String category) {
        finish("2", 0, null, null, category);
    }

    private void finish(String status, int tokens, Double costUsd, String cohort, String failureCat) {
        if (done) return;
        done = true;
        span.setAttribute("routeiq.task.completion_status", status);
        if (tokens > 0)        span.setAttribute("routeiq.task.total_tokens",     tokens);
        if (costUsd != null)   span.setAttribute("routeiq.task.cost_usd",         costUsd);
        if (cohort != null)    span.setAttribute("routeiq.task.cohort",            cohort);
        if (failureCat != null) span.setAttribute("routeiq.task.failure_category", failureCat);
    }

    @Override
    public void close() {
        if (!done) complete();
        span.end();
    }
}
