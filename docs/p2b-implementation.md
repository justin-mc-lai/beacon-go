# P2b 实施报告：长跑看护（2026-10-21）

一句话：goal 现在能「长跑」了——有个司机（driver）一轮一轮推着任务走、到发布门口自动停下等人；有个监工（supervisor）盯着心跳，任务闷死（长时间没心跳）会被判卡死并标记好怎么恢复；零散发现的新活先排队（queue），排队的东西永远不能冒充「已完成」；中途改方向（steering）会被所有入口摆在明面上直到人确认；任务自带待办清单和油量表（预算）。二进制 14,679,584 字节；测试 68 个全过（P0/P1/P2a 的 48 个零回归，本期新增 20 个）。

## 1. 逐 G 条落地表

| G 条 | 落地文件 / 函数 | CLI | 测试 |
|---|---|---|---|
| G-06 driver 长跑 | `orchestrator/driver.go`：`DriverStart/DriverStatus/DriverStop/DriverRunOnce/DriverRunLoop`；租约 CAS 在 `store/dao_p2b.go` `RunDao.UpdateRunStateCAS` | `goal driver start/status/stop/run-once/run-loop`（run-loop 独立子命令，不塞进 start 参数——循环可能跑很久，独立命令才好看进度、好中断） | `TestDriverDuplicateStartRejected`、`TestDriverLeaseTakeoverAfterExpiry`、`TestDriverRunLoopStopsAtReleaseGate`、`TestDriverBudgetExhaustedStopsLoop`、`TestDriverRunOnceTurnBookkeeping`、`TestDriverStopHalts` |
| G-07 supervisor 看护 | `orchestrator/supervisor.go`：`AttachSupervisor/DetachSupervisor/SuperviseOnce/LongrunDoctor` | `goal supervise attach/detach/once`、`goal longrun-doctor` | `TestStallThresholdResolution`、`TestSuperviseStallDetectedAndResumeReplays`、`TestSuperviseNoFalseStall`、`TestLongrunDoctorReport` |
| G-09 discovery queue | `orchestrator/queue.go`：`EnqueueDiscovery/ListQueue/ConsumeNext/RejectQueueComplete`；`Complete` 内的队列拦截在 `complete.go`；队列扫描 `store/dao_p2b.go` `RunDao.ListByKind` | `goal queue enqueue/list/consume-queue/mark-complete` + 顶层 `goal consume-queue`（同一实现两个名字：冻结的 v1 命令名 + 分组名） | `TestQueueFIFOConsumeCreatesRuns`、`TestQueueItemCannotComplete`、CLI 链 |
| G-19 self-heal 三点 | `orchestrator/selfheal.go`：`PlanSelfHeal/ExecuteSelfHeal`，步骤表逐字移植 progressive map 的 `goal_transversal.self_heal` | `goal self-heal [--plan] [--phase pre\|mid\|post]`（默认执行自动项，`--plan` 只看计划） | `TestSelfHealPrePlanAndExecute`、`TestSelfHealMidRepairsEvidenceManifest`、`TestSelfHealPostNeverAutoExecutes` |
| G-21 护栏组 | `orchestrator/guardrails.go`：`ConfigureBudget/Steer/AckSteering/TodoAdd/List/Done/Remove/Claim`；词表 `core/reasons.go`（`ReasonCatalog` + `IsKnownReasonCode`），P2a 的出口门代码已改指 core 常量（`exitgates.go` 别名） | `goal steer [--ack]`、`goal todo add/list/done/remove`；预算经 `goal driver start --max-turns/--max-duration-sec` 与状态面呈现 | `TestSteeringSurfacedUntilAcked`、`TestTodosAndStatusSummary`、`TestConfigureBudget`、`TestReasonVocabularyVerbatim`、CLI 链 |

状态落点（口径第 7 条守住）：driver 租约、supervisor 会话、预算账本、steering 记录、todo 清单、队列项——全部只进全局库（runs 的 metadata_json 快照 + lifecycle_events 追加轨迹 + checkpoints），没有任何一处自带 JSON 状态文件。

## 2. 与源仓语义差异 / 简化清单（逐项注明）

