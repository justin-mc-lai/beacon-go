# P1 实现报告：spine 查询 + projects 注册表 CLI

范围：冻结 v2 truth 的 P1 出口（AC-085）——Spine 只读面（show/list/artifacts/gate）与 projects 注册表（register/list/show）落地为 Go CLI，全部 DB-first（AC-015），JSON schema 冻结口径（AC-065）。源仓 `~/workspace/repos/beacon/` 零改动。

验证口径：`go build ./...`、`go vet ./...`、`gofmt -l .`（空）、`go test ./... -count=1` 全过——共 **30 个测试**（core 7 + store 18 + cli 5），其中 P0 的 20 个不回归，P1 新增 10 个（store 5 + cli 5），均为行为级断言，无弱 TC。

## 1. 逐 AC/TC 落地表

| AC/TC | 落地 | 测试 | 结果 |
|---|---|---|---|
| AC-017 spine show / TC-017 | `beacon spine show [--project] [--feature] [--json]`，`internal/cli/spine.go`；无 --feature 时选 lifecycle 最新者，单项目机自动定位 | `TestSpineShowGateArtifacts`、`TestSpineShowSingleProjectDefault` | ✅ |
| AC-018 spine list / TC-018 | `beacon spine list [--project] [--status] [--json]` | `TestSpineListStatusFilterCLI`、`TestSpineListFilters` | ✅ |
| AC-019 spine artifacts / TC-019（汇总表标 P2，本期按任务要求一并做只读实现） | `beacon spine artifacts <run-id> [--json]`，含 stale 标记（§3） | `TestSpineShowGateArtifacts`（artifacts 段） | ✅ |
| AC-020 spine gate 只读面 / TC-020（评估写入属 P3，本期只读） | `beacon spine gate <feature> [--version] [--json]`：只读 DB 最新 verdict/类型，不评估、不写库 | `TestSpineShowGateArtifacts`（gate 段，含写后行数不变断言） | ✅ |
| AC-006 DB 权威 / TC-006 相关 | gate/show/list 从不读项目 JSON；测试在项目下写入与 DB 矛盾的 `.beacon/gate-report.json`（qa=fail/release=pass），输出仍为 DB 的 qa=pass/release=blocked | `TestSpineShowGateArtifacts`（poison 段） | ✅ |
| AC-021 projects / TC-021 | `beacon projects register --path [--name] [--version] [--docs-root] [--json]`（幂等）、`list`、`show --project`（含 project_key、versions[]、is_current、current_version） | `TestProjectsRegisterListShowClosure` | ✅ |
| AC-015 DB-first / TC-015 | 快照构建只查九表；唯一文件接触是 artifact hash 校验，失败降级为 stale 标记。测试删除唯一真实 blob 后 show 仍返回 lifecycle+gate | `TestSpineSnapshotDBFirstNoBlobs`、`TestSpineShowGateArtifacts`（删 blob 段） | ✅ |
| AC-065 --json schema / TC-065 | 各命令顶层字段冻结（§4），测试逐键断言存在性与关键字段类型（bool/数组/字符串） | `TestSpineShowGateArtifacts`、`TestSpineSnapshotFullFields`（marshal 键断言） | ✅ |
| AC-081 SpineDao 全字段 / TC-081 | `SpineDao.Snapshot` 扩展到冻结 SpineSnapshot 全字段（§2），联查 projects/project_versions/feature_packages | `TestSpineSnapshotFullFields` | ✅ |
| AC-082 list 过滤 / TC-082 | `SpineDao.List(SpineListFilter)`：project 过滤 + lifecycle/run 两轴过滤；CLI `--status` 命中任一轴 | `TestSpineListFilters`、`TestSpineListStatusFilterCLI` | ✅ |
| AC-083 绝对路径 / TC-083 | P0 已落地（ArtifactDao 拒绝相对路径），P1 的 artifacts 视图沿用同一 DAO | P0 `TestArtifactDaoAndStale`（不回归） | ✅ |
| AC-010 blob stale / TC-010 | `store.ViewOf/ArtifactStaleness`：实际文件 SHA-256 与存储 hash 比对（§3） | `TestArtifactViewHashMismatch`、`TestSpineSnapshotFullFields` | ✅ |
| AC-085 P1 出口 | spine/projects 面全量可用 + 本报告 | 全量测试 + §5 实跑 | ✅ |

