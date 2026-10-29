---
name: beacon-qa
description: Evidence-based acceptance. Register real artifacts with hashes, evaluate the qa gate, and score skills without editing them.
triggers: 验收, QA, evidence, scorecard, 证据
---

# beacon-qa

1. Produce the artifact (report, test output, screenshot) and register it:
   `beacon goal artifacts register --run-id <id> --kind qa_report --path <file> --by <id>`
2. Evaluate: `beacon qa evaluate --run-id <id> --evaluator <id>` (persists the verdict).
3. Inspect: `beacon gate status --project . --feature <slug>`.
4. Optional skill health: `beacon qa score <skill>` records only; it never edits the skill.
5. Tampering is caught: every gate re-hashes registered artifacts and marks stale ones.

Rules: scores are health checks, not verdicts; qa never rewrites truth to pass.

## Route examples

Three worked routes; open the file before acting (examples are not
requirement authority):

- `examples/register-then-evaluate.md` — chain `full-delivery`: register the artifact, then evaluate the qa gate.
- `examples/failed-verdict.md` — chain `honest-block`: a blocked verdict is reported with its reason codes, not retried silently.
- `examples/score-does-not-edit.md` — chain `honest-block`: a score is a health check and never edits the skill or passes a gate.

## HARD GATE

- Only registered artifacts count. A file on disk that was never registered is
  not evidence; a gate re-hashes what is registered and marks the rest stale.
- Scores never pass or block a gate, and never edit a skill.
- Never rewrite the truth package, the tests, or the artifact to make a verdict pass.
- A failed verdict is a result to report, not a step to retry silently.

## Load before acting

Read `references/core.md` completely before the first command. It carries the
evidence rules, the scorecard dimensions and the reason codes. Do not act from
this summary alone.

## Output contract (minimum)

- `mode_id`: one of `qa-gate|qa-review|qa-score`
- `verdict` with its reason codes, or the score with its dimensions
- `evidence_refs`: registered artifacts (path + hash) and the command output
- `next`: one of `implement|release|stop`
- `plain_summary`: one sentence a non-engineer understands

## Refuse (no fake delivery)

Stop and say so — do not proceed — when any of these is true:

1. You have not read the artifact you are about to judge.
2. You would report "pass" without a gate evaluation output.
3. The evidence is a document describing tests rather than test output.
4. You are about to write "测试应该都过了" instead of running them.
5. The tests do not cover the AC they claim to cover.
6. You are the maker of the code you are judging.
7. You are waiting for evidence that nobody produced, and have not said so.
8. You would score a skill and treat the score as a verdict.