1. **driver 锁：从「锁文件 + 进程存活检查」换成「库内租约 + 原子换手」**。源仓用每 run 一个 `driver.lock` 文件、靠 pid 是否存活判双开。Go 版把租约（持有者 + 到期时间，默认 10 分钟）写进 run 快照，`DriverStart` 用 CAS（compare-and-swap，只有库里那份快照还是我读到的那份才准写）抢租约——两个进程同时抢，只有一个能成，另一个收到 `agent_driver_dual_start_forbidden`；租约过期可被新司机接管（留 `driver_lease_takeover` 事件）。**诚实边界**：租约只锁住「启动」这一动作，不是执行期的 fencing token（强制隔离令牌）——某个司机租约中途过期，它的当前轮不会被强行掐断；所以同一 run 仍不许两个进程同时 run-once，这是文档约束、不是机制强制。
2. **一轮的定义变了**：源仓一轮 = 拼一份提示词、拉起一个宿主 agent 进程、等它退出。P2b 不接真 agent（边界），一轮 = 调 P2a 的 `Tick` 一次（卡住时调 `Continue` 解软卡）。轮末的动作与源仓对齐：写心跳、写 checkpoint（substep=`driver-turn-N`）、生成下一轮提示摘要。提示摘要是 InjectBundle 的浓缩版（steering → 当前阶段/下一技能 → 下一动作，只留摘要不拼全文模板）；steering 模板文件（continuation/budget_limit/objective_updated 三份 fixture）未移植，列 P2c 宿主桥接入时再做。
3. **司机停机条件**：源仓把 goal 的 blocked 当终局。Go 版：tick 后仍 blocked（硬卡或出口门卡住、司机清不了）→ `halt_terminal` 停机留原因；软卡由 `Continue` 先解一次。发布门口（release_gate）→ 新状态 `awaiting_human` 停住等人（源仓状态表里没有这个值，是本期按任务口径加的扩展，`auto_release` 恒 false、司机永不调 Complete——结构上就没有这条代码路径）。
4. **stall 阈值口径**：显式参数 > run 上记录的心跳间隔（attach 时写入，源仓口径默认 120 秒、下限 30 秒）> 30 分钟兜底（1800 秒常量 `DefaultStallThresholdSec`）。只有前两级都查不到时才用 30 分钟——正常 attach 过的 run 永远走 120 秒那一级。判 stall 的阶段/状态门与源仓 `detect_stall` 一致：已完成、已卡住、发布阶段都不判（发布是在等人，不算卡死）。
5. **supervise 不给自己写心跳（与源仓分歧，故意）**：源仓 `supervise_once` 判完 stall 后立刻补一记「监工心跳」，等于把刚判出的卡死又盖住、要再等一整个间隔才判得出。Go 版监工轮询只更新 `last_poll_at`，stall 保持可见，直到真有活干起来、心跳恢复。
6. **stall 标记不造第二套中断表示**：判 stall 后走 P2a 已有的 `RecordInterrupt`（InterruptPending + `host_interrupt` 事件），下一次 `Resume` 返回 `replay_substep`（AC-034），与宿主真中断完全同一条路。
7. **longrun-doctor 的口径变了（按口径稿 G-07）**：源仓这个命令查的是「本机装没装好、桥接命令配没配」。口径稿把 G-07 定义为 per-run 体检，Go 版报告的是：司机轮次、心跳年龄、stall 次数（数事件轨迹，是权威来源）、中断次数、blocked streak、repair 计数、预算、健康问题清单。桥接就绪检查归 doctor 支撑面（口径 slim 项），不在本片。
8. **queue 存储方案（重点论证，未动冻结九表）**：冻结 schema 没有队列表，加表会破 AC-067..075 合同，所以**没加表**（任务说「确需新表先列待拍板」——结论是不需要）。队列项 = `runs` 表里 `run_kind="discovery_queue"` 的行：FIFO 顺序用 started_at+rowid、事件轨迹现成、`feature_id` 外键正好是 runs 表自带的。队列状态（queued/consumed）存该行 metadata，runs.status 只做粗粒度镜像（Created=排队中、Closed=已消费）。代价两条：① 队列项的 id 长着 `run_` 前缀（它就是一行 runs）；② 入队必须先解析出一个 feature（源仓队列是项目级、feature 可空；runs.feature_id 非空约束不允许悬空项），解析失败的报错与 `goal run` 同一套。消费语义按源仓 `consume_discovery_queue`：取队首 → 正式建 goal run（带 `from_queue_item` 出处）→ 标记已消费。
9. **防冒充两道断言都落了**：① 每个队列项永远带 `product_complete=false`、`may_mark_product_complete=false`；② 对队列项调 Complete 直接拒绝（`loop_cannot_mark_product_complete`），CLI `goal queue mark-complete` 是 loop_goal.py 那个「故意禁止」命令的移植——永远拒绝、带重定向提示。
10. **预算只做轮次 + 时长，不做 token**：源仓预算/配额家族有 token 账本和 spend 幂等账本（goal_budget 的 spend ledger、goal_quota 的 slot 账本）。P2b 没有 token 来源，编一个就是冒充——只落「最大轮次 + 最大时长（秒）」两个可配额度，司机每轮在 tick 写回后扣一轮（源仓 spend-after-writeback 规则的结构版：先干活写回、后记账）。超限 → `budget_exhausted` 事件 + `halt_budget`，原因码用源仓 driver 的 `agent_driver_budget_exhausted`。token/slot 账本列为待宿主接入后（P2c+）再补。
11. **steering 落地范围**：记记录、objective 类转向直接顶替 run 的目标话术（NEW_OBJECTIVE_SUPERSEDES 语义）、未确认的一直被 tick/resume/status 摆出来（`pending_steering` 字段）、`--ack` 确认。源仓 steering 的三份提示模板渲染未移植（见第 2 条）。
12. **todos 存 run 快照、不存 checkpoints**：checkpoints 是只增的恢复点（每次内容是整份状态），当清单存会让「列清单」要回放全部恢复点，不合适；清单和 run 快照同进同出、原子一致，状态汇总直接由同一份清单算出——清单和汇总永远不可能不一致（源仓 goal_todos 的 lease/supersede 全语义只落了 claim + double_claim 拒绝，其余留后续）。
13. **ExitGate 的 P2a 代码归位**：`exitgates.go` 的原因码常量改为 core 词表的别名（字符串不变），满足「词表只有一个家」。