错误面（全部命令一致）：查不到时 exit 非 0、stderr 带 reason_code、无 panic、JSON 模式下 stdout 不写半截文档——`project_not_registered` / `feature_not_found` / `run_not_found`，由 `TestNotFoundReasonCodes` 覆盖（含无 DB 文件、全新机器、已知项目内未知 feature/run 三种情形；`projects list` 在无 DB 时返回空列表而非错误）。

## 2. SpineSnapshot 字段来源表（AC-017 冻结代码块，共 21 个冻结字段）

| 冻结字段 | 来源 | 说明 |
|---|---|---|
| run_id | `runs` 最新一行 id | 无 run 时 `""` |
| project_key | **派生（v1 等值复刻）** | `ProjectKey()` 逐字复刻 v1 `runtime_projection._project_key`：resolved 绝对路径的 SHA1 hex 前 10 位 + `--` + 路径 basename（`my-app--6fa94a1eea`）；`TestProjectKeyParity` 用 Python 算法独立复算断言 |
| project_root | `projects.root_path` | 联查得到，存的是 ResolveRootPath 规范化结果（展开 ~、绝对化、解符号链接，与 Python `Path.resolve()` 对齐） |
| version | `project_versions.version`（feature 所属版本行） | |
| feature_slug | `feature_packages.slug` | Go 字段 `Slug` 保留为同值别名（P0 兼容），JSON 同时输出 `feature_slug` 与 `slug` |
| lifecycle_stage / lifecycle_status | `feature_lifecycle.stage/status` | 无快照行时为空 |
| phase | metadata `phase` → 推导 | 优先最新 run `metadata_json.phase`；否则 Status/RunStatus 为 Repairing 时 `repair`；否则 stage→phase 映射（Think/UserStory/Prd/TestCase/Freeze→truth、Prototype→design、Implement→generate、Qa→qa、Release→release、Archive→archive，源自 v1.6 spine 文档的 phase 词表） |
| phases_completed | metadata `phases_completed` → **推导（缺口，见下）** | |
| phase_promoted | metadata `phase_promoted` → 推导 | 任一最新 gate verdict ∈ {blocked, fail, failed, no-go} 时 false（v1.6 语义：上次 harness 失败必须人工看）；无 lifecycle 且无 run 时 false |
| next_harness / next_action | metadata 同名字段 → 推导 | Blocked 时 `resolve_harness_blocker`（v1.6 词表）；否则按 pipeline 次序取下一 stage 的 harness 与 `phase:<phase>`（如 Implement→`qa` / `phase:qa`） |
| progress_status | metadata → 推导 | run 轴优先：Succeeded/Closed→done，活动态→in_progress，Failed/Stopped→unknown；无 run 时看 lifecycle（Completed→done，InProgress/Blocked/Repairing→in_progress，其余 unknown）。词表 done/in_progress/unknown 与 v1.6 spine 文档一致 |
| active_run_id | **推导**：最新一行非终态 run（终态=Succeeded/Failed/Closed/Stopped） | 全部终态时 `""` |
| run_status / iteration | `runs` 最新一行 | |
| gate_results | `gate_results` 每 gate_type 最新一行（`GateDao.LatestByType`，按 gate_type 排序） | 对应 Go 字段 `LatestGates`（P0 名），JSON 名即冻结名 `gate_results` |
| artifact_refs | `artifact_refs`（该 feature 全部 run 的指针）+ 实时 stale 判定（§3） | Go 侧另有 `artifact_count` 计数（P0 字段保留） |
| blocking_reason_codes | `feature_lifecycle.reason_codes_json` | |
| updated_at | `feature_lifecycle.updated_at`，缺失时回退最新 run 的 `started_at` | |

