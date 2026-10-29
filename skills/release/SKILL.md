---
name: beacon-release
description: Final go/no-go. Release passes only when every gate is green and a human confirms; otherwise it stays blocked with named reasons.
triggers: 发布, release, 上线, gate
---

# beacon-release

1. `beacon release evaluate --run-id <id> --evaluator <id>` computes the gate.
2. Fix the named blocks (truth, design, implement, qa) — do not lower the bar.
3. `beacon release complete --run-id <id> --human-confirmed...` completes only at the
   release gate, with a verifier attached and the human confirmation recorded.

Rules: no human nod, no release. The release gate never infers consent.

## Route examples

Three worked routes; open the file before acting (examples are not
requirement authority):

- `examples/human-confirmation.md` — chain `human-gate`: all gates green still waits for an explicit human confirmation.
- `examples/blocked-release.md` — chain `honest-block`: a missing prerequisite gate keeps the run blocked with named reasons.
- `examples/hotfix-fast-lane.md` — chain `human-gate`: a hotfix plan states the fix version and rollback before release review.

## HARD GATE

- All gates green + an explicit human confirmation, or it stays blocked with the
  named reasons. There is no "close enough".
- Never lower a gate to make release pass; fix the named block instead.
- Never infer consent from silence, from a previous approval, or from "the user
  seemed happy". The confirmation must be recorded for this run.
- A verifier must be attached and must not be the maker.

## Load before acting

Read `references/core.md` completely before the first command. It carries the
release conditions and the reason codes. Do not act from this summary alone.

## Output contract (minimum)

- `mode_id`: `rel-check|rel-complete`
- `verdict` with its reason codes, or the completion record
- `evidence_refs`: the gate output and the confirmation record
- `next`: `release|qa|implement|stop`
- `plain_summary`: one sentence a non-engineer understands

## Refuse (no fake delivery)

Stop and say so — do not proceed — when any of these is true:

1. You have not read the latest gate verdicts for this run.
2. You would claim "released" without the `release complete` output.
3. The only change since the last release is documentation.
4. You are about to write "可以发了" instead of evaluating the gate.
5. A gate is blocked and you are looking for a flag that skips it.
6. You are treating your own approval as the human confirmation.
7. The human has not actually answered, and you would proceed anyway.
8. You would complete a run whose verifier is the maker.

