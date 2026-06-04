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

    StepHandle(RouteIQ riq, TaskHandle task, String action, String rationale, String model, int index) {
        this.riq  = riq;
        this.task = task;
        this.span = riq.tracer.spanBuilder("step:" + stepId).startSpan();

        try (var scope = span.makeCurrent()) {
            riq.envelope(task, this).forEach((k, v) -> span.setAttribute(k, v.toString()));
            span.setAttribute("routeiq.event.type", "4");
            span.setAttribute("routeiq.step.index", (long) index);
            if (action    != null) span.setAttribute("routeiq.step.selected_action",  action);
            if (rationale != null) span.setAttribute("routeiq.step.action_rationale", rationale);
            if (model     != null) span.setAttribute("routeiq.step.model",            model);
        }
    }

    public ToolHandle tool(String name) {
        return new ToolHandle(riq, task, this, name, null, "READ_ONLY");
    }

    public ToolHandle tool(String name, Map<String, Object> args, String permission) {
        return new ToolHandle(riq, task, this, name, args, permission);
    }

    public void guardrail(String type, boolean blocked) {
        var gSpan = riq.tracer.spanBuilder("guardrail:" + type).startSpan();
        try (var scope = gSpan.makeCurrent()) {
            riq.envelope(task, this).forEach((k, v) -> gSpan.setAttribute(k, v.toString()));
            gSpan.setAttribute("routeiq.event.type",       "9");
            gSpan.setAttribute("routeiq.guardrail.type",    type);
            gSpan.setAttribute("routeiq.guardrail.blocked", String.valueOf(blocked));
        } finally {
            gSpan.end();
        }
    }

    public void replan(String reason) {
        span.setAttribute("routeiq.replan.triggered", "true");
        span.setAttribute("routeiq.replan.reason",
            reason.length() <= 256 ? reason : reason.substring(0, 256));
    }

    public void complete()                      { finish("1", null, 0, 0); }
    public void complete(int tokensIn, int tokensOut) { finish("1", null, tokensIn, tokensOut); }
    public void fail()                          { fail(null); }
    public void fail(String category)           { finish("2", category, 0, 0); }

    private void finish(String status, String category, int tokensIn, int tokensOut) {
        if (done) return;
        done = true;
        span.setAttribute("routeiq.step.completion_status", status);
        if (category != null) span.setAttribute("routeiq.step.failure_category", category);
        if (tokensIn  > 0)    span.setAttribute("routeiq.step.tokens_in",  (long) tokensIn);
        if (tokensOut > 0)    span.setAttribute("routeiq.step.tokens_out", (long) tokensOut);
    }

    @Override
    public void close() {
        if (!done) complete();
        span.end();
    }
}
