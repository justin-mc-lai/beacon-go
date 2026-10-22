# P2a 实施报告：goal 核心引擎（2026-10-21）

一句话：`beacon goal` 正式入口落地——建 run、一步步推进（tick）、卡住留原因、发布前四道条件齐全才算完，全部状态只存全局数据库一份。二进制 14,469,198 字节；测试 48 个全过（P0/P1 的 30 个不回归，本期新增 18 个）。

## 0. 落账（口径方案拍板）

- `docs/scope-charter.md` 头部状态已改为「已拍板（2026-10-21，用户确认 6 条全按推荐通过）」，第 6 节 6 条逐条标注决定结果（全选推荐 A）。
- 第 5 条（入口叫 goal 不叫 loop）已注明：**需对冻结包走一次 gen-change 落账，列入 P3 前待办——尚未执行，不假装已走完**。
- CLI 侧 loop 占位处置：**保留占位但只做指针**（打印一行「已被 goal 取代」并指向 `beacon goal --help`），不做带自身行为的别名。理由：防长胖口径下别名迟早长出第二套行为；指针零行为、零维护，同时老用户敲 `beacon loop` 不会扑空。

## 1. 逐 G 条落地表

| G 条 | 落地文件 / 函数 | 测试 |
|---|---|---|
| G-01 run/create（create_goal_run + build_stage_order + stage plan） | `orchestrator/engine.go` `Engine.CreateRun`、`meta.go` `BuildStageOrder`/`ProgressiveSkills`；CLI `cli/goal.go` `goal run` | `TestCreateRunStageOrder`（含 UX 插入 design）、`TestGoalCLIFullChain` |
| G-02 tick/continue（tick_goal_run + goal_continue_run + 出口检查推进） | `orchestrator/tick.go` `Engine.Tick`/`Continue`、`exitgates.go` `EvaluateStageExit`；CLI `goal tick`/`goal continue`（`--executor recording` 显式启用占位执行器） | `TestTickIdempotentWhenBlocked`、`TestTickWithoutExecutorBlocks`、`TestRepairLoopCeilingRoutesToErrorRecovery`、CLI 全链路 |
| G-03 stage exit gates（evaluate_stage_exit 最小真实判定，只读） | `orchestrator/exitgates.go` `EvaluateStageExit`（纯函数）+ `Engine.ExitGate`/`buildExitInput` | `TestEvaluateStageExitBranches`（14 个分支）、`TestTickBlockedLeavesLifecycleReason` |
| G-05 block/attach-verifier/complete（record_blocker + attach_verifier + complete_goal_run） | `orchestrator/complete.go` `Engine.Block`/`AttachVerifier`/`Complete`；CLI 三个同名子命令 | `TestCompleteFourGates`（四路拒绝+成功）、`TestAttachVerifierMakerChecker`、CLI 全链路 |
| G-08 heartbeat/checkpoint/resume（goal_heartbeat + goal_checkpoint + build_resume） | `orchestrator/resume.go` `Engine.Heartbeat`/`Checkpoint`/`RecordInterrupt`/`Resume`；CLI `goal heartbeat`/`checkpoint`/`resume` | `TestCheckpointResumeAndInterrupt`（含 AC-034 replay、AC-035 短词） |
| G-17 trace 最小面（事件聚合，只读） | `orchestrator/resume.go` `Engine.Trace`（`store.ListEventsByRun`）；CLI `goal trace` | `TestTraceOrderAndBundle`、CLI 全链路 |
| G-20 loop-goal 12 态状态机（transition/check_stop_when/check_release_gate/increment_repair_loop/build_resume 语义） | `core/loopgoal.go` `LoopState.Transition`/`CheckReleaseGate`/`IncrementRepairLoop`、`LoopSkillForStage`；与生命周期轴、runs.status 轴完全分开 | core `loopgoal_test.go` 6 个测试（非法转换、未确认不能 done、stop_when 拦截、repair 上限、AC-035、12 态/技能表） |

支撑改动：`store/dao_goal.go` 新增 `RunDao.UpdateRunState`（status+metadata+ended_at 原子更新）与 `Store.ListEventsByRun`；`cli/root.go` 注册 `goal` 命令组 + loop 指针占位。

## 2. 与源仓语义差异清单（逐项注明）

