# P2c 实施报告：外包与并行（G-04 / G-10 / G-11 / G-18 / G-22）

> 日期：2026-10-21 ｜ 状态：完成，待拍板后进 P2d
> 口径：[scope-charter.md](scope-charter.md)（已拍板）第 2 节 G-04/G-10/G-11/G-18/G-22、第 3.4 节 design slim 边界
> 一句话：写代码这步现在有统一的进门检查（admission）和统一的收活规矩：三条干法（单人 / 团队 / ralph 循环）进门同一把尺、出门同一本账；整包外包给 ralph 的打包和收回都落了面；多湖并行只出计划不真建工位；带界面的活没绑好设计规矩不许开工。本片所有「执行」仍走 StageExecutor 接口 + RecordingExecutor 占位，**没有冒充真跑了外部执行器**，真集成在 P3。

---

## 1. 逐 G 落地表

| G | 落地 | 文件 / 函数 | CLI |
|---|---|---|---|
| G-22 三 lane 统一口径 | LaneRouter 选路（纯函数 RouteLane）；lane 执行器注册表（RegisterLaneExecutor / LaneExecutor）；ExecuteLaneStage（admission 先行、按 lane 取执行器、未注册 blocked）；收活共用 validateHarvest（产物指针 + implement 证据，两缺一不算收活）；team 交接校验 ValidateTeamHandoff（纯函数，from/to/产物引用缺一拒绝） | `orchestrator/lanes.go`、`meta_p2c.go` | `goal implement plan`（选路 + admission，只读） |
| G-10 delegate-ralph | DelegateRalph 打包：需求包路径（PackageRoot/TruthPath/TestsPath）+ stage plan + admission 结果 + 验收口径（AcceptanceRuleText）+ 回滚要求（RollbackRuleText），写 `implement_delegated` 事件 + delegation 记录（delegated → returned/failed）；ReceiveRalphResult 按共用收活规矩校验（证据必须是外包**之后**产生的），不合格拒绝记 `implement_result_rejected`；收回不推进、不完成 run（v1 语义「not product complete」） | `orchestrator/delegate.go` | `goal delegate-ralph [--simulate]`、`goal receive-result` |
| G-11 parallel lakes | BuildParallelPlan（纯函数）：单湖 single_worktree；多湖 main_star_satellite_pool（一湖一 satellite、merge 只许 satellite → main_star、satellite 禁止在主线写 truth——约束逐字写进每个 satellite）；触碰面冲突检出（同面 ≥2 湖 → conflict + requires_decision，不静默并行）；ApplyParallelPlan 落 run metadata + 事件 + 绑定工位 ref。只计划不建工位，`planned_only=true` 处处标注 | `orchestrator/parallel.go` | `goal parallel-plan --lake … --touch lake=面 … [--parallel]` |
| G-18 design-stage | DesignStageStatus（要不要设计 / 绑没绑 / 桥接说明）；BindDesignPrimitive（记录 primitive 集名 + 版本 + 来源引用，写事件，不生成任何原型文件）；未绑 → admission 的 design 项拒绝（与 G-04 联动）；prototype 生成只标 bridge 点（DesignBridgeNote），本片不实现生成器 | `orchestrator/design.go` | `goal design-stage status|bind` |
| G-04 admission 本体 | CheckImplementAdmission：五项逐项查——① truth 已 frozen（复用 P2a 的 store 推导，未冻给 `freeze_ack_required`，v1 goal 面原码）② 工位绑定与 parallel plan 一致（无 plan 时绑定即项目根 `.`）③ design 该绑已绑（仅 UX run 适用）④ 无未清 blocker streak ⑤ 无未确认 steering（P2b 联动）。Tick 执行 implement 前、DelegateRalph 打包、`goal admit` 三路共用同一 checkAdmission，无第二套判定；schema 沿 v1 `goal-implement-admission.v1`，通过码 `goal_implement_admission_ok` | `orchestrator/admission.go`、`tick.go`（接线） | `goal admit`（逐项输出，不过 exit 非 0） |

