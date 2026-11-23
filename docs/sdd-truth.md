# SDD truth contract (two files)

`beacon truth create --sdd` lays down two files:

| File | Role |
|---|---|
| `truth.md` | the SDD Spec: intent snapshot, open questions, intent coverage, subagent plan, FSM (diagram + transitions), acceptance criteria, AC/TC matrix, out-of-scope |
| `tasks.md` | task DAG with dependencies, AC bindings and evidence paths |

`tests.md`, `evidence.md` and `restatement.md` are not required. Evidence lives in
`evidence/` and is referenced from tasks and the coverage matrix. Old four-piece
packages keep working unchanged.

## Freeze floors

The SDD contract is blocked at freeze unless all hold:

1. At least 5 acceptance criteria.
2. At least 3 executable TCs per AC, with at least 2 negative TCs per AC.
3. A `## FSM` section with a Mermaid `stateDiagram` and a From/To table of at least 3 transitions.
4. A `## Subagent Plan` whose roles cover the utterance plan (`beacon truth roles`).
5. No placeholder phrases.

## Flow

```bash
beacon truth create --project . --slug order-flow --sdd
beacon truth roles --utterance "订单审批流程..."      # see which roles must appear
# fill truth.md and tasks.md
beacon truth intent --project . --feature order-flow --utterance ... --goal ... --non-goals ... --constraints ...
beacon truth confirm --project . --feature order-flow --by <name>
beacon truth freeze  --project . --feature order-flow --by <name>
```

## Compatibility

- Legacy packages (no `package_contract: truth-tasks`) still require the four-piece set.
- Migration of old packages is out of scope for this version; they remain readable and freezable.