1. **Complete 比源仓更严（按冻结 truth 收紧）**：源仓 `complete_goal_run` 只查 verifier；本实现四项齐全才放行——loop 在 release_gate、human confirmed、verifier 已挂、qa gate 最新 verdict=pass，缺一逐项给 reason_code（`not_at_release_gate` / `release_requires_human_confirmation` / `product_goal_complete_requires_verifier` / `qa_gate_not_passed`）。源仓 `_qa_stage_passed` 查的是 stage 状态，本实现查 gate_results 最新 qa verdict（DB-first 口径）。
2. **Tick 顺序**：源仓是先 evaluate_stage_exit 再由宿主执行；本引擎一格 tick = 先调 StageExecutor 执行当前 stage、再判出口门、再推进（占位执行器没有宿主回合，只能如此闭合）。release stage 例外：不执行、出口门恒 blocked（源仓原义）。
3. **Tick 幂等是按冻结 AC-143 补的**：源仓 product-goal tick 本身无显式幂等。本实现用「状态指纹」（run+loop 态+stage+最新 gate 事实）判定：指纹未变的重复 tick 返回 `up_to_date`、不重跑执行器、不重复记 blocker。执行失败不消耗指纹（可直接再 tick 重试，计 repair）；新 gate 结果会改变指纹、重新放行判定。
4. **maker≠checker 的 reason_code**：源仓 attach_verifier 没有这条校验。本实现按任务口径对齐 stop_when 家族，用 `stop_when_unverified`（complete_blocked_stop_when_unverified_execution 的家族根码），verifier source 与 run 的 executor 标识相同即拒绝。
5. **短续跑词映射是推导**：源码 `build_resume` 实际未消费 utterance 参数；本实现按冻结 AC-035 落地——有 next skill 的 resume 一律返回 load_next_skill，短词（继续/continue/resume/go）与显式 resume 同路。interrupt 的 replay_substep 来自源码 record_host_interrupt 语义（AC-034），resume 时一次性消费。
6. **frozen 判定 DB 化**：源仓读 truth.md frontmatter；本实现按 DB-first（AC-015）只认 lifecycle 快照（Freeze/Completed）或 truth_freeze gate=pass。truth 包缺失/draft 的区分用 feature_packages 的 truth_path 是否为空、status 是否 draft 代替文件系统检查。
7. **design 的 loop 态是推导**：loop_goal_state.py 没有 design 路由态，本实现把 design stage 映射到 review_routing（设计评审），已在 `meta.go` 标注 DERIVED。产品 stage → 生命周期轴映射（plan→Prd、truth→Freeze、design→Prototype、implement→Implement、qa→Qa、release→Release）同为推导，标注在同一处。
8. **状态落点选择**：heartbeat 时间存 runs.metadata_json（runs 表无心跳列，不改冻结九表）；run 快照整体存 metadata_json，runs.status 由引擎状态映射（Running/Paused/Repairing/Succeeded，推导映射，见 `engine.go runStatusFor`）。引擎在推进/阻塞时回写 feature_lifecycle 快照——自此 goal 引擎是该 feature 生命周期快照的写者（P1 时为推导填充，后续由引擎逐步接管，与 P1 报告的预告一致）。
9. **stage 状态失败处理简化**：源仓 stage 失败后由宿主技能自愈；本实现 implement/qa 失败计 repair loop（上限 3，超限 error_recovery 并在事件里带 recommended_route=beacon-gov-debug），其余 stage 失败记 blocker。blocked_streak≥3 后 reason_code 硬化为 product_goal_blocked_streak（源仓 record_blocker 原义），Continue 不再自动解封。
10. **CLI 细节差异**：v1 被 blocked 时 exit 2，本实现统一 exit 1（main 现有约定）但 stderr 带 reason_code、stdout 先打印完整 JSON；`goal run` 的 `--lake`、`tick` 的 `--note`、`complete` 的 `--force` 只作 v1 兼容接收、不产生行为（help 里注明；`--force` 永不绕过四道 complete 条件）。
11. **core 转换表比源仓松**：loop_goal_state.py 的 transition 本身不设邻接表（只拦 invalid_stage、stop_when、done 门），本实现照此——任意合法态之间可转换，done 门与 repair 上限是仅有的硬规则，没有自创邻接限制。

## 3. 实跑样例（真二进制，临时库）

环境：`BEACON_STORE_PATH=/tmp/p2a-smoke/beacon.db`，先 `projects register` 建项目，再用 sqlite3 种 feature_packages 一行 + truth_freeze/qa 两条 gate pass（feature 包的创建 UI 属 truth 面，P4 才有；此处仅为冒烟种子）。

