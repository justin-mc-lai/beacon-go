# R2b 第二批补修报告：编排与发布组（7 项）

一句话总结：这一批补的是「评审和编排的大脑」——评审结论要真正改变 run 的走向（堵住、转人工、走变更流程），长跑要有配额和记账规矩，发布检查要从「单个功能包」升到「整个版本」，另外把实现审查（review）和需求图谱（truth map）做成了能单独调用的命令。全部以 Python v1.6.19 源码行为为准移植。测试从 265 个涨到 283 个，全绿；E2E 全链实跑 PASS，脚本没动。

## 逐项落点（Python 出处 → Go 落点）

### H5 评审结论回写状态机

- Python 出处：`utils/loop_goal_state.py` 的 `record_review_decision`——评审结论（分级 P0/P1/P2/P3、建议路线、需人拍板）按固定顺序路由：包不完整 → 送出错恢复；跑偏路线（指向 truth 变更类）→ 进 drift_routing；需人拍板且决定缓存没命中 → 送出错恢复等人工；有 P0 → 送出错恢复并堵住；P1 且强制人工门开着 → 等人工；其余可执行路线 → 进标准执行预检（P1 记一笔「带警告继续」审计）；没人认领的路线 → 送出错恢复。顺序就是源码顺序，一步没挪。
- Go 落点：纯判定 `internal/core/loopgoal_records.go` 的 `RouteReviewDecision`；引擎 `internal/orchestrator/reviewdecision.go` 的 `recordReviewDecisionOn` 把路由结果写回 run：记评审记录、挪 loop 阶段、堵/解堵、累加人工停指标、落 `planner_review_decision` 事件。
- 消费者（不是只存不用）：准入检查新增第 9 项 `review_decision`——有未决的堵截型评审时拒绝 implement，拒绝码就是评审自己的路由码；`Tick` 在 loop 处于出错恢复/跑偏路由且评审未决时不推进；`Continue` 不许把评审堵截随手清掉（按仓内既有口径返回带原因码的 held 视图）；`Resume` 对评审堵截不清堵。
- planner-review 本体接线：`internal/orchestrator/planner_review.go` 的车道结论现在会组装成结构化评审包（未分级的 block 车道默认按 P0 处理，宁可堵错不可放过）并走同一条回写通道。

### H6 quota 五段判定

- Python 出处：`utils/goal_quota.py` 的 `normalize_quota_config`（默认值：compute 1.0、窗口 24 小时、槽位 1 分钟；未给槽位上限时按 窗口×60÷槽位×compute 推导）与 `quota_should_run`（五段顺序：健康异常 → 等人工 → 等证据 → 等焦点 → 时间窗槽位配额；skip 时把恢复动作等字段透传；记账必须有已验证回写，否则拒 `spend_without_validated_writeback`）。
- Go 落点：纯判定 `internal/core/quota.go`；引擎 `internal/orchestrator/quota.go`（配置存 run 快照、每次判定重新规范化，与源码「normalize 在判定口做」一致）；CLI `goal quota status/configure`。
- driver 消费：`DriverRunOnce` 在急停之后、预算门之前先问配额。等待态 → 只刷心跳、返回 requeue 并带原因码，**不 tick、不花预算、不误判停滞**；配额耗尽 → 按 `quota_exhausted` 停机（halt，resume_action=none）；每轮推进后记 1 个槽位。

### H7 release 版本级检查 + 档案降级

- Python 出处：`commands/release.py` 的 `check`（版本级检查，约 25 项）+ `utils/prod_ops_v168.py` 的 `select_release_profile`（档案与门清单）。
- Go 落点：纯判定 `internal/gate/release_version.go`（档案选择 + 11 项版本级检查 + 降级改写）；事实层 `internal/orchestrator/release_check.go`（按版本列出全部功能包，逐包取冻结事实、四门最新结论、完整性实时判定、QA 产物数、AC 覆盖率、人工确认；结论逐包落 `release_version_check` 事件）；CLI `release check --project --version --profile`。
- 11 项检查：版本已注册、需求包全部冻结、无待重冻变更、完整性结论全过、四门全过、QA 证据齐全、覆盖率达标（0.8，源码原值）、材料不过期、人工确认齐全、项目元数据契约、版本面治理（拓扑门事实，非 git 项目不适用）。
- **审计纠正一处**：审计稿说 check() 对 hotfix 与 docs_only 都做降级。实读源码：`check()` 里真正执行降级的只有 hotfix 的「软集」（三项：implement 提示词边界、语义契约、项目元数据契约，fail 改 warn）；docs_only 的门清单 `[docs_consistency, human_release]` 只在 `select_release_profile` 里定义，`check()` 并没有消费。本批按任务要求让 docs_only 真正消费（清单外检查降为警告，文档一致性对应项 + 人工发布保持硬性）——这是**补实**，比 Python 现行行为多走一步，口径在此明示。hotfix 软集三项里只有「项目元数据契约」在 Go 有对应检查项，另两项无事实源（见残留差异）。

