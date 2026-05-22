"""
no_progress_detector_v1 — detects runs where step count has grown but the
task completion_status has not moved toward SUCCESS for an extended window.

A run has no progress when the last `window_steps` step events all have
completion_status FAILURE or PARTIAL with no intervening SUCCESS step.
"""
from __future__ import annotations

from dataclasses import dataclass
from typing import Sequence


@dataclass
class StepRecord:
    completion_status: str  # "SUCCESS" | "FAILURE" | "PARTIAL" | "CANCELLED" | "TIMEOUT"


@dataclass
class NoProgressResult:
    no_progress: bool
    consecutive_non_success: int = 0


def detect(
    steps: Sequence[StepRecord],
    *,
    window_steps: int = 5,
) -> NoProgressResult:
    if len(steps) < window_steps:
        return NoProgressResult(no_progress=False)

    window = list(steps[-window_steps:])
    non_success = sum(1 for s in window if s.completion_status != "SUCCESS")
    if non_success >= window_steps:
        return NoProgressResult(no_progress=True, consecutive_non_success=non_success)
    return NoProgressResult(no_progress=False)