**无数据源缺口（诚实清单）**：`phases_completed`、`phase_promoted`、精确的 `phase`/`next_*` 本应由 orchestrator 的 phase controller 在运行时写入（v1.6 靠 `beacon-phase.json`，v2 设计靠 TraceSpine/run metadata）。P1 尚无 orchestrator（P2），故：①已定义 metadata 优先口径——P2 只要把这些键写进 `runs.metadata_json`，快照即原样透出，不改 schema；②在 metadata 缺失时用上表推导填充，推导规则全部写在 `internal/store/spine_derive.go` 注释与本表，不冒充权威数据。其中 `phases_completed` 的推导假设线性推进（到达 stage S 即此前 stage 均完成，按 stage→phase 映射去重），是最弱的一环，P2 接管后应以 metadata 为准。

## 3. artifacts stale 语义（AC-010/015/019）

`store.ArtifactStaleness`：打开 `abs_path` 算 SHA-256 与存储 `content_hash` 比对——文件不存在→`stale=true, stale_reason=file_missing`；不可读→`file_unreadable`；hash 不符→`hash_mismatch`；一致→`stale=false`（无 reason）。任何文件问题都不报错、不中断查询（DB-first）。`spine artifacts` 与快照内 `artifact_refs[]` 用同一视图类型 `ArtifactView`。

## 4. JSON schema 顶层字段清单（AC-065）

- `spine show`：SpineSnapshot 本体，25 个键 = 冻结 21 字段 + `feature_id`、`slug`（别名）、`run_count`、`latest_run`（其中 `ended_at` 本期顺手从 P0 的 `sql.NullString` 对象形修为字符串/null，见 `Run.MarshalJSON`）。
- `spine list`：`{"snapshots": [SpineSnapshot...]}`。
- `spine artifacts`：`{"run_id", "artifact_refs": [ArtifactView...]}`，ArtifactView = artifact_refs 行字段 + `stale`（bool）+ `stale_reason`（可选）。
- `spine gate`：`{"feature_slug", "version", "project_key", "gate_results": [GateResult...]}`。
- `projects register/show`：`{"id", "project_key", "root_path", "name", "registered_at", "current_version", "versions": [{id, project_id, version, docs_root, is_current}]}`。
- `projects list`：`{"projects": [同 show 结构...]}`。
- `--json` 时 stdout 恰好一个 JSON 文档（测试用 Decoder 断言无第二文档）；非 --json 为简洁人读输出。

## 5. 实跑样例（真实二进制，临时库 /tmp/p1-demo，种子数据经 projects register + 直接写九表）

```
$ beacon projects register --path /tmp/p1-demo/my-app --name demo-app --version v2.0.0 --json
{ "id": "proj_a3fc2a683f1808fd6eec2a8f", "project_key": "my-app--6fa94a1eea",
  "root_path": "/tmp/p1-demo/my-app", "current_version": "v2.0.0",
  "versions": [{ "version": "v2.0.0", "docs_root": "docs/beacon/v2.0.0", "is_current": true, ... }] }

$ beacon spine show --project /tmp/p1-demo/my-app --feature demo-feature --json   # 节选
{ "run_id": "run_demo", "project_key": "my-app--6fa94a1eea", "version": "v2.0.0",
  "feature_slug": "demo-feature", "lifecycle_stage": "Qa", "lifecycle_status": "Blocked",
  "phase": "qa", "phases_completed": ["truth", "design", "generate"], "phase_promoted": false,
  "next_harness": "qa", "next_action": "resolve_harness_blocker", "progress_status": "in_progress",
  "active_run_id": "run_demo", "run_status": "Running", "iteration": 2,
  "blocking_reason_codes": ["evidence_missing"],
  "gate_results": [ {qa: pass}, {release: blocked [qa_not_passed]} ],
  "artifact_refs": [ {evidence: stale=false}, {log: stale=true, stale_reason=file_missing} ] }

$ beacon spine gate demo-feature
qa           pass       [] (2026-10-21T02:30:00Z)
release      blocked    [qa_not_passed] (2026-10-21T01:30:00Z)

$ beacon spine artifacts run_demo
evidence   /tmp/p1-demo/evidence.json hash=4062edaf… size=10
log        /tmp/p1-demo/gone.log hash=deadbeef size=0 stale(file_missing)

$ beacon spine show --project /tmp/not-registered --feature x --json   # stderr + exit 1
Error: spine: project not registered: /tmp/not-registered (project_not_registered)
```

