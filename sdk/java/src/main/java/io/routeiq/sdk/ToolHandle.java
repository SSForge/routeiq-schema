package io.routeiq.sdk;

import io.opentelemetry.api.trace.Span;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.Map;
import java.util.TreeMap;

public final class ToolHandle implements AutoCloseable {

    private static final Map<String, String> PERMISSION = Map.of(
            "READ_ONLY",  "1",
            "READ_WRITE", "2",
            "PRIVILEGED", "3"
    );

    private final Span span;
    private final long startNs;
    private boolean done = false;

    ToolHandle(RouteIQ riq, TaskHandle task, StepHandle step,
               String name, Map<String, Object> args, String permission) {

        this.span    = riq.tracer.spanBuilder("tool:" + name).startSpan();
        this.startNs = System.nanoTime();
        task.recordTool(name);

        try (var scope = span.makeCurrent()) {
            riq.envelope(task, step).forEach((k, v) -> span.setAttribute(k, v.toString()));
            span.setAttribute("routeiq.event.type",           "7");
            span.setAttribute("routeiq.tool.name",             name);
            span.setAttribute("routeiq.tool.arguments_hash",   argsHash(args));
            span.setAttribute("routeiq.tool.permission_level", PERMISSION.getOrDefault(permission, "1"));
        }
    }

    public void success()                              { finish("1", null, null, null, null, null); }
    public void success(double latencyMs)              { finish("1", null, latencyMs, null, null, null); }
    public void success(double latencyMs, int tokensIn, int tokensOut) {
        finish("1", null, latencyMs, null, tokensIn, tokensOut);
    }
    public void successTokens(int tokensIn, int tokensOut) {
        finish("1", null, null, null, tokensIn, tokensOut);
    }

    public void fail(String errorCode)                 { finish("2", errorCode, null, null, null, null); }
    public void fail(String errorCode, double latMs)   { finish("2", errorCode, latMs, null, null, null); }
    public void failRetry(String errorCode, int retryCount) {
        finish("2", errorCode, null, retryCount, null, null);
    }

    private void finish(String status, String errorCode, Double latencyMs,
                        Integer retryCount, Integer tokensIn, Integer tokensOut) {
        if (done) return;
        done = true;
        double elapsed = (System.nanoTime() - startNs) / 1_000_000.0;
        span.setAttribute("routeiq.tool.result_status", status);
        span.setAttribute("routeiq.tool.latency_ms",    latencyMs != null ? latencyMs : elapsed);
        if (errorCode  != null) span.setAttribute("routeiq.tool.error_code",  errorCode);
        if (retryCount != null) span.setAttribute("routeiq.tool.retry_count", (long) retryCount.intValue());
        if (tokensIn   != null) span.setAttribute("routeiq.tool.tokens_in",   (long) tokensIn.intValue());
        if (tokensOut  != null) span.setAttribute("routeiq.tool.tokens_out",  (long) tokensOut.intValue());
    }

    @Override
    public void close() {
        if (!done) success();
        span.end();
    }

    private static String argsHash(Map<String, Object> args) {
        try {
            var sorted = new TreeMap<>(args != null ? args : Map.of());
            var bytes  = sorted.toString().getBytes(StandardCharsets.UTF_8);
            var digest = MessageDigest.getInstance("SHA-256").digest(bytes);
            var sb     = new StringBuilder();
            for (int i = 0; i < 8; i++) sb.append(String.format("%02x", digest[i]));
            return sb.toString();
        } catch (NoSuchAlgorithmException e) {
            return "0000000000000000";
        }
    }
}
