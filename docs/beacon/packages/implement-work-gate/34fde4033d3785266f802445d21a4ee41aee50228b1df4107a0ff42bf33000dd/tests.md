---
slug: implement-work-gate
version: v2.0.0
materials_status: current
---

# Tests: implement-work-gate

## AC Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-IW-001 | behavior | go test ./internal/orchestrator -run TestImplementGatePassesWhenWorkExists | exit_code_0；有未提交改动或有新提交时 pass |
| AC-002 | TC-IW-002 | behavior | go test ./internal/orchestrator -run TestImplementGateBlocksEmptyRun | exit_code_0；干净且未移动 -> blocked + implement_no_work |
| AC-003 | TC-IW-003 | behavior | go test ./internal/orchestrator -run TestImplementGateAllowsEmpty | exit_code_0；allow-empty -> not_applicable + implement_empty_allowed |
| AC-004 | TC-IW-004 | behavior | go test ./internal/orchestrator -run TestImplementGateUnverifiedIsNotRed | exit_code_0；非 git 目录 -> not_applicable + implement_work_unverified |
| AC-005 | TC-IW-005 | behavior | go test ./internal/gate -run TestOtherGatesUnchanged | exit_code_0；其余门检查项集合不变 |
| AC-006 | TC-IW-006 | behavior | go test ./internal/orchestrator -run TestImplementGateIsReadOnly | exit_code_0；评估前后 git status 与 HEAD 不变 |

## Execution slots

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-005 | TC-IW-EXEC-001 | behavior | go test ./... | exit_code_0 |
