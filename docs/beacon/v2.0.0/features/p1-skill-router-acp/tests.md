---
slug: p1-skill-router-acp
version: v2.0.0
materials_status: current
---

# Tests: p1-skill-router-acp

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./skills/ -run TestRouterTableSchema -count=1` | router_table_schema_holds（路由表可解析，每个技能含触发信号、例句与排除条件） |
| AC-002 | TC-002 | behavior | `go test ./skills/ -run TestRouterMatchesEverySample -count=1` | router_hits_expected_skill（表内每条例句都命中其所属技能，且唯一） |
| AC-003 | TC-003 | behavior | `go test ./skills/ -run TestRouterRefusesToGuess -count=1` | router_no_guess（无命中与多义命中返回未命中加候选，不猜一个技能） |
| AC-004 | TC-004 | behavior | `go test ./skills/ -run TestProgressiveMapOrder -count=1` | progressive_map_declared（每个技能声明首批必读参考顺序，顺序可断言） |
| AC-005 | TC-005 | behavior | `go test ./skills/ -run TestProgressiveMapPathsExist -count=1` | must_read_paths_exist（路由结果附带的必读参考路径在磁盘真实存在） |
| AC-006 | TC-006 | behavior | `go test ./internal/harness/ -run TestAcpRealIteration -tags=acpreal -count=1` | acp_iteration_holds（对真实 ACP agent 目标完成 1 次 session/prompt 迭代，输出符合 AC-042 schema） |
| AC-007 | TC-007 | behavior | `go test ./internal/harness/ -run TestAcpRefusesWithoutTarget -count=1` | acp_refuses_not_fabricates（无真实目标时返回明确 reason_code 且不返回伪造结果；超时被正确约束） |

## 真跑口径

- TC-006 需真实 ACP agent 目标（全局 AC-046 的前置条件），用 build tag `acpreal` 隔离：默认 `go test ./...` 不跑。
- 目标不在场时测试 skip；skip 不等于通过，AC-006 保持未达成直到真目标证据入库。