状态存放：全部进 store——run 快照（runs.metadata_json 新增 lakes/task_scale/implement/parallel_plan/design_binding 字段）、lifecycle_events（新事件：implement_lane_routed / implement_lane_executed / implement_lane_blocked / lane_result_harvested / lane_result_rejected / lane_result_failed / implement_delegated / implement_delegation_blocked / implement_delegation_simulated / implement_delegation_returned / implement_delegation_failed / implement_result_rejected / parallel_plan_recorded / design_primitive_bound / implement_admission_blocked）、artifact_refs（收活产物指针）。没有自带任何 JSON 状态文件（口径第 7 条）。冻结 schema 九表零改动。

Tick 接线（G-04 与既有引擎的接合点）：Tick 执行 implement 阶段前先跑 admission，不过则记 blocker 停住（事件 `implement_admission_blocked`）；执行方解析改为——run 上已选路的 lane 只走 lane 注册表（未注册按 P2a 口径 `stage_executor_not_registered` blocked），未选路仍走 stage 注册表。P2a/P2b 既有行为因此零变化（它们的 run 从未选路）。

## 2. 语义差异 / 推导清单

1. **lane 选路规则的来源（DERIVED 为主）**：v1 只有显式 `--mode`（`_normalize_mode` 只认 single/team/ralph）+ 按 mode 定 runner 默认（single → native-single），**没有按规模自动选路的规则**。本片按任务兜底口径实现四级优先：显式参数（源仓语义）> 规模字段（`scale=large` → ralph，推导：大活适合一轮轮自检的循环）> 湖数（>1 → team，推导自 parallel plan 的多工位交接语义）> 默认 single（对齐 v1 默认 runner）。推导两级在代码注释、plan 输出的 reason 字段里都标了 DERIVED。
2. **admission 五项的合成口径**：v1 的 `ensure_implement_admission` 本体查的是 feature 必填、freeze、workspace admit（服务工位图、分支合并等重型检查）和写授权。本片五项是按任务书提炼的最小真实集：freeze 项逐字对齐 v1 goal 面（未冻 = `freeze_ack_required`）；工位项是 v1 workspace admit 的 plan 级替身（真工位校验/创建是 P3）；design、blocker、steering 三项是 goal 侧前置检查串（写代码命令里散落的检查 + G-18 口径 + P2b 护栏）的归一。v1 没有的设计/steering 两项 reason code 为新增 core 常量（`design_primitive_not_bound`、`steering_unacknowledged`），已登记在 `core/reasons.go` P2c 家族。
3. **blocker streak 项与 repair loop 的分界（实现中发现并定的口径）**：P2a 的 repair loop 靠「失败一次 streak +1、Tick 重试」运转，若 admission 的 streak 项无条件拦 streak ≥ 3，第 4 次重试会被 admission 抢先拦下、永远到不了 MAX_REPAIR_LOOPS 的 error_recovery 路由（实测撞出，已修）。定口径：streak 项只拦「不在 repair loop 中的硬卡死」（`BlockedStreak ≥ 3 且 RepairLoopCount == 0`，如人工 Block 三次）；repair 中的重试由上限机制管。五项逐项失败测试据此用人工 block 构造。
4. **design 绑定的存储论证**：候选三处——feature_packages 加列（动冻结 schema，否）、gate_results 写一行 design pass（gate 结果是「评估结论」，写回属 P3 边界，且 P2a 出口门已把 lifecycle Prototype/Completed 与 design gate pass 都算设计证据，再写一行会造第二事实源，否）、run metadata（采纳：绑定是**这个 run** 的属性——同一 feature 的两个 run 可以绑不同 primitive 版本；run 快照本就是 goal 状态的唯一存放处）。代价：绑定不跨 run 共享，每个 UX run 各绑一次，与 v1「每 run 跑一次 design stage」的粒度一致。
5. **delegate 比 v1 严两处（fail-closed 收紧）**：v1 `delegate_implement_to_ralph` 不查阶段、不查准入，只做标注。本片要求 run 必须正处在 implement 阶段（`delegation_stage_not_implement`）、admission 必须通过（不过记 `implement_delegation_blocked` 事件并以 admission 的 reason 拒绝）、且同时只许一份 delegation 在外（`delegation_already_active`）。理由：打包的包里就含 admission 结果，准入不过的包发出去自相矛盾。
6. **receive-result 是新名**：源仓 goal.py 没有收回命令（v1 靠宿主循环把结果落回 run payload）。本片需要一个显式收活面，命名 `goal receive-result`，在命令 Long 与本报告中都已注明与源仓不符之处。收回的证据锚点是该 delegation 的 `implement_delegated` 事件在轨迹中的位置——证据必须产生于外包之后，外包前干的活不能充当回包证据。
7. **detect_parallel_intent 最小版**：v1 用话术词表（parallel/并行/satellite/主星…）猜意图；本片按任务书不做文本猜测，只认显式 `--parallel` 与湖数 > 1（恰是 v1 `build_parallel_plan` 里非文本的两条分支）。satellite 工位路径、main_star 记录形状逐字对齐 v1（含池模式零湖时的 `lake-placeholder`）。
8. **PlanImplement 只读的实现选择**：任务书要求 plan 只读——故选路决定不落 run（与 v1 stage plan 会回写 payload 不同）。选路真正落账的时机是 DelegateRalph 打包（lane=ralph）与后续 P3 的 lane 执行命令；Tick 只认已落账的 lane。plan 面的 executors 字段如实报告每条 lane 有没有注册执行器。
9. **team 交接的字段集**：v1 handoff（team_surface）字段很多（summary_ref / technical_archive_ref / changed_files / implementation_plan_ref / design_truth…），其中必填的只有身份与产物。本片按任务书取最小必填集 from/to/artifact_refs（summary_ref 选填），校验为纯函数；其余字段留 P3 真集成时随 handoff 文件一起落。

