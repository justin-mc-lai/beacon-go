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
