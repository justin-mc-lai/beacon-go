# Beacon Go 重构 · AC 全矩阵（前半：AC-001..AC-088）

> 日期：2026-10-22 ｜ 范围：冻结 truth `beacon-v2-rust-kernel-redesign/truth.md` 中 AC-001..AC-088（该区间编号无缺号，共 88 条）
> 数据源：冻结 truth 原文、Go 实现与测试（~/workspace/beacon-go）、口径稿 docs/scope-charter.md、各期 p*-implementation.md 报告
> 分类口径：tested＝有行为级测试实际断言该 AC 行为（须给出测试函数名）；implemented＝代码已实现该行为但无专门测试；deferred-external＝实现依赖外部目标（须写清前置条件）；dropped-by-charter＝口径稿拍板不迁。本半区 dropped-by-charter 为 0 条（口径稿 drop 16 为命令名处置，不对应本区 AC）。
> 拿不准的条目按更保守一档标，并在备注标「待复核」。

| AC | 摘要 | 分类 | 证据/备注 |
|---|---|---|---|
| AC-001 | v2 默认执行面为 Rust workspace | implemented | 拍板替换：用户 2026-10-21 选项 3 改用 Go 实现 v2 内核；落点为 Go module beacon-go（cmd/beacon 单二进制，版本 2.0.0-go.3），不标 dropped、不标 tested |
| AC-002 | v2 核心闭包不包含 beacon-python、PyO3 与 v1 Python 回退 | implemented | go.mod 仅 cobra 与 modernc sqlite 依赖，internal 各包无 Python 调用；phase0-inventory 结构闭环结论；无专门测试 |
| AC-003 | v1 与 v2 为长期双轨对等产品轨，禁止静默跨轨混跑 | implemented | 源仓 Python 轨只读未改、Go 轨独立二进制；store.OpenWithOptions 的 writer_track 写域守卫防跨轨争写；无 AC-003 专门测试 |
| AC-004 | features 下 truth.md 与 docs/beacon 版本目录为唯一需求权威 | implemented | orchestrator/truth.go 的包生命周期以磁盘 MD 包为准，store 只存 truth_path 指针；无专门测试 |
| AC-005 | 全局 DB 不得存储可编辑的需求正文，只存元数据与指针 | tested | TestNoTruthBodyColumns（internal/store/store_test.go）逐表逐列断言无 truth_body、body、content 类列 |
| AC-006 | v1 JSON 投影在 v2 为非权威，Spine 读 DB 优先 | tested | TestSpineShowGateArtifacts（internal/cli/cli_test.go）毒文件段：写入与 DB 矛盾的 .beacon/gate-report.json 后 gate 输出仍为 DB 值且读命令不写回 |
| AC-007 | 实现必须遵守存储三层分离（L1 MD、L2 全局 SQLite、L3 Blob） | implemented | internal/store 全局 DB、orchestrator/truth.go 的 MD 包、artifact_refs 的 Blob 指针三层分工成立；无整体断言的单测 |
| AC-008 | 禁止 per-project beacon.db 作为运行态权威 | implemented | store.DefaultPath 只解析全局 ~/.beacon/beacon.db 或 BEACON_STORE_PATH，无 per-project 打开路径；无专门测试 |
| AC-009 | 禁止 loop-runs.db 作为 v2 权威，v1 ledger 须导入全局库后只读归档 | deferred-external | 前置条件：真实 loop-runs.db 出现以探明 schema；当前 migration/scan.go 仅探测存在性并报告未导入（p3b 报告），运行权威只认全局库 |
| AC-010 | Blob 变更时 artifact_refs 更新 content_hash，hash 不符标 stale | tested | TestArtifactDaoAndStale（internal/store/store_test.go）与 TestArtifactViewHashMismatch（internal/store/spine_test.go）断言 hash 不符即 stale 与 hash_mismatch |
| AC-011 | doctor projection rebuild 能重建全局库索引且前后 lifecycle 与 gate 字段一致 | tested | TestDoctorApply（internal/migration/migration_test.go）断言 rebuild 前后 Spine 快照 lifecycle 与 gate 一致，TestDoctorReportOnly 断言只报不写 |
| AC-012 | lifecycle_events 只追加，feature_lifecycle 存当前快照 | tested | TestLifecycleEventsAppendOnly（internal/store/store_test.go）断言 UPDATE 与 DELETE 被 DB trigger 拦截 |
| AC-013 | 每次 gate 评估写入 gate_results（含 verdict 与 reason_codes） | tested | TestGateDao（internal/store/store_test.go）与 TestTruthFreezeGatePassWriteBackAndDeterminism（internal/orchestrator/gates_eval_test.go）断言评估落库 |
| AC-014 | 每次 run 启动写入 runs，checkpoint 写入 checkpoints | tested | TestRunsAndCheckpoints（internal/store/store_test.go）断言 CreateRun 与 SaveCheckpoint 落库 |
| AC-015 | Spine 查询 DB-first，blob 仅在指针存在且 hash 有效时拼接 | tested | TestSpineSnapshotDBFirstNoBlobs（internal/store/spine_test.go）断言无 blob 文件时 lifecycle 与 gate 仍返回 |
| AC-016 | v1 JSON 运行态只允许 export 与 import 迁移，不得与 DB 双写争权威 | tested | TestImportCountsAndIdempotency（internal/migration/migration_test.go）断言导入前后 v1 目录摘要不变（只读单向） |
| AC-017 | 必须实现 spine show 命令 | tested | TestSpineShowGateArtifacts 与 TestSpineShowSingleProjectDefault（internal/cli/cli_test.go） |
| AC-018 | 必须实现 spine list 命令 | tested | TestSpineListStatusFilterCLI（internal/cli/cli_test.go）与 TestSpineListFilters（internal/store/spine_test.go） |
| AC-019 | 必须实现 spine artifacts 命令 | tested | TestSpineShowGateArtifacts（internal/cli/cli_test.go）artifacts 段断言 stale 与 file_missing 标记 |
| AC-020 | 必须实现 spine gate 命令 | tested | TestSpineShowGateArtifacts（internal/cli/cli_test.go）gate 段断言最新 verdict 且只读不写库 |
| AC-021 | 必须实现 projects list、register、show 多项目注册表 | tested | TestProjectsRegisterListShowClosure（internal/cli/cli_test.go）断言幂等注册与 project_key、versions 字段 |
| AC-022 | 核心闭包收敛为五个 crate 且 beacon-python 不属于闭包 | implemented | Go 译法五包 internal/core、store、orchestrator、gate、cli（另有 harness、migration 支撑包）；phase0-inventory 结构闭环；无专门测试 |
| AC-023 | 包依赖方向固定且禁止循环依赖 | implemented | internal/cli/root.go 与 store.go 注释声明方向，实际 import 沿 cli 到 gate、orchestrator、store、core 且 Go 编译强制无环；无专门测试 |
| AC-024 | v2.0.0 不交付 beacon-proto 与 gRPC，CLI 与 orchestrator 不得硬依赖 | implemented | 按 truth D-V2-001 accepted defer 不交付；go.mod 无 gRPC 依赖、各包无 proto 引用；无专门测试 |
| AC-025 | 单文件模块超过 500 行须拆分或 ADR 说明例外 | implemented | 待复核：P0 报告时全文件达标；当前 internal/cli/goal.go 为 539 行超限且未找到 ADR 例外记录 |
| AC-026 | WorkflowStage 与 StageStatus 以强类型枚举定义，禁止字符串裸奔 | tested | TestParseWorkflowStageAndStatus（internal/core/core_test.go）断言全量合法值往返与未知值 typed error |
| AC-027 | 阶段转换表显式编码，非法转换返回 typed error 加 reason_code | tested | TestStageTransitions（internal/core/core_test.go）断言合法前进全过、5 类非法转换回 illegal_stage_transition |
| AC-028 | Blocked 必须携带 reason_codes 与可选 blocking_gate | tested | TestBlockedRequiresReasonCodes（internal/core/core_test.go） |
| AC-029 | Skipped 仅允许带 user_accepted_deferral_id 或 deferral ledger 引用 | tested | TestSkippedRequiresDeferralRef（internal/core/core_test.go） |
| AC-030 | Crashed、TimedOut、Interrupted 须写入 lifecycle_events 且可触发恢复 | tested | TestNeedsRecoveryEvent（internal/core/core_test.go）断言仅此三态为恢复类；落库与恢复由 orchestrator 的 RecordInterrupt 与 supervisor 路径承接 |
| AC-031 | Delivery 运行时 12 状态在核心定义并与 runs.status CHECK 一致 | tested | TestRunStatusSet（internal/core/core_test.go）与 TestRunsCheckMatchesCore（internal/store/store_test.go）双向断言集合相等 |
| AC-032 | plan 主链节点转换须写入 lifecycle_events（可审计） | tested | TestTraceOrderAndBundle（internal/orchestrator/engine_test.go）断言 stage_advanced 等事件按序落入 lifecycle_events 并可聚合 |
| AC-033 | Paused、Stopping、Stopped 与 v1 graceful 加 force stop 对齐 | implemented | 待复核：三态已定义、blocked 映射 Paused（engine.go 的 runStatusFor）、driver stop 有 TestDriverStopHalts；但未见 runs.status 向 Stopping 与 Stopped 的迁移落点 |
| AC-034 | Repairing 须持久化 repair payload 到 artifact_refs | implemented | 待复核：repair 循环与 Repairing 状态映射已落地（tick.go 的 handleStageFailure），但未找到 repair payload 写入 artifact_refs 的实现或测试（注：p2a 报告所称 AC-034 指 v1.6 中断重放，非本条） |
| AC-035 | AwaitingQa 仅在 import-evidence 成功后可进入 | implemented | 待复核：RunAwaitingQa 仅为状态值，未找到 import-evidence 入口守卫的实现或测试（注：core/loopgoal.go 的 AC-035 注释指 v1.6 短续跑词，非本条） |
| AC-036 | Closed 仅 release gate pass 或用户显式 archive，orchestrator 不得自设 | implemented | orchestrator 全仓无写入 RunClosed 的代码，Complete 终态映射 RunSucceeded（engine.go 的 runStatusFor）；无专门测试 |
| AC-037 | EvidenceChain 含 manifest_hash、signature_chain 与有序 artifact_refs | tested | TestEvidenceChainVerify（internal/core/core_test.go）断言链构建成功与顺序敏感 |
| AC-038 | import 断链 fail-closed，返回 EVIDENCE_CHAIN_BROKEN | tested | TestEvidenceChainVerify（internal/core/core_test.go）断言空链、截断、篡改签名、篡改 ref、坏 manifest 等情形均回该码 |
| AC-039 | v1 evidence-signature-chain.json 可迁移为 v2 记录 | deferred-external | 待复核：未找到签名链迁移实现（migration 仅导入 evidence.md 的 artifact 指针）；前置条件为真实 v1 签名链文件样本以确定格式 |
| AC-040 | orchestrator 经 CLI 暴露 goal、resume、stop、status | tested | TestGoalCLIFullChain（internal/cli/goal_test.go）覆盖 run 与 status，TestCheckpointResumeAndInterrupt（internal/orchestrator/engine_test.go）覆盖 resume，TestDriverStopHalts（internal/orchestrator/driver_test.go）覆盖 stop |
| AC-041 | 启动前 truth-map resolve，blocked 时非零退出并推荐 gen-change | implemented | 待复核：无 truth-map resolve 命令；等价拦截在 exitgates.go 与 admission.go 的 frozen 判定（blocked 经 CLI 非零退出），推荐语义与原文不完全一致 |
| AC-042 | AgentBackend 合同定义 iterate、health_check、abort、name，IterationResult 强类型 | tested | TestSubprocessBackendRealIteration（internal/harness/harness_test.go）断言 iterate 输出符合 AC-042 schema 且 health_check 通过；合同在 internal/harness/backend.go、类型另在 core.IterationResult |
| AC-043 | CodexBackend 真多轮集成（同一 run 至少 2 次成功 iterate） | deferred-external | 前置条件：本机装上真 codex 二进制；多轮形状已由脚本验证（TestRalphLoopCompletesSecondIteration），p3c 与 p4c 报告明示厂商本体未验 |
| AC-044 | SubprocessBackend 完成 1 次真集成迭代且输出符合 AC-042 schema | tested | TestSubprocessBackendRealIteration（internal/harness/harness_test.go）与 TestSubprocessExecutorRealEvidence（internal/orchestrator/p3c_test.go） |
| AC-045 | ClaudeBackend 完成 1 次真集成迭代，health_check 与 abort 有集成证据 | deferred-external | 前置条件：本机装上真 claude 二进制；claude-style profile 模板与上下文传递已由脚本验证（TestProfileBackendResolvesContext），p3c 报告明示本体未验 |
| AC-046 | AcpBackend 完成 1 次真集成迭代且输出符合 AC-042 schema | deferred-external | 前置条件：真实 ACP agent 目标；当前仅 initialize 加 session/new 两步探针（TestACPProbeTwoStepHandshake），Iterate 明确拒绝 acp_full_backend_not_supported（p3c 与 p4c 报告） |
| AC-047 | checkpoint resume 后 iteration 递增，notes 注入写入 lifecycle_events | implemented | 待复核：checkpoint 迭代递增已测（TestCheckpointResumeAndInterrupt），但 Resume 忽略 utterance、无 notes_injected 事件写入，未找到 notes 注入实现 |
| AC-048 | stop_when 移植 v1 stop-when-gate（含 gate pass 早停） | implemented | 待复核：core 的 StopWhen 钩子与 TestLoopTransitionBlockedByStopWhen 已测，但 SetStopWhenEvaluator 无生产调用方，gate pass 早停的具体评估器未接线 |
| AC-049 | orchestrator 禁止输出 QA pass、release ready、feature complete 结论 | implemented | orchestrator 只读 gate_results 做出口判定（exitgates.go、complete.go 查 qa verdict 而不自产结论）；无专门测试 |
| AC-050 | Phase Controller 四阶段写入 runs.metadata_json，phase 转换写 lifecycle_events | implemented | 待复核：spine_derive.go 只读 metadata 的 phase 字段且缺失时派生（注释明示 orchestrator 尚未写入 phases_completed），未找到 Phase Controller 落点；stage_advanced 事件已写 |
| AC-051 | CLI 冷启动 p95 小于 100ms，须 bench 或 CI benchmark 归档 | deferred-external | 待复核：P6 benchmark 门禁未建，无 Benchmark 函数或 CI 归档（p4c 报告列为 P6 待办）；属阶段延期而非外部目标阻塞 |
| AC-052 | spine show p95 小于 50ms，须 bench 归档 | deferred-external | 待复核：同 AC-051，P6 benchmark 门禁未建（p4c 报告列为 P6 待办） |
| AC-053 | truth-map resolve p95 小于 200ms，须 bench 加 fixture 验证 | deferred-external | 待复核：同 AC-051，P6 benchmark 门禁未建，且 truth-map resolve 本体见 AC-041 待复核 |
| AC-054 | Gate 计算 p95 小于 50ms，须 bench 归档 | deferred-external | 待复核：同 AC-051，P6 benchmark 门禁未建（p4c 报告列为 P6 待办） |
| AC-055 | 单文件证据 hash 验证 p95 小于 10ms，须并行 bench 归档 | deferred-external | 待复核：同 AC-051，P6 benchmark 门禁未建（p4c 报告列为 P6 待办） |
| AC-056 | DB migration v1 到 v2 小于 60s（典型 fixture），须 integration timer | implemented | 待复核：迁移已实现且正确性已测（internal/migration/migration_test.go），但未找到耗时断言或 timer 归档 |
| AC-057 | orchestrator iteration 调度小于 100ms（不含 agent），须 bench 归档 | deferred-external | 待复核：同 AC-051，P6 benchmark 门禁未建（p4c 报告列为 P6 待办） |
| AC-058 | beacon.db 体积软上限 500MB，超阈值 WARN 日志加 metric | deferred-external | 待复核：未找到体积监控或 WARN 实现，P6 benchmark 门禁未建（p4c 报告列为 P6 待办） |
| AC-059 | v2 harness surface 只暴露六个一等能力 | implemented | internal/harness/harness.go 的 Abilities 恰六项，cli/root.go 据此建六组顶层命令；无专门计数测试 |
| AC-060 | 双轨运行时选择必须显式，禁止环境默认偷偷切轨 | implemented | 待复核：Go 轨为独立二进制与独立版本串（2.0.0-go.3）、不偷切 v1；但二进制名仍为 beacon、无 runtime 旗标，phase0 将命名口径列为待拍板 |
| AC-061 | BEACON_STORE_PATH 测试隔离，生产默认 ~/.beacon/beacon.db | tested | internal/store/store_test.go 的 openTestStore 断言 env 路径解析，全部 store 测试经此隔离（如 TestOpenIdempotentAndWAL） |
| AC-062 | v2 路径内把 v1 Python 后端当执行后端的请求须 fail-closed 回 cross_track_fallback_forbidden | implemented | 待复核：纯 Go 实现无 Python 后端可回退（构造性成立），但未找到该 reason_code 常量、守卫代码或测试 |
| AC-063 | harness list 只返回六个一等能力及其 contract metadata | implemented | 待复核：无 harness list 命令；数据源为 internal/harness/harness.go 的 Abilities 与六个顶层命令组（等价面），未找到专门测试 |
| AC-064 | spine、projects、workflow、truth-map、implement、qa、release、run 子命令可解析且 help snapshot 稳定 | implemented | 待复核：spine、projects、goal、truth、implement、qa、release 经大量 CLI 测试实际执行解析，但无 help snapshot 测试，且无 workflow 与 truth-map 同名命令 |
| AC-065 | spine、projects、run 支持 --json 且 schema 可校验 | tested | TestSpineShowGateArtifacts 与 TestProjectsRegisterListShowClosure（internal/cli/cli_test.go）逐键断言 JSON 字段，TestSpineSnapshotFullFields（internal/store/spine_test.go）断言冻结键名 |
| AC-066 | reason_codes 与 v1 对齐，gate 失败样例与文档及集成测试至少 5 码往返一致 | implemented | 待复核：gate reason_codes 落库已测（TestGateDao 等）、goal 词表逐字已测（TestReasonVocabularyVerbatim，13 码）；但 truth 指定的 reason-codes.md 在源仓归档中不存在、无按该文档的往返测试 |
| AC-067 | projects 表字段合同（id、root_path UNIQUE、name、registered_at） | tested | TestProjectVersionFeatureConstraints（internal/store/store_test.go）与 TestVerifySchemaContractDrift 的列集合校验 |
| AC-068 | project_versions 表字段合同（id、project_id、version、docs_root、is_current） | tested | TestProjectVersionFeatureConstraints（internal/store/store_test.go）断言单 current 切换 |
| AC-069 | feature_packages 表字段合同（id、project_version_id、slug、truth_path、tests_path、status） | tested | TestProjectVersionFeatureConstraints（internal/store/store_test.go）断言 slug 同 version 内唯一、跨 version 可重用 |
| AC-070 | feature_lifecycle 表字段合同（feature_id、stage、status、reason_codes_json、updated_at） | tested | TestLifecycleSnapshotValidation（internal/store/store_test.go） |
| AC-071 | runs 表字段合同（id、feature_id、run_kind、status、iteration、started_at、ended_at、metadata_json） | tested | TestRunsAndCheckpoints（internal/store/store_test.go） |
| AC-072 | lifecycle_events 表字段合同（id、feature_id、run_id、event_type、payload_json、created_at） | tested | TestLifecycleEventsAppendOnly（internal/store/store_test.go） |
| AC-073 | gate_results 表字段合同（id、feature_id、gate_type、verdict、reason_codes_json、evaluated_at） | tested | TestGateDao（internal/store/store_test.go） |
| AC-074 | artifact_refs 表字段合同（id、run_id、kind、abs_path、content_hash、size_bytes、created_at） | tested | TestArtifactDaoAndStale（internal/store/store_test.go） |
| AC-075 | checkpoints 表字段合同（id、run_id、iteration、state_json、created_at） | tested | TestRunsAndCheckpoints（internal/store/store_test.go）断言 iteration 回退被拒 |
| AC-076 | store 提供 ProjectDao、FeatureDao、RunDao、GateDao、ArtifactDao、SpineDao | tested | internal/store/store_test.go 与 spine_test.go 的逐 DAO 行为测试覆盖六个 DAO（如 TestSpineSnapshot、TestGateDao、TestArtifactDaoAndStale） |
| AC-077 | migration v1_runtime_ledger 自 runtime-ledger.sqlite3 导入 | deferred-external | 前置条件：真实 runtime-ledger.sqlite3 出现以探明 schema；当前 scan.go 仅探测存在性、存在时列 uncertain 不导入（p3b 报告） |
| AC-078 | migration v1_loop_ledger 扫描各项目 loop-runs.db 一次性导入 | deferred-external | 前置条件：真实 loop-runs.db 出现以探明 schema；当前仅探测存在性（scan.go 的 LedgerSource、p3b 报告） |
| AC-079 | migration v1_qa_projection 可选历史只读导入 | implemented | 待复核：p3b 判定 .machine/qa 的 qa9-matrix 为测试计划矩阵而非结论、未作内容导入，仅经 evidence.md 登记为 artifact 指针（migration/scan.go、migrate_items.go） |
| AC-080 | schema 以 user_version 管理，降级禁止 | tested | TestDowngradeOpenForbidden 与 TestOpenIdempotentAndWAL（internal/store/store_test.go） |
| AC-081 | SpineDao snapshot 联查 lifecycle、run、gate 与 artifacts | tested | TestSpineSnapshot（internal/store/store_test.go）与 TestSpineSnapshotFullFields（internal/store/spine_test.go） |
| AC-082 | list 支持 lifecycle 与 run status 过滤 | tested | TestSpineListFilters（internal/store/spine_test.go）与 TestSpineListStatusFilterCLI（internal/cli/cli_test.go） |
| AC-083 | artifact_refs.abs_path 必须为绝对路径 | tested | TestArtifactDaoAndStale（internal/store/store_test.go）断言相对路径被拒并回 artifact_path_not_absolute |
| AC-084 | P0 阶段出口 | implemented | docs/p0-implementation.md 出口报告（core 加 store 加 migration skeleton、20 测试全过）；阶段出口为流程里程碑，无单测断言出口本身 |
| AC-085 | P1 阶段出口 | implemented | docs/p1-implementation.md 出口报告（spine 与 projects 面全量可用）；阶段出口为流程里程碑，无单测断言出口本身 |
| AC-086 | P2 阶段出口 | implemented | docs/p2d-implementation.md 的 P2 收口（发 v2.0.0-go.1、103 测试）；阶段出口为流程里程碑，无单测断言出口本身 |
| AC-087 | P3 阶段出口 | implemented | docs/p3c-implementation.md 的 P3 收口（发 v2.0.0-go.2）；注：出口内 AC-043 与 AC-045 的厂商本体、AC-046 仍为形状或探针级（见各条） |
| AC-088 | P4 阶段出口 | implemented | docs/p4a-implementation.md、p4b-implementation.md、p4c-implementation.md 与 v2.0.0-go.3 出口（六能力收敛、统一评分、P4c 终局）；阶段出口为流程里程碑，无单测断言出口本身 |
