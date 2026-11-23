---
slug: program-file-graph
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
  truth: docs/beacon/v2.0.0/features/program-file-graph/truth.md
  tests: docs/beacon/v2.0.0/features/program-file-graph/tests.md
  tasks: docs/beacon/v2.0.0/features/program-file-graph/tasks.md
  evidence: docs/beacon/v2.0.0/features/program-file-graph/evidence.md
---

# Requirement Truth: program-file-graph

## 人话

给 Beacon 内核的使用者：磁盘上的程序图（`docs/beacon/<v>/programs/<slug>/feature-graph.json`）现在没有任何已安装工具能校验，也没有任何 Go 代码读它——它是"计划书"，但不是内核认的那份。本包给两个真动作：`goal program validate --file` 按 v1 的必填键 + 既有依赖/环规则检查这份文件；`goal program import --file --run-id` 把它导入 store，让文件的图变成内核的图，且 `parity_subset` / `status` / `scope_mode` 不丢。做到"图能被检查、能被内核读到"算完。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "那现在 按照 以上帮我 修复。"
goal: "让磁盘程序图可校验、可导入 store，成为内核认的同一份图"
non_goals: "不写 .machine 投影；不复活 v1 Python CLI；不做 status 自动派生（本轮不做 export/drift 消除）"
constraints: "复用 ValidateProgramGraph；只扩展字段；不改变既有子命令语义；失败一律非零退出"
scope_mode: "focused"
```

## Open Questions

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 只加校验器，还是让它真正被内核读？ | 两个都做，import 才是价值所在 | answered | validate + import；不做 standalone 报告面 | human |
| Q2 | v1 的 exit 2 要不要照搬？ | 不照搬：本仓统一 exit 1，单命令特例更糟 | answered | 失败返回非零（1），命令层说明 | human |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 目标「图能被检查」 | beacon/utils/program_graph.py 的必填键集；docs/parity-audit-python-v16.md:107（仅部分对应） |
| AC-002 | 约束「复用既有规则」 | internal/orchestrator/program.go ValidateProgramGraph |
| AC-003 | 目标「不 panic」 | 现有 CLI 无文件读取面，file_missing 无承接 |
| AC-004 | 目标「成为内核认的图」 | internal/orchestrator/program.go ProgramPlan 只吃 ProgramLakeInput |
| AC-005 | 目标「import 前先校验」 | ProgramPlan 内部已调 ValidateProgramGraph；文件键校验必须在写前 |
| AC-006 | Q2 的答案 | cmd/beacon/main.go:24（统一 exit 1） |
| AC-007 | 非目标「不改既有语义」 | internal/cli/goal_program.go 五个既有子命令 |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `program validate --file` 读取图文件；缺 program_slug/version/scope_mode/lakes 各报一条 `missing:<key>`；缺湖字段报 `lakes[i]:missing:<key>` |
| AC-002 | 依赖重复/自依赖/缺依赖/成环/空湖复用既有 reason code（program_lake_duplicate 等） |
| AC-003 | 文件不存在报 `file_missing` 并非零退出；不 panic |
| AC-004 | `program import --file --run-id` 写入 store；`parity_subset`、`status`、`scope_mode`、lake goal 均保留 |
| AC-005 | import 先校验；校验失败不写 store（run 的 program 保持原样） |
| AC-006 | validate 失败时进程非零退出（沿用仓库 exit 1） |
| AC-007 | 既有 plan/change/freeze/admit/route 的参数、输出与语义不变 |

## Parity Matrix

| Capability | Covered | Evidence |
|------------|---------|----------|
| v1 `program validate-graph` 必填键 | 是 | AC-001 |
| v1 依赖/环检查 | 是（复用） | AC-002 |
| v1 `exit 2` | 否（改 exit 1） | AC-006 |
| 文件 → store | 是（v1 无此能力） | AC-004 |
| status 自动派生 / 防 drift | 否（非目标，需要 export 才能做） | — |
