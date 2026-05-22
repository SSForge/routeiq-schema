# plan_appropriateness_v1 — Frontier Judge Rubric

**Version:** 1.0
**Judge model target:** claude-sonnet-4 or equivalent
**Used by metric:** `plan_appropriateness` (catalog/layer_2_quality/plan_appropriateness.yaml)

---

## Purpose

Score whether the sequence of decisions an agent made was an appropriate
strategy for the given task, independent of whether the outcome was achieved.

---

## Input Fields

| Field | Source proto path | Description |
|---|---|---|
| `input_intent` | `payload.task.input_intent` | The user's original request. |
| `decisions` | `payload.decision.*` (all DecisionEvents in run) | Sequence of decision records: type, options considered, chosen option, rationale. |
| `actual_outcome` | `payload.task.actual_outcome` | What the agent ultimately produced. |

---

## Scoring Prompt

You are evaluating whether an AI agent's decision-making strategy was appropriate for its task.

You will receive:
- **Input intent**: the user's original request
- **Decisions**: a sequence of decisions the agent made (tool selection, path choices, etc.)
- **Actual outcome**: what the agent ultimately produced

Score the agent's plan/decision sequence on a scale of 0–4:

| Score | Meaning |
|---|---|
| 4 | **Highly appropriate.** The strategy was well-matched to the task. The agent chose a direct, efficient path with no unnecessary steps or risky detours. |
| 3 | **Mostly appropriate.** Minor suboptimalities (an unnecessary tool call, a slightly roundabout path) but no significant missteps. |
| 2 | **Partially appropriate.** The strategy addressed the task but included significant waste or a questionable choice that a competent agent would avoid. |
| 1 | **Poorly appropriate.** The decision sequence shows systematic misjudgement — wrong tool class, redundant loops, escalating complexity inappropriately. |
| 0 | **Inappropriate.** The strategy was unrelated to or actively harmful for the task. |

---

## Output Format

Respond with a JSON object only. No prose before or after.

```json
{
  "score": <integer 0-4>,
  "rationale": "<one to three sentences>",
  "primary_issue": "<null if score >= 3, else one of: wrong_tool_choice | unnecessary_steps | risky_escalation | goal_mismatch | inefficient_path>"
}
```
