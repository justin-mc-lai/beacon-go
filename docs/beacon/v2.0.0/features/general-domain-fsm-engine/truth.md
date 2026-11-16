---
slug: general-domain-fsm-engine
surface_strict: true
version: v2.0.0
status: frozen
language: zh
domain_required: false
ux_required: false
surfaces:
  - cli
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: draft
materials_status: current
intent_confirmed: true
canonical_refs:
  truth: docs/beacon/v2.0.0/features/general-domain-fsm-engine/truth.md
  tests: docs/beacon/v2.0.0/features/general-domain-fsm-engine/tests.md
  tasks: docs/beacon/v2.0.0/features/general-domain-fsm-engine/tasks.md
  evidence: docs/beacon/v2.0.0/features/general-domain-fsm-engine/evidence.md
---

# Requirement Truth: general-domain-fsm-engine

## 人话

在 beacon-go 内核中解耦通用业务状态机（Domain FSM）审查模型，支持 platform/workflow/general 多域类型，消除非电商特性的旅程索要限制，并向外暴露静态 FSM 审查与运行时状态转移判定 CLI，给上层调度器与 LLM 编译器提供机器级支撑。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "确认"
goal: "解决 GitHub Issue #9：解耦 domain_fsm 规则，放宽非电商领域的 User Journey 阻断；暴露 truth fsm check 与 fsm transition CLI 接口"
non_goals: "不破坏现有电商包（business）的完整 DDD 审查；不向内核硬编码特定业务状态"
constraints: "Go 语言原生实现；保持向前兼容；保持 1+6 门禁严格单测"
scope_mode: focused
```

## Open Questions

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 怎样区分电商与非电商状态机包？ | 显式 frontmatter domain_kind，缺省时保留启发式判定 | answered | 显式 domain_kind: platform/workflow/general 时豁免电商 User Journey 审查 |

## Intent Coverage

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 多域类型解耦 | Issue #9: 解耦 domain_kind，非电商包不索要 User Journey |
| AC-002 | 静态 FSM 审查与投影 CLI | Issue #9: 提供 beacon-go truth fsm check 子命令 |
| AC-003 | 运行时状态跃迁裁决器 CLI | Issue #9: 提供 beacon-go fsm transition 运行时判断 |
| AC-004 | 自动化测试与质量门禁 | 1+6 车道要求：提供 Go 单元测试与 autotest 证据 |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 内核 gate/intent.go 与 truthmd.go 能够识别 `domain_kind: platform \| workflow \| general`，豁免电商用户旅程（User Journey）阻断并正常通过静态审查 restates=1 |
| AC-002 | 提供 `beacon truth fsm check --project ... --feature ... [--json]` CLI 命令，能够导出特性包的状态机结构、实体序及非法跃迁覆盖度 restates=2 |
| AC-003 | 提供 `beacon fsm transition --project ... --feature ... --from <S1> --to <S2>` CLI 命令，输出结构化跃迁判定（包含合法通过、守卫未满足、命中非法跃迁三种明确判定结果） restates=2 |
| AC-004 | 编写 Go 单元测试覆盖上述三个能力，并通过 `beacon-go qa autotest` 验证全绿 restates=3 |
