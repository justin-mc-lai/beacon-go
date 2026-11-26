---
name: beacon-go-change
description: Change frozen truth through the one canonical entry. Use for 变更, change request, 改需求, refreeze, 失效子树 or 需求变更; invalidate a frozen tree subtree, drive the bottom-up refreeze, and route legacy package changes to `change legacy`.
---

# beacon-change

`change` is the only first-class change surface in the seven abilities
(plan | truth | change | implement | verify | release | goal). Requirement
truth is the intent tree; `change` invalidates frozen truth and drives the
bottom-up refreeze.

## The one entry

1. `beacon change request --project . --node <node-id> --summary "..." --by <id>`
   — invalidates one frozen subtree; its ancestors become `change_pending`.
   The frozen fingerprint covers the contract face only; evidence appended
   later does not move it.
2. Edit the tree nodes/leaves that the change authorizes. Never edit a
   frozen contract in place without a request first.
3. Refreeze bottom-up until the tree is consistent again. Use `truth refreeze`
   on any legacy package touched by the change.
4. `beacon change list` — read the change records, newest first.

`--base-fingerprint` is an optional optimistic-concurrency guard: pass the
fingerprint you reviewed so a concurrent change is refused instead of silently
overwritten.

## Legacy packages

Old `docs/beacon/<version>/features/<slug>/` packages are readable legacy
evidence, not a second authority. Their change path lives under the same
ability as `beacon change legacy --feature <slug> --summary "..."` (the former
`truth change`), followed by an edit and `beacon truth refreeze`. Tree-first:
prefer `change request` whenever the change is on a tree node.

## Change + refreeze invariant

A frozen body changes only through request -> edit -> refreeze. A reason-only
or machine-only edit that does not move the human narrative is refused at
refreeze; a body edited outside the flow is caught by `truth drift`, not
silently accepted.

## Route examples

Worked routes (open the file before acting; examples are not requirement authority):

- `examples/change-and-refreeze.md` — chain `change-refreeze`: request -> edit -> refreeze on a tree node.
- `examples/drift-check.md` — chain `change-refreeze`: an out-of-flow edit is caught by drift.
- `examples/legacy-package.md` — chain `change-refreeze`: an old feature package changes via `change legacy`.

## HARD GATE

- Never edit a frozen contract in place: `change request` (or `change legacy`)
  first, then edit, then refreeze.
- Never treat a legacy package as a second truth authority; the tree wins.
- Never claim a refreeze passed without the command output that refroze it.
- A pending change blocks freeze; resolve it before attempting to freeze again.

## Load before acting

Read `references/core.md` completely before the first command. Do not act
from this summary alone.

## Output contract (minimum)

- `mode_id`: one of `ch-request|ch-list|ch-legacy|ch-refreeze`
- `change_id` and `node_id` (or legacy `feature` slug) you produced
- `evidence_refs`: commands run and their output
- `next`: one of `implement|truth|stop`
- `plain_summary`: one sentence a non-engineer understands

## Refuse (no fake delivery)

Stop and say so when any of these is true:

1. You have not read the frozen node/package you are about to change.
2. You would refreeze without the command output that did it.
3. The only change is wording or formatting with no contract impact.
4. You would write "应该可以直接改" instead of issuing a change request.
5. You would report a drift or refreeze result you did not evaluate.
6. You would edit a frozen contract in place to make a refreeze gate pass.
