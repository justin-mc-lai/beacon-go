# Plan core reference

## Modes

- `pln-brainstorm`: ask one question at a time, each with a recommendation.
- `pln-fsm`: model domain states, transitions, and illegal moves.
- `pln-review`: multi-angle review; one blocking reviewer blocks the plan.
- `pln-source`: read the repo before claiming anything.
- `pln-brownfield`: intake an inherited codebase; `truth code2truth` produces candidate claims, not authority. Record unknowns and optional candidate structure, then hand off to truth.
- `pln-tree`: propose an optional candidate tree.
- `pln-friction`: record friction as a truth question.

## Tree planning

plan 是调查、头脑风暴、分析；没有既有树、HTML 或已冻结包也可以开始。
完成条件是有来源的发现、目标/范围选项、风险和待决取舍。候选树与候选 HTML 均可选。
truth 才将本轮范围形成唯一 canonical 树，交 HTML 审阅并冻结。
`plan init/expand`、`plan apply` 是工具命名，不倒推工作流阶段。
若已有 canonical 树，后续节点变更交 change，不另起一棵平行树。

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
