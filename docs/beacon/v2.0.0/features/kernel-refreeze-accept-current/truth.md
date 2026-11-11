---
slug: kernel-refreeze-accept-current
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
  truth: docs/beacon/v2.0.0/features/kernel-refreeze-accept-current/truth.md
  tests: docs/beacon/v2.0.0/features/kernel-refreeze-accept-current/tests.md
  tasks: docs/beacon/v2.0.0/features/kernel-refreeze-accept-current/tasks.md
  evidence: docs/beacon/v2.0.0/features/kernel-refreeze-accept-current/evidence.md
---

# Requirement Truth: kernel-refreeze-accept-current

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "把 refreeze --accept-current 补成正式 feature（含红 seed）"
goal: "把已经实现但只有实跑证据的 refreeze --accept-current 固化成带红 seed 的正式需求：正文在变更打开前已移动时，显式 --accept-current 可收口并记录新的冻结指纹；无挂起变更时该标志拒绝且不写事件；严格路径不回退"
non_goals: "不放宽 revision drift 的默认拒绝；不新增 reason code；不改 refreeze 的严格路径；不 push 不发布"
constraints: "Go 内核；BEACON_BIN 驱动；三条 seed 独立隔离 store；human gate 与 release 语义不动"
scope_mode: focused
```

## 人话

`truth refreeze` 的严格路径会在正文于 change 打开前被移动时拒绝（`revision_authority_drift`），这是对的。但这类包会永久停在 `change_pending`：没有出口，只能靠改账本绕过。本需求把 `--accept-current` 变成一条有测试保护的正式出口：所有者显式确认"当前正文就是基线"后收口，并由内核记录新的冻结指纹。

## Acceptance Criteria

| AC ID | 说明 | 级别 |
|-------|------|------|
| AC-001 | 正文在 change 打开前已移动时，plain `truth refreeze` 拒绝并给出 `revision_authority_drift`，包停在待收口状态；带上 `--accept-current` 后同一命令成功，包回到 `frozen`、待收口列表清空，并追加一条 `truth_refrozen` 事件 | Must |
| AC-002 | 无挂起变更时 `truth refreeze --accept-current` 以非 0 退出、提示没有待收口的变更，状态不变，且不写任何 freeze 事件（该标志不能凭空制造一次冻结） | Must |
| AC-003 | 严格路径不回退：正文未移动且 base 有效时，plain `truth refreeze` 仍然成功收口，不需要该标志 | Must |


## Intent Coverage

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | utterance：把 --accept-current 变成有测试保护的正式出口 | seeds/seed1_accept_current_heals_drift.sh |
| AC-002 | goal：该标志不能凭空制造一次冻结 | seeds/seed2_accept_current_requires_pending.sh |
| AC-003 | non_goals：不放宽严格路径 | seeds/seed3_strict_refreeze_unchanged.sh |

## 设计约定

1. `--accept-current` 只影响三处 fail-closed 点：change base 校验、refreeze 事务守卫、AC diff 基线缺失；其余预检（完整性、四件套、human gate）一律照跑。
2. 收口必须走 `applyFreeze`，因此新的包指纹、truth sha 与 `truth_refrozen` 事件都会被重建，包不会停在"冻结但指纹过期"的状态。
3. 该标志不允许用于 release 或 human gate 原因，`goal recover` 的语义不变。
