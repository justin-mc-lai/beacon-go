---
slug: kernel-effect-journal
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
  truth: docs/beacon/v2.0.0/features/kernel-effect-journal/truth.md
  tests: docs/beacon/v2.0.0/features/kernel-effect-journal/tests.md
  tasks: docs/beacon/v2.0.0/features/kernel-effect-journal/tasks.md
  evidence: docs/beacon/v2.0.0/features/kernel-effect-journal/evidence.md
---

# Requirement Truth: kernel-effect-journal

## 人话

给 Beacon 内核维护者与验收方：implement 阶段的副作用现在没有幂等身份，也没有落盘账本，进程中断后 resume 只能凭猜测重跑，可能重复生效或丢掉已完成的阶段。本包要求 implement 阶段每个副作用携带 idempotency key 并写入 effect journal；replay 依据幂等键去重，不重复生效，resume 必须输出已证明完成的阶段清单；resume 缺少 journal 时 fail-closed，不静默重跑。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "【内核硬化续 · P5】kernel-effect-journal：AC-J1 implement 阶段每个副作用带 idempotency key 并写 effect journal；AC-J2 replay 不重复生效，resume 输出已证明阶段清单；AC-J3 resume 缺 journal 时 fail-closed，不静默重跑。参考 loopx/control_plane/effect_runtime*、settlement_driver.py。"
goal: "implement 阶段每个副作用带 idempotency key 并写 effect journal；replay 按幂等键去重不重复生效；resume 输出已证明完成的阶段清单；缺 journal 时 fail-closed，不静默重跑。"
non_goals: "本包不含 P3 kernel-change-quality；不改 kernel-stride-control、kernel-progress-drift、kernel-eval-gate、kernel-truth-traceability 四个已冻结包；不改版本号与对外发布说明；本包只写 requirement truth，不写实现代码。"
constraints: "Go 内核；工作区 beacon-go，canonical 分支 main；truth 写入 canonical-only；AC 原文照抄，不扩张；每条 AC 至少一条可执行 TC；TASK 的 evidence 指向包内真实文件且不自指；maker 与 checker 分离；失败不降级；不削弱既有 gate 与 AC-009。"
scope_mode: focused
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 用一句话说清：谁、什么问题、什么算完成？ | 先写目标与非目标，再写一条可验证的完成标准 | answered | justin-mc-lai 派单原话：内核维护者与验收方需要可重放的 install 阶段副作用账本；完成标准是每个副作用带 idempotency key 并写 journal、replay 不重复生效、resume 输出已证明阶段清单、缺 journal fail-closed 且不静默重跑。 |
| Q2 | journal 与幂等逻辑落在哪一层，TC 用什么命令验证？ | 新建 internal/effectjournal 承载 key 生成、journal 落盘、replay 去重与 resume 证明清单，TC 全部用 go test 选择器 | answered | idempotency key 规则、journal 记录结构、replay 去重与 resume 阶段证明都放 internal/effectjournal；所有 TC 用 go test 选择器，一条命令可直接跑。 |
| Q3 | 「已证明阶段」和「replay 不重复生效」的判据是什么？ | 只有 journal 内有成功记录且校验通过的阶段才算已证明；replay 遇到同 key 的成功记录即跳过，不再调用副作用 | answered | 已证明 = 阶段对应副作用在 journal 中有成功记录且记录完整可校验；replay 以 idempotency key 为主键，命中成功记录直接返回既有结果，不再次执行；journal 缺失或不可解析即 fail-closed。 |

## AC ID 映射

上游派单使用 AC-J1 至 AC-J3 的编号；本仓 parser 只识别三位数字的 AC 编号，因此冻结文本用 AC-001 至 AC-003 建链，判据文字照抄上游原文。