### M8 编排记录四件

- Python 出处：`utils/loop_goal_state.py` 的 `record_intent_fork` / `record_user_decision` / `_ensure_human_stop_metrics` / `route_to_error_recovery`，与 `utils/goal_gates.py` 的 gate 卡三件（build_gate_card / resolve_gate / assert_gate_resolved，schema `goal-user-gate.v1`）。
- Go 落点：纯判定 `internal/core/loopgoal_records.go`；引擎 `internal/orchestrator/records.go`；CLI `goal decide`、`goal gate card/resolve/check`、`goal fork`、`goal error-recovery`。
- 行为要点：用户决定进缓存列表，同一问题问过不再问（gate 卡命中缓存直接回缓存答案）；重复 resolve 报 `already_resolved`；未决断言报 `gate_unresolved`；人工打断统计（human_stops_per_run、stop_reason_counts、p1_continue_count）由评审路由、宿主打断（RecordInterrupt 同步累加）、送出错恢复共同喂养。
- **实证一处**：`pseudo_stall_flag` 在 Python 全仓只有初始化为 False、从无置 True 的代码——移植照此（字段在、永不变真），测试里专门钉住这一点，免得后人「顺手补上」。

### M9 预算契约三件

- Python 出处：`utils/goal_budget.py`：`enforce_handoff_budget`（交接文本截断到 16 行 / 1800 字并报告 truncated，不拒绝）、`spend_eligible`（五道守卫按源码顺序：静默来源不花钱 → 无已验证回写 → 同 digest 已存在报幂等冲突 → 来源不在白名单 → 配额耗尽）、`commit_spend`（同 digest 重复提交算幂等回放）、耗尽停机包（state=halt_budget、resume_action=none）。
- Go 落点：纯判定 `internal/core/turnbudget.go`；引擎 `internal/orchestrator/guardrails.go`（公开门 `RecordSpend` + driver 内部记账共用 `recordSpendOn`）；CLI `goal spend`。
- driver 消费：每轮的下一轮提示词先过交接限长（截断事实记进 `driver_turn_end` 事件）；轮后按来源记账（正常轮 delivery_completion、堵过的轮 repair，digest=「run:turn:n」），撞上 `program_drive_budget_exhausted` 时轮后停机。
- 口径一处：core 保留源码的边界行为「上限为 0 = 从一开始就耗尽」；但 Go 的 run 可能根本没配过上限，此时公开记账门仍要执行白名单/回写/幂等三道守卫——引擎层用探针账本（仅评估用）过守卫，落盘的仍是真实账本（上限保持 0 = 未配上限）。driver 内部记账在无上限时不设门，与原来一致。

### M15 review 实现审查面

- Python 出处：`commands/review.py` 的 spec / quality / design 三类审查（问题分 critical/warn；有 critical 则 fail，否则有问题为 warn；gate_ready = 无 critical）。
- Go 落点：`internal/orchestrator/review.go`（三类清单的核心子集，事实取自包文件、冻结事实、指纹漂移、门结论、产物新鲜度、最新 run 状态）；CLI 顶层 `beacon review spec|quality|design`。结论经 H5 通道回写（critical→P0、warn→P2、漂移类建议走 beacon-gen-change 路线），与 planner-review 同一条路。
- 检查映射：spec 查四件套齐全（truth.md 在 Go 包里同时承载用户故事/PRD/AC 矩阵，缺失时按源码三个码一并报）、实现标记缺失（warn）、漂移（critical）、材料过期（warn）、计划堵住（warn）；quality 查验证未过、证据未闭合、产物待更新、正式包未关闭（均 critical）、覆盖不全（warn）、计划堵住（critical，与源码 quality 口径一致）；design 查规格输入缺失、原型缺失（规格也缺时 critical、否则 warn，源码原规则）、design 门堵住（critical）/要求修订（warn）。

### M16 truth-map 独立面

- Python 出处：`commands/truth_map.py` + `utils/truth_map.py` 的 resolve/verify（契约 `truth_map`、authority_source=`cli_static_resolver_plus_markdown_feature_package`、json_authority=false；verify 状态 pass/blocked，诊断码含 `feature_package_not_resolved`、`machine_json_source_hash_stale`）。
- Go 落点：共享判定 `internal/orchestrator/truthmap.go` 的 `truthMapLayoutProblems`——准入束的 truth_map_verify 项已改为读同一函数（Detail 文案与行为原样，既有 bundle 测试全过）；独立面 `Engine.TruthMapResolve/TruthMapVerify`；CLI `truth map resolve/verify`。新鲜度一条腿用冻结指纹 vs 当前包指纹对比（与 bundle 的材料项同一比法），漂移时报源码的 `machine_json_source_hash_stale`。

