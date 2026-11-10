---
slug: acceptance-seed-red-test
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: acceptance-seed-red-test

## Task Ledger

- [x] TASK-000 plan 轮：落四件套、可编译红测试、build 与 FAIL 原始输出 · ac=GATES · evidence=docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/red-orchestrator-pkg.txt
- [ ] TASK-001 proposal 校验拒收不可执行的 acceptance_seed，错误文案指出需 go test 命令 · ac=AC-001 · evidence=internal/orchestrator/lake_proposal.go
- [ ] TASK-002 harvest 要求实现产物与验收测试产物同时存在，只带测试产物即拒 · ac=AC-002 · evidence=internal/orchestrator/lanes.go
- [ ] TASK-003 harvest 要求 acceptance_seed_red 基线证据存在且早于实现产物 · ac=AC-003 · evidence=internal/orchestrator/lanes.go
- [ ] TASK-004 更新既有 fixture 的散文 seed 为可执行命令（只改测试数据） · ac=AC-001 · evidence=internal/orchestrator/lake_proposal_test.go、internal/cli/goal_p2d_test.go、internal/cli/goal_proposal_gate_test.go
- [ ] TASK-005 复跑三条种子命令由 FAIL 转 PASS，`go test ./...` 全绿，落 red 与 green 双原始输出 · ac=AC-001..AC-003 · evidence=docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/
- [ ] TASK-006 独立验收三方（spec/AC、运行时行为、负向回归）并行复核，三份 verdict 齐备后才可进入合入 · ac=GATES · evidence=docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/

## Boundary

本包 plan 轮停在 freeze 前：不执行 `truth freeze`，不 commit、不 push、不发布、不打 tag，不碰 release 与价格。TASK-000 只新增一个测试文件与四份文档，未改任何 internal 非测试生产代码。实现轮先合入测试、确认红，再写生产逻辑转绿；禁止执行者改写断言或自出 verdict。
