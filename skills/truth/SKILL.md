---
name: beacon-go-truth
description: Requirement truth package lifecycle. Ask first, sign the intent, freeze the package, govern changes, and self-check requirement-level drift.
triggers: 冻需求, truth, freeze, change, refreeze, 需求真相
---

# beacon-truth

The chain is **ask -> sign -> freeze -> govern -> self-check**:

1. `beacon truth create --project . --slug <slug>`
2. `beacon truth ask --feature <slug> --question "..." --recommend "..."`
3. `beacon truth answer --feature <slug> --id Q1 --answer "..."`
4. `beacon truth intent --feature <slug> --utterance "..." --goal "..." --non-goals "..." --constraints "..."`
5. `beacon truth confirm --feature <slug>` — refuses while any intent finding remains.
6. `beacon truth freeze --feature <slug>` — completeness + integrity + intent gate;
   SDD packages add the two-file gates below.
7. `beacon truth change --feature <slug> --summary "..."` then edit, then
   `beacon truth refreeze` (or `beacon truth rebase --change <id> --accept-current`
   after a parallel change landed).
8. `beacon truth drift --feature <slug>` — names the ACs that moved.

Rules: truth lives only in `docs/beacon/<version>/features/<slug>/*.md`; the store
keeps pointers and events, never the body. Freeze is canonical-branch only.

## Version = delivery line (SDD)

- Greenfield: the first `truth create` auto-opens `v0.0.1`.
- Same iteration: every feature shares the project's current docs version.
- New iteration: `beacon projects bump --level patch` (minor/major only when explicit).
- `truth create --version auto` = current; `latest` = largest registered line.
- Docs versions are pure `vX.Y.Z`; runtime lines (`2.0.0-go.12`) are a separate
  axis and are refused as docs versions.

## SDD two-file contract

`beacon truth create --sdd` writes only `truth.md` + `tasks.md`:

- `truth.md` is the Spec: intent snapshot, open questions, intent coverage,
  subagent plan, FSM (Mermaid `stateDiagram` + From/To table), acceptance
  criteria, AC/TC matrix, out-of-scope.
- `tasks.md` is the task DAG with dependencies, AC bindings and evidence paths.
- `tests.md`, `evidence.md` and `restatement.md` are optional. Evidence lives in
  `evidence/` and is referenced from tasks and the matrix.

Freeze floors for an SDD package:

1. At least 5 acceptance criteria.
2. At least 3 executable TCs per AC, at least 2 of them negative.
3. A `## FSM` section with a Mermaid `stateDiagram` and a From/To transition
   table of at least 3 rows.
4. A `## Subagent Plan` covering the roles `beacon truth roles` selects.
5. No placeholder phrases.

`beacon truth roles --utterance "..."` prints the on-demand subagent plan:
stages, selected roles, deferred specialists and coverage. A workflow utterance
without `domain-fsm-architect` is not ready.

Legacy four-piece packages keep the old contract; only packages declaring
`package_contract: truth-tasks` use the two-file gates.

## Integrity and feature map

- `beacon gate integrity --strict` forces block mode for an already-frozen package
  (release-candidate tightening); the default stays AC-101 audit-only.
- `beacon truth map build` writes `feature-map.md` (AC -> entry point + runnable
  verification command); `beacon truth map verify` reports fresh/stale/missing.
- Release/Completed is terminal: a new run, refreeze, or tick never downgrades a
  released feature's lifecycle.

## Route examples

Three worked routes; open the file before acting (examples are not
requirement authority — the frozen body is):

- `examples/freeze-readiness.md` — chain `full-delivery`: ask → answer → confirm → freeze on a complete package.
- `examples/change-and-refreeze.md` — chain `change-refreeze`: a frozen body changes only through change → edit → refreeze.
- `examples/drift-check.md` — chain `change-refreeze`: a body edited outside the flow is caught by drift, not silently accepted.

## HARD GATE

- Never freeze while an intent question is open or an AC has no intent source.
- Never edit a frozen body in place: `truth change` first, then edit, then `refreeze`.
- Never move requirement text into the database; the Markdown package is the only authority.
- Freeze on the canonical branch only; a dirty or side branch is a stop, not a workaround.

## Load before acting

Read `references/core.md` completely before the first command. It carries the
freeze preconditions, the integrity dimensions and the reason codes. Do not act
from this summary alone.

## Output contract (minimum)

- `mode_id`: one of `tr-ask|tr-sign|tr-freeze|tr-change|tr-drift`
- `package_path` and the fingerprint or change id you produced
- `evidence_refs`: commands run and their output
- `next`: one of `truth|design|implement|qa|stop`
- `plain_summary`: one sentence a non-engineer understands

## Refuse (no fake delivery)

Stop and say so — do not proceed — when any of these is true:

1. You have not read the package you are about to change.
2. You would claim "frozen" without the command output that froze it.
3. The only change is wording, formatting, or renaming with no AC impact.
4. You are about to write "应该可以冻", "大概没问题" instead of running the gate.
5. The user has not answered the open question and you would answer it for them.
6. You would edit the frozen body directly to make a gate pass.
7. You are re-asking a question the intent snapshot already records.
8. You would report a drift or integrity result you did not evaluate.

