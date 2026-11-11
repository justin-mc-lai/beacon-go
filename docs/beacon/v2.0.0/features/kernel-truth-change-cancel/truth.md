---
slug: kernel-truth-change-cancel
version: v2.0.0
status: frozen
revision_id: R1-draft
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: filled
materials_status: current
intent_confirmed: true
canonical_refs:
  truth: docs/beacon/v2.0.0/features/kernel-truth-change-cancel/truth.md
  tests: docs/beacon/v2.0.0/features/kernel-truth-change-cancel/tests.md
  tasks: docs/beacon/v2.0.0/features/kernel-truth-change-cancel/tasks.md
  evidence: docs/beacon/v2.0.0/features/kernel-truth-change-cancel/evidence.md
---

# Requirement Truth: truth change cancel (R1-draft)

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "给 truth 变更流程补一条合法出口：正文没改就别逼我 refreeze"
goal: "beacon truth change --cancel 在挂起变更的正文仍等于冻结基线时直接关闭变更并把包恢复到 frozen；基线指纹缺失时允许 --accept-current 关闭；正文确有改动时拒绝并提示走 refreeze"
non_goals: "不改 refreeze 语义；不放宽 freeze precheck；不新增表；不 push 不发布"
constraints: "Go 内核；追加事件而不删历史；正文只读；退出码可观测"
scope_mode: focused
```

## 人话

现在变更流程只有单行道：`beacon truth change` 打开挂起变更后，只能靠 `beacon truth refreeze` 关闭。作者开了变更、看完又决定不改正文时，refreeze 会因为正文没动而被指纹链拒绝，于是包永久卡在 `change_pending`，任何读写后续都被挡住。本 feature 增加 `beacon truth change --cancel`：正文与冻结基线一致时直接关闭挂起变更、回到 `frozen`；挂起记录里没有基线指纹（历史遗留变更）时，必须显式 `--accept-current` 才允许以当前正文关闭；正文确实被改过时拒绝，并明确指向 `refreeze`。整个过程只追加事件，不改正文、不删变更文档、不改账本历史。

## 起点现场

- 单一真源为 beacon-go 仓库，本 feature 包位于 `docs/beacon/v2.0.0/features/kernel-truth-change-cancel/`；派单时 HEAD 为 `0efd235`，打包期间另一条线的 docs-only 提交把 HEAD 推进到 `199e875`，三条 seed 已在 `199e875` 重跑。
- 派单时工作区另有其他 feature 的未提交改动；打包期间这些改动已被另一条线提交，本包始终未接触、未提交它们。
- `truth change` 的实现位于 `internal/cli/truth.go`（命令）与 `internal/orchestrator/truth_change.go`（引擎）；`--summary` 当前被 `MarkFlagRequired` 强制。
- 挂起状态由两处扫描得出：`internal/orchestrator/truth.go` 的 `pendingChangeIDs` 与 `internal/orchestrator/branch.go` 的 `trailRecords`；两处都只在 `truth_frozen` / `truth_refrozen` 时清空挂起列表。
- 本阶段未建 goal run、未 tick、未 freeze、未改生产代码。

## 当前行为（隔离 store 实测，0efd235 与 199e875 结果一致）

| ID | 实测事实 |
|----|----------|
| F1 | `truth change --cancel --feature demo` 退出 1，stderr 为 `unknown flag: --cancel`，包仍是 `change_pending`，无 `truth_change_cancelled` 事件。 |
| F2 | `truth change` 成功后写 `changes/chg_<id>.md`，文档 frontmatter 记录 `base_truth_sha256`，trail 事件 `truth_change_requested` 记录同一 hash 与 `package_fingerprint`；正文一个字节未动。 |
| F3 | `truth show --json` 在挂起期间返回 `status=change_pending`、`change_pending=true`、`pending_change_ids=[chg_...]`；取消成功后必须回到 `status=frozen`、`change_pending=false`、`pending_change_ids=[]`。 |
| F4 | 挂起记录缺基线时，`truth refreeze` 走 `truth_change_ac_diff_base_missing` 失败关闭；说明“无基线”是已存在的一等状态，值得单独给出退出路径。 |
| F5 | `truth refreeze` 在正文未动时会因指纹链校验失败，这正是本 feature 要替代的人工死角。 |
| F6 | 两个挂起扫描器都只认 `truth_frozen` / `truth_refrozen` 作为清空事件；取消事件必须同时被两处识别，否则 `truth show` 与门禁会读数不一致。 |
| F7 | `ReasonTruthNoChangePending`（`truth_no_change_pending`）已存在，被 refreeze 用于“没有挂起变更”的错误；cancel 的幂等分支不能复用它报错，要退出 0。 |

## Scope

### In scope

- `truth change` 新增 `--cancel` 布尔标志；带该标志时不再要求 `--summary`。
- 新增 `--accept-current` 布尔标志，仅在基线指纹缺失时允许关闭，并在事件里记录 `accepted_current=true`。
- 引擎新增取消方法：解析挂起变更、比对当前正文 sha256 与挂起记录里的 `base_truth_sha256`、按分支决定关闭或拒绝。
- 关闭时追加 `truth_change_cancelled` 事件，并让两个挂起扫描器把它当作清空事件。
- 三个分支的退出码、stderr 文案、状态变化与事件数量都可观测。

### Out of scope

- 修改 refreeze、freeze precheck、指纹链语义或 reason code 集合。
- 删除 `changes/<id>.md`、改写历史事件、重写正文或自动回滚作者的编辑。
- 新增数据库表或列；取消状态全部落在既有 `lifecycle_events`。
- 通过 `--accept-current` 绕过“正文确有改动”的拒绝。

## Execution Contract

### CLI 面

```text
beacon truth change --cancel [--accept-current] --feature <slug> [--project <root>] [--by <actor>] [--json]
```

- `--cancel` 与创建变更互斥；带 `--cancel` 时 `--summary` 可省略，带了也忽略。
- `--accept-current` 只与 `--cancel` 一起使用；单独出现时报用法错误并非零退出。
- 非 _cancel 路径保持现状：`--summary` 必填，行为与事件不变。

### 基线指纹解析

1. 取当前唯一挂起变更的 `base_truth_sha256`：先读 `truth_change_requested` 事件 payload，为空再读变更文档 frontmatter。
2. 两处都为空即“基线指纹缺失”。缺失时不得回退到 freeze/refreeze 的 `truth_sha256_after` 或 baseline 记录静默补齐，必须走 `--accept-current`。
3. 计算当前 `truth.md` 的 sha256（原始字节，与 `truth baseline` 同算法）。

### 三支语义

| 分支 | 条件 | 结果 | 退出码 |
|------|------|------|--------|
| A 一致关闭 | 基线指纹存在且等于当前正文 sha256 | 追加取消事件，挂起清空，状态回 `frozen` | 0 |
| B 正文已改 | 基线指纹存在但不等于当前正文 sha256 | 拒绝；stderr 提示改用 `truth refreeze`；状态保持 `change_pending`；不写事件 | 非 0 |
| C 基线缺失 | 基线指纹为空，未带 `--accept-current` | 拒绝；stderr 提示 `--accept-current` 或 `refreeze`；状态保持 `change_pending`；不写事件 | 非 0 |
| C 基线缺失且接受当前 | 基线指纹为空且带 `--accept-current` | 追加取消事件（`accepted_current=true`、`baseline_missing=true`），关闭回 `frozen` | 0 |
| D 无挂起变更 | 没有待关闭变更 | 成功空操作：不改状态、不写事件、stderr 为空 | 0 |

### 取消事件契约

关闭时追加一条 `feature` 级 `lifecycle_events` 记录，`event_type=truth_change_cancelled`，payload 至少包含：

```json
{"change_id": "chg_...", "by": "cli", "decision": "cancel", "accepted_current": false, "baseline_missing": false, "base_truth_sha256": "<记录值或空>", "truth_sha256": "<当前正文 sha256>", "change_doc": "docs/.../changes/chg_....md"}
```

`decision` 取 `cancel` 或 `accept_current`；`accepted_current` 与 `baseline_missing` 用布尔值，便于 seed 与后续审计直接读取。事件只追加，不修改既有 `truth_change_requested` 与 `truth_frozen` 记录。

### 状态与可观测输出

- 成功关闭后，`truth show` / `truth show --json` 立即返回 `status=frozen`、`change_pending=false`、`pending_change_ids` 为空；frontmatter 的 `status: frozen` 保持不变。
- 拒绝时 CLI 保留既有视图输出习惯，错误走 stderr 且进程退出非 0。
- 幂等分支不打印 warning、不写事件，便于脚本重复调用。

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 挂起变更的正文与冻结基线一致时，`truth change --cancel` 退出 0，状态回到 `frozen`，`change_pending=false`，挂起 id 列表清空，且恰好追加一条 `truth_change_cancelled` 事件（`decision=cancel`、`accepted_current=false`、`baseline_missing=false`）。 |
| AC-002 | 挂起变更的正文确有改动时，`truth change --cancel` 退出非 0，stderr 明确指向 `truth refreeze`，状态保持 `change_pending`，不追加取消事件，也不写 refreeze 事件，正文与变更文档都保持原样。 |
| AC-003 | 挂起记录缺基线指纹时：不带 `--accept-current` 退出非 0 且提示该标志，状态保持 `change_pending`；带 `--accept-current` 退出 0 并关闭，事件记录 `accepted_current=true` 与 `baseline_missing=true`。 |
| AC-004 | 没有挂起变更时，`truth change --cancel` 退出 0、stderr 为空、状态不变、不追加任何事件；连续调用两次结果一致。 |
| AC-005 | 取消全程只追加事件：不修改 `truth.md` 字节、不删除，也不改写 `changes/<id>.md`、不修改既有事件、不落 refreeze 事件；两个挂起扫描器一致识别取消事件；写入遵循 canonical-branch 约束。 |

## Intent Coverage

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 派单 restatement：正文回到基线时 cancel 成功并显示 frozen | docs/beacon/v2.0.0/features/kernel-truth-change-cancel/evidence/seed1.canon |
| AC-002 | 派单 restatement：正文确有改动时拒绝并保持 change_pending | docs/beacon/v2.0.0/features/kernel-truth-change-cancel/evidence/seed2.canon |
| AC-003 | 派单 restatement：基线指纹缺失时允许 --accept-current 语义关闭 | docs/beacon/v2.0.0/features/kernel-truth-change-cancel/evidence/seed1.canon |
| AC-004 | 派单 dod：无挂起变更时 cancel 幂等返回非错 | docs/beacon/v2.0.0/features/kernel-truth-change-cancel/evidence/seed3.canon |
| AC-005 | 派单 restatement：不得靠改正文或篡改账本绕过，走 change 事件追加、不删历史 | docs/beacon/v2.0.0/features/kernel-truth-change-cancel/evidence/seed2.canon |

## Non-negotiable constraints（不得放宽）

| # | Constraint | 现有兜底校验点 |
|---|---|---|
| N1 | 已冻结的 truth/tests/tasks/evidence/seeds 只读；实现阶段只改 `cmd/**` 与 `internal/**` | 冻结基线 + refreeze 指纹链 |
| N2 | 正文改动一律拒绝，`--accept-current` 不得覆盖该拒绝 | `internal/orchestrator/truth_change.go` |
| N3 | 取消只追加 `truth_change_cancelled`，不改写、不删除任何历史事件 | `internal/store` 事件表为 append-only 使用方式 |
| N4 | 两个挂起扫描器（`pendingChangeIDs`、`trailRecords`）必须同时识别取消事件，避免视图与门禁分叉 | `internal/orchestrator/truth.go`、`internal/orchestrator/branch.go` |
| N5 | 不做 freeze、不 tick、不写业务代码以外的东西；QA 由独立验收方出具 | 本包 Stop Line |

## Stop Line

本包只到 plan 打包：四件套与三条 seed 齐备，三条 seed 当前全红（`unknown flag: --cancel`）。不 freeze、不 tick、不写业务代码、不写任何 QA verdict。
