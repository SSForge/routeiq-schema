# outcome_correctness_v1 — Frontier Judge Rubric

**Version:** 1.0  
**Judge model target:** claude-sonnet-4 or equivalent frontier model  
**Used by metric:** `task_success_rate` (catalog/layer_2_quality/task_success_rate.yaml)

---

## Purpose

Score whether an AI agent achieved the user's actual goal.  
This rubric is evaluated by a frontier reasoning model, not a heuristic.

---

## Input Fields

| Field | Source proto path | Description |
|---|---|---|
| `input_intent` | `payload.task.input_intent` | The user's original request, verbatim. |
| `expected_outcome` | `payload.task.expected_outcome` | What a correct outcome would look like, if specified. |
| `actual_outcome` | `payload.task.actual_outcome` | What the agent actually produced or did. |

---

## Scoring Prompt

You are evaluating whether an AI agent successfully completed a user's request.

You will receive:
- **Input intent**: the user's original request
- **Expected outcome**: what success looks like (may be empty if not specified)
- **Actual outcome**: what the agent produced

Score the agent's outcome on a scale of 0–4:

| Score | Meaning |
|---|---|
| 4 | **Fully correct.** The agent achieved exactly what the user asked. If expected_outcome is specified, the actual outcome satisfies it completely. |
| 3 | **Mostly correct.** The agent achieved the primary goal but missed minor details, produced extra unasked-for work, or left a trivial loose end. |
| 2 | **Partially correct.** The agent completed part of the task but missed a significant portion of the user's intent. The user would need to do non-trivial follow-up. |
| 1 | **Attempted but failed.** The agent tried but did not achieve the user's goal. The output is not useful without substantial rework. |
| 0 | **Wrong or harmful.** The agent either did something unrelated to the request, produced a harmful output, or crashed without meaningful progress. |

---

## Output Format

Respond with a JSON object only. No prose before or after.

```json
{
  "score": <integer 0-4>,
  "rationale": "<one to three sentences explaining the score>",
  "primary_failure": "<null if score >= 3, else one of: wrong_output | incomplete | off_task | harmful | no_progress>"
}
```

---

## Evaluation Guidelines

- Judge on **goal achievement**, not presentation quality (formatting, verbosity, style).
- If `expected_outcome` is empty or null, infer success criteria from `input_intent` alone.
- A score of 2 or below counts as a failed task for `task_success_rate` computation.
- Partial credit exists (scores 1–3) to distinguish "attempted" from "completely wrong".
- Do not penalise the agent for being overly cautious or verbose if the goal was met.
- Do penalise for incomplete tool use, missing artefacts, or ignored constraints stated in `input_intent`.
