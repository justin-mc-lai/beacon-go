# Beacon Go 重构 — P0 实现报告（core 类型层 + store 九表）

> 日期：2026-10-21 ｜ 范围：冻结 v2 真相的 P0 出口（AC-084 的 Go 轨部分）——`internal/core` 纯类型层 + `internal/store` 九表与六 DAO。
> 真相源（只读，未改动）：`~/workspace/repos/beacon/docs/beacon/archive/v2.0.0/features/beacon-v2-rust-kernel-redesign/{truth,tests}.md`。
> 前置报告：[phase0-inventory.md](phase0-inventory.md)（§4.2 P0 清单、§5 Rust→Go 映射）。

说人话：这一期把新版的「规矩」和「账本」做成了真代码——状态只能按规矩流转、卡住必须写明原因、证据链断了立刻报错；运行记录统一存进一个数据库，九张表按冻结合同建好，谁也不能往里塞需求正文，别的轨道想抢着写会被当场拒绝。每条规矩都有行为级测试，不是摆设。

---

## 1. 逐 AC 落地表

| AC | 落地文件 | 行为级测试 | 结果 |
|---|---|---|---|
| AC-026 强类型 + Parse/Valid | `internal/core/fsm.go`（All*/Valid/Parse*） | `TestParseWorkflowStageAndStatus`：全量合法值往返 + 未知值 typed error | ✅ |
| AC-027 转换表 + 非法转换 typed error | `internal/core/fsm.go`（NextStage/CanTransition/ValidateTransition） | `TestStageTransitions`：9 条合法前进全过；跳步/倒退/自环/过 Archive 5 例均返回 `reason_code=illegal_stage_transition` | ✅ |
| AC-028 Blocked 必带 reason_codes + 可选 blocking_gate | `internal/core/fsm.go`（StageState.Validate/NewBlockedState）、`internal/store/dao.go`、`dao_rest.go`（GateDao 同规则） | `TestBlockedRequiresReasonCodes`、`TestLifecycleSnapshotValidation`、`TestGateDao`（blocked 无码被拒） | ✅ |
| AC-029 Skipped 须 deferral 引用 | `internal/core/fsm.go`（NewSkippedState） | `TestSkippedRequiresDeferralRef`：无引用拒绝、id 路/ledger 路两路通过 | ✅ |
| AC-030 恢复类状态判定 | `internal/core/fsm.go`（NeedsRecoveryEvent） | `TestNeedsRecoveryEvent`：10 个状态逐个断言，仅 Crashed/TimedOut/Interrupted 为真 | ✅ |
| AC-031 RunStatus 12 态 + 与 DB CHECK 一致 | `internal/core/fsm.go`（AllRunStatuses）、`internal/store/schema.go`（CHECK 由 core 列表生成） | core `TestRunStatusSet`（恰 12 态、无重复、未知值拒绝）；store `TestRunsCheckMatchesCore`（解析 sqlite_master 中 CHECK 集合与 core 集合严格相等 + DAO 与 DB 双层拒绝非法 status） | ✅ |
| AC-037/038 EvidenceChain + 断链 fail-closed | `internal/core/evidence.go`（NewEvidenceChain/Verify） | `TestEvidenceChainVerify`：新鲜链通过；空链/截断/篡改签名/篡改 ref/坏 manifest 6 例均返回 `EVIDENCE_CHAIN_BROKEN`；顺序敏感性断言 | ✅ |
| AC-005 DB 不存需求正文 | `internal/store/schema.go`（九表无正文列） | `TestNoTruthBodyColumns`：逐表逐列扫描 truth_body/body/content 类列名 | ✅ |
| AC-012/072 lifecycle_events append-only | `internal/store/schema.go`（UPDATE/DELETE trigger）、`dao_runs.go`（仅 Append/List） | `TestLifecycleEventsAppendOnly`：直写 SQL 的 UPDATE/DELETE 均被 trigger 拦截，事件原文不变 | ✅ |
| AC-061 BEACON_STORE_PATH 隔离 | `internal/store/store.go`（DefaultPath/Open） | 全部 store 测试经 `t.Setenv(BEACON_STORE_PATH)` + `t.TempDir()`，不碰真实 `~/.beacon`；`openTestStore` 内断言路径解析 | ✅ |
| AC-067..075 九表字段合同 | `internal/store/schema.go`（SchemaContract + DDL，见 §2） | `TestOpenIdempotentAndWAL` 内 VerifySchemaContract 通过 + 各 DAO 测试 | ✅ |
| AC-068 同 project 仅一 current | `dao.go`（AddVersion/SetCurrent 同事务清旧 + partial unique index 双保险） | `TestProjectVersionFeatureConstraints`：切换后 current 恰 1 个 | ✅ |
| AC-069 slug 同 version 内唯一 | `schema.go`（UNIQUE(project_version_id, slug)） | 同上：同 version 重复 slug 被拒、跨 version 同 slug 允许 | ✅ |
| AC-070 lifecycle 快照 | `dao.go`（UpdateLifecycle/GetLifecycle，经 core 校验） | `TestLifecycleSnapshotValidation`（含 upsert 前进） | ✅ |
| AC-071 runs | `dao_runs.go`（CreateRun/GetByID/UpdateStatus/ListByFeature） | `TestRunsAndCheckpoints` | ✅ |
| AC-073 gate_results | `dao_rest.go`（Record/Latest/ListByFeature） | `TestGateDao`：Latest 取最新一条 | ✅ |
| AC-074/010/083 artifact 指针 + stale | `dao_rest.go`（Add/ListByRun/ListByHash/IsStale；相对路径拒绝 `artifact_path_not_absolute`） | `TestArtifactDaoAndStale`：hash 相同不 stale、不同即 stale | ✅ |
| AC-075 checkpoint 单调递增 | `dao_runs.go`（SaveCheckpoint 查 MAX 后校验 + UNIQUE(run_id, iteration)） | 同 runs 测试：同 iteration 回退返回 `checkpoint_iteration_regression` | ✅ |
| AC-076 六 DAO | `dao.go`/`dao_runs.go`/`dao_rest.go`（Project/Feature/Run/Gate/Artifact/Spine） | 上述各测试逐 DAO 覆盖真实 CRUD/查询 | ✅ |
| AC-080 user_version + 禁降级 | `store.go`（init：version>当前 → `schema_downgrade_forbidden`；version=0 建表打版；重复 Open 幂等并 Verify） | `TestDowngradeOpenForbidden`（手工造 user_version=99 的库被拒）、`TestOpenIdempotentAndWAL`（重开幂等、journal_mode=wal） | ✅ |
| AC-081 SpineDao snapshot 聚合 | `dao_rest.go`（Snapshot：lifecycle + run 计数/最新 run + 每类最新 gate + artifact 计数） | `TestSpineSnapshot`：四路聚合逐字段断言 | ✅ |
| AC-158 按轨写域 | `store.go`（store_meta.writer_track + Open 绑定 + checkWritable 纵深校验） | `TestTrackWriteDomain`：异轨写打开 fail-closed（`dual_track_writer_conflict`）、异轨只读打开可读、只读句柄写操作返回 `store_read_only` | ✅ |
| AC-161 schema 合同漂移闭环 | `schema.go`（VerifySchemaContract） | `TestVerifySchemaContractDrift`：干净库通过；加列 → drift 报错；删表 → drift 报错 | ✅ |

