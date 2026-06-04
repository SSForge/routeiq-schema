"""
RouteIQ — the developer-facing client.

Sets up OTel once in __init__; exposes task() / step() / tool() context managers
that emit the correct routeiq.* span attributes without any proto or OTel knowledge
required from the caller.
"""

import uuid
from typing import Optional

from opentelemetry import trace
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor

from .handles import TaskHandle

_SDK_VERSION = "0.3.0"


class RouteIQ:
    """
    One instance per agent process. Handles OTel setup and session tracking.

    Args:
        agent_id:           Identifies this agent in dashboards (e.g. "support-agent-prod")
        system_id:          Groups this agent under a named system (e.g. "checkout-bot")
        user_id:            End-user ID for per-user analytics
        otlp_endpoint:      gRPC endpoint for the OTel collector. Default: http://localhost:4317
        tenant_id:          Tenant/org identifier. Default: "default"
        model:              LLM model name — populates routeiq.version.model.name on every span
        environment:        Deployment environment. Default: "production"
        agent_version:      Your agent's version string. Default: "1.0.0"
        api_key:            API key for RouteIQ SaaS (adds Authorization header).
        slo_success_target: Minimum acceptable success rate (0–1), e.g. 0.95
        slo_p95_ms_target:  p95 latency SLO in milliseconds, e.g. 2000

    Example::

        from routeiq import RouteIQ

        riq = RouteIQ(agent_id="my-agent", system_id="checkout-bot",
                      model="gpt-4o", slo_success_target=0.95, slo_p95_ms_target=2000)

        with riq.task(intent=user_input) as task:
            with task.step(action="tool_call") as step:
                with step.tool("search", args={"query": "Paris"}) as tool:
                    result = search("Paris")
            task.complete(tokens_in=300, tokens_out=84, cost_usd=0.002)
    """

    def __init__(
        self,
        agent_id: str,
        system_id: Optional[str] = None,
        user_id: Optional[str] = None,
        otlp_endpoint: str = "http://localhost:4317",
        tenant_id: str = "default",
        model: Optional[str] = None,
        environment: str = "production",
        agent_version: str = "1.0.0",
        api_key: Optional[str] = None,
        slo_success_target: Optional[float] = None,
        slo_p95_ms_target: Optional[float] = None,
    ):
        self.agent_id = agent_id
        self.system_id = system_id
        self.user_id = user_id
        self.tenant_id = tenant_id
        self.model = model
        self.environment = environment
        self.agent_version = agent_version
        self.session_id = str(uuid.uuid4())
        self.slo_success_target = slo_success_target
        self.slo_p95_ms_target = slo_p95_ms_target

        resource = Resource.create({
            "service.name": agent_id,
            "service.version": agent_version,
            "routeiq.sdk.version": _SDK_VERSION,
        })
        self._provider = TracerProvider(resource=resource)
        self._provider.add_span_processor(
            BatchSpanProcessor(_make_exporter(otlp_endpoint, api_key))
        )
        self._tracer = trace.get_tracer("routeiq.sdk", tracer_provider=self._provider)

    # ── Public API ────────────────────────────────────────────────────────────

    def task(
        self,
        intent: str,
        task_type: Optional[str] = None,
    ) -> TaskHandle:
        """
        Start a task span. Use as a context manager.

        Args:
            intent:    The user's input / goal for this task.
            task_type: Optional category e.g. "research", "coding", "qa".
        """
        return TaskHandle(self, intent=intent, task_type=task_type)

    def flush(self) -> None:
        """Force-flush pending spans. Call before process exit in scripts."""
        self._provider.force_flush()

    # ── Internal helpers ──────────────────────────────────────────────────────

    def _envelope(self, task=None, step=None) -> dict:
        """Shared attributes stamped on every span."""
        attrs: dict = {
            "routeiq.agent.id":    self.agent_id,
            "routeiq.tenant.id":   self.tenant_id,
            "routeiq.environment": self.environment,
            "routeiq.session.id":  self.session_id,
        }
        if self.system_id:
            attrs["routeiq.system.id"] = self.system_id
        if self.user_id:
            attrs["routeiq.user.id"] = self.user_id
        if task is not None:
            attrs["routeiq.task.id"] = task.task_id
            attrs["routeiq.run.id"]  = task.run_id
        if step is not None:
            attrs["routeiq.step.id"] = step.step_id
        if self.model:
            attrs["routeiq.version.model.name"] = self.model
        if self.agent_version:
            attrs["routeiq.version.agent"] = self.agent_version
        if self.slo_success_target is not None:
            attrs["routeiq.slo.success_target"] = self.slo_success_target
        if self.slo_p95_ms_target is not None:
            attrs["routeiq.slo.p95_ms_target"] = self.slo_p95_ms_target
        return attrs


# ── Transport factory ─────────────────────────────────────────────────────────

def _make_exporter(endpoint: str, api_key: Optional[str]):
    """Return a gRPC or HTTP OTLP exporter depending on the endpoint scheme."""
    headers = {}
    if api_key:
        headers["authorization"] = f"Bearer {api_key}"

    if endpoint.startswith("https://") or ":4318" in endpoint:
        from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter as HTTPExporter
        return HTTPExporter(
            endpoint=f"{endpoint.rstrip('/')}/v1/traces",
            headers=headers,
        )

    # Default: gRPC (local OTel collector on :4317)
    from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter as GRPCExporter
    return GRPCExporter(
        endpoint=endpoint,
        insecure=not api_key,
        headers=headers if headers else None,
    )