```
$ beacon goal run "给登录页加二次验证" --project /tmp/p2a-smoke/proj --feature smoke-feature --executor-id host-1 --json
{ "run_id": "run_5e0327dd3d73ca76558bbd6c", "status": "running", "run_status": "Running",
  "current_stage": "plan", "loop_stage": "governance_precheck", "next_skill": "beacon-plan",
  "stage_plan": [plan(active, 8 skills), truth, implement, qa, release], "verifier": {"present": false}, ... }

$ beacon goal tick --run-id <R> --executor recording --json   ×4（每格摘要）
tick: truth     truth_routing      blocked: False
tick: implement implement_routing  blocked: False
tick: qa        qa_routing         blocked: False
tick: release   release_gate       blocked: False

$ beacon goal tick --run-id <R> --executor recording --json   # release 永不自动出门
release tick: release release_gate blocked: True release_requires_human_confirmation
stderr: Error: goal: run <R> is blocked (release_requires_human_confirmation)

$ beacon goal resume --run-id <R> --utterance "继续" --json
{ "resume_action": "await_human_confirmation", "next_skill": "beacon-eval-release", "current_stage": "release", ... }

$ beacon goal complete --run-id <R> --json ; echo exit=$?
exit=1 : Error: goal.Complete: release requires explicit human confirmation (release_requires_human_confirmation)

$ beacon goal attach-verifier --run-id <R> --ref evidence/qa-report.md --source qa-bot --json
verifier: {'required_for_complete': True, 'present': True, 'refs': ['evidence/qa-report.md'], 'source': 'qa-bot'}

$ beacon goal complete --run-id <R> --human-confirmed --confirmed-by lai --json
final: completed done Succeeded

$ beacon goal trace --run-id <R> --json   → events: 16
  goal_run_created, stage_executed, stage_advanced, ×4 组, stage_exit_blocked,
  goal_heartbeat, goal_checkpoint, complete_rejected, verifier_attached,
  release_confirmed, goal_completed

$ beacon goal status --run-id run_nope --json ; echo exit=$?
exit=1 : Error: goal.loadRun: no such goal run: run_nope (product_goal_run_not_found)

$ beacon loop
beacon loop: superseded — the formal entry for this ability is `beacon goal` (charter decision 5, 2026-10-21). Run `beacon goal --help`.

$ beacon goal tick --run-id <R> --executor recording --json   # done 后重复 tick
tick after done: completed up_to_date: True loop_goal_already_done
```

## 4. 验证与交付物

- `go build ./...` / `go vet ./...` / `gofmt -l .`（无输出）/ `go test ./... -count=1` 全过：**48 个测试**（core 13 = 原 7 + 新 6；store 18 不变；cli 7 = 原 5 + 新 2；orchestrator 10 全新）。P0/P1 的 30 个测试零回归。
- 任务清单要求的覆盖点逐项有测试：非法转换拒绝 ✓、release 未确认不能 done ✓、repair 超 3 进 error_recovery ✓、tick 幂等 ✓、exit gate 未过 tick 被 block 且 lifecycle 留 reason_code ✓、complete 三路拒绝（verifier/qa/human，另加 not_at_release_gate 共四路）✓、verifier=执行者被拒 ✓、checkpoint 后 resume 正确 stage+action ✓、interrupt 后 replay_substep ✓、trace 顺序 ✓、CLI 全链路冒烟 ✓。
- 文件大小（字节）：bin/beacon 14,469,198（P1 为 14,234,489）；internal/core/loopgoal.go 8,815（test 3,740）；internal/store/dao_goal.go 2,326；internal/orchestrator/ 共 10 个文件：engine.go 9,831、tick.go 7,606、complete.go 4,980、resume.go 7,809、exitgates.go 7,350、meta.go 6,799、executor.go 2,528、orchestrator.go 849、engine_test.go 15,465；internal/cli/goal.go 15,088（goal_test.go 5,959）、root.go 2,778（改）；docs/scope-charter.md 34,498（落账标注后）。单文件均 <500 行。
- 未做（按边界）：driver/supervisor（P2b）、真 agent 后端/ralph（P2c/P3）、queue/program/parallel/hotfix/memory/axis（P2d）、gate 评估写库四道门（P3，本片 exit gate 只读判定不写 gate_results）、Python 源仓零改动。
