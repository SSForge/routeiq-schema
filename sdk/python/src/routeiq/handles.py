"""
TaskHandle, StepHandle, ToolHandle — sync context managers for the RouteIQ SDK.

Attribute keys match conventions/telemetry.yaml in routeiq-schema.
Enum values match entities.proto (SUCCESS=1, FAILURE=2, TOOL_SUCCESS=1, TOOL_FAILURE=2).
"""

import hashlib
import json
import time
import uuid
from typing import TYPE_CHECKING, Optional

if TYPE_CHECKING:
    from .client import RouteIQ

# ── Enum values (mirror entities.proto) ──────────────────────────────────────

_COMPLETION_SUCCESS = "1"
_COMPLETION_FAILURE = "2"
_TOOL_SUCCESS = "1"
_TOOL_FAILURE = "2"

PERMISSION = {
    "READ_ONLY":  "1",
    "READ_WRITE": "2",
    "PRIVILEGED": "3",
}


# ── ToolHandle ────────────────────────────────────────────────────────────────

class ToolHandle:
    """Context manager for a single tool invocation inside a step."""

    def __init__(
        self,
        step: "StepHandle",
        name: str,
        args: Optional[dict] = None,
        permission: str = "READ_ONLY",
    ):
        self._step = step
        self.name = name
        self._args = args or {}
        self._permission = PERMISSION.get(permission, "1")
        self._start: float = 0.0
        self._span_cm = None
        self._span = None
        self._done = False

    def __enter__(self) -> "ToolHandle":
        riq = self._step._task._riq
        self._start = time.monotonic()
        args_hash = hashlib.sha256(
            json.dumps(self._args, sort_keys=True).encode()
        ).hexdigest()[:16]
        self._span_cm = riq._tracer.start_as_current_span(f"tool:{self.name}")
        self._span = self._span_cm.__enter__()
        self._span.set_attributes({
            "routeiq.event.type": "7",  # TOOL_CALLED
            **riq._envelope(self._step._task, self._step),
            "routeiq.tool.name": self.name,
            "routeiq.tool.arguments_hash": args_hash,
            "routeiq.tool.permission_level": self._permission,
        })
        # Register with task for same_tool_count tracking
        self._step._task._record_tool(self.name)
        return self

    def success(
        self,
        latency_ms: Optional[float] = None,
        tokens_in: Optional[int] = None,
        tokens_out: Optional[int] = None,
    ) -> None:
        """Record a successful tool result."""
        self._finish(_TOOL_SUCCESS, latency_ms=latency_ms, tokens_in=tokens_in, tokens_out=tokens_out)

    def fail(
        self,
        error_code: str = "",
        latency_ms: Optional[float] = None,
        retry_count: Optional[int] = None,
        tokens_in: Optional[int] = None,
        tokens_out: Optional[int] = None,
    ) -> None:
        """Record a failed tool result."""
        self._finish(_TOOL_FAILURE, error_code=error_code, latency_ms=latency_ms,
                     retry_count=retry_count, tokens_in=tokens_in, tokens_out=tokens_out)

    def _finish(
        self,
        status: str,
        error_code: str = "",
        latency_ms: Optional[float] = None,
        retry_count: Optional[int] = None,
        tokens_in: Optional[int] = None,
        tokens_out: Optional[int] = None,
    ):
        if self._done:
            return
        self._done = True
        elapsed = (time.monotonic() - self._start) * 1000
        attrs: dict = {
            "routeiq.tool.result_status": status,
            "routeiq.tool.latency_ms": latency_ms if latency_ms is not None else elapsed,
        }
        if error_code:                   attrs["routeiq.tool.error_code"]  = error_code
        if retry_count is not None:      attrs["routeiq.tool.retry_count"] = retry_count
        if tokens_in is not None:        attrs["routeiq.tool.tokens_in"]   = tokens_in
        if tokens_out is not None:       attrs["routeiq.tool.tokens_out"]  = tokens_out
        if self._span:
            self._span.set_attributes(attrs)

    def __exit__(self, exc_type, exc_val, exc_tb):
        if not self._done:
            elapsed = (time.monotonic() - self._start) * 1000
            if exc_type is not None:
                self.fail(latency_ms=elapsed)
            else:
                self.success(latency_ms=elapsed)
        self._span_cm.__exit__(exc_type, exc_val, exc_tb)
        return False


