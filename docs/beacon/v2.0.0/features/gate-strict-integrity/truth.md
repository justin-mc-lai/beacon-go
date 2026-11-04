---
slug: gate-strict-integrity
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
  truth: docs/beacon/v2.0.0/features/gate-strict-integrity/truth.md
  tests: docs/beacon/v2.0.0/features/gate-strict-integrity/tests.md
  tasks: docs/beacon/v2.0.0/features/gate-strict-integrity/tasks.md
  evidence: docs/beacon/v2.0.0/features/gate-strict-integrity/evidence.md
---

# Requirement Truth: gate-strict-integrity

## 人话

给 Beacon 内核的使用者：现在「冻结之后」的包完整性检查默认降级成只报告不拦截（audit_only），于是 `tasks.md` 少写 `ac=` 这类问题在 release 候选包里也可能一路绿灯。本包把「本次要交付的包」拉回 block 模式，并提供显式 `--strict` 开关；历史冻结包保持 AC-101 的只审计语义，不回溯误伤。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "确认按照推荐方案做，确保beacon-go 在需求真相跟qa验证足够稳定可靠，不做agent"
goal: "让 release 候选包的需求真相完整性检查真正拦得住，同时不回溯改历史冻结包的审计语义"
non_goals: "不做 agent 运行时；不改 gate evaluate release 的四门前置语义；不要求历史冻结包重新冻结"
constraints: "复用既有 CheckIntegrity 与 reason code；默认行为不变，只对候选包收紧；不新增顶层命令"
scope_mode: focused
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 冻结包默认 audit_only 是 AC-101 的故意设计，直接全量改成 block 会误伤历史包，怎么收窄？ | 只对「有 run 的本次候选包」用 block；历史包继续 audit_only | answered | 采用：候选包 = 该 feature 至少有一条 run |
| Q2 | 是否顺便把 integrity 加进 `gate evaluate release` 的四门前置？ | 不：那是 charter 语义变更，本包只改 release 预检与 CLI 开关 | answered | 不改 `EvaluateRelease`；只改版本 release 检查与 `gate integrity --strict` |
| Q3 | 默认值要不要跟着变？ | 不变：默认仍按 AC-101；只提供显式 `--strict` | answered | 默认不变，help 与文档写清差异 |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 目标「显式 strict 开关」 | internal/cli/gate.go:240-294（integrity 子命令现状） |
| AC-002 | Q1 的答案「候选包才 block」 | internal/orchestrator/gates_integrity.go:71-75（默认 audit_only） |
| AC-003 | Q3 的答案「默认不变」 | internal/gate/integrity.go:7（AC-101 原文） |
| AC-004 | 约束「复用既有判定」 | internal/orchestrator/release_check.go:110-115 |
| AC-005 | 非目标「不改四门前置」 | internal/gate/evaluate.go:294-310（release 不含 integrity） |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `beacon gate integrity --strict` 强制 block 模式：包已冻结且存在完整性违规时，verdict 为 blocked、退出码非 0、reason code 稳定 |
| AC-002 | 版本 release 检查对「有 run 的候选包」使用 block 模式；对「无 run 的历史冻结包」仍使用 audit_only |
| AC-003 | 不传 `--strict` 且包已冻结时，`gate integrity` 行为与今天一致（audit_only 默认，AC-101 不回归） |
| AC-004 | 完整性违规的 reason code 与修复路由沿用既有值，不新增第二套词表 |
| AC-005 | `gate evaluate release` 的四门前置语义不变：integrity 仍不是 release 的独立前置 |

## Boundary

- 本包只收紧「候选包」与显式 strict 两条路径；不改冻结前行为（冻结前本来就是 block）。
- 不新增顶层命令；不开 agent；不改 `release_policy`。