## 3. 实跑样例（真二进制，`BEACON_STORE_PATH=/tmp/p2c-smoke/beacon.db`，feature 以 sqlite 播种——feature 包创建 UI 属 truth 面，P4 才有，与 P2a/P2b 冒烟同法；truth_freeze gate 预置 pass）

```
$ beacon goal run "给登录页加二次验证" --project /tmp/p2c-smoke/proj --feature smoke-feature --executor-id host-1 --json
{ "run_id": "run_c31f1d177fbaec3fa9b785e1", "current_stage": "plan", "loop_stage": "governance_precheck", ... }
$ beacon goal tick --run-id <R> --executor recording --json   ×2 → current_stage: truth → implement

$ beacon goal admit --run-id <R>
admission: pass (goal_implement_admission_ok) for run run_c31f1d177fbaec3fa9b785e1
  truth_frozen     ok=true truth frozen
  worktree_binding ok=true bound to .
  design_bound     ok=true run requires no design (no --ux)
  blocker_streak   ok=true no uncleared hard block
  steering_ack     ok=true no pending steering

$ beacon goal parallel-plan --run-id <R> --lake smoke-feature --lake billing \
    --touch smoke-feature=api.go --touch billing=api.go
mode: main_star_satellite_pool (planned_only=true, intent lake_count)
  satellite smoke-feature -> .beacon/worktrees/satellite/smoke-feature (merge to main_star, truth on main: false)
  satellite billing -> .beacon/worktrees/satellite/billing (merge to main_star, truth on main: false)
  conflict: api.go touched by smoke-feature, billing — serial or human decision required

$ beacon goal admit --run-id <R>          # 并行计划落账后工位项随计划走
  worktree_binding ok=true bound to .beacon/worktrees/satellite/smoke-feature

$ beacon goal delegate-ralph --run-id <R> --simulate --json   （节选）
delegation: id=dlg-4daf90006852 status=delegated simulated=true
  package.truth_path = /tmp/p2c-smoke/proj/docs/features/smoke-feature/truth.md
  package.worktree_ref = .beacon/worktrees/satellite/smoke-feature
  package.admission.reason_code = goal_implement_admission_ok （五项全过，包内含逐项结果）
  package.acceptance[0] = artifact pointers must land in artifact_refs (absolute paths)
  result_summary = "simulated: recording executor placeholder (no real stage work performed)"

$ beacon goal receive-result --run-id <R> --delegation-id dlg-4daf90006852 --json ; echo exit=$?
exit=1 : Error: goal.ReceiveRalphResult: delegation return harvest incomplete: result_artifacts_missing (result_artifacts_missing)
         （delegation 仍为 delegated，未被收掉）

$ beacon goal receive-result --run-id <R> --delegation-id dlg-4daf90006852 \
    --artifact /tmp/p2c-smoke/impl.go --summary "login 2fa done" --json
delegation: status=returned simulated=true

$ beacon goal run "做个设置页面" --project … --feature smoke-feature --ux --json   → run_c222110ad6ee63013a2bf79c
$ beacon goal design-stage status --run-id <U>
required: true bound: false stage: pending
bridge: prototype generation is delegated to the host agent via the design bridge (P3); P2c records primitive bindings only and generates no prototype files
$ beacon goal admit --run-id <U> ; echo exit=$?
admission: blocked (design_primitive_not_bound) … design_bound ok=false UX run has no design primitive binding; exit=1
$ beacon goal design-stage bind --run-id <U> --primitive-set restrained-product --version v1 --source-ref DESIGN.md
bound: true primitives: restrained-product (v1)
$ beacon goal admit --run-id <U> ; echo exit=$?
admission: pass (goal_implement_admission_ok) … design_bound ok=true primitives bound: restrained-product; exit=0
```

