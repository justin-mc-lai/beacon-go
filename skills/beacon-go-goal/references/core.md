<!-- Source: distilled from Python v1.6.19 skills/beacon/beacon-goal/
     (SKILL.md hard gate + references/modes/loop-goal.md). Content is
     re-expressed against the Go binary's surfaces; the Python mode
     files remain the deeper reference. -->

# beacon-goal — core reference

## Hard gate (Python original)

Orchestrate the pipeline only. Complete needs verifier evidence.
Release is always a human gate. Long-run and supervision live only
on goal. Stage exits are hard (`goal_stage_exit_gates`); entering
implement requires workspace admission + freeze ack — a filled form
is not a pass.

## The run's shape in the Go kernel

- One utterance -> one run: `beacon goal run "<utterance>" --project . --feature <slug>`.
- Stage plan: plan -> truth -> design (only when UX is required) ->
  implement -> qa -> release. The 12-state loop machine is separate
  from the stage plan; both must agree before anything advances.
- Advance with `beacon goal tick` (idempotent) or let the driver
  push: `beacon goal driver run-loop --run-id <id>` — it stops at
  the release gate on its own; budgets and quota decide whether a
  turn may run at all (waiting for a human/evidence/focus is a
  requeue, not a failure).
- Interruptions are normal: heartbeat + checkpoint, then
  `beacon goal resume --run-id <id>` replays the interrupted
  substep. The kill switch `.beacon/loop/pause-all` halts drivers.
- Completion is a four-condition hard gate: at release_gate + human
  confirmation on record + an independent verifier attached
  (maker != checker) + latest qa verdict pass. `--force` never
  bypasses any of the four.

## Shared preamble

1. Lake vs sea — boil lakes; split/defer seas.
2. Search before invent — truth, source, evidence, memory first.
3. User sovereignty on scope and freeze.
4. No fake delivery (placeholder, docs-only, zero assertions).
