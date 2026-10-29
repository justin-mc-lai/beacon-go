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

## Consequences

- Splitting any of these is still welcome; it must first prove the result is
  smaller and clearer, not merely more files.
- The guard is `scripts/check_file_size.sh` (also run in CI). Deleting or
  renaming an exception does not require an ADR update, but adding one does.
