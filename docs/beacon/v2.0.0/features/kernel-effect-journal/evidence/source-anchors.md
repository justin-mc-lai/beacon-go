# P5 源锚点：kernel-effect-journal

- 上游仓库：https://github.com/loopx-project/loopx（镜像/前身上游 https://github.com/huangruiteng/loopx）
- 基线 commit：c9c0644be428bbb0ab10726637f5b36602972912（2.3.0 发布窗口 HEAD）
- 取舍依据说明：上游历史快照 91d8ed06 尚未建立 TS 控制面与 Python 结算适配体系（loopx/control_plane/effect_runtime.py 与 settlement_driver.py 在该 commit 下不存在），统一以实际核验过的 2.3.0 发布 HEAD c9c0644 作为权威基线。
- 锚点清单来源：BeaconGo 研究 2026-11-09 落盘的 loopx-p1-p2-p3-p5-source-anchors.md，本文件只摘 P5 部分
- 主锚点文件：`loopx/control_plane/effect_runtime.py`、`loopx/control_plane/settlement_driver.py`、`loopx/control_plane/turn_driver/turn_journal.ts`、`loopx/control_plane/turn_driver/turn_journal_effects.ts`、`loopx/control_plane/turn_driver/recovery.py`

## AC-001（上游 AC-J1）

- 判据：implement 阶段每个副作用带 idempotency key 并写 effect journal。
- 文件与行号：`loopx/control_plane/turn_driver/turn_journal_effects.ts` L47-L53、L140-L165；`loopx/control_plane/settlement_driver.py` L1-L8、L49-L65；`loopx/control_plane/effect_program.ts` L683-L700、L702-L730
- 支撑点摘录：
  - `turn_journal_effects.ts` L47-L53 与 L140-L165：每个副作用操作要求唯一确定的 `journalEffectId`（即 idempotency key），在 `commitTurnJournal` 时原子写入本地 turn journal 文件。
    ```typescript
    export interface TurnJournalEffectRecord {
      journalEffectId: string;
      phase: TurnPhase;
      operationId: string;
      status: "pending" | "committed" | "failed";
      recordedAt: string;
    }
    ```
  - `settlement_driver.py` L49-L65：`effect_ids_match` 函数严格核验执行步骤中的 effect ID 与 plan/journal 中的标识符一致性。
  - `effect_program.ts` L683-L730：`settlementReceipt` 携带强类型 `effect_id`，由 plan 统一绑定分配。

## AC-002（上游 AC-J2）

- 判据：replay 不重复生效；resume 输出已证明阶段清单。
- 文件与行号：`loopx/control_plane/turn_driver/turn_journal_effects.ts` L39-L41、L80-L92、L185-L195；`loopx/control_plane/turn_driver/turn_journal.ts` L132-L146、L623-L630；`loopx/control_plane/turn_driver/recovery.py` L105-L125
- 支撑点摘录：
  - `turn_journal_effects.ts` L185-L195：重放检查若 `operationId` 已在 journal 中记录，直接返回 `replayed: true, appended: false`，跳过底层副作用执行，确保幂等不重复生效。
  - `turn_journal.ts` L132-L146 与 L623-L630：`legalPhaseAdvances` 校验阶段单调性，`completed_phases` 必须构成事务阶段的严格有序前缀。
  - `recovery.py` L105-L125：`build_turn_recovery_audit` 显式对外输出包含 `completed_phases` 的已证明阶段清单。
    ```python
    def build_turn_recovery_audit(journal_entries: list[dict[str, Any]]) -> dict[str, Any]:
        ...
        return {
            "status": "ready",
            "completed_phases": verified_phases,
            "replayed_effects_count": len(replayed_ids),
        }
    ```

## AC-003（上游 AC-J3）

- 判据：resume 缺 journal 时 fail-closed，不静默重跑。
- 文件与行号：`loopx/control_plane/turn_driver/recovery.py` L19-L25、L85-L103；`loopx/control_plane/turn_driver/turn_journal.ts` L355-L380；`loopx/control_plane/turn_driver/turn_journal_effects.ts` L165-L178
- 支撑点摘录：
  - `recovery.py` L19-L25 与 L85-L103：定义 `TurnRecoveryBlockedError`；在 `require_turn_recovery_continuation` 恢复流程中，若状态不明或 journal 缺失，严禁重试，直接抛出阻断异常。
  - `turn_journal.ts` L355-L380：journal 缺失或校验不一致时，决策返回 `{ action: "blocked", reason: "journal_inconsistent", reinvoke_host: false }`，绝对不静默重跑。
  - `turn_journal_effects.ts` L165-L178：既有 journal 冲突时抛出 `EffectRuntimeConflictError`，实现 fail-closed。
