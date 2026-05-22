# handoff_completeness_v1 — Frontier Judge Rubric

**Version:** 1.0
**Judge model target:** claude-sonnet-4 or equivalent
**Used by metric:** `handoff_completeness` (catalog/layer_2_quality/handoff_completeness.yaml)

---

## Purpose

Score whether a HandoffEvent passed sufficient, accurate context for the
receiving agent to continue the task without needing to re-derive information.

---

## Input Fields

| Field | Source proto path | Description |
|---|---|---|
| `from_agent_id` | `payload.handoff.from_agent_id` | Sending agent identifier. |
| `to_agent_id` | `payload.handoff.to_agent_id` | Receiving agent identifier. |
| `handoff_reason` | `payload.handoff.handoff_reason` | Why the handoff occurred. |
| `context_summary` | `payload.handoff.context_summary` | The context passed to the receiving agent. |
| `input_intent` | `payload.task.input_intent` | Original user intent for the task. |

---

## Scoring Prompt

You are evaluating whether an agent-to-agent handoff transferred sufficient context.

Score the handoff context on a scale of 0–4:

| Score | Meaning |
|---|---|
| 4 | **Complete.** The context summary is accurate, covers task state, relevant history, open questions, and the receiving agent has everything needed to continue without re-deriving any prior work. |
| 3 | **Mostly complete.** Minor gaps (a missing detail, slightly ambiguous state description) but a competent receiving agent could proceed. |
| 2 | **Partially complete.** Important context is missing — the receiving agent would need to re-do significant work or ask follow-up questions before proceeding. |
| 1 | **Insufficient.** The context summary is too thin or inaccurate to be useful. |
| 0 | **Missing or harmful.** No useful context was passed, or the summary is actively misleading. |

---

## Output Format

Respond with a JSON object only. No prose before or after.

```json
{
  "score": <integer 0-4>,
  "rationale": "<one to three sentences>",
  "missing_elements": ["<list of what was missing, empty array if score >= 3>"]
}
```
