---
slug: acceptance-seed-red-test
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
  truth: docs/beacon/v2.0.0/features/acceptance-seed-red-test/truth.md
  tests: docs/beacon/v2.0.0/features/acceptance-seed-red-test/tests.md
  tasks: docs/beacon/v2.0.0/features/acceptance-seed-red-test/tasks.md
  evidence: docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence.md
---

# Requirement Truth: acceptance-seed-red-test

## 人话

给 Beacon 内核维护者与验收方：现在 lake proposal 的 acceptance_seed 与 truth 的 acceptance_seeds 都是自由文本（internal/orchestrator/lake_proposal.go:41、internal/core/decompose.go:155），验收写在 Markdown 里，执行者写一段话就能自称交付。本包要求每条验收种子本身就是一条当前会失败的可执行测试：propose 阶段拒收非命令种子，plan 阶段落下真实红测试并记录 seed 文件 hash 与 FAIL 输出，implement 的 harvest 必须看到实现产物、验收测试产物、red 基线证据三类分开的产物才收工；red→green 由独立方复核。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "justin-mc-lai：把验收从 Markdown 断言换成可执行红测试 —— acceptance_seed 必须是一条当前会失败的测试；plan 阶段只产出可编译骨架 + 红测试；implement 的 harvest 必须校验 red→green，且测试产物与实现产物分离（堵执行者自证）。"
goal: "acceptance_seed 是一条可执行且当前为红的测试；plan 阶段产出可编译骨架与失败测试；implement harvest 校验 red→green，并拒绝测试产物与实现产物不分离的回报。"
non_goals: "不改 internal/** 已有非测试生产代码；不在 plan 阶段 freeze；不 commit/push/发布/tag；不碰 release 与价格；本包不改 decompose 的 journey acceptance_seeds（列为后续包）；不重写既有 gate 语义。"
constraints: "Go 内核；main 起手工作区干净；红测试必须是断言失败而非编译失败；maker 与 checker 分离；失败不降级；停在 freeze 前，待冻条目交人类确认。"
scope_mode: focused
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 红测试由谁在 plan 阶段执行，evidence 放哪？ | proposer 本地执行一次 seed 命令，输出与 sha256 落 evidence/<ac>-red.txt，tests.md 固定 seed 文件 hash | answered | 按派单：plan 产出可编译骨架 + 失败测试；evidence 放红测试实际输出、sha256 清单、build 输出 | human |
| Q2 | harvest 怎样表达测试产物与实现产物分离？ | artifact kind 三分类：implement_artifact / acceptance_test / acceptance_seed_red，harvest 缺任一类即拒 | answered | 采纳推荐：artifact 三分类 implement_artifact / acceptance_test / acceptance_seed_red，harvest 缺任一类即拒，拒绝理由写明缺哪一类。 | human |
| Q3 | red→green 由谁复核？ | maker 交 red 与 green 两次原始输出，独立 QA 复跑 seed 命令，内核只校验证据存在与先后顺序 | answered | 采纳推荐：maker 只交 red 与 green 两次原始输出；独立 QA 复跑同一 seed 命令；内核只校验证据存在与先后顺序，不自证。 | human |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | "acceptance_seed 必须是一条当前会失败的测试"：propose 先拒收不可执行种子 | evidence/red-ac001.txt |
| AC-002 | "测试产物与实现产物分离（堵执行者自证）"：harvest 拒收只带测试产物的回报 | evidence/red-ac002.txt |
| AC-003 | "implement harvest 校验 red→green"：缺 red 基线证据的回报不可收割 | evidence/red-ac003.txt |

## 设计约定

1. seed 命令形态固定为 `go test ./<pkg> -run <TestName> -count=1`。propose 校验只做语法判定，不在 ValidateLakeProposal 内执行命令，避免测试内递归跑测试；真正的 FAIL 由 plan 出口执行并留证。
2. plan 出口必须落 evidence/<ac>-red.txt（原始 FAIL 输出，退出码非 0）与 seed 文件 sha256；tests.md 每行固定同一 hash。
3. harvest 在既有「artifact 指针 + stage_executed 证据」之上增加三类产物：implement_artifact、acceptance_test、acceptance_seed_red。三类缺一即拒，拒绝理由写明缺哪一类。
4. red→green 不可由执行者自证：maker 只提交原始输出，独立 QA 复跑同一 seed 命令并在实现前看到 FAIL、实现后看到 PASS。

## Acceptance Criteria

### AC-001 proposal 拒收不可执行的 acceptance_seed

Given 一个 lake proposal；When acceptance_seed 是自然语言而非一条 go test 命令；Then propose 返回非零错误，文案指出 acceptance_seed 必须是可执行测试命令，且 proposal 不被记录。
可判定观察点：`TestValidateLakeProposalRejectsProseAcceptanceSeed` 由 FAIL 转 PASS；可执行种子对照组仍被接受。既有 `TestValidateLakeProposal` 正例只允许把 fixture 改成可执行种子，不允许放宽断言。

### AC-002 harvest 拒绝只带测试产物的回报

Given implement 阶段的一个 done 回报；When artifact_refs 只有 acceptance_test 类测试产物、没有实现产物；Then HarvestLaneResult 返回错误、Accepted=false，理由指出实现产物缺失。
可判定观察点：`TestHarvestRejectsTestOnlyArtifacts` 由 FAIL 转 PASS。

### AC-003 harvest 拒绝无法证明 red 基线的回报

Given 一个带实现产物与验收测试产物的 done 回报；When 没有 acceptance_seed_red 基线证据（或证据晚于实现开始）；Then HarvestLaneResult 返回错误、Accepted=false，理由指出 red 基线缺失。
可判定观察点：`TestHarvestRequiresSeedRedEvidence` 由 FAIL 转 PASS。

## Impacted Surfaces

| 面 | 位置 | 本包动作 |
|----|------|----------|
| proposal 校验 | internal/orchestrator/lake_proposal.go `ValidateLakeProposal` | implement 轮加种子语法判定 |
| harvest 规则 | internal/orchestrator/lanes.go `validateHarvest` | implement 轮加三类产物与 red 基线判定 |
| 种子测试 | internal/orchestrator/acceptance_seed_test.go | plan 轮已落，断言不可改 |
| 既有 fixture（测试文件） | internal/orchestrator/lake_proposal_test.go、internal/cli/goal_p2d_test.go、internal/cli/goal_proposal_gate_test.go | implement 轮把自由文本 seed 改成可执行种子；只改测试数据，不放宽断言 |
| journey seeds | internal/core/decompose.go、internal/orchestrator/decompose.go | 本包不动，列为后续包 |

## 待冻条目

1. AC-001：propose 拒收非 `go test` 形态的 acceptance_seed，错误文案说明可执行命令要求。
2. AC-002：harvest 拒收只有 acceptance_test、没有实现产物的 done 回报。
3. AC-003：harvest 拒收缺 acceptance_seed_red 证据（或证据顺序不成立）的回报。
4. 设计约定 1：propose 只做语法判定，不在 ValidateLakeProposal 内执行命令。
5. 设计约定 3：三类 artifact kind 词表（implement_artifact / acceptance_test / acceptance_seed_red）作为冻结词表。
6. 设计约定 4：red→green 由独立 QA 复跑，内核只校验证据存在与顺序（对应 Q3）。
7. 范围：decompose 的 journey acceptance_seeds 不在本包，后续单独开湖。
