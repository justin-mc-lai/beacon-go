---
slug: package-task-binding
version: v2.0.0
status: frozen
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: filled
materials_status: current
intent_confirmed: true
canonical_refs:
  truth: docs/beacon/v2.0.0/features/package-task-binding/truth.md
  tests: docs/beacon/v2.0.0/features/package-task-binding/tests.md
  tasks: docs/beacon/v2.0.0/features/package-task-binding/tasks.md
  evidence: docs/beacon/v2.0.0/features/package-task-binding/evidence.md
---

# Requirement Truth: package-task-binding

## 人话

给 Beacon 内核的使用者：任务清单（tasks.md）现在可以自己打勾而没人拦——`beacon gate integrity` 只查 evidence.md 引用的文件在不在，不查 tasks.md 那一行有没有写"对应哪个需求、证据是什么"。本包把 v1 的 `task_binding_missing` 规则补进 Go 的包校验：每行必须写 `ac=`，已勾选的行必须写 `evidence=`，且当 evidence 是路径形态时该文件必须存在且非空。做到"打勾必须能指到一个真实或可核对的证据，不能凭空勾"算完。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "那现在 按照 以上帮我 修复。"
goal: "把 v1 的 task ac=/evidence= 绑定规则移植进 Go 的 pkgvalidate，让任务清单不能脱离证据打勾"
non_goals: "不复活 Python v1 CLI；不改已冻结 truth 的 AC 判定；不新增 truth validate-package 子命令；不要求 evidence 必须是路径"
constraints: "复用 gate.CheckIntegrity 的注入式 ResolveRef/FileExists；诊断走既有 gate integrity 面"
scope_mode: "focused"
```

## Open Questions

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | v1 要求 evidence= 必须是存在的文件，本仓却是测试函数名，怎么落地？ | 路径形态才查文件，标识符形态算声明（迁移成本与诚实性折中） | answered | 路径形态（含 `/` 或扩展名）查存在性且 size>=2；标识符形态按声明接受 | human |
| Q2 | 未勾选行是否也强制 evidence=？ | 不强制：未声称完成就不能算"无证据完成" | answered | 只要求未勾选行有 ac= | human |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 目标「任务清单不能脱离证据」 | internal/gate/pkgvalidate.go:13-16（明确写了未移植该规则）；docs/parity-audit-python-v16.md:129（弱承接） |
| AC-002 | 同上 | 同上 |
| AC-003 | Q1 的答案 | beacon/utils/feature_package.py:380-408（v1 检查存在与 size>=2） |
| AC-004 | Q2 的答案 | beacon/utils/feature_package.py:367-379（v1 对未勾选行也强制，本包收窄） |
| AC-005 | 非目标「不改 AC 判定」 | internal/gate/integrity.go:CheckIntegrity 的既有检查链 |
| AC-006 | 约束「迁移后启用」 | 本仓 30 行任务中 15 行缺 ac=、9 行缺 evidence=（实测） |
| AC-007 | 非目标「不新增 CLI」 | docs/parity-audit-python-v16.md:69（validate-feature-package merged→gate integrity） |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | tasks.md 任一行缺 `ac=` 时产生 `task_binding_missing`，Detail 含任务 id，并进入 gate integrity 的 findings |
| AC-002 | 已勾选行缺 `evidence=` 时产生 `task_binding_missing` |
| AC-003 | 已勾选行的 evidence 为路径形态时，文件缺失、文件小于 2 字节这两种情形均产生 `checkbox_checked_without_evidence_file`；标识符形态不查文件 |
| AC-004 | 未勾选行缺 `evidence=` 不产生 finding（缺 `ac=` 仍报） |
| AC-005 | 既有 AC/TC/evidence/frontmatter 检查的 kind 与 verdict 不变；新规则只追加 findings |
| AC-006 | 迁移后本仓四个既有冻结包在 `gate integrity` 下不新增 task-binding findings |
| AC-007 | 不新增 CLI 子命令；诊断通过既有 `gate integrity --feature` 暴露 |

## Parity Matrix

| Capability | Covered | Evidence |
|------------|---------|----------|
| v1 `task_binding_missing` | 是（收窄：未勾选不要 evidence=） | AC-001/002/004 |
| v1 `checkbox_checked_without_evidence_file` | 是（仅路径形态） | AC-003 |
| v1 `_evidence_width_mismatch_hint` | 否 | 非目标：文件名宽度提示，价值低于成本 |
