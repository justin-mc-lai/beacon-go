---
slug: release-policy-shadow
version: v2.0.0
materials_status: current
---

# Tests: release-policy-shadow

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./internal/orchestrator -run TestCreateRunRejectsUnsupportedReleasePolicy -count=1` | `auto` 与未知值被拒绝，reason code 稳定；`human` / `auto_shadow` 接受 |
| AC-002 | TC-002 | behavior | `go test ./internal/orchestrator -run TestReleaseShadowRecordsVerdictButKeepsHumanGate -count=1` | auto_shadow 下影子判定为 pass，且 `ReleaseHumanConfirmed` 仍为 false |
| AC-003 | TC-003 | behavior | `go test ./internal/orchestrator -run TestReleaseShadowStillRequiresHumanComplete -count=1` | 影子 pass 后 `Complete` 不带人工确认仍被拒绝 |
| AC-004 | TC-004 | behavior | `go test ./internal/cli -run TestGoalReleaseShadowCLI -count=1` | `goal release-shadow --run-id` 输出 verdict 并写事件；status 暴露 policy |
| AC-005 | TC-005 | behavior | `go test ./internal/orchestrator -run TestReleasePolicyAutoNotImplemented -count=1` | `auto` 拒绝文案说明未启用，不存在静默降级 |
