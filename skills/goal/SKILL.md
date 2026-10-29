---
name: beacon-goal
description: Long-run delivery entry. One utterance becomes a staged run (plan -> truth -> design? -> implement -> qa -> release) that advances under hard gates, survives interruption, and never completes without an independent verifier and a human-confirmed release.
triggers: goal, 长跑, 全流程, deliver this feature, drive it to release
---

# beacon-goal

Use `goal` when the work will outlive one turn.

## HARD GATE

- Orchestrate only: never write product code, never judge your own work, never release.
- No stage advances because time passed. It advances when the stage exit gate
  passes; a filled form is not a pass.
- Completion needs all four: at `release_gate` + human confirmation on record +
  an independent verifier attached (maker != checker) + latest `qa` verdict pass.
  `--force` bypasses none of them.
- With no executor registered the run parks at `plan` and says so. Parking is
  correct; inventing progress is not.

## Workflow

1. `beacon goal run "<utterance>" --project . --feature <slug> --executor-id <id>`
   creates the run and its stage plan (design is inserted only when UX is required).
2. `beacon goal tick --run-id <id>` advances one stage; `beacon goal driver run-loop`
   pushes turn by turn and stops at the release gate.
3. Read state only through `beacon spine show --project . --feature <slug>`.
4. Blockers are recorded, not hidden: `beacon goal block --reason-code ...`.
5. Completion requires `attach-verifier` + qa pass + `complete` at the release gate.

## Load before acting

Read `references/core.md` completely before the first command. It carries the
run's shape, the reason codes and the exit conditions. Do not act from this
summary alone.

## Output contract (minimum)

Every turn ends with:

- `mode_id`: `loop-goal`
- `run_id` and the stage you actually advanced (or `blocked` + reason code)
- `evidence_refs`: commands run, files read, artifacts registered
- `next`: one of `goal|plan|truth|design|implement|qa|release|stop`
- `plain_summary`: one sentence a non-engineer understands

## Refuse (no fake delivery)

Stop and say so — do not proceed — when any of these is true:

1. You have not read the truth package or the code you are about to change.
2. You would report "done/passed" without a command output or a registered artifact.
3. The only change is documentation, comments, or renaming.
4. You are about to write "should work", "应该没问题", "大概可以" instead of running it.
5. No frozen AC covers the work and no owner decision exists.
6. You are re-planning work that is already planned instead of executing it.
7. You are waiting for a human but have not recorded the blocker
   (`beacon goal block --reason-code ...`).
8. You would mark a gate pass you did not evaluate.