# ── StepHandle ────────────────────────────────────────────────────────────────

class StepHandle:
    """Context manager for one reasoning step within a task."""

    def __init__(
        self,
        task: "TaskHandle",
        action: Optional[str] = None,
        rationale: Optional[str] = None,
        index: int = 1,
        model: Optional[str] = None,
    ):
        self._task = task
        self.step_id = str(uuid.uuid4())
        self._action = action
        self._rationale = rationale
        self._index = index
        self._model = model
        self._span_cm = None
        self._span = None
        self._done = False

    def __enter__(self) -> "StepHandle":
        riq = self._task._riq
        self._span_cm = riq._tracer.start_as_current_span(f"step:{self.step_id}")
        self._span = self._span_cm.__enter__()
        attrs = {
            "routeiq.event.type": "4",  # STEP_STARTED
            **riq._envelope(self._task, self),
        }
        if self._action:    attrs["routeiq.step.selected_action"]  = self._action
        if self._rationale: attrs["routeiq.step.action_rationale"] = self._rationale
        if self._model:     attrs["routeiq.step.model"]            = self._model
        attrs["routeiq.step.index"] = self._index
        self._span.set_attributes(attrs)
        return self

    def tool(
        self,
        name: str,
        args: Optional[dict] = None,
        permission: str = "READ_ONLY",
    ) -> ToolHandle:
        """Start a tool call within this step."""
        return ToolHandle(self, name=name, args=args, permission=permission)

    def guardrail(self, type: str, blocked: bool) -> None:
        """Emit a guardrail check span. Call whenever a policy/guardrail fires."""
        riq = self._task._riq
        with riq._tracer.start_as_current_span(f"guardrail:{type}") as span:
            span.set_attributes({
                "routeiq.event.type": "9",
                **riq._envelope(self._task, self),
                "routeiq.guardrail.type":    type,
                "routeiq.guardrail.blocked": str(blocked).lower(),
            })

    def replan(self, reason: str) -> None:
        """Mark that the agent replanned mid-step."""
        if self._span:
            self._span.set_attributes({
                "routeiq.replan.triggered": "true",
                "routeiq.replan.reason":    reason[:256],
            })

    def complete(self, tokens_in: Optional[int] = None, tokens_out: Optional[int] = None) -> None:
        """Mark step as successfully completed."""
        self._finish(_COMPLETION_SUCCESS, tokens_in=tokens_in, tokens_out=tokens_out)

    def fail(self, category: str = "") -> None:
        """Mark step as failed."""
        self._finish(_COMPLETION_FAILURE, failure_category=category)

    def _finish(
        self,
        status: str,
        failure_category: str = "",
        tokens_in: Optional[int] = None,
        tokens_out: Optional[int] = None,
    ):
        if self._done:
            return
        self._done = True
        attrs: dict = {"routeiq.step.completion_status": status}
        if failure_category:        attrs["routeiq.step.failure_category"] = failure_category
        if tokens_in is not None:   attrs["routeiq.step.tokens_in"]        = tokens_in
        if tokens_out is not None:  attrs["routeiq.step.tokens_out"]       = tokens_out
        if self._span:
            self._span.set_attributes(attrs)

    def __exit__(self, exc_type, exc_val, exc_tb):
        if not self._done:
            if exc_type is not None:
                self.fail()
            else:
                self.complete()
        self._span_cm.__exit__(exc_type, exc_val, exc_tb)
        return False


# ── TaskHandle ────────────────────────────────────────────────────────────────

