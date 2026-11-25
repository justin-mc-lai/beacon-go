# Plan core reference

## Modes

- `pln-brainstorm`: ask one question at a time, each with a recommendation.
- `pln-fsm`: model domain states, transitions, and illegal moves.
- `pln-review`: multi-angle review; one blocking reviewer blocks the plan.
- `pln-source`: read the repo before claiming anything.
- `pln-tree`: build the intent tree.
- `pln-friction`: record friction as a truth question.

## Tree planning

Kind matrix:

```
intent -> intent | module
module -> module | feature
feature -> touchpoint
touchpoint -> (leaf)
```

Commands:

```bash
beacon plan init   --project . --title <title> --intent <intent>
beacon plan expand --node <id> --children proposal.json   # validate only
beacon plan apply  --node <id> --proposal proposal.json   # write drafts
beacon truth tree  --project . --json
```

Rules:

- At most 7 children per expansion.
- Every child needs a title; touchpoints need an AC and a verification spec.
- The proposal may carry `provenance` (model, prompt hash, skill, tokens, cost);
  the kernel records it and never calls a model.
- The host writes proposals; only the kernel mints ids and paths.
