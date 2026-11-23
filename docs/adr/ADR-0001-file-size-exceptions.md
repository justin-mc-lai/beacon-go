# ADR-0001: file-size exceptions for the 500-line rule

- Status: accepted
- Date: 2026-10-29
- Applies to: AC-025 ("单文件模块超过 500 行须拆分或 ADR 说明例外")

## Decision

The 500-line ceiling stays the default. The files below are the recorded
exceptions; a new file over 500 lines fails `scripts/check_file_size.sh`
until it is either split or added here with a reason. This is the "ADR
说明例外" half of AC-025, not a waiver of the rule.

## Exceptions (measured 2026-10-29, non-test Go files)

| File | Lines | Why one unit |
|---|---:|---|
| `internal/orchestrator/hooks.go` | 726 | One hook lifecycle (contract → admission → confirmed execution → post-release evidence). The state machine is a single ordered switch; splitting it across files separated the guard from the write it protects. |
| `internal/gate/domainfsm.go` | 724 | One FSM graph parser + validator. States, transitions, illegal-move checks and triples share one grammar; a split would duplicate the tokenizer. |
| `internal/core/reasons.go` | 624 | The reason-code vocabulary is one closed table. It is data, not logic; splitting it makes "is this code defined?" a multi-file lookup. |
| `internal/core/decompose.go` | 579 | Ocean→lake decomposition: detection, plan shape and record writer are one transaction with one schema. |
| `internal/cli/goal.go` | 569 | The goal command group's constructors. Each subcommand is a thin flag binder over the engine; splitting by file only moves the same 20-line functions. |
| `internal/workspace/workspace.go` | 567 | The desktop/CLI shared read model. One open/read/write contract; the HTTP layer that used to justify a split is gone (desktop-only decision). |
| `internal/orchestrator/driver.go` | 566 | Driver lease + turn loop + checkpoint. The lease and the turn it protects must be read together to reason about dual-start safety. |
| `internal/gate/fsm.go` | 564 | Truth-FSM validator; same grammar as `domainfsm.go` but for requirement truth. |
| `internal/gate/integrity.go` | 543 | The Truth Integrity Gate scanner: one pass over one package with shared finding/reason types. |
| `internal/gate/intent.go` | 538 | The ask-before-sign gate: one ordered checklist. Splitting hides the order, which is the contract. |
| `internal/gate/qastrategy.go` | 535 | qa9 strategy inference + applicability + presets; the preset table is data. |
| `internal/orchestrator/bundle.go` | 528 | Verification bundle assembly: admission + topology + evidence in one artifact shape. |
| `internal/orchestrator/truth.go` | 522 | The freeze/refreeze service: precheck, write, fingerprint and baseline in one transaction. |

### Release review 2026-11-23

The following existing units are retained for this release. New task-DAG and
semantic-review code is split into separate files; this entry does not permit
unbounded growth. Splitting these unrelated stateful services during release
would broaden the change beyond the tested SDD cutover.

| File | Lines | Why retained as one unit |
|---|---:|---|
| `internal/cli/goal_program.go` | 555 | Existing program command constructors share flag and engine routing; no SDD task execution was added here. |
| `internal/gate/pkgvalidate.go` | 546 | Legacy package validation and its contract-specific DAG dispatch share diagnostic generation; the DAG parser itself is separate. |
| `internal/orchestrator/autotest.go` | 514 | Existing discovery/execution/registration transaction; task repair was implemented separately rather than expanding this unit. |
| `internal/orchestrator/intent.go` | 506 | Existing intent ledger mutations and signing order; independent receipt parsing and fingerprint code live in separate units. |
| `internal/orchestrator/program_integrated.go` | 1146 | Existing multi-lake integration state machine predates this repair; splitting requires its own transaction-boundary review. |
| `internal/orchestrator/truth_change.go` | 601 | Existing governed change/refreeze transaction; not expanded by the SDD task runner. |
| `internal/orchestrator/engine.go` | 565 | Existing goal-run lifecycle and persistence entry points; no new task execution loop added here. |
| `internal/orchestrator/version_manifest.go` | 637 | Existing immutable delivery-line manifest lifecycle; outside the SDD runtime cutover. |
| `internal/orchestrator/delivery_paradigm.go` | 688 | Existing conformance evidence composition; this repair changes its coverage source/default, not its workflow shape. |

## Consequences

- Splitting any of these is still welcome; it must first prove the result is
  smaller and clearer, not merely more files.
- The guard is `scripts/check_file_size.sh` (also run in CI). Deleting or
  renaming an exception does not require an ADR update, but adding one does.
