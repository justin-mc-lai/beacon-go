# Migration to 3.0.0

## 1. Binary and version

- `beacon --version` must print `Beacon CLI v3.0.0`.
- Version is a single source: `internal/version/version.go`.

## 2. Ledger (SQLite)

Opening a v1 ledger migrates it to schema v2 automatically:

- `artifact_refs` gains `scope_kind` + `scope_id`;
- rows backfill scope from the run's feature;
- an artifact whose run has no feature aborts the migration with
  `artifact_scope_backfill_failed` instead of guessing.

Back up `~/.beacon/beacon-go.db` (and `-wal`/`-shm`) before first open. To roll back,
stop beacon, restore the backup files, and reinstall the previous binary.

## 3. Old sea/lake packages

`goal program`, `truth decompose`, and `goal parallel-plan` are removed. Old flat
feature packages are imported into the tree as `feature` nodes:

```bash
beacon migrate tree --project <root> --version <v>
beacon plan tree   --project <root> --version <v>
```

Migration is snapshot-first and idempotent. It does not rewrite package truth or
evidence; the imported nodes keep `legacy_truth_path` / `legacy_tests_path` until a
`change`/`refreeze` moves them into the canonical JSON body.

## 4. Commands renamed or removed

| Old | New |
|---|---|
| `beacon qa <sub>` | `beacon verify <sub>` (status/evaluate/score/autotest/…) |
| `beacon goal program …` | removed; use `plan init/expand` + `truth node freeze` |
| `beacon truth decompose` | removed; tree expansion is host-driven via `plan expand` |
| `beacon goal parallel-plan` | removed |
| `beacon goal delegate-ralph` | removed; use `implement report` + `receive-result --external` |
| `--executor subprocess|ralph|profile` | removed; use the manual lane and `goal tick --by` |

## 5. Host-driven execution

The kernel no longer runs agents. The host agent:

1. expands nodes with `plan apply` (proposal JSON includes provenance);
2. freezes leaves with `truth node freeze` (AC + verification required);
3. executes work and reports it with `implement report --artifact <path> --by <id>`;
4. registers verification evidence with `verify leaf --artifact <path> --by <id>`;
5. rolls parents up with `verify rollup --node <parent>`.

`goal tick --run-id <id> --by <identity>` advances plan/truth/implement/qa; the tree
stage gate blocks on draft or unverified nodes.

## 6. Desktop / HTTP / LLM

The desktop workspace, `goal serve`, the kernel LLM client, and agentapi are removed
in 3.0.0. Hosts that used them must call the CLI or read the repo JSON directly.

## 7. Rollback

- Restore the SQLite backup; schema v1 is not written back by v3.0.0.
- Reinstall the previous binary through `scripts/local_release.py`.
- Tree JSON is additive; delete the version's `tree/` directory only if the project
  never consumed it.