| 上游编号 | 本包 AC ID | 判据原文 |
|----------|------------|----------|
| AC-J1 | AC-001 | implement 阶段每个副作用带 idempotency key 并写 effect journal。 |
| AC-J2 | AC-002 | replay 不重复生效；resume 输出已证明阶段清单。 |
| AC-J3 | AC-003 | resume 缺 journal 时 fail-closed，不静默重跑。 |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 用户原话 AC-J1：implement 阶段每个副作用带 idempotency key 并写 effect journal | docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/ac-j1-idempotency-key.md |
| AC-002 | 用户原话 AC-J2：replay 不重复生效；resume 输出已证明阶段清单 | docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/ac-j2-replay-resume.md |
| AC-003 | 用户原话 AC-J3：resume 缺 journal 时 fail-closed，不静默重跑 | docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/ac-j3-missing-journal.md |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | implement 阶段每个副作用带 idempotency key 并写 effect journal。 |
| AC-002 | replay 不重复生效；resume 输出已证明阶段清单。 |
| AC-003 | resume 缺 journal 时 fail-closed，不静默重跑。 |

## Effect Journal 契约

journal 是按 run 落盘的 append-only 记录，每个副作用一条，字段固定为：`schema_version`、`run_id`、`stage`、`idempotency_key`（非空，同一 run 内唯一）、`effect_kind`、`status`（`succeeded` / `failed` / `skipped_replay`）、`recorded_at`。缺少 idempotency key 的副作用不得执行。

replay 以 `(run_id, idempotency_key)` 为主键：命中 `succeeded` 记录时直接返回既有结果并记 `skipped_replay`，不再调用底层副作用；未命中才执行。resume 只把 journal 中 `succeeded` 且记录完整的阶段列入「已证明阶段清单」，其余阶段不得计入证明。

journal 文件缺失、不可解析或版本不识别时，resume 一律 fail-closed 并返回明确原因，不静默重跑任何阶段。

## 源锚点

上游参考仓 loopx，commit `c9c0644be428bbb0ab10726637f5b36602972912`（2.3.0 发布窗口 HEAD）。派单中的 `91d8ed06` 为历史快照，尚未建立 TS 控制面与 Python 结算适配体系（`effect_runtime.py`、`settlement_driver.py` 在该 commit 下不存在），取舍依据见 `evidence/source-anchors.md`。本包只引用文件与小节，行号与摘录留在包内证据文件。

| AC ID | 上游文件 | 小节 / 行号 |
|-------|----------|-------------|
| AC-001 | `loopx/control_plane/turn_driver/turn_journal_effects.ts`；`loopx/control_plane/settlement_driver.py`；`loopx/control_plane/effect_program.ts` | `turn_journal_effects.ts` L47-L53、L140-L165（`journalEffectId` 幂等键与原子写 journal）；`settlement_driver.py` L1-L8、L49-L65（`effect_ids_match`）；`effect_program.ts` L683-L730（`settlementReceipt` 携带强类型 `effect_id`） |
| AC-002 | `loopx/control_plane/turn_driver/turn_journal_effects.ts`；`loopx/control_plane/turn_driver/turn_journal.ts`；`loopx/control_plane/turn_driver/recovery.py` | `turn_journal_effects.ts` L185-L195（`replayed: true, appended: false` 跳过副作用）、L39-L41、L80-L92（`legalPhaseAdvances`）；`turn_journal.ts` L132-L146、L623-L630（`completed_phases` 严格有序前缀）；`recovery.py` L105-L125（`build_turn_recovery_audit` 输出 `completed_phases`） |
| AC-003 | `loopx/control_plane/turn_driver/recovery.py`；`loopx/control_plane/turn_driver/turn_journal.ts`；`loopx/control_plane/turn_driver/turn_journal_effects.ts` | `recovery.py` L19-L25、L85-L103（`TurnRecoveryBlockedError` 与 `require_turn_recovery_continuation`）；`turn_journal.ts` L355-L380（`action: "blocked", reason: "journal_inconsistent", reinvoke_host: false`）；`turn_journal_effects.ts` L165-L178（`EffectRuntimeConflictError`） |

## Non-Goals

- 本包不含 P3 kernel-change-quality。
- 不改 kernel-stride-control、kernel-progress-drift、kernel-eval-gate、kernel-truth-traceability 四个已冻结包。
- 不改版本号与对外发布说明；这两项由人确认。
- 本包只写 requirement truth，不写 cmd/internal 实现代码。

## Deferral Ledger

| Capability | Item | Reason | Status | Approved By |
|------------|------|--------|--------|-------------|
| none | none | none | dropped | n/a |
