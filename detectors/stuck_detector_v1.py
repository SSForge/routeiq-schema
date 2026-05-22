"""
stuck_detector_v1 — detects agents that keep calling the same tool with no
state change, suggesting they are stuck waiting for a side-effect that never
arrives.

A run is stuck when:
  - the last `window_steps` steps all share the same `selected_action`, AND
  - none of those steps produced a TOOL_SUCCESS result.
"""
from __future__ import annotations

from dataclasses import dataclass
from typing import Sequence


@dataclass
class StepRecord:
    selected_action: str
    tool_result_status: str  # "TOOL_SUCCESS" | "TOOL_FAILURE" | "TOOL_TIMEOUT" | ""


@dataclass
class StuckDetectionResult:
    is_stuck: bool
    action: str | None = None
    consecutive_non_success: int = 0


def detect(
    steps: Sequence[StepRecord],
    *,
    window_steps: int = 3,
) -> StuckDetectionResult:
    if len(steps) < window_steps:
        return StuckDetectionResult(is_stuck=False)

    window = list(steps[-window_steps:])
    actions = {s.selected_action for s in window}
    if len(actions) != 1:
        return StuckDetectionResult(is_stuck=False)

    action = next(iter(actions))
    non_success = sum(1 for s in window if s.tool_result_status != "TOOL_SUCCESS")
    if non_success >= window_steps:
        return StuckDetectionResult(
            is_stuck=True, action=action, consecutive_non_success=non_success
        )
    return StuckDetectionResult(is_stuck=False)
