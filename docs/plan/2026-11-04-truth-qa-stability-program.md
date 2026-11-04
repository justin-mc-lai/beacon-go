# Program: truth + QA 稳定交付（2026-11-04）

> 模式：`pln-program`（海拆湖）+ `pln-source`（对着源码，不猜）
> 状态：待冻结（本文件是计划，不是需求真相；冻结后以 `docs/beacon/v2.0.0/features/*/truth.md` 为准）
> 人话：先把“绿灯 = 真的检查过”这件事做硬，再让多块需求能自动接力，最后让验证能力可以被分发。

## 0. 一句话目标

让 beacon-go 的**需求真相**和**QA 验证**从“看起来有门”变成“门确实拦得住、证据确实可复跑”；不引入 agent 运行时。

## 1. 第一性原理拆解

用户真正要的不是“更多功能”，而是**信任**：看到绿灯就敢交付。

把“信任”拆到不可再分：

| 不可再分的事实 | 推论 | 对应湖 |
|---|---|---|
| 绿灯必须无法伪造 | 门必须是可执行判定，且对“本次要交付的包”默认 fail-closed | L1 |
| 多块需求不能靠人脑搬运状态 | 湖↔run↔stage↔证据必须有一条可恢复的链 | L3 |
| 验证不能每次重写 | 验证命令必须是绑 AC 的可复用资产 | L4 |
| 签字要签在“判断”层，不是“仪式”层 | 先影子预演，数据够了再谈自动 | L2 |
| 能力要能被别人装上 | 技能是分发面，不是附赠品 | L5 |

### 明确不做（非目标）

- **不做 agent 运行时**（issue #1：分层 sub-agent、沙箱、模型分流、成本看板）。
- **不做 release `auto` 真放行**（只做 `auto_shadow`）。
- **不新增 `loop` 顶层命令**（issue #7 与 charter decision 5）。
- **不做容器 / 云端沙箱**。
- **不改 AC-101 的历史冻结包审计语义**（只对“本次候选包”收紧）。

## 2. 湖图（执行顺序）

```text
L1 gate-strict-integrity ──► L2 release-policy-shadow
        │
        └──────────────────► L4 feature-map-verification ──► L5 skills-distribution

L3 program-drive（独立，可并行；不依赖 L1/L2/L4）
```

| 湖 | slug（= feature slug） | 来源 | 依赖 | 为什么这个顺序 |
|---|---|---|---|---|
| L1 | `gate-strict-integrity` | beacon-go#3 | — | 最小、最便宜，保护“门是真的”这个核心承诺 |
| L2 | `release-policy-shadow` | beacon-go#8 | L1 | 影子判定要读一个可信的 integrity 结论 |
| L3 | `program-drive` | beacon-go#7 | — | 机械劳动最大，且是多湖交付的地基 |
| L4 | `feature-map-verification` | 推文 1（Feature Map + CLI） | L1 | 验证命令要建立在可信包校验之上 |
| L5 | `skills-distribution` | 推文 2（skills 分发） | L4 | 技能要能带着“可复跑的验证”出门 |

> 依赖只表达“真实前置”。L3 与 L4 可并行；为降低单次交付风险，按 L1 → L3 → L4 → L2 → L5 推进也可接受。

## 3. 每个湖的验收草案（冻结时以 truth.md 为准）

### L1 `gate-strict-integrity`

**源码事实（已复现）**

- `truth freeze` 已经是 block 模式：去掉一行 `ac=` 后 freeze 返回 `feature_package_validation_failed`。
- `integrityReportLive(ctx, "")` 对已冻结包默认 `audit_only`（`internal/orchestrator/gates_integrity.go:71-75`）。
- `versionFeatureFacts` 也走这个默认（`internal/orchestrator/release_check.go:110`），所以 release 候选包的 integrity 结论是 audit，`IntegrityBlocked=false`。
- `gate evaluate release` 明确不把 integrity 作为前置（`internal/gate/evaluate.go:296`，AC-101）。

**AC 草案**

| AC | 可验证标准 |
|---|---|
| AC-1 | `beacon gate integrity --strict` 强制 block 模式（即使包已冻结）；违规时退出码非 0 且 reason code 稳定 |
| AC-2 | 版本 release 检查对“有 run 的候选包”使用 block 模式；无 run 的历史冻结包仍用 audit_only（AC-101 不回归） |
| AC-3 | 默认行为不变；`--strict` 与默认差异写进命令 help 与 gate 文档 |
| AC-4 | 测试覆盖四种组合：未冻结+违规、已冻结+无 run+违规、已冻结+有 run+违规、已冻结+有 run+干净 |
| AC-5 | 不改 `gate evaluate release` 的四门前置语义（charter 边界） |

### L2 `release-policy-shadow`

**AC 草案**

