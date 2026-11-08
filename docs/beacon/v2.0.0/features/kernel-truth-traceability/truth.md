---
slug: kernel-truth-traceability
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
traceability_required: true
canonical_refs:
  truth: docs/beacon/v2.0.0/features/kernel-truth-traceability/truth.md
  tests: docs/beacon/v2.0.0/features/kernel-truth-traceability/tests.md
  tasks: docs/beacon/v2.0.0/features/kernel-truth-traceability/tasks.md
  evidence: docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence.md
---

# Requirement Truth: kernel-truth-traceability

## 人话

给 Beacon 内核的使用者与验收方：`goal conformance` 现在只回答 truth 冻结、门已评估、验证人已挂、状态已导出四件事，回答不了“每条 AC 有没有测试、每条测试有没有落在包内的证据”；`truth change` 只记录一段摘要，回答不了“这次 change 覆盖了哪些 AC”。本包把这两条补成可判定的内核行为：声明追溯契约的包在 conformance 缺 TC 时以 `traceability_missing_tc` 失败并点名 AC，缺证据时以 `traceability_missing_evidence` 失败；change 关闭时把 base→after 的 AC 差异写进 change 文档与事件，任何验收方都能对照本次 change 覆盖了哪些 AC，且 AC-009 等既有冻结行为不回归。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "justin-mc-lai：【内核演进 · #1+#2】truth 可追溯 + 内核回归评测门。项目 beacon-go（main，最高优先级，质量优先于速度）。#1 kernel-truth-traceability：goal conformance 增加 AC→TC→evidence 覆盖率硬检查；truth change 的 spec 差异必须可对照（谁能证明本次 change 覆盖了哪些 AC）；失败 fail-closed，带明确 reason code（如 traceability_missing_tc、traceability_missing_evidence）；AC-009 等既有冻结行为不得回归。验收：缺 TC 的包 conformance 失败并指出缺哪条 AC；完整包通过。"
goal: "goal conformance 对声明追溯契约的包硬检查 AC→TC→evidence 覆盖，缺 TC 用 traceability_missing_tc、缺证据用 traceability_missing_evidence 非零失败并点名；truth change 关闭时留下 base→after 的 AC 差异，能直接回答本次 change 覆盖了哪些 AC。"
non_goals: "不改 kernel-eval-gate；不重写 delivery-paradigm 现有四项判定；不回溯要求历史冻结包补 Evidence 列；不做 UI；不改发布流程与版本号；本包只写 truth，不写 cmd/internal 代码。"
constraints: "Go 内核；工作区 beacon-go main ae95587；只写 truth 包，不写 cmd/internal、不 commit、不 push；maker≠checker；每个交付至少三份独立 QA verdict；AC-009 提案门不回归；失败一律 fail-closed。"
scope_mode: focused
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 用一句话说清：谁、什么问题、什么算完成？ | 先写目标与非目标，再写一条可验证的完成标准 | answered | justin-mc-lai 本轮原话：Beacon 内核维护者与用 beacon-go 交付的外部房间；conformance 与 truth change 证明不了 AC 被测试和证据覆盖；完成标准是声明追溯契约的包缺 TC、缺证据时 conformance 非零并点名 AC，change 关闭时留下可对照的 AC 差异，且 AC-009 等既有冻结行为不回归。 |
| Q2 | AC→TC→evidence 的 evidence 用什么口径判定，才不靠人工解释？ | 在 tests.md 覆盖矩阵增加 Evidence 列，写包内 evidence 目录下的项目相对路径 | answered | 覆盖矩阵每行增加 `Evidence` 列，值必须位于该 feature 的 `evidence/` 目录内且文件存在；缺列、空值、越出目录、文件不存在都算 `traceability_missing_evidence`。 |
| Q3 | 既有已冻结包没有 Evidence 列，怎样避免回溯回归？ | 只对 frontmatter 声明 `traceability_required: true` 的包硬检查 | answered | 声明 `traceability_required: true` 的包硬检查；未声明的历史冻结包输出 `not_required`，退出码仍由原有四项决定，四项名称与语义不变。 |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 用户原话：goal conformance 增加 AC→TC→evidence 覆盖率硬检查 | docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/traceability-covered.json |
| AC-002 | 用户原话：失败 fail-closed，带 traceability_missing_tc / traceability_missing_evidence；缺 TC 的包失败并指出缺哪条 AC | docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/traceability-missing-tc.json |
| AC-003 | 用户原话：AC-009 等既有冻结行为不得回归 | docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/traceability-not-required.json |
| AC-004 | 用户原话：truth change 的 spec 差异必须可对照（谁能证明本次 change 覆盖了哪些 AC） | docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/change-ac-diff.md |
| AC-005 | 用户原话：AC-009 等既有冻结行为不得回归；go test ./... 全绿 | docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/go-test-all.txt |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `goal conformance` 输出新增检查项 `ac_tc_evidence_coverage`。当 run 绑定的 feature 包在 truth.md frontmatter 声明 `traceability_required: true` 时，该检查按三条规则判定：truth 的每条 AC 在 tests.md 覆盖矩阵中有非软 TC 行；该 TC 行的 Command 属于执行层命令，既有 `docs_only_tc` 判定不命中；该 TC 行的 Evidence 列给出项目相对路径，路径位于该包 `evidence/` 目录内且文件存在。三条全部成立的包判定 pass，整体 conformance 通过时退出码为 0。 |
| AC-002 | 对声明 `traceability_required: true` 的包，任一 AC 没有有效 TC 行时 `ac_tc_evidence_coverage` 判定 fail，reason code 为 `traceability_missing_tc`，detail 逗号分隔列出全部缺 TC 的 AC ID；任一 TC 行的 Evidence 缺失、路径越出包 `evidence/` 目录、文件不存在时同一检查判定 fail，reason code 为 `traceability_missing_evidence`，detail 列出 `AC-ID/TC-ID`。tests.md 缺失、覆盖矩阵不可解析、truth 没有定义 AC 时判定 fail 并落到上述两个 reason code，不得输出 pass。失败时 `goal conformance` 退出码非 0，文本输出、JSON 的 items 与 failed 都携带 reason code 和缺失 ID。 |
| AC-003 | 未声明 `traceability_required: true` 的历史冻结包保持原有判定：conformance 新增项的 status 为 `not_required`，退出码只由 `truth_frozen`、`gates_evaluated`、`verifier_attached`、`state_exported` 四项决定，四项名称与 pass/fail 语义不变。 |
| AC-004 | `truth change` 打开的 change 文档记录 base 的各 AC 指纹；`truth refreeze` 关闭 change 前比较 base 与即将冻结的 truth，把 `## AC Diff` 写入 `changes/<change-id>.md`，分段列出 added、changed、removed 三组 AC ID，changed 行同时给出 before 与 after 的 SHA256 前 12 位；同一差异写入 refreeze 事件 payload 的 `change_ac_diff`。冻结后 `truth show --feature <slug> --json` 返回最近的 `change_ac_diff`，答案能直接指出本次 change 覆盖了哪些 AC。AC Diff 无法计算、base 指纹缺失、change 文档写入失败时 refreeze 非 0 退出并保持 change 处于打开状态。 |
| AC-005 | 既有冻结行为不回归：`TestProgramPlanRequiresProposal`、`TestProgramPlanFreezeChange`、`TestProgramSelectRecordsDecision` 三个 AC-009 提案门测试原样通过；delivery-paradigm 的四项 conformance 测试与端到端测试原样通过；`go test ./...` 全绿。 |

