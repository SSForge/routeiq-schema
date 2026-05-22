"""
Smoke tests for all three heuristic detectors.
Each detector is imported directly (no install required) and exercised with
known-looping / known-stuck / known-no-progress and clean cases.
"""
import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from detectors import loop_detector_v1, stuck_detector_v1, no_progress_detector_v1

errors = []


def check(label, condition):
    if not condition:
        errors.append(f"FAIL: {label}")


# ── loop_detector_v1 ──────────────────────────────────────────────────────────

LD = loop_detector_v1
SR = LD.StepRecord

# 5-step window with action "search" repeated 3 times → looping
looping_steps = [
    SR("search", "hash-a"),
    SR("search", "hash-a"),
    SR("read", "hash-b"),
    SR("search", "hash-a"),
    SR("write", "hash-c"),
]
res = LD.detect(looping_steps, window_steps=5, repeat_threshold=3)
check("loop_detector: detects loop", res.is_looping)
check("loop_detector: correct pair", res.repeated_pair == ("search", "hash-a"))
check("loop_detector: repeat_count >= 3", res.repeat_count >= 3)

# Distinct steps → not looping
clean_steps = [SR(f"action_{i}", f"hash-{i}") for i in range(6)]
res_clean = LD.detect(clean_steps, window_steps=5, repeat_threshold=3)
check("loop_detector: clean run not looping", not res_clean.is_looping)

# Too few steps → not looping
check("loop_detector: too few steps not looping", not LD.detect([SR("a", "h")], window_steps=5).is_looping)

# ── stuck_detector_v1 ─────────────────────────────────────────────────────────

SD = stuck_detector_v1
SDSR = SD.StepRecord

# Last 3 steps all same action with no TOOL_SUCCESS → stuck
stuck_steps = [
    SDSR("read_file", "TOOL_TIMEOUT"),
    SDSR("read_file", "TOOL_FAILURE"),
    SDSR("read_file", "TOOL_TIMEOUT"),
]
res_stuck = SD.detect(stuck_steps, window_steps=3)
check("stuck_detector: detects stuck", res_stuck.is_stuck)
check("stuck_detector: correct action", res_stuck.action == "read_file")
check("stuck_detector: consecutive_non_success == 3", res_stuck.consecutive_non_success == 3)

# Mixed actions → not stuck
mixed_steps = [
    SDSR("read_file", "TOOL_SUCCESS"),
    SDSR("write_file", "TOOL_SUCCESS"),
    SDSR("read_file", "TOOL_FAILURE"),
]
check("stuck_detector: mixed actions not stuck", not SD.detect(mixed_steps, window_steps=3).is_stuck)

# Same action but one SUCCESS → not stuck
partial_success = [
    SDSR("read_file", "TOOL_FAILURE"),
    SDSR("read_file", "TOOL_SUCCESS"),
    SDSR("read_file", "TOOL_FAILURE"),
]
check("stuck_detector: one success not stuck", not SD.detect(partial_success, window_steps=3).is_stuck)

# ── no_progress_detector_v1 ───────────────────────────────────────────────────

NP = no_progress_detector_v1
NPSR = NP.StepRecord

# All FAILURE in last 5 → no progress
no_prog_steps = [NPSR("FAILURE")] * 5
res_np = NP.detect(no_prog_steps, window_steps=5)
check("no_progress_detector: detects no progress", res_np.no_progress)
check("no_progress_detector: consecutive_non_success == 5", res_np.consecutive_non_success == 5)

# Mix with one SUCCESS → has progress
progress_steps = [NPSR("FAILURE"), NPSR("FAILURE"), NPSR("SUCCESS"), NPSR("FAILURE"), NPSR("FAILURE")]
check("no_progress_detector: one success = has progress", not NP.detect(progress_steps, window_steps=5).no_progress)

# Too few steps → no issue
check("no_progress_detector: too few steps ok", not NP.detect([NPSR("FAILURE")], window_steps=5).no_progress)

# ── Report ────────────────────────────────────────────────────────────────────

if errors:
    for e in errors:
        print(e, file=sys.stderr)
    sys.exit(1)

print("Detectors: OK — loop_detector_v1, stuck_detector_v1, no_progress_detector_v1 all verified")
