---
slug: skills-distribution
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
  truth: docs/beacon/v2.0.0/features/skills-distribution/truth.md
  tests: docs/beacon/v2.0.0/features/skills-distribution/tests.md
  tasks: docs/beacon/v2.0.0/features/skills-distribution/tasks.md
  evidence: docs/beacon/v2.0.0/features/skills-distribution/evidence.md
---

# Requirement Truth: skills-distribution

## 人话

skills 已经是 agent 时代的分发形态。beacon-go 已能把 1+6 技能装进 8 个宿主，但仓库本身还不是「一键装」的插件形态，也没有一个命令能告诉你某个宿主装漏了哪个技能。本包补上插件清单和 `skills verify` 漂移检查；不追技能数量，只保证装得上、装得全、不碰 Python v1 的旧技能。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "把上面的推荐全部走 /beacon-go-plan 第一性原理 /beacon-go-goal 冻结并实现"
goal: "让 beacon-go 的 1+6 技能可被一键安装、可校验、且不覆盖 v1"
non_goals: "不新增第七个技能；不追技能数量；不改 v1 的 beacon-* 安装"
constraints: "复用 skills/hosts.go 的宿主表；安装前缀保持 beacon-go；插件清单只列 1+6"
scope_mode: focused
```

## Open Questions

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 插件清单列出哪些技能？ | 只列 1+6（goal/plan/truth/design/implement/qa/release） | answered | 采用，不列 gov 内部面 |
| Q2 | `skills verify` 默认是否让 CI 失败？ | 默认只报告；`--strict` 才非零退出 | answered | 采用，避免把开发机漂移变成 CI 噪声 |
| Q3 | v1 的 `beacon-*` 目录算不算 extra？ | 不算；只校验 `beacon-go*` 命名空间 | answered | 采用，绝不删/改 v1 安装 |

## Intent Coverage

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | Q1 的答案 | skills/skills.go:InstallPrefix；skills 目录 |
| AC-002 | 目标「装漏了能发现」 | skills/hosts.go:HostDirs |
| AC-003 | Q2 的答案 | internal/cli/skills_install.go:newSkillsHostsCmd |
| AC-004 | Q3 的答案 | docs/changes/change-skill-namespace-2026-10-29.md |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 仓库提供 `.claude-plugin/plugin.json` 与 `.claude-plugin/marketplace.json`，插件列出 1+6 技能的路径 |
| AC-002 | `beacon skills verify` 对每个已存在的宿主报告缺失/多余的 `beacon-go*` 技能 |
| AC-003 | `skills verify` 默认只报告并以 0 退出；`--strict` 在任一已存在宿主缺失技能时非零退出且 reason code 稳定 |
| AC-004 | 校验与安装只认 `beacon-go*` 命名空间；Python v1 的 `beacon-*` 不删除、不覆盖、不计为 extra |

## Boundary

- 本包是分发面，不改六个技能的内容语义。
- 不新增顶层命令；能力落在 `beacon skills` 下。
