# SDD truth contract (two files)

`beacon truth create --sdd` lays down two files:

| File | Role |
|---|---|
| `truth.md` | the SDD Spec: intent snapshot, open questions, intent coverage, subagent plan, FSM (diagram + transitions), acceptance criteria, AC/TC matrix, out-of-scope |
| `tasks.md` | task DAG with dependencies, AC bindings and evidence paths |

`tests.md`, `evidence.md` and `restatement.md` are not required. Evidence lives in
`evidence/` and is referenced from tasks and the coverage matrix. Old four-piece
packages keep working unchanged.

## Freeze contract

Creation produces an unfilled scaffold, never a pre-approved requirement. Fill the
intent, grounding table (`Source | Reference | Supports`), assumptions, AC/TC
matrix and executable task graph. Declare `package_author`, `fsm_required` and
`negative_required` explicitly. SDD evidence traceability cannot be disabled.

Every AC needs a positive behavior case. When `negative_required: true`, each AC
also needs a negative case. TC IDs must be unique; empty, documentation-only and
success-only commands do not prove behavior. Counts are not a substitute for coverage.
Stateful requirements carry a normalized FSM plus `Transition Coverage` rows:
`From | To | Trigger | TC ID`. Stateless work declares `fsm_required: false`.
For identical transition triples in different aggregates, prepend an `Aggregate`
column and bind each aggregate separately. Optional prose-only FSM rationale is
not a model; a present malformed transition table still blocks.

Before confirmation, an independent reviewer supplies a JSON receipt:

```json
{"fingerprint":"<current fingerprint>","reviewer":"checker","verdict":"pass","findings":[],"summary":"What was reviewed and why the requirements agree with the grounded intent"}
```

Inspect the fingerprint with `beacon truth semantic-review --project . --feature
<slug> --json`; record the external receipt with the same command plus `--file
review.json`. Then use `truth confirm --by <owner>` and `truth freeze`. The kernel
records the receipt on its event trail. Editing truth or task definitions invalidates
the review and confirmation. Identities are caller-attested, not authenticated;
the kernel does not perform semantic judgment or call a model.

## Task execution

`tasks.md` uses the exact table:

```text
TASK | Action | Depends | AC | Command | Verify | Evidence | Auto
```

Dependencies and ACs are comma-separated; `-` means no dependency. Cycles,
unknown dependencies, duplicate task IDs and unknown ACs are refused. Use
`beacon implement tasks run --run-id <id>` after admission. The runner executes
Command then Verify in deterministic dependency order and stops on failure.
`beacon implement tasks status --run-id <id>` reads the latest bound report.
Before success, every verification runs again against the final candidate so a
later task cannot silently invalidate an earlier task. Evidence files are copied
to immutable invocation/task snapshots; shared authored evidence paths do not
overwrite history. Candidate hashes cover project inputs with explicit recorded
runtime/output exclusions. Direct `go test` verification forces fresh verbose
execution and rejects zero tests or skipped cases; other custom commands remain
trusted verification programs, not a proof of semantic assertion quality.

Repair is disabled by default. Opt in with `--repair-cmd <program>`, repeatable
`--repair-arg`, `--max-repairs 1..5` and `--timeout-sec`. The external repair
receives pointers to failed result/stdout/stderr through environment variables.
Each attempt is retained under `.beacon/task-runs/` and registered by hash.
Repairing the requirement contract, stale candidates, cancellation, timeout and
exhaustion fail rather than becoming completion. No automatic release or install
lane is provided. Executables are trusted project code; this is workdir isolation,
not a container or security sandbox.

## FSM runtime

Both `State | Event | Guard | To | Action` and
`From | To | Trigger | Guard | Side effect` normalize to the same graph.
`truth fsm check` validates the graph; `fsm transition --from A --to B` refuses
missing, invalid or ambiguous edges. Use `--event` to disambiguate and repeat
`--guard '<exact text>'` for caller-attested satisfied guards. This is not an
evaluation of live business state. Refusals emit structured output and exit nonzero.

## Compatibility

- Legacy packages (no `package_contract: truth-tasks`) still require the four-piece set.
- `truth migrate --dry-run` previews legacy-to-SDD adoption; `--tasks-file` supplies an authored executable DAG when a checkbox ledger cannot be translated without guessing. Original tests/evidence/restatement files remain intact and truth/tasks are backed up. Migration invalidates old confirmation and requires normal completion/review/signing again. See `docs/beacon/v2.1.5/sdd-migration-examples.md`.

## Deterministic draft intake

`truth decompose` performs non-LLM rule matching and reports `semantic_ready: false`.
For SDD it updates only its own suggestions section and a runtime decomposition
artifact. It does not replace authored ACs, coverage or tasks, and it never
turns a discovered test command into claimed semantic coverage.

## Content-bearing lake proposals

Opt in with proposal `package_contract: truth-tasks` and each lake's authored
`truth` / `tasks` strings. The proposal validates applicability, AC/TC/DAG references
and applicable state models, but reports `sdd_content_validated_draft`, not frozen.
Legacy proposals report `legacy_shape_only`. Selection creates two-file drafts;
every selected SDD package must pass its own reviewed freeze before program freeze.