## 残留差异（如实清单）

1. **H7 未移植的 Python 检查**（无 Go 事实源，逐项列明）：auto-test 三项（浏览器阻断类，Go 无浏览器测试跑者）、evidence manifest 的文件级三项（Go 的证据是已注册产物指针+新鲜度，已被「QA 证据齐全」一项覆盖其判定面，但 manifest 文件本身不存在）、legacy cutover preflight（无旧版切换面）、runtime/acceptance projection transaction（无对应投影事务）、release scorecard（Go 有 goal score 面，未并入版本检查）、semantic contract ready 与 implement prompt boundary verdict（hotfix 软集另两项，无事实源）、version change 四项（版本号变更治理，Go 无版本变更单面）。
2. **M15 未移植项**：review.py 的 ceo/accept/writeback 三个子命令未移植（不在本批核心子集口径内）；spec 的 closure 系列细项、design 的原型文件级绑定检查未逐项移植，只取了上文列明的核心子集。
3. **M16 未移植项**：`rebuild-machine`（重建机器 JSON——Go 口径 json_authority=false，无机器 JSON 可重建）与 `entrypoint-inventory`（入口清单）未移植。
4. **quota 的事实映射取舍**（Go 无 Python 宿主的原始信号，按 run 状态映射，已在代码注释标明）：健康异常=loop 在出错恢复；等人工=有待确认的 steering 或评审停在需人拍板；等焦点=有待处理的宿主打断；等证据=当前在 qa 阶段、qa 已执行且该功能 QA 产物为 0。四种等待都真实存在过判定路径，但与 Python 宿主信号不是同一个来源，后续若接入宿主信号应在此处替换。
5. **loop 阶段历史**：Go 的 StageHistory 只存阶段名，Python 存富条目（带时间与子步）；评审路由追加的也只是阶段名。
6. **评审包完整性判定**：Go 用结构化字段（DeclaredFindingCount 指针 + SynthesisMarker 布尔）复刻源码「键在不在」的语义，包的线上形状与 Python 的 dict 不同，判定结果等价。
7. **release check 的元数据契约**：Go 的实现是「注册行一致性」（功能包的项目/版本行互相一致），Python 查的是项目元数据文件契约——判定面相近，事实源不同。
8. **planner-review 的人工门开关**：引擎字段（AutoDecide、HumanGateOverrides）已备好并被路由消费，但 CLI 暂未加 `--human-gate-p1` 类开关，需要时再补面。

## 既有测试改动声明（逐条）

1. `internal/cli/goal_p2c_test.go`：准入项数断言 8 → 9。理由：H5 要求准入消费评审路由，清单合法增长一项（`review_decision`，无未决评审时 not_applicable）；该测试文件本身有先例（S2 时 5→7、S3 时 7→8 都按同样方式跟随并留注释），判定未放宽，仅计数跟随清单。
2. `internal/orchestrator/gates_integrity.go`：把 EvaluateIntegrity 的「取包+定模式+跑判定」抽成 `integrityReportLive` 供版本检查/truth-map 实时判定复用。行为不变（该文件既有测试全过），属重构非改判。
3. `internal/orchestrator/bundle.go`：bundleTruthMap 的布局判定改读共享函数 truthMapLayoutProblems，问题枚举逐字搬移，Detail 文案原样，既有 bundle 测试全过。
4. 其余既有测试零改动；R2a 的行为（意图门、材料过期、scope 词表）未动。

## 测试与验证

- 新增测试 18 个：core 5（路由七分支、quota 规范化+五段、交接截断、花费守卫+幂等，含 Halt 包）、gate 2（档案选择、版本检查+两种降级）、orchestrator 10（H5 四个后果测试、M8 记录全套、H6 driver 跳过与耗尽、M9 引擎守卫、H7 引擎面、M15 三面+回写、M16 独立面与 bundle 一致）、cli 1（全部新命令面冒烟）。
- 总数：265 → **283**，`go build ./...`、`go vet ./...`、`go test ./... -count=1` 全绿。
- E2E：脚本不走 planner-review，但走 driver 与 admit（本批都动过）——已用本批二进制实跑 `e2e/run_e2e.sh`，结果 PASS（全链完成、run done），脚本零改动；运行产物 `e2e/last_run.log` 已恢复原状。
- 版本号未动（统一在最后一批升 go.9）；schema 九表未动（新数据全走 run 快照与 events）；未提交、未推送。
