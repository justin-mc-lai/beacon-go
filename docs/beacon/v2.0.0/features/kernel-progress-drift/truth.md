---
slug: kernel-progress-drift
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
  truth: docs/beacon/v2.0.0/features/kernel-progress-drift/truth.md
  tests: docs/beacon/v2.0.0/features/kernel-progress-drift/tests.md
  tasks: docs/beacon/v2.0.0/features/kernel-progress-drift/tasks.md
  evidence: docs/beacon/v2.0.0/features/kernel-progress-drift/evidence.md
---

# Requirement Truth: kernel-progress-drift

## 人话

给 Beacon 内核维护者与验收方：模型按固定节奏自评「进展是否偏离」，现在只有零散判断，没有可审计的回执，也无法区分「真的处理了」和「评估失败或过期」。本包要求进度评估产出固定 schema 的回执，默认关闭、可先以 shadow 模式只记录不驱动决策；只有连续 N 条 `completed` 且 `drift=true` 的回执才提升为既有 replan 义务，`unknown/abstained/failed/stale` 永不算已处理；回执必须绑定 goal 的固定 revision，绑定到别处的不计数并上报 `rebind_hint`。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "justin-mc-lai：【内核硬化程序】P2 kernel-progress-drift：AC-D1 progress_review_receipt_v0 schema + shadow 模式，默认 off；AC-D2 连续 N 条 completed 且 drift=true 才提升为既有 replan 义务，unknown/abstained/failed/stale 永不算已处理；AC-D3 回执必须绑定 goal 固定 revision，绑定到别处不计数并上报 rebind_hint。"
goal: "进度评估产出 progress_review_receipt_v0 回执，默认 off、shadow 只记录；连续 N 条 completed 且 drift=true 才触发既有 replan 义务；非 completed 状态永不算已处理；回执绑定 goal 固定 revision，偏离的绑定不计数并上报 rebind_hint。"
non_goals: "不实现 P3 kernel-change-quality、P5 kernel-effect-journal；不改 kernel-truth-traceability、kernel-eval-gate、kernel-stride-control 三个已冻结包；不改版本号与对外发布说明；本包只写 requirement truth，不写实现代码。"
constraints: "Go 内核；工作区 beacon-go，canonical 分支 main；truth 写入 canonical-only；AC 原文照抄，不扩张；每条 AC 至少一条可执行 TC；TASK 的 evidence 指向包内真实文件且不自指；maker 与 checker 分离；失败不降级。"
scope_mode: focused
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 用一句话说清：谁、什么问题、什么算完成？ | 先写目标与非目标，再写一条可验证的完成标准 | answered | justin-mc-lai 本轮原话：内核维护者与验收方需要可审计的进度评估回执；完成标准是固定 schema 回执、默认 off 的 shadow 模式、连续 N 条 completed 且 drift=true 才触发 replan、非 completed 状态不计、回执绑定 goal 固定 revision 并对偏离上报 rebind_hint。 |
| Q2 | 回执与策略落在运行时哪一层，TC 用什么命令验证？ | 新建 internal/progressreview 承载 schema、模式与计数策略，TC 全部用 go test 选择器 | answered | 回执结构、状态词表、shadow/off/assist 模式、连续 drift 计数与 revision 绑定校验都放 internal/progressreview；所有 TC 用 go test 选择器，一条命令可直接跑。 |

## AC ID 映射

上游派单使用 AC-D1 至 AC-D3 的编号；本仓 parser 只识别三位数字的 AC 编号，因此冻结文本用 AC-001 至 AC-003 建链，判据文字照抄上游原文。