## Traceability Contract v1

本包把追溯口径写死，后续实现与 QA 不再另行解释：

| 维度 | 数据源 | 通过条件 | 失败 reason code |
|------|--------|----------|------------------|
| AC 集合 | truth.md `## Acceptance Criteria` 表定义的 AC ID | 集合非空 | `traceability_missing_tc` |
| TC 覆盖 | tests.md 覆盖矩阵（表头含 `AC ID`、`TC ID`、`Layer`、`Command`、`Assertion`、`Evidence`） | 每条 AC 有一行 Command 非空的非软 TC，且 Command 属于执行层命令，既有 `docs_only_tc` 判定不命中 | `traceability_missing_tc` |
| evidence 覆盖 | TC 行 `Evidence` 列的项目相对路径 | 路径归一化后位于该包 `evidence/` 目录内，且文件存在 | `traceability_missing_evidence` |

检查名固定为 `ac_tc_evidence_coverage`，与新 reason code 一起进入统一 reason 词表。失败时 detail 必须点名 AC ID；evidence 失败同时点名 TC ID。声明 `traceability_required: true` 的包在 tests.md 缺失、覆盖矩阵不可解析、truth 无 AC 定义时一律 fail-closed，不允许降低为 audit 后放行。

历史包兼容是显式的：未声明 `traceability_required: true` 的已冻结包输出 `not_required`，原有四项判定与退出码语义保持不变，避免把新契约回溯强加到已交付的包上。

## Change AC Diff Contract

`truth change` 打开 change 时，把冻结 base 的 `requirement_fingerprints` 写入 change 文档；`truth refreeze` 在关闭 change 前计算即将冻结 truth 的 AC 指纹，与 base 对照后写入 `## AC Diff` 与事件 payload。diff 的三组语义固定：added 为 after 有、base 无的 AC；removed 为 base 有、after 无的 AC；changed 为两侧都有、指纹不同的 AC。三组皆空说明本次 change 没有改动 AC 文本，仍由既有 AC-137 surface fingerprint 规则拒绝空 refreeze。

## Evidence Map

| 观察 | 证明方式 | 归属 |
|------|----------|------|
| 完整包通过覆盖率检查 | `goal conformance --run-id <id> --json` 中 `ac_tc_evidence_coverage` 为 pass，退出码 0 | AC-001 |
| 缺 TC 的包失败并点名 | 删除一条 TC 行后 conformance 非 0，reason code 为 `traceability_missing_tc`，detail 含被删 AC | AC-002 |
| 缺证据的包失败并点名 | Evidence 路径不存在时 conformance 非 0，reason code 为 `traceability_missing_evidence`，detail 含 AC/TC | AC-002 |
| 历史包不被回溯判死 | 未声明契约的包新增项为 `not_required`，原有四项决定退出码 | AC-003 |
| change 覆盖的 AC 可对照 | refreeze 后 change 文档 `## AC Diff` 与 `truth show --json` 的 `change_ac_diff` 一致 | AC-004 |
| 提案门不回归 | 三个 AC-009 测试原样通过，全量 go test 通过 | AC-005 |

## Non-Goals

- 不改 kernel-eval-gate 包；#2 等 #1 frozen 且实现落地后另行派发。
- 不重写 delivery-paradigm 的四项 conformance 判定。
- 不要求历史冻结包补 Evidence 列。
- 不改版本号与对外发布说明；这两项由人确认。
- 本包不写 cmd/internal 代码、不 commit、不 push。

## Deferral Ledger

| Capability | Item | Reason | Status | Approved By |
|------------|------|--------|--------|-------------|
| none | none | none | dropped | n/a |
