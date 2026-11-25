---
name: beacon-go-truth
description: Requirement truth package lifecycle. Use for 冻需求, truth, freeze, change, refreeze, or 需求真相; ask one question at a time, sign the intent, freeze the package, govern changes through change/refreeze, and self-check requirement-level drift.
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

- `truth.md` is the Spec: grounded intent, questions, assumptions, AC/TC matrix,
  explicit `fsm_required` / `negative_required`, applicable FSM and transition coverage.
- `tasks.md` is the executable DAG (`TASK | Action | Depends | AC | Command | Verify | Evidence | Auto`).
- `tests.md`, `evidence.md` and `restatement.md` are optional. Evidence lives in
  `evidence/` and is referenced from tasks and the matrix.

Freeze requirements for an SDD package:

1. Replace scaffold content; record `package_author`, grounding and resolved assumptions.
2. Every AC has a positive observable case; negative cases follow the explicit risk policy.
3. Unique TC IDs and real verification commands. Fixed AC/TC counts are not quality proof.
4. Stateful work uses the shared parsed FSM and transition-to-TC coverage; stateless work explicitly declares `fsm_required: false`.
5. Obtain the content fingerprint with `truth semantic-review --feature <slug> --json`.
   An independent reviewer returns JSON (`fingerprint`, `reviewer`, `verdict`, `findings`, `summary`).
   Record with `truth semantic-review --feature <slug> --file <receipt.json>`.
6. `truth confirm --by <owner>` binds that exact reviewed content; edits invalidate it.

`truth roles` recommends specialists only. Role names are not execution receipts,
semantic review evidence or freeze prerequisites. The external harness performs
the review; the kernel checks the recorded receipt and never calls an LLM.
Execute admitted task graphs via `implement tasks run`; optional bounded repair
uses an explicit external command. Release and installation remain human actions.

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