**待拍板清单：无新增。** 队列新表问题已论证为不需要（第 8 条）；若后续要做「项目级多队列」或「悬空 feature 入队」，才需要回头议加表。

## 3. 实跑样例（真二进制，临时库 /tmp/p2b-smoke）

种子同 P2a：`projects register` 后 sqlite3 种 feature_packages 一行 + truth_freeze/qa 两条 gate pass。

```
$ beacon goal run "给登录页加二次验证" --project /tmp/p2b-smoke/proj --feature smoke-feature --executor-id host-1 --json
{ "run_id": "run_33f3dbbc6945843005568e05", "current_stage": "plan", "loop_stage": "governance_precheck", ... }

$ beacon goal driver start --run-id <R> --max-turns 10 --json   # 摘要
state: arming  driver: drv-ed4827fe7d8a  lease_active: true  supervisor_attached: true

$ beacon goal driver start --run-id <R> --json ; echo exit=$?   # 双开被拒
exit=1
stderr: Error: goal.DriverStart: driver drv-ed4827fe7d8a already holds a live lease (agent_driver_dual_start_forbidden)

$ beacon goal driver run-loop --run-id <R> --executor recording --max-rounds 10 --json   # 摘要
rounds: 4  final_state: awaiting_human  run_status: running  loop_stage: release_gate  auto_release: false
# 司机自己停在发布门口，run 没完成、没发布

$ beacon goal supervise attach --run-id <R2> --json  →  attached: true, interval: 120
# 把 R2 的 last_heartbeat_at 回拨 2 小时（演示时间旅行）后：
$ beacon goal supervise once --run-id <R2> --threshold-sec 1800 --json   # 摘要
stall: true  reason: supervisor_stall_detected  age: 7200  actions: [stall_detected, host_interrupted]
$ beacon goal resume --run-id <R2> --json   # 摘要
resume_action: replay_substep  next: replay_substep:plan

$ beacon goal longrun-doctor --run-id <R2> --json   # 摘要
{'driver_state': 'idle', 'driver_rounds': 0, 'heartbeat_age_sec': 7206, 'stall_count': 1,
 'interrupt_count': 1, 'healthy': False, 'issues': ['heartbeat_stale', 'stall_detected_previously', 'driver_not_started']}

$ beacon goal steer --run-id <R> --text "先补文档再发布" --json  →  pending_steering: 先补文档再发布
$ beacon goal todo add --run-id <R> --title "补发布说明" --json  →  todo: todo-1 补发布说明
$ beacon goal self-heal --run-id <R> --phase post --json  →  post step: post-release-local-install requires_confirmation
$ beacon goal self-heal --run-id <R> --phase mid --json   →  mid steps: [(repair-evidence-manifest, executed), (workspace-admit, manual_only)]

$ beacon goal queue enqueue "发现：登录页在弱网下白屏" --project ... --feature smoke-feature --json   # 摘要
queued: run_db22c2b8a9a46924033a82a7  depth: 1  product_complete: false
$ beacon goal queue mark-complete --json ; echo exit=$?
{ "allowed": false, "blocked": true, "reason_code": "loop_cannot_mark_product_complete",
  "message": "loop/queue surfaces are support discovery only; use `beacon goal complete` on a goal run" }
exit=1
$ beacon goal consume-queue --json   # 摘要
consume: consumed -> run run_65e42c69506d7df6f7ee16cb  from_queue_item: run_db22c2b8a9a46924033a82a7  remaining: 0
$ beacon goal consume-queue --json   # 摘要：空队列不是错误
consume again: empty goal_queue_empty

$ beacon goal trace --run-id <R> --json   # 事件类型序（节选）
[goal_run_created, driver_start, supervise_attach, (stage_executed, stage_advanced,
 driver_turn_end, goal_checkpoint) ×4, driver_awaiting_human, goal_checkpoint,
 goal_steering_recorded, goal_todo_added, self_heal_executed, evidence_manifest_repaired, self_heal_executed]
```

