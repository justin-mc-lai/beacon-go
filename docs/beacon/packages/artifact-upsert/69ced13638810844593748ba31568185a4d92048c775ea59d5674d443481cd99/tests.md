---
slug: artifact-upsert
version: v2.0.0
materials_status: current
---

# Tests: artifact-upsert

## AC Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-AU-001 | behavior | go test ./internal/store -run TestAddUpsertsByPath | exit_code_0；两次 Add 同一路径后行数为 1，hash 为最新 |
| AC-002 | TC-AU-002 | behavior | go test ./internal/store -run TestAddKeepsIdentity | exit_code_0；更新后 id 与首次一致 |
| AC-003 | TC-AU-003 | behavior | go test ./internal/orchestrator -run TestQAStalenessUsesLatestPerPath | exit_code_0；历史行 hash 不符不影响判定 |
| AC-004 | TC-AU-004 | behavior | go test ./internal/orchestrator -run TestQALegacyDuplicatesNoMigration | exit_code_0；预置重复行仍判 pass |
| AC-005 | TC-AU-005 | behavior | go test ./internal/orchestrator -run TestQAReRegisterClearsStale | exit_code_0；改文件再注册后不再 stale |
| AC-006 | TC-AU-006 | behavior | go test ./... | exit_code_0；schema 列集合不变 |

## Execution slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-006 | TC-AU-EXEC-001 | behavior | go test ./... | exit_code_0 |