错误类型统一：`internal/core/errors.go` 的 `core.Error{ReasonCode, Op, Detail}`，reason_code 是结构体字段（非字符串拼接）；store 层复用同一类型（store → core 依赖方向符合 AC-023 翻译）。

## 2. 九表 DDL 摘要

字段严格按 AC-067..075（列名与顺序与 `SchemaContract` 一致，运行时可校验）：

| 表 | 关键定义 |
|---|---|
| `projects` | `id TEXT PK, root_path TEXT UNIQUE, name, registered_at` |
| `project_versions` | `id PK, project_id FK, version, docs_root, is_current INT` + partial unique index `WHERE is_current=1`（同 project 至多一行 current） |
| `feature_packages` | `id PK, project_version_id FK, slug, truth_path, tests_path, status, UNIQUE(project_version_id, slug)` |
| `feature_lifecycle` | `feature_id PK FK, stage, status, reason_codes_json DEFAULT '[]', updated_at`（当前快照，一 feature 一行） |
| `runs` | `id PK, feature_id FK, run_kind, status CHECK (IN core 12 态), iteration, started_at, ended_at NULL, metadata_json DEFAULT '{}'` |
| `lifecycle_events` | `id PK, feature_id, run_id NULL, event_type, payload_json, created_at` + `BEFORE UPDATE/DELETE` trigger `RAISE(ABORT,'lifecycle_events_append_only')` |
| `gate_results` | `id PK, feature_id FK, gate_type, verdict, reason_codes_json, evaluated_at` |
| `artifact_refs` | `id PK, run_id, kind, abs_path, content_hash, size_bytes, created_at`（仅指针，无正文） |
| `checkpoints` | `id PK, run_id FK, iteration, state_json, created_at, UNIQUE(run_id, iteration)` |

另有内部记账表 `store_meta(key, value)`（存 writer track），在合同比对中显式豁免并在此披露；九表之外的未知表会被 VerifySchemaContract 判为 drift。

## 3. 依赖变更