class TaskHandle:
    """Context manager for a complete agent task."""

    def __init__(
        self,
        riq: "RouteIQ",
        intent: str,
        task_type: Optional[str] = None,
    ):
        self._riq = riq
        self.intent = intent
        self.task_type = task_type
        self.task_id = str(uuid.uuid4())
        self.run_id = str(uuid.uuid4())
        self._span_cm = None
        self._span = None
        self._done = False
        self._step_index = 0
        self._tool_sequence: list = []

    def __enter__(self) -> "TaskHandle":
        self._span_cm = self._riq._tracer.start_as_current_span(f"task:{self.task_id}")
        self._span = self._span_cm.__enter__()
        attrs = {
            "routeiq.event.type": "1",  # TASK_STARTED
            **self._riq._envelope(self),
            "routeiq.task.input_intent": self.intent[:256],
        }
        if self.task_type:
            attrs["routeiq.task.type"] = self.task_type
        self._span.set_attributes(attrs)
        return self

    def step(
        self,
        action: Optional[str] = None,
        rationale: Optional[str] = None,
        model: Optional[str] = None,
    ) -> StepHandle:
        """Start a reasoning step within this task."""
        self._step_index += 1
        return StepHandle(self, action=action, rationale=rationale,
                          index=self._step_index, model=model)

    def escalate(self, reason: Optional[str] = None, target: Optional[str] = None) -> None:
        """Emit a human-escalation span. Call when handing off to a human operator."""
        riq = self._riq
        with riq._tracer.start_as_current_span(f"escalation:{self.task_id}") as span:
            attrs: dict = {
                "routeiq.event.type": "8",
                **riq._envelope(self),
                "routeiq.escalation.triggered": "true",
            }
            if reason: attrs["routeiq.escalation.reason"] = reason[:256]
            if target: attrs["routeiq.escalation.target"] = target
            span.set_attributes(attrs)

    def _record_tool(self, name: str) -> None:
        """Called by ToolHandle to track tool call sequence for same_tool_count."""
        self._tool_sequence.append(name)

    def _max_same_tool_count(self) -> int:
        if not self._tool_sequence:
            return 0
        max_count = cur = 1
        for i in range(1, len(self._tool_sequence)):
            cur = cur + 1 if self._tool_sequence[i] == self._tool_sequence[i - 1] else 1
            if cur > max_count:
                max_count = cur
        return max_count

    def complete(
        self,
        tokens: int = 0,
        tokens_in: Optional[int] = None,
        tokens_out: Optional[int] = None,
        cost_usd: Optional[float] = None,
        cohort: Optional[str] = None,
    ) -> None:
        """Mark task as successfully completed."""
        self._finish(_COMPLETION_SUCCESS, tokens=tokens, tokens_in=tokens_in,
                     tokens_out=tokens_out, cost_usd=cost_usd, cohort=cohort)

    def fail(self, category: str = "") -> None:
        """Mark task as failed."""
        self._finish(_COMPLETION_FAILURE, failure_category=category)

    def _finish(
        self,
        status: str,
        tokens: int = 0,
        tokens_in: Optional[int] = None,
        tokens_out: Optional[int] = None,
        cost_usd: Optional[float] = None,
        cohort: Optional[str] = None,
        failure_category: str = "",
    ):
        if self._done:
            return
        self._done = True
        attrs: dict = {"routeiq.task.completion_status": status}
        if tokens_in is not None:   attrs["routeiq.task.tokens_in"]  = tokens_in
        if tokens_out is not None:  attrs["routeiq.task.tokens_out"] = tokens_out
        total = tokens or (
            (tokens_in + tokens_out) if tokens_in is not None and tokens_out is not None else 0
        )
        if total:                   attrs["routeiq.task.total_tokens"]    = total
        if cost_usd is not None:    attrs["routeiq.task.cost_usd"]        = cost_usd
        if cohort:                  attrs["routeiq.task.cohort"]          = cohort
        if failure_category:        attrs["routeiq.task.failure_category"] = failure_category
        # Emit max consecutive same-tool count for loop detection
        same_tool_count = self._max_same_tool_count()
        if same_tool_count > 1:     attrs["routeiq.same_tool_count"] = same_tool_count
        if self._span:
            self._span.set_attributes(attrs)

    def __exit__(self, exc_type, exc_val, exc_tb):
        if not self._done:
            if exc_type is not None:
                self.fail()
            else:
                self.complete()
        self._span_cm.__exit__(exc_type, exc_val, exc_tb)
        return False