## 4. 验证与交付物

- `go build ./...` / `go vet ./...` / `gofmt -l .`（无输出）/ `go test ./... -count=1` 全过：**68 个测试**（core 13、store 18 不变；orchestrator 29 = P2a 10 + 本期 19；cli 8 = 此前 7 + 本期 1 条全链路）。P0/P1/P2a 的 48 个零回归。
- 任务清单要求的覆盖点逐项有测试：司机重复启动被拒 ✓、司机到 release_gate 自动停不 complete ✓（含轨迹里无 goal_completed）、预算耗尽停 ✓、supervise 心跳超时判 stall 且 resume 变 replay_substep ✓、未超时不误判 ✓（含 detached 不判）、queue FIFO ✓、queue 项冒充 complete 被拒 ✓、空队列消费不报错 ✓、self-heal post 永不自动执行 ✓、steering 记录后 tick 返回带提示 ✓、todo 增删与 status 汇总一致 ✓（含 claim 双占拒绝、ID 不复用）。
- 文件大小（字节）：bin/beacon 14,679,584（P2a 为 14,469,198）；新增源文件：internal/core/reasons.go 5,035；internal/store/dao_p2b.go 2,387；internal/orchestrator/meta_p2b.go 7,484、driver.go 15,150、supervisor.go 11,173、queue.go 10,485、selfheal.go 7,855、guardrails.go 8,274；internal/cli/goal_driver.go 11,085、goal_watch.go 11,331；改动：meta.go/engine.go/resume.go/complete.go/exitgates.go（加字段与拦截）、cli/goal.go（注册 8 个新命令/组）。新增测试 6 个文件（driver/supervisor/queue/guardrails/selfheal 的 orchestrator 测试 + cli/goal_p2b_test.go）。单文件均 <500 行（最大 driver.go 420 行）。
- 未做（按边界）：真 agent 后端与宿主桥接（P2c，含 steering 模板渲染、driver 拉起宿主进程）、ralph（P2c）、program/parallel/hotfix/memory/axis（P2d）、token/slot spend 账本（待宿主接入）、Python 源仓零改动、冻结 schema 九表零改动。
