package io.routeiq.sdk;

import io.opentelemetry.api.trace.Tracer;
import io.opentelemetry.sdk.trace.SdkTracerProvider;
import io.opentelemetry.sdk.trace.export.BatchSpanProcessor;
import io.opentelemetry.sdk.resources.Resource;
import io.opentelemetry.exporter.otlp.trace.OtlpGrpcSpanExporter;
import io.opentelemetry.exporter.otlp.http.trace.OtlpHttpSpanExporter;
import io.opentelemetry.semconv.ResourceAttributes;

import java.util.HashMap;
import java.util.Map;
import java.util.UUID;

public final class RouteIQ implements AutoCloseable {

    private static final String SDK_VERSION = "0.2.0";

    final String agentId;
    final String tenantId;
    final String model;
    final String environment;
    final String agentVersion;
    final String sessionId;
    final Tracer tracer;
    private final SdkTracerProvider provider;

    public RouteIQ(
            String agentId,
            String otlpEndpoint,
            String tenantId,
            String model,
            String environment,
            String agentVersion,
            String apiKey) {

        this.agentId      = agentId;
        this.tenantId     = tenantId != null ? tenantId : "default";
        this.model        = model;
        this.environment  = environment != null ? environment : "production";
        this.agentVersion = agentVersion != null ? agentVersion : "1.0.0";
        this.sessionId    = UUID.randomUUID().toString();

        String ep = otlpEndpoint != null ? otlpEndpoint : "http://localhost:4317";

        var spanExporter = ep.startsWith("https://") || ep.contains(":4318")
                ? OtlpHttpSpanExporter.builder().setEndpoint(ep + "/v1/traces")
                    .addHeader("authorization", apiKey != null ? "Bearer " + apiKey : "").build()
                : OtlpGrpcSpanExporter.builder().setEndpoint(ep).build();

        var resource = Resource.getDefault().toBuilder()
                .put(ResourceAttributes.SERVICE_NAME, agentId)
                .put(ResourceAttributes.SERVICE_VERSION, this.agentVersion)
                .put("routeiq.sdk.version", SDK_VERSION)
                .build();

        this.provider = SdkTracerProvider.builder()
                .addSpanProcessor(BatchSpanProcessor.builder(spanExporter).build())
                .setResource(resource)
                .build();
        this.tracer = provider.get("routeiq.sdk", SDK_VERSION);
    }

    /** Constructor for tests — inject a pre-built provider. */
    RouteIQ(String agentId, SdkTracerProvider provider) {
        this.agentId      = agentId;
        this.tenantId     = "test-tenant";
        this.model        = "gpt-4o";
        this.environment  = "test";
        this.agentVersion = "1.0.0";
        this.sessionId    = UUID.randomUUID().toString();
        this.provider     = provider;
        this.tracer       = provider.get("routeiq.sdk", SDK_VERSION);
    }

    public TaskHandle task(String intent) {
        return new TaskHandle(this, intent, null);
    }

    public TaskHandle task(String intent, String taskType) {
        return new TaskHandle(this, intent, taskType);
    }

    public void flush() {
        provider.forceFlush().join(5, java.util.concurrent.TimeUnit.SECONDS);
    }

    @Override
    public void close() {
        flush();
        provider.close();
    }

    Map<String, Object> envelope(TaskHandle task, StepHandle step) {
        var attrs = new HashMap<String, Object>();
        attrs.put("routeiq.agent.id",    agentId);
        attrs.put("routeiq.tenant.id",   tenantId);
        attrs.put("routeiq.environment", environment);
        attrs.put("routeiq.session.id",  sessionId);
        if (task != null) {
            attrs.put("routeiq.task.id", task.taskId);
            attrs.put("routeiq.run.id",  task.runId);
        }
        if (step != null) attrs.put("routeiq.step.id", step.stepId);
        if (model        != null) attrs.put("routeiq.version.model.name", model);
        if (agentVersion != null) attrs.put("routeiq.version.agent",      agentVersion);
        return attrs;
    }
}
