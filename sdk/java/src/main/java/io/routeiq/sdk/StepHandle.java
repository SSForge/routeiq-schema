package io.routeiq.sdk;

import io.opentelemetry.api.trace.Span;
import java.util.Map;
import java.util.UUID;

public final class StepHandle implements AutoCloseable {

    final String stepId = UUID.randomUUID().toString();

    private final RouteIQ riq;
    private final TaskHandle task;
    private final Span span;
    private boolean done = false;

    StepHandle(RouteIQ riq, TaskHandle task, String action, String rationale, int index) {
        this.riq  = riq;
        this.task = task;
        this.span = riq.tracer.spanBuilder("step:" + stepId).startSpan();

        try (var scope = span.makeCurrent()) {
            riq.envelope(task, this).forEach((k, v) -> span.setAttribute(k, v.toString()));
            span.setAttribute("routeiq.event.type", "4");
            span.setAttribute("routeiq.step.index", index);
            if (action    != null) span.setAttribute("routeiq.step.selected_action",  action);
            if (rationale != null) span.setAttribute("routeiq.step.action_rationale", rationale);
        }
    }

    public ToolHandle tool(String name) {
        return new ToolHandle(riq, task, this, name, null, "READ_ONLY");
    }

    public ToolHandle tool(String name, Map<String, Object> args, String permission) {
        return new ToolHandle(riq, task, this, name, args, permission);
    }

    public void complete() { finish("1", null); }
    public void fail()     { fail(null); }
    public void fail(String category) { finish("2", category); }

    private void finish(String status, String category) {
        if (done) return;
        done = true;
        span.setAttribute("routeiq.step.completion_status", status);
        if (category != null) span.setAttribute("routeiq.step.failure_category", category);
    }

    @Override
    public void close() {
        if (!done) complete();
        span.end();
    }
}
