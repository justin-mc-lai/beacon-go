<!-- Source: distilled from Python v1.6.19 skills/beacon/beacon-plan/
     (SKILL.md + references/modes/INDEX.md). The Python package
     carries one file per mode; this core keeps the mode index and
     the load discipline, re-expressed for the Go surfaces. -->

# beacon-plan — core reference

## Load discipline (Python original)

Select exactly ONE primary mode before acting, read its contract
completely, emit the mode output + `mode_id` +
`recommended_next_harness`, and stop at the harness boundary —
route to another public core instead of stretching this one.

## Mode index

| mode_id | purpose | notes |
|---------|---------|-------|
| pln-review | multi-angle planner review | reviewers run as real subprocesses in Go; one valid block blocks all; the verdict writes back into the run's state machine (P0 blocks implement, drift routes to change/refreeze) |
| pln-brainstorm | brainstorm before freeze | Socratic four questions + first-principles decomposition; in Go the questions live in the truth ask/answer ledger, one at a time, each carrying a recommendation |
| pln-source | source-anchored research | anchor every claim to code that exists; "对着源码，别瞎编" — findings feed the intent snapshot |
| pln-program | sea-to-lake program plan | dependency graph over lakes; a cycle or a missing dependency is refused, and a frozen plan's graph changes only through a recorded change |
| pln-program-auto / pln-program-interactive | program variants | same graph, different driving: automatic advance vs step-by-step human routing |
| pln-friction | friction intake | capture what hurt during delivery as a first-class input to the next plan |

## What planning produces in the Go kernel

- A question ledger with no open questions.
- An intent snapshot (utterance, goal, non-goals, constraints,
  scope mode) that survives verbatim into the freeze gate.
- For full_parity scope: a deferral ledger entry for anything
  deferred — silent scope reduction is refused at sign time.
- A planner-review verdict recorded against the run, with its
  severity routing applied (not just filed).
