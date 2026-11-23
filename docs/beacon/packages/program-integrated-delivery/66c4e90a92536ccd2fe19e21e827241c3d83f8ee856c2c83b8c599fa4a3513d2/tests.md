---
slug: program-integrated-delivery
version: v2.0.0
materials_status: current
---

# Tests: program-integrated-delivery

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./internal/orchestrator -run '^TestProgramIntegratedHandoff$' -count=1 -v` | 三个入口对未集成、错误 SHA、交接后证据失效和有效交接判定一致；下游基线包含上游被验成果。 |
| AC-002 | TC-002 | behavior | `go test ./internal/orchestrator -run '^TestProgramIntegratedParallel$' -count=1 -v` | 两个真实 worker 在独立工作树用同步屏障证明执行重叠；冲突阻断 D；建工位和派工中断恢复不重复；未知和共享改动面跨批串行；残留目录未提交文件不丢。 |
| AC-003 | TC-003 | behavior | `go test ./internal/orchestrator -run '^TestProgramIntegratedApproval$' -count=1 -v` | 总 QA 失败或过期不能签；拒绝单湖绕过；中断后旧签字仅对同候选续跑，最终关闭宿主。 |

独立 QA 检查每条目标测试确实运行，另跑构建、全量测试、`go vet ./...`、`bash e2e/run_e2e.sh`，记录候选 SHA 和结果。当前为验收合同，测试证据待实现后登记。

Illegal transitions：TC-001 验证集成前不放行；TC-003 验证未签直接完成、单湖绕过和候选失效均被拒绝。
