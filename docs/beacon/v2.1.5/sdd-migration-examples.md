# Legacy package migration examples

Migration changes the package contract, not its approval status. It never invokes
an LLM, invents test commands, signs a restatement, or records a semantic review.
Use a registered project and the feature's canonical branch. These examples are
command recipes, not a claim that any particular package has passed acceptance.

## Existing executable task DAG

A legacy package with a valid executable TASK table can keep its tasks unchanged:

```sh
beacon truth migrate --project /path/to/project --feature account-export \
  --by requirement-author --dry-run --json
beacon truth migrate --project /path/to/project --feature account-export \
  --by requirement-author --json
```

Dry-run changes neither authored package files nor ledger data. SQLite uses a
consistent online backup for queries; its source lock/index sidecars may appear.
The temporary snapshot is cleaned up. Apply creates a unique `.beacon/migrations/sdd-*` directory
containing the exact prior `truth.md`, exact prior canonical tasks bytes stored as
`tasks.md`, and a report identifying the original canonical paths. The source
`tests.md`, `evidence.md`, and `restatement.md` are never deleted or rewritten.
The old coverage tables, including commands, assertions, layers, evidence paths,
and required test names, become the truth Coverage Matrix without re-rendering
their rows. All coverage tables are copied, not just the first one.

## Checkbox tasks require an authored DAG

Checkboxes do not determine executable commands or dependencies. Author a real
TASK table using commands that exist in your project; do not copy hypothetical
commands into a real migration. For example, a Go project that really contains
`TestExport` and `TestExportRefusal` could supply `migration-tasks.md`:

```markdown
| TASK | Action | Depends | AC | Command | Verify | Evidence | Auto |
|---|---|---|---|---|---|---|---|
| TASK-export | Implement export | - | AC-001 | go build ./cmd/export | go test ./internal/export -run TestExport -count=1 | evidence/export.txt | false |
| TASK-refusal | Verify invalid input refusal | TASK-export | AC-001 | go test ./internal/export -run TestExportRefusal -count=1 | go test ./internal/export -run TestExportRefusal -count=1 | evidence/refusal.txt | false |
```

```sh
beacon truth migrate --project /path/to/project --feature account-export \
  --tasks-file /path/to/migration-tasks.md --by requirement-author --dry-run --json
beacon truth migrate --project /path/to/project --feature account-export \
  --tasks-file /path/to/migration-tasks.md --by requirement-author --json
```

The explicitly supplied DAG must pass the same `ParseTaskDAG` validation as task
execution. A conflicting existing executable DAG is refused rather than silently
replaced. The original checkbox document remains available in the migration
backup. Without `--tasks-file`, checkbox-only tasks are refused.

## Frozen packages must open a change

A frozen legacy package cannot be migrated directly:

```sh
beacon truth change --project /path/to/project --feature account-export \
  --summary 'Adopt the two-file SDD contract while retaining legacy sources' \
  --by requirement-owner
beacon truth migrate --project /path/to/project --feature account-export \
  --tasks-file /path/to/migration-tasks.md --by requirement-author --dry-run --json
```

Apply uses the same command without `--dry-run`. The package remains
`change_pending`; migration does not close its opened change. Merely editing a
status to `change_pending` does not count as a genuine change request. The trail
and matching pending change document must exist.

## Refusals and subsequent authoring

- A distinct Coverage Matrix already present in truth is a conflict. Resolve it
  deliberately before migration; neither matrix is selected by guessing.
- Unsupported columns, malformed rows, and tables that would lose rows are
  refused. Original source files remain intact.
- Missing or invalid executable tasks, unknown AC references, and conflicting
  task dependencies are refused.
- Original truth and tasks are backed up before staging. A failed truth commit
  rolls tasks back. If rollback itself fails, the error and reported backup path
  must be used to restore the exact originals before continuing.
- An already-SDD package with unchanged input is reported as `already_sdd`
  without rewriting files or creating another backup. Migration is not a task
  editing command.

The migrated truth is an unsigned `package_maturity: scaffold` with
`intent_confirmed: false`. Existing review and confirmation metadata in truth is
cleared. Retained legacy signed files cannot approve the new SDD contract.
User intent, open questions, and human-authored journeys remain in truth.

Complete the applicable grounding, risk/assumptions, intent coverage, FSM or
explicit stateless declaration, negative cases, and other SDD sections. Declare
the real package author. Obtain and record an independent semantic review for the
current content, then request fresh human confirmation. Only then run the normal
freeze or refreeze gates. None of these steps is performed by migration.