| AC | 可验证标准 |
|---|---|
| AC-1 | run 级 `release_policy` 字段，取值 `human`（默认）/ `auto_shadow`；未知值与 `auto` 拒绝并给稳定 reason code |
| AC-2 | `auto_shadow` 在 release 时产出“如果自动放行会通过 / 被哪条卡住”的判定，落库为事件/证据；`goal complete` 仍要求人工确认 |
| AC-3 | 影子判定在放行瞬间重算证据新鲜度，不复用缓存 verdict |
| AC-4 | `gate evaluate release` 输出能区分人工确认与策略判定来源（审计可查） |
| AC-5 | 不实现 `auto` 真放行；错误信息明确说“auto 尚未启用” |

### L3 `program-drive`

**AC 草案**

| AC | 可验证标准 |
|---|---|
| AC-1 | 新增 `beacon goal program drive --run-id <host>`；单次调用最多把一个就绪湖推到它的 release gate |
| AC-2 | 湖↔run 绑定幂等：已有 running run 复用，不重复创建；绑定写入 program 状态 |
| AC-3 | 到达 release gate 后记录该湖 `awaiting_release`；人工 `goal complete` 后，下一次 drive 自动进入下一湖 |
| AC-4 | 某湖 qa/implement 失败时 program 记 `blocked` + reason_code + 湖 slug；依赖它的湖保持 pending |
| AC-5 | `program route` 输出新增每湖 run_id、当前 stage、program 级 rollup |
| AC-6 | drive 可中断可恢复：再次调用从最后一个非终态湖继续，不重复执行已 complete 的湖 |
| AC-7 | 全部湖 complete → `delivered`；存在 blocked 湖时不得 `delivered` |
| AC-8 | 不新增顶层命令；`beacon loop` 文案不变 |

### L4 `feature-map-verification`

**源码事实**

- Go 的 `truth map` 只是**包布局地图**（`internal/orchestrator/truthmap.go:42`），不是运行期特征地图。
- Python v1 有 `beacon/utils/feature_map.py`：入口点 + 绑定 AC 的可执行验证命令，单向投影、不改 truth。

**AC 草案**

| AC | 可验证标准 |
|---|---|
| AC-1 | 新增 `feature-map.md` 生成：入口点（文件/路由/命令）与验证命令，每条绑定 AC |
| AC-2 | 地图是 truth/tests 的单向投影；生成器不改写 truth 正文 |
| AC-3 | 验证命令只能来自 tests.md 的 Command 列或显式声明的脚本，不凭空发明 |
| AC-4 | `beacon truth map verify` 增加 freshness：truth/tests 变更后地图标记 stale |
| AC-5 | 地图随技能分发，agent 可按“症状 → 入口 → 验证命令”定位，而不是重写脚本 |

### L5 `skills-distribution`

**AC 草案**

| AC | 可验证标准 |
|---|---|
| AC-1 | 仓库提供可一键安装的插件/技能包形态（`.claude-plugin/plugin.json` + `marketplace.json`） |
| AC-2 | 安装后 8 个宿主行为一致：`beacon-go` 路由 + 6 个子技能 + 命名空间 `beacon-go-*` |
| AC-3 | `beacon skills verify` 报告每个宿主的安装漂移（缺哪些、版本是否匹配） |
| AC-4 | 技能内包含 L4 的验证工作流，示例能跑出真实证据 |
| AC-5 | 不改变 Python v1 的 `beacon-*` 软链，不覆盖旧安装 |

## 4. 证据与来源

| 结论 | 证据 |
|---|---|
| freeze 已拦 task_binding | 本地最小复现：`beacon-go truth freeze` 返回 `feature_package_validation_failed` |
| 冻结后默认 audit_only | `internal/orchestrator/gates_integrity.go:71-75` |
| release 候选包读 audit 结果 | `internal/orchestrator/release_check.go:110-115` |
| release 不以 integrity 为前置 | `internal/gate/evaluate.go:296` |
| program 只有计划无驱动 | `internal/cli/goal_program.go:26`；`internal/orchestrator/program_route.go:184` 仅算 `NextBatch` |
| truth map 是包布局不是运行期地图 | `internal/orchestrator/truthmap.go:42`；`beacon truth map resolve` 输出 |
| skills 已嵌入、8 宿主、无插件清单 | `skills/skills.go:26`、`skills/hosts.go`、仓库无 `.claude-plugin` |
| token 成本当前不可用 | `docs/deferred-external.md` AC-120（子进程 `TokenUsage` 恒 0） |

## 5. 交付纪律

- 每个湖：先冻结 truth（`beacon truth freeze`），再实现，再跑 `go build ./... && go test ./... && go vet ./...`，涉及链路时跑 `bash e2e/run_e2e.sh`。
- 失败不许说完成；证据写进该湖的 `evidence.md`。
- commit / push / tag / 改冻结 truth / 改 release 语义：先问用户（AGENTS.md）。
- 发布仍走人工确认。

## 6. 输出契约

- `mode_id`: `pln-program`
- `lakes`: 5（见第 2 节）
- `out_of_scope`: agent runtime、release auto、容器沙箱、新 loop 命令、改 AC-101 历史语义
- `next`: `goal`
- `plain_summary`: 先把“绿灯必须真检查过”做硬，再让多块需求自动接力，最后把可复跑的验证做成可分发的技能；不做 agent。
