---
name: beacon-goal
description: Long-run delivery entry. One utterance becomes a staged run (plan -> truth -> design? -> implement -> qa -> release) that advances under hard gates, survives interruption, and never completes without an independent verifier and a human-confirmed release.
triggers: goal, 长跑, 全流程, deliver this feature, drive this to release
---

# beacon-goal

Use `goal` when the work will outlive one turn.

1. `beacon goal run "<utterance>" --project . --feature <slug> --executor-id <id>`
   Creates the run and its stage plan (design is inserted only when UX is required).
2. `beacon goal tick --run-id <id>` advances one stage; `beacon goal driver run-loop`
   pushes turn by turn and stops at the release gate.
3. Read state only through `beacon spine show --project . --feature <slug>`.
4. Blockers are recorded, not hidden: `beacon goal block --reason-code ...`.
5. Completion requires `attach-verifier` + qa pass + `complete` at the release gate.

Boundaries: goal orchestrates; it never implements, never self-passes QA, and never
releases without a human confirmation.
