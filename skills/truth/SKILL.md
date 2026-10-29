---
name: beacon-truth
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
6. `beacon truth freeze --feature <slug>` — four-piece completeness + integrity + intent gate.
7. `beacon truth change --feature <slug> --summary "..."` then edit, then
   `beacon truth refreeze` (or `beacon truth rebase --change <id> --accept-current`
   after a parallel change landed).
8. `beacon truth drift --feature <slug>` — names the ACs that moved.

Rules: truth lives only in `docs/beacon/<version>/features/<slug>/*.md`; the store
keeps pointers and events, never the body. Freeze is canonical-branch only.

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