`--help` 顶层可见 `spine`、`projects` 两个命令组，六能力占位命令不变；`--version` 仍为 `Beacon CLI v2.0.0-go-dev`。

## 6. 交付物（文件名 + 大小，字节）

| 文件 | 大小 | 内容 |
|---|---|---|
| internal/store/spine.go | 9,793 | SpineSnapshot 全字段结构、Snapshot 构建、List 过滤、ResolveFeature/ListFeatureContexts、LatestByType、GetVersionByID |
| internal/store/spine_derive.go | 7,479 | ResolveRootPath/ProjectKey（v1 等值）、ArtifactView/ArtifactStaleness、phase/next/progress 推导 |
| internal/store/spine_test.go | 10,144 | 5 个 DAO 级行为测试（project_key parity、全字段、DB-first、List 过滤、hash_mismatch） |
| internal/store/dao_rest.go | 5,895 | 移除旧 Spine 子集（迁至 spine.go），保留 Gate/Artifact DAO |
| internal/store/dao_runs.go | 8,540 | 新增 Run.MarshalJSON（ended_at 字符串/null） |
| internal/cli/helpers.go | 2,077 | 只读/写打开（无 DB 文件的空机语义）、printJSON、reason_code 常量 |
| internal/cli/spine.go | 9,985 | spine 四命令 |
| internal/cli/projects.go | 6,685 | projects 三命令 + projectView |
| internal/cli/root.go | 1,866 | 注册 spine/projects 命令组（六能力占位不动） |
| internal/cli/cli_test.go | 15,428 | 5 个 CLI 行为测试（cobra 实跑 + JSON 解析断言） |
| bin/beacon | 14,234,489 | 重新构建的二进制 |
| docs/p1-implementation.md | 本报告 | |

单文件行数上限 500：最大为 internal/cli/cli_test.go 411 行、internal/store/spine.go 294 行，均合规（spine.go 初版 544 行已拆分）。

**二进制体积变化**：P0 的 5,958,207 B → P1 的 14,234,489 B（+8.3M）。原因是 P0 二进制的 CLI 尚未链接 store（P0 报告已注明 SQLite 体积成本在接线时计入），P1 首次把 modernc.org/sqlite 链入 CLI。属预期内一次性成本，后续 phase 不再有同量级跳变。

## 7. 未决项

1. `phases_completed` 推导口径（§2 缺口）待 P2 orchestrator 写 metadata 后切换为权威来源；切换点已预留（metadata 优先），不需要改 schema。
2. `spine show` 无 --feature 且项目多 feature 时按 lifecycle updated_at 最新者选（启发式）；若要冻结为「必须显式 --feature」口径，需走 change 改 AC-017 语义，本期未改。
3. `spine gate --version` 之外未提供 --project 参数（按冻结签名实现）；同 slug 跨项目并存时取当前版本优先的第一条，跨项目消歧待真实场景出现后再议。
4. doctor projection rebuild（AC-011）与 gate 评估写入（AC-013）仍属 P3，未做；orchestrator/TraceSpine 属 P2，未做。