| 上游编号 | 本包 AC ID | 判据原文 |
|----------|------------|----------|
| AC-D1 | AC-001 | progress_review_receipt_v0 schema + shadow 模式，默认 off。 |
| AC-D2 | AC-002 | 连续 N 条 completed 且 drift=true 才提升为既有 replan 义务；unknown/abstained/failed/stale 永不算已处理。 |
| AC-D3 | AC-003 | 回执必须绑定 goal 固定 revision，绑定到别处不计数并上报 rebind_hint。 |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 用户原话 AC-D1：progress_review_receipt_v0 schema 与 shadow 模式，默认 off | docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/ac-d1-receipt-schema.md |
| AC-002 | 用户原话 AC-D2：连续 N 条 completed 且 drift=true 才提升 replan，非 completed 状态永不算已处理 | docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/ac-d2-drift-promotion.md |
| AC-003 | 用户原话 AC-D3：回执绑定 goal 固定 revision，绑定到别处不计数并上报 rebind_hint | docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/ac-d3-revision-binding.md |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | progress_review_receipt_v0 schema + shadow 模式，默认 off。 |
| AC-002 | 连续 N 条 completed 且 drift=true 才提升为既有 replan 义务；unknown/abstained/failed/stale 永不算已处理。 |
| AC-003 | 回执必须绑定 goal 固定 revision，绑定到别处不计数并上报 rebind_hint。 |

## Progress Review 回执契约

回执是进度评估的事实记录，不是完成声明，也不直接推进 run。字段固定如下，实现轮次按此命名：

| 字段 | 取值 | 说明 |
|------|------|------|
| `schema_version` | `progress_review_receipt_v0` | 回执 schema 版本，固定字符串 |
| `event_id` | 非空字符串 | 触发本次评估的事件标识 |
| `evidence_id` | 非空字符串 | 本次判断所依据的证据标识 |
| `status` | `completed` / `abstained` / `failed` / `not_evaluated` / `stale` | 本次评估状态；表外的 `unknown` 与未识别值按未处理处理，永不算已处理 |
| `run` | 结构体 | 本次评估关联的 run 快照事实 |
| `judgments` | 列表 | 本次评估的逐项判断 |
| `drift_signal` | 布尔 | 本次评估是否观察到进展偏离 |
| `question_version` | 非空字符串 | 本次评估所问问题的版本 |
| `contract_revision` | 固定 revision（SHA256 hex）或空 | goal 的固定 revision；空表示未固定 |

模式与计数规则：模式词表为 `off` / `shadow` / `assist`，默认 `off`；`shadow` 只记录回执、不驱动 replan 义务；连续 N 条 `completed` 且 `drift_signal=true` 才提升为既有 replan 义务，默认阈值 N=2；`abstained` / `failed` / `not_evaluated` / `stale` 以及未识别状态永不算已处理，并打断连续计数；`contract_revision` 未固定时 `assist_blocked_reason=contract_revision_unpinned`，绑定到其它 revision 或最新 revision 已偏离时该回执不计数并上报 `summary["rebind_hint"]="newer_receipts_under_unpinned_revision"`。

## 源锚点

上游参考仓 loopx，commit `c9c0644be428bbb0ab10726637f5b36602972912`。本包只引用文件与小节，行号与摘录留在包内证据文件。

| AC ID | 上游文件 | 小节 / 行号 |
|-------|----------|-------------|
| AC-001 | `loopx/capabilities/progress_review/receipt.py`；`loopx/control_plane/work_items/progress_review_policy.py` | `receipt.py` L27-L38（schema 与模式常量）；`progress_review_policy.py` L16-L23 |
| AC-002 | `loopx/control_plane/work_items/progress_review_policy.py`；`loopx/capabilities/progress_review/receipt.py` | `progress_review_policy.py` L19-L55（默认阈值与计数）；`receipt.py` L28-L34（状态词表） |
| AC-003 | `loopx/capabilities/progress_review/receipt.py`；`loopx/control_plane/work_items/progress_review_policy.py` | `receipt.py` L470-L490（revision 偏离与 rebind_hint）；`progress_review_policy.py` L57-L70（`normalize_progress_review_contract_revision`） |

## Non-Goals

- 不实现 P3 kernel-change-quality、P5 kernel-effect-journal。
- 不改 kernel-truth-traceability、kernel-eval-gate、kernel-stride-control 三个已冻结包。
- 不改版本号与对外发布说明；这两项由人确认。
- 本包只写 requirement truth，不写 cmd/internal 实现代码。

## Deferral Ledger

| Capability | Item | Reason | Status | Approved By |
|------------|------|--------|--------|-------------|
| none | none | none | dropped | n/a |
