---
slug: truth-version-iteration
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
  truth: docs/beacon/v2.0.0/features/truth-version-iteration/truth.md
  tests: docs/beacon/v2.0.0/features/truth-version-iteration/tests.md
  tasks: docs/beacon/v2.0.0/features/truth-version-iteration/tasks.md
  evidence: docs/beacon/v2.0.0/features/truth-version-iteration/evidence.md
---

# Requirement Truth: truth-version-iteration

## 人话

给 Beacon 内核的使用者：SDD 的版本是“交付迭代线”，不是“每个 feature 的序号”。本包把这条规则落成可执行行为：绿项目第一次建 truth 自动开 `v0.0.1`；同一迭代里的多个 feature 共享当前版本；新迭代必须显式 `projects bump` 升 patch/minor/major；版本统一归一化成 `vX.Y.Z`，并拒绝把 docs 版本写成 runtime 版本。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "第一性原理思考下，sdd 规范要求就是要顺序迭代演进，是否沿用旧的python方案，还是有更好的方案"
goal: "版本按迭代线顺序演进：绿项目 v0.0.1，同迭代共享，显式 bump 开新线，docs/runtime 双轴不混"
non_goals: "不做每次 truth create 自动 +1；不改 runtime 版本号规则；不迁移历史 docs 目录"
constraints: "一个 current 指针；显式 bump；版本归一化；双轴拒绝；不静默创建未注册版本"
scope_mode: focused
```

## Open Questions

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 每次建 truth 要不要自动 +1？ | 不要；同迭代多 feature 共享版本 | answered | 显式 `projects bump` 才开新线 |
| Q2 | 绿项目默认版本？ | `v0.0.1` | answered | 首次 `truth create` 自动注册并进入 v0.0.1 |
| Q3 | docs 版本能不能等于 runtime 版本？ | 不能；必须归一化为纯 `vX.Y.Z` | answered | 拒绝 `x.y.z-go.N` 形态 |

## Intent Coverage

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | Q2 的答案 | internal/orchestrator/truth.go:CreateTruthPackage |
| AC-002 | Q1 的答案 | internal/orchestrator/truth.go |
| AC-003 | Q3 的答案 | internal/orchestrator/versioning.go |
| AC-004 | 目标「双轴不混」 | internal/orchestrator/versioning.go |
| AC-005 | 目标「显式开新线」 | internal/cli/projects.go |
| AC-006 | 目标「skill 可执行」 | skills/truth/SKILL.md |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 无任何已注册版本的绿项目执行 `truth create`（不传 `--version`）时，自动注册 `v0.0.1` 并在其下建包 |
| AC-002 | 已有版本的项目的 `truth create` 默认使用 current；`--version auto` 用 current，`--version latest` 用最大 docs 版本 |
| AC-003 | 显式 docs 版本归一化为 `vX.Y.Z`；非法格式拒绝；未注册的显式版本拒绝并提示先 bump/register |
| AC-004 | docs 版本为 runtime 形态（含 `-go.`）时拒绝，reason code 稳定 |
| AC-005 | `beacon projects bump --level patch` 从 current/最大版本升 patch 并设为 current；minor/major 仅在显式 level 时发生 |
| AC-006 | truth skill 写清绿默认、显式 bump、docs/runtime 双轴；全仓 test/vet/e2e 通过 |

## Boundary

- 不自动把历史 feature 迁到新版本；版本切换只影响后续 `truth create`。
- 不改 runtime `v2.0.0-go.N` 的规则；只保证 docs 版本不写成 runtime 形态。
