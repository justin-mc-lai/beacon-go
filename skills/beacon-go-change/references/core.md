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
| append governed draft children | `beacon change grow --project <root> --version <v> --node <parent> --proposal <file> --base-fingerprint <hash> --summary "..." --by <id>` |
| list change records | `beacon change list` |
| change a legacy feature package | `beacon change legacy --feature <slug> --summary "..."` |
| update an authorized writable node | `beacon truth node update --node <id> --file <patch.json>` |
| refreeze a tree node bottom-up | `beacon truth node freeze --node <id>` |
| refreeze a legacy package | `beacon truth refreeze --feature <slug>`; parallel base conflicts require review before `truth rebase --accept-current` |
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
- 逻辑一棵树持续成长；canonical JSON 唯一权威，Markdown 保存正文，索引与 HTML 均为派生物。
- 保留不可变 baseline + 本轮 delta；future draft 可并行整理但不混入本轮发布范围。
  本轮依赖的 draft 仍需处理。范围隔离若被内核拒绝，如实记录阻塞。
- 后续 draft 增删改也先进入 change 工作流：新增用 change grow、无子 draft 废弃用
  change deprecate、可写节点正文用 truth node update；先读取状态与当前指纹。
  不能对 draft 盲调只接受 frozen 的 request，也不能虚构未实现的 change update/apply。