| 项 | Phase 0 | P0 后 |
|---|---|---|
| go.mod 直接依赖 | cobra v1.10.2 | + `modernc.org/sqlite v1.60.1`（纯 Go，`CGO_ENABLED=0`，选型理由见 Phase 0 报告 §5.1） |
| go.mod 大小 | 181 B | 644 B |
| go.sum 大小 | 900 B | 5,035 B |
| bin/beacon 大小 | 5,982,064 B | 5,958,207 B（基本持平：cmd/beacon 尚未链接 store，SQLite 驱动的体积成本在 P1 接线 spine/projects 命令时才计入，届时如实再报） |

## 4. 构建 / 测试实跑输出（2026-10-21 本机实跑）

```text
$ CGO_ENABLED=0 go build ./...   → OK（无输出）
$ go vet ./...                   → OK（无输出）
$ gofmt -l .                     → （空）
$ go test ./... -count=1
?   beacon-go/cmd/beacon          [no test files]
?   beacon-go/internal/cli        [no test files]
ok  beacon-go/internal/core       0.003s   （7 个测试全 PASS）
?   beacon-go/internal/gate       [no test files]
?   beacon-go/internal/harness    [no test files]
?   beacon-go/internal/orchestrator [no test files]
ok  beacon-go/internal/store      0.285s   （13 个测试全 PASS）
?   beacon-go/skills              [no test files]
$ ./bin/beacon --version → Beacon CLI v2.0.0-go-dev
$ ./bin/beacon truth     → beacon truth: not implemented yet (Phase 0 skeleton; …)，exit 0（骨架不回归）
```

测试总数 20（core 7 + store 13），全部行为级：无「文件存在 / --help 可跑」式弱断言（AC-164）。

## 5. 推导项（原文未逐条枚举，按任务口径标注）

1. **阶段转换表（AC-027）**：truth 只给出 stage 集合与管线顺序，未逐条枚举 (from,to)。本实现推导为「仅允许前进到紧邻的下一阶段」，代码注释与 `fsm.go` 文件头已标注 DERIVED。影响：若后续冻结口径允许跳过 Prototype（design 可选）等弹性，需要走 change/refreeze 调整转换表，不要静默改。
2. **EvidenceChain 哈希公式（AC-037/038）**：truth 只定形状与断链码，未定公式。本实现定为 `manifest=sha256(join(refs,"\n"))`、逐位签名链式哈希，公式写在 `evidence.go` 文件头。与 v1 签名链的互通（AC-039 迁移）在 P3 处理，届时若公式不一致需显式适配层。
3. **RunStatus 转换表**：P0 只做 12 态集合与校验（AC-031 的 P0 口径）；run 状态间的合法转换（如 AwaitingQa 仅经 import-evidence，AC-035；Closed 仅经 release gate，AC-036）属 orchestrator/gate 语义，随 P2/P3 落地，不在本期。
4. **feature_lifecycle 的 Skipped 落盘**：冻结九表里 lifecycle 表没有 deferral 列（AC-070 字段固定），故经 FeatureDao 写 Skipped 快照会因缺 deferral 引用被 core 规则 fail-closed 拒绝——这是按合同字段行事的结果；deferral 引用的持久化位置（payload/evidence 侧）留待 truth 侧补充口径后再接，先拒绝不硬存。

## 6. 未决项

1. **迁移（AC-077..079）**：P0 只做 migration skeleton（user_version + 建表）。v1 runtime-ledger / loop-ledger / qa 投影的真实导入按 phase 图在 P3，本期未做。
2. **Spine CLI 面（AC-017..021）**：SpineDao 聚合已就绪，`beacon spine show/list/artifacts/gate` 与 `beacon projects` 命令面属 P1，未做。
3. **store_meta 豁免**：writer track 记账表不在九表合同内，已用显式豁免 + 本报告披露处理；若治理口径要求零额外表，可改存 PRAGMA application_id，需拍板。
4. **多连接 WAL 读**：当前 `SetMaxOpenConns(1)`（单写者，AC-093）；P1 若要并发读 Spine 再评估放开。

## 7. 交付物清单（文件名 + 大小，字节）

| 文件 | 大小 |
|---|---:|
| `internal/core/core.go` | 3,048 |
| `internal/core/fsm.go` | 7,134 |
| `internal/core/errors.go` | 1,682 |
| `internal/core/evidence.go` | 2,568 |
| `internal/core/core_test.go` | 6,895 |
| `internal/store/store.go` | 7,637 |
| `internal/store/schema.go` | 6,923 |
| `internal/store/dao.go` | 9,371 |
| `internal/store/dao_runs.go` | 7,708 |
| `internal/store/dao_rest.go` | 8,048 |
| `internal/store/store_test.go` | 16,293 |
| `go.mod` / `go.sum` | 644 / 5,035 |
| `bin/beacon`（构建产物） | 5,958,207 |
| `docs/p0-implementation.md`（本报告） | 落盘后以文件系统为准 |

所有 Go 源文件均 < 500 行（最大 `store_test.go` 435 行），满足 AC-025 翻译口径。源仓 `~/workspace/repos/beacon/` 未做任何修改。
