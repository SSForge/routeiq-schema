"""
loop_detector_v1 — heuristic detector for cyclical agent step patterns.

A run is classified as looping when, within a sliding window of `window_steps`
consecutive steps, any (action, arguments_hash) pair repeats at least
`repeat_threshold` times.

catalog YAML reference:
  formula:
    heuristic:
      detector_id: loop_detector_v1
      parameters:
        window_steps: 5
        repeat_threshold: 3
        similarity_threshold: 0.92   # reserved for future fuzzy matching
"""
from __future__ import annotations

from collections import Counter
from dataclasses import dataclass
from typing import Sequence


@dataclass
class StepRecord:
    """Minimal projection of a StepEvent + ToolCallEvent pair."""
    selected_action: str
    arguments_hash: str = ""


@dataclass
class LoopDetectionResult:
    is_looping: bool
    repeated_pair: tuple[str, str] | None = None
    repeat_count: int = 0
    window_start: int = 0


def detect(
    steps: Sequence[StepRecord],
    *,
    window_steps: int = 5,
    repeat_threshold: int = 3,
    similarity_threshold: float = 0.92,  # noqa: ARG001 — reserved
) -> LoopDetectionResult:
    """Return a LoopDetectionResult for the given step sequence.

    Slides a window of `window_steps` over `steps`. If any (action,
    arguments_hash) pair appears >= `repeat_threshold` times within the
    window the run is flagged as looping.
    """
    if len(steps) < window_steps:
        return LoopDetectionResult(is_looping=False)

    for start in range(len(steps) - window_steps + 1):
        window = steps[start : start + window_steps]
        counts: Counter[tuple[str, str]] = Counter(
            (s.selected_action, s.arguments_hash) for s in window
        )
        for pair, count in counts.items():
            if count >= repeat_threshold:
                return LoopDetectionResult(
                    is_looping=True,
                    repeated_pair=pair,
                    repeat_count=count,
                    window_start=start,
                )

    return LoopDetectionResult(is_looping=False)
