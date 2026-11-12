---
slug: qa-surface-e2e
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: qa-surface-e2e

## Task Ledger

- [ ] TASK-SUR-01 冻结 truth：确认 AC-001 到 AC-010、两条硬门规则、legacy waiver 与扩展名白名单 · ac=AC-001,AC-002,AC-003,AC-004,AC-005,AC-006,AC-007,AC-008,AC-009,AC-010
- [ ] TASK-SUR-02 surface 模型：解析 frontmatter 的 surfaces 列表与 surface_strict，AC 行读取 surface= 列并在缺省时继承，冻结记录与 truth show --json 暴露同一集合 · ac=AC-001,AC-002
- [ ] TASK-SUR-03 freeze 校验与老包兼容：empty/invalid/undeclared 三种拒绝 reason，新冻包自动 strict，缺 surfaces 的老冻包不回溯，gate status 只给 surface_undeclared warn · ac=AC-001,AC-002,AC-003
- [ ] TASK-SUR-04 desktop-app 类型：QAProjectTypes 加值，InferProjectType 识别 electron/tauri/appId/entitlements 且优先于 web-app，QAApplicability 加 desktop-app 列并在注释写明与 web-app 的差异 · ac=AC-004,AC-005
- [ ] TASK-SUR-05 ui-runner：functional 与 compatibility 分支认 ego-browser 与 agent-desktop，生成 web 与 desktop 两条 qae2e 命令，其他 runner 行为不变 · ac=AC-006
- [ ] TASK-SUR-06 证据类型：登记白名单与 qaKind 加 a11y_tree/browser_trace/e2e_report，按扩展名拒绝并在写库与事件之前失败 · ac=AC-007
- [ ] TASK-SUR-07 qa 事实采集：evalQA 读取 .beacon/autotest 的 run 目录与登记产物，读取 truth 的 surfaces/surface_strict，按面收集证据命中 · ac=AC-008,AC-009,AC-010
- [ ] TASK-SUR-08 EvaluateQA 硬门：qa_autotest_recorded 与 qa_surface_evidence 两条 fail-closed；strict 失败关闭，legacy 只给 legacy_waiver_surface warn；任一 FAIL 令 verdict 为 fail · ac=AC-008,AC-009,AC-010
- [ ] TASK-SUR-09 gate 单测：surface freeze 三拒绝、AC 继承、desktop 推断与矩阵、两条 runner 命令、两条硬门与 legacy waiver · ac=AC-001,AC-002,AC-004,AC-005,AC-006,AC-008,AC-009,AC-010
- [ ] TASK-SUR-10 orchestrator 单测：老包 status warn、artifact 扩展名接受与拒绝、真实包 evaluate 的 PASS 与 FAIL JSON · ac=AC-003,AC-007,AC-008,AC-009
- [ ] TASK-SUR-11 全量验证：go test ./... 全绿；git status 无意外文件；commit 只到本地，不 push，不安装 · ac=GATES

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound test or artifact exists (completion ≠ self-tick). 本阶段不勾选任何任务；实现阶段只改 `internal/**` 与 `cmd/**`，不动已冻结的 truth/tests/tasks/evidence。

