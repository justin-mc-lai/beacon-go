<!-- Source: distilled from Python v1.6.19 skills/beacon/beacon-truth/
     (SKILL.md hard gate + references/modes: gen-truth, gen-truth-init,
     truth-review, eval-truth-review, gen-change, gen-refreeze).
     Re-expressed against the Go truth lifecycle. -->

# beacon-truth — core reference

## Hard gate (Python original)

Write or freeze package truth only after plan readiness.
Forbidden: implement, fake freeze, QA self-pass.

## Mode index (Python) -> Go surface

| mode | Go surface |
|------|------------|
| gen-truth-init | `beacon truth create` (four-piece skeleton: truth/tests/tasks/evidence) |
| gen-truth | fill the skeleton, then the ask -> sign chain of beacon-plan |
| truth-review | the freeze precheck: intent gate (A intent coverage, B domain-FSM graph, C testability incl. docs-only rejection, D fill depth, E vague-word refusal) + the Truth Integrity Gate |
| eval-truth-review | `beacon gate integrity` — verdict persisted, findings itemised with reason codes |
| gen-change | `beacon truth change` — records the change, never edits the frozen body; materials are marked stale |
| gen-refreeze | `beacon truth refreeze` — refuses an empty refreeze, a revision drift, and a diverged parallel-change base (`beacon truth rebase --accept-current` is the explicit escape) |

## Freeze discipline

- Freeze is canonical-branch only, and a frozen package counts as
  frozen only once its body is a committed baseline
  (`beacon truth baseline` records it).
- Requirement-level fingerprints are taken at freeze time;
  `beacon truth drift` names the added/changed/removed ACs instead
  of reporting a vague "package changed".
- The store never holds truth text: pointers, lifecycle and events
  only. The package markdown is the single authority.
