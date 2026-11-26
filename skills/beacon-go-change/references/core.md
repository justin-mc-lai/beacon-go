<!-- Re-expressed against the Go intent-tree change surface. -->

# beacon-change — core reference

## Hard gate

A frozen contract changes only through request -> edit -> refreeze.
Forbidden: in-place edits of frozen truth, refreeze without command output,
treating a legacy package as a second authority.

## Surface index

| intent | Go surface |
|--------|------------|
| invalidate a frozen tree subtree | `beacon change request --node <id> --summary "..." --by <id>` |
| list change records | `beacon change list` |
| change a legacy feature package | `beacon change legacy --feature <slug> --summary "..."` |
| close a pending change | `beacon truth refreeze` (tree) or `beacon truth rebase --accept-current` (parallel base) |
| detect out-of-flow edits | `beacon truth drift --feature <slug>` |

## Change discipline

- `change request` marks one subtree invalid; ancestors become `change_pending`.
  `--base-fingerprint` is the optimistic-concurrency guard: a concurrent
  change is refused instead of silently overwritten.
- The frozen fingerprint covers the contract face only; evidence appended
  later does not move it. A reason-only or machine-only refreeze is refused.
- A pending change blocks freeze; resolve it before freezing again.
- Release/Completed is terminal: a change or refreeze never downgrades a
  released node's lifecycle.
- The store never holds truth text: change records point at the tree; the
  intent-tree JSON and the package markdown are the only authorities.
