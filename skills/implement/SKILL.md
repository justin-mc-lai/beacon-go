---
name: beacon-implement
description: Implementation lanes behind one admission check and one harvest rule. Single, team, and ralph lanes share the same entry gate and result contract.
triggers: 实现, implement, 写代码, ralph, worktree
---

# beacon-implement

1. `beacon implement plan --run-id <id>` shows the lane routing and admission verdict.
2. `beacon implement admit --run-id <id>` is read-only; a blocked admission stops all writes.
3. `beacon implement worktree create|merge|cleanup|status --run-id <id>` isolates parallel work.
   Satellites merge into the main star only; protected paths never merge.
4. `beacon implement delegate-ralph --run-id <id>` hands the stage to the ralph lane.
5. `beacon implement receive-result` harvests artifacts + evidence under one rule.

Rules: no admission, no implement; no evidence, no completion claim.

## Route examples

Three worked routes; open the file before acting (examples are not
requirement authority):

- `examples/admission-blocked.md` — chain `honest-block`: an unfrozen package or unbound design blocks before code is written.
- `examples/parallel-worktrees.md` — chain `full-delivery`: parallel lakes stay in separate worktrees; satellites merge into the main star.
- `examples/delegate-ralph.md` — chain `full-delivery`: the implement stage is handed to a real loop and the result is harvested, not assumed.

## HARD GATE

- No admission, no write. A blocked admission stops every lane, not just one.
- Satellites merge into the main star only; protected paths (`.env`, keys, auth,
  payment, migrations) never merge.
- The maker never certifies: the same executor that wrote the code may not attach
  the verifier or pass the qa gate.
- Never mark a stage done without an artifact registered under the harvest rule.

## Load before acting

Read `references/core.md` completely before the first command. It carries the
lane routing, the admission items and the harvest rule. Do not act from this
summary alone.

## Output contract (minimum)

- `mode_id`: one of `im-single|im-team|im-ralph`
- `run_id`, the lane, and the worktree path you actually used
- `evidence_refs`: commits, commands, registered artifacts
- `next`: one of `qa|implement|stop`
- `plain_summary`: one sentence a non-engineer understands

## Refuse (no fake delivery)

Stop and say so — do not proceed — when any of these is true:

1. You have not read the truth package or the files you are about to change.
2. You would claim "implemented" without a command output or a commit hash.
3. The only change is documentation, comments, or renaming.
4. You are about to write "应该能跑", "大概没问题" instead of running the build and tests.
5. Admission is blocked and you are looking for a way around it.
6. You are re-planning instead of writing the code the plan already names.
7. You are blocked by a dependency but have not recorded it.
8. You would attach a verifier to your own work.