## 4. 验证与交付物

- `go build ./...` / `go vet ./...` / `gofmt -l .`（无输出）全过；`go test ./... -count=1` **90 个测试全过**（core 13、store 18、orchestrator 50、cli 9），P0–P2b 的 68 个零回归。其中一处真回归在开发中撞出并修复：admission streak 项与 P2a repair loop 的分界（见 §2.3），`TestRepairLoopCeilingRoutesToErrorRecovery` 复绿。
- 新增测试覆盖任务清单逐项：选路三路 + 未知 lane 拒绝（TestRouteLaneThreeWays）、team 交接缺项拒绝（纯函数 + 收活两层）、三 lane 收活缺产物被拒且全件收齐放行（TestLaneHarvestSharedRule，三 lane 同一用例）、lane 执行器未注册 blocked（ExecuteLaneStage + Tick 选路后两路）、delegate 打包完整 + simulated 标注（TestDelegateRalphSimulatedFullFlow）、无外包后证据收回被拒（TestDelegateReceiveWithoutEvidenceRejected）、parallel 单湖/多湖两形态 + 冲突检出 + satellite 写 truth 约束显式存在（BuildParallelPlan 两测）、design 未绑 admission 拒绝且绑定后放行（TestDesignBindingAdmission）、admission 五项逐项失败各有 reason code（TestAdmissionItemsEachFail 四子测 + design 项在 G-18 测试里）、Tick 与 delegate 与 admit 共用同一判定（TestAdmissionSharedAcrossTickDelegateAdmit：同一 run 同一状态三路同码 `steering_unacknowledged`）、CLI 全链路（TestGoalP2CChain）。
- 文件大小（字节）：bin/beacon 14,809,037（P2b 为 14,679,584）；新增 `internal/orchestrator/meta_p2c.go` 12,001、`admission.go` 6,544、`lanes.go` 12,835、`delegate.go` 10,492、`parallel.go` 5,785、`design.go` 3,859、`p2c_test.go` 17,573；`internal/cli/goal_p2c.go` 12,472、`goal_p2c_test.go` 5,021；改动 `internal/core/reasons.go` 6,476（新增 P2c 四个 reason 家族）、`internal/orchestrator/engine.go` 12,508（lane 注册表 + RunView/创建选项扩展）、`tick.go` 8,414（implement 入口 admission + lane 解析）、`meta.go` 8,977（runMeta P2c 字段）、`internal/cli/goal.go` 15,963（注册 P2c 命令 + run 的 --lake/--scale 落账 + recording 同步注册 lane 执行器）。单文件均 <500 行（最大 lanes.go 355 行）。
- 边界守住：未真建 git worktree（计划处处 planned_only）、未真调 ralph/外部执行器（执行只有 RecordingExecutor 且 simulated 标注贯穿 delegation 记录、事件与全部输出）、未做 program/hotfix/memory/axis（P2d）、未改 Python 源仓、冻结 schema 零改动、无自带 JSON 状态文件。
