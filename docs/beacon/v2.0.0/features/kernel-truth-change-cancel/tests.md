---
slug: kernel-truth-change-cancel
version: v2.0.0
status: draft
revision_id: R1-draft
materials_status: stale
---

# Tests: kernel-truth-change-cancel

所有命令都在 beacon-go 仓库根执行。先构建一次：

```bash
go build -o /tmp/beacon-seed ./cmd/beacon
```

Command 列是仓库根下的单条可执行命令。每条 seed 自己 `mktemp` 建隔离 store 与临时项目，不读写全局 `~/.beacon` 与仓库 `.beacon/state`；seed 通过 `BEACON_BIN` 接收构建产物。

## TC 矩阵

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-CAN-001 | behavior | `env BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-truth-change-cancel/seeds/seed1_cancel_identical_returns_frozen.sh` | 基线存在且正文未改时 cancel 退出 0；`status=frozen`、`change_pending=false`、`pending_change_ids=[]`；恰好 1 条 requested + 1 条 cancelled 事件；正文 sha256 不变 |
| AC-002 | TC-CAN-002 | negative | `env BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-truth-change-cancel/seeds/seed2_cancel_modified_refuses.sh` | 正文改动后 cancel 非零退出，stderr 含 `refreeze`；`status=change_pending`；cancelled=0、refrozen=0；正文与变更文档原样 |
| AC-003 | TC-CAN-003 | negative | `env BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-truth-change-cancel/seeds/seed1_cancel_identical_returns_frozen.sh` | 基线缺失且未带 `--accept-current` 时非零退出，stderr 含 `--accept-current`；状态仍 `change_pending` |
| AC-003 | TC-CAN-004 | behavior | `env BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-truth-change-cancel/seeds/seed1_cancel_identical_returns_frozen.sh` | 基线缺失且带 `--accept-current` 时退出 0，回到 `frozen`；事件 `accepted_current=true`、`baseline_missing=true` |
| AC-004 | TC-CAN-005 | guard | `env BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-truth-change-cancel/seeds/seed3_cancel_no_pending_idempotent.sh` | 无挂起变更时 cancel 连续两次退出 0、stderr 为空、状态 `frozen`、事件 0 条、正文不变 |
| AC-005 | TC-CAN-006 | guard | `env BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-truth-change-cancel/seeds/seed1_cancel_identical_returns_frozen.sh` | 两次关闭后 requested 与 cancelled 各 2 条，历史未被删除；两次关闭前后正文 sha256 均不变 |
| AC-005 | TC-CAN-007 | negative | `env BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-truth-change-cancel/seeds/seed2_cancel_modified_refuses.sh` | 拒绝路径不写 cancelled 事件、不写 refrozen 事件、不删除 `changes/<id>.md`、不改正文 |

## 当前实测（HEAD 199e875，2026-11-11，beacon-go 根）

| Seed | Exit | canon sha256 | 关键实测 | 归类 |
|------|------|--------------|----------|------|
| seed1_cancel_identical_returns_frozen | 1 | `731e27a739945b1060e7d2e5dc5a89e4826bec192dd60168703af689c6a0b48b` | case A 正常打开变更后，cancel 报 `unknown flag: --cancel`，状态仍 `change_pending`、cancelled=0 | 红，实现后须转绿 |
| seed2_cancel_modified_refuses | 1 | `a35b51227f220195efaef733afd6aa4a43bf62902ddee7d06f3016ca6a620e08` | 变更打开、正文已追加；cancel 报 `unknown flag: --cancel`，拒绝文案未指向 refreeze | 红，实现后须转绿 |
| seed3_cancel_no_pending_idempotent | 1 | `47654ea58d10f698c7d41e260817571506d527429db7183bc405eff29d06698e` | 无挂起变更时两次调用都报 `unknown flag: --cancel`，退出 1；状态未被破坏 | 红，实现后须转绿（幂等守护） |

S1 注：当前红态下 case A 的挂起变更关不掉，第二个 `truth change` 会被 `change_pending` 挡住，因此 seed1 的 case B（基线缺失分支）本轮未进入；其断言已写在 seed 里，等 case A 转绿后自动执行。这不是通过，AC-003 仍按红记录。

S2 归类：当前为红。拒绝路径实际上保住了 `change_pending`，但原因是标志不存在，文案没有指向 refreeze，属于错误原因的红，不算通过。

原始输出：`docs/beacon/v2.0.0/features/kernel-truth-change-cancel/evidence/seed1_cancel_identical_returns_frozen.out`、`docs/beacon/v2.0.0/features/kernel-truth-change-cancel/evidence/seed2_cancel_modified_refuses.out`、`docs/beacon/v2.0.0/features/kernel-truth-change-cancel/evidence/seed3_cancel_no_pending_idempotent.out`。

## 修复后行为契约

| 场景 | 退出码 | status | change_pending | cancelled 事件 | 正文 |
|------|--------|--------|----------------|----------------|------|
| 基线存在且正文一致 | 0 | frozen | false | 1 | 不变 |
| 基线存在但正文已改 | 非 0 | change_pending | true | 0 | 不变 |
| 基线缺失，未带 `--accept-current` | 非 0 | change_pending | true | 0 | 不变 |
| 基线缺失，带 `--accept-current` | 0 | frozen | false | 1（接受当前） | 不变 |
| 无挂起变更 | 0 | 不变 | false | 0 | 不变 |

## 负向断言

- 正文改动后 `--accept-current` 也不得放行；该标志只覆盖基线缺失。
- 拒绝路径不得追加 `truth_change_cancelled`，不得删除或重写 `changes/<id>.md`。
- 取消不得写 `truth_refrozen` / `truth_frozen` 事件，不得顺手跑 freeze precheck。
- 无挂起变更不得报错、不得追加事件；脚本重复调用必须稳定。
- 取消事件必须同时被 `pendingChangeIDs` 与 `trailRecords` 识别；只改一处视为未完成。

## Seed 文件 sha256

| 文件 | sha256 |
|------|--------|
| docs/beacon/v2.0.0/features/kernel-truth-change-cancel/seeds/lib.sh | `33a5919a80f0edee421854198b1c340bdceceedd05c2980aba53c13424799f97` |
| docs/beacon/v2.0.0/features/kernel-truth-change-cancel/seeds/seed1_cancel_identical_returns_frozen.sh | `1336b94b4ef19fd1bf9305718742cf8029f534b44ba48e90b522d6c104eaa46f` |
| docs/beacon/v2.0.0/features/kernel-truth-change-cancel/seeds/seed2_cancel_modified_refuses.sh | `76b53b05ce63a3e886f69b538fe835c7b99b298bcf7dc975e3b2cc43dc3edf5e` |
| docs/beacon/v2.0.0/features/kernel-truth-change-cancel/seeds/seed3_cancel_no_pending_idempotent.sh | `523338d7b8f73fc8a85c06cb78d54aebafb068d52444cef911b052d1883285b7` |
