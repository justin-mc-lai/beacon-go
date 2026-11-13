---
slug: restatement-gate
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: restatement-gate

实现阶段只在 truth 冻结后开始；勾选一项必须附可复跑证据。

- [ ] TASK-01 `internal/core/reasons.go` 增加五个 reason code 常量 · ac=AC-006
- [ ] TASK-02 `internal/gate/restatement.go`：frontmatter 解析 + 正文 sha256 + 五段/子标题校验 · ac=AC-001
- [ ] TASK-03 `freezePrecheck` 插入 restatement 校验（surface 之后、EvaluateIntegrity 之前），首次冻结强制 · ac=AC-002
- [ ] TASK-04 refreeze 复用 `recs.freeze == nil` 走 legacy waiver，只 warn · ac=AC-003
- [ ] TASK-05 AC 行 `restates=` 解析与节号追溯，老包 warn · ac=AC-004
- [ ] TASK-06 `TruthPackageView` 增加 restatement 块并在 `truth show --json` 输出 · ac=AC-005
- [ ] TASK-07 单测五负例 + 两正例（含插入顺序断言） · ac=AC-007
- [ ] TASK-08 e2e seed：真实二进制 8 场景 + 两次 freeze 完整输出 · ac=AC-007
- [ ] TASK-09 `go test ./...` 全绿，既有测试零回归 · ac=AC-007
- [ ] TASK-10 本地 commit，不 push、不安装 · ac=AC-008
