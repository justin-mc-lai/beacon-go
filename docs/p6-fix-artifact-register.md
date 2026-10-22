# P6 修复：QA 产物登记面（v2.0.0-go.5）

> 日期：2026-10-22 ｜ 范围：补 artifact 登记写路径，让 P6 E2E 全链 11 步跑通
> 结论先行：**E2E 已全通（exit 0）**，qa 门判定标准一字未改；204 个顶层测试全过（199 既有零回归 + 5 新）。

---

## 1. 缺口与修法一句话

P6 的 E2E 停在第 10 步：qa 阶段的真产物（qa-report.md）落了盘，但系统里没有任何入口能把一个文件登记进 `artifact_refs`（带正确 sha256）。执行器只写自己的捕获类产物（stage_stdout/stage_stderr/stage_context），receive-result 只收 implement 外包的产物。qa 门只认登记过的产物，于是判证据缺失，complete 被拦。

修法：新增登记面 `goal artifacts register/list`（折叠进 goal 组，不新设顶层命令），把「文件 → 带真哈希的产物指针」这条写路径补上。门不放宽，路补通。

## 2. 接口对齐说明（剧本期待 vs 实现）

先读了 `e2e/run_e2e.sh` 全文：**剧本没有写死登记命令的接口**——第 9 步对 qa 门只写了 `any` 期望，第 10 步用一段 GAP 横幅把缺口如实报出（exit 3）。所以登记面按任务书设计，剧本同步改到新接口。改动逐处声明如下：

| # | 剧本改动 | 内容 |
|---|---|---|
| 1 | 头部标题行 | 版本标注 v2.0.0-go.3 → v2.0.0-go.5 |
| 2 | 头部链路说明 | 第 8 步后新增 8b 步说明（登记 QA 报告）；第 9 步说明改为「qa 必须 pass、release 仍 blocked」 |
| 3 | 头部 exit codes | 删除 exit 3（缺口）条目，改为历史注记：go.3/go.4 曾在此停，go.5 已关闭 |
| 4 | 版本检查 | `grep -q "v2.0.0-go.3"` → `v2.0.0-go.5`（含 FAIL 文案） |
| 5 | 新增 STEP 8b | driver 推完、产物验盘后：`goal artifacts register --run-id <R> --kind qa_report --path <proj>/e2e-work/qa-report.md --by <evaluator> --json`，断言 kind=qa_report，并 `goal artifacts list` 读回 |
| 6 | 第 9 步 | `eval_gate qa any`（记录现状）→ `eval_gate qa pass`（登记后必须 pass）；删除随之失效的 `QA_VERDICT` 捕获与 gap 注释；release 的 blocked 理由注释改为只剩「缺人工确认」 |
| 7 | 第 10 步 | 删除整段 GAP 横幅/exit 3 分支（约 40 行）；complete 失败即普通 FAIL |

剧本其余部分（1–8、11 步、隔离方式、判读方式）未动。

## 3. 登记面口径

命令面（`internal/cli/goal_artifacts.go`，判定与记账在 `internal/orchestrator/artifacts.go` 的 `Engine.RegisterArtifact`）：

- `goal artifacts register --run-id <run> --kind <kind> --path <file> --by <登记者> [--json]`
- `goal artifacts list --run-id <run> [--json]`：只读，呈现与 `spine artifacts` 同一份（`store.ViewOf`：stale 标记 + 原因），不复制第二套。

登记动作：

- 文件必须真实存在且是普通文件，不存在/是目录 → `artifact_file_missing` 拒绝；
- 当场流式算 sha256 + size，写 `artifact_refs` 一行（绑定 run_id），并写 `artifact_registered` 事件（载荷含登记者、哈希、产物 id）；
- `--by` 为空 → `artifact_registrar_required`；run 不存在 → `product_goal_run_not_found`；
- kind 受控词表（`ValidRegisterKind`）：QA 家族与 qa 门的 `qaKind()` **逐字对齐**（`qa`、`qa_*` 前缀、`scorecard`、`test_result`、`test_results`），外加通用类 `evidence`、`implement_artifact`、`report`、`log`、`artifact`；表外一律 `artifact_kind_not_allowed`。执行器专属的捕获类（stage_stdout/stage_stderr/stage_context）**不在词表内**——手工登记不能冒充内核捕获；
- 幂等：同一 run+kind+path 且哈希未变 → 返回既有行并标 `already_registered=true`，不重复造行、不重复写事件；
- 内容变更：同 run+kind+path、哈希已变 → 新增一行，返回 `supersedes=<旧行 id>`，旧行不删不改（历史完整，旧行在 list 里会如实显示 stale/hash_mismatch）。

**信任边界（重要）**：登记只记账，不判好坏——它是登记者的一份声明：「这个文件是这个 run 的产物」，附登记时的哈希。防线不在登记处，在门：每次 gate 评估都按登记哈希重算文件（`store.ArtifactStaleness`），文件被篡改/替换/删除立刻转 stale 并拦。也就是说，登记能作弊的只有「登记一个假文件」，但假文件的内容质量仍由 qa 门背后的评审与证据链负责，与内核自写指针的信任等级相同（内核捕获同样只证明「当时跑出了这份字节」）。

## 4. E2E 全通证据

原样重跑 `bash e2e/run_e2e.sh`（隔离临时库 + 临时项目）：

```
E2E exit=0
E2E RESULT: PASS — full chain completed (run run_c965b982b07804b46e101056 done)
```

关键段（全文 2,432 行在 `e2e/last_run.log`）：

- STEP 8b 登记成功：`art_d83b208ce15ea0eff60d2234`，kind=qa_report，content_hash=`06d88e07…b2421`，size=222；
- STEP 9：`gate qa verdict=pass`；release 仍 `blocked`（只缺人工确认，符合剧本期望）；
- STEP 10：attach-verifier 后 `complete cli_rc=0`、`run completed (loop_stage=done)`；
- STEP 11：spine show/list、goal trace、goal status 读回全为 done。

## 5. 测试清单（新增 5 个顶层测试）

orchestrator（`artifacts_test.go`）：

- `TestRegisterArtifactBasics`：哈希/大小正确、行落库、事件含登记者+哈希、list 显示非 stale；
- `TestRegisterArtifactRefusals`：文件不存在、路径是目录、词表外 kind、冒充内核 kind（stage_stdout）、登记者为空、run 不存在——六路各带稳定 reason_code 且零写入；
- `TestRegisterArtifactIdempotentAndSupersede`：同哈希重登记幂等（同一 id、无新行）；改内容后新行 + supersedes 指旧行，旧行转 stale(hash_mismatch)、新行为 fresh；
- `TestRegisterArtifactQAGateFlow`（门禁联动全程）：执行了 qa 阶段但未登记 → qa 门 blocked（原有行为不变）；经登记面登记 → pass；登记后篡改文件 → blocked 且首位 reason_code 为 `qa_artifact_stale`。

cli（`goal_artifacts_test.go`）：

- `TestGoalArtifactsRegisterAndList`：命令面 register/list JSON 形状、哈希为 64 位 sha256、CLI 层幂等、缺文件/坏 kind/未知 run 三路错误码。

全量：`go build/vet/gofmt` 全净，`go test ./... -count=1` **204 个顶层测试全过**（按 `^--- PASS` 计数：199 既有 + 5 新，既有零改动——唯一例外是版本断言 `goal_p2d_test.go` 随升版 go.4 → go.5，按约定声明）。

## 6. 出口

- 版本号：`internal/cli/root.go` → **v2.0.0-go.5**（二进制实跑确认 `Beacon CLI v2.0.0-go.5`），定位为 E2E 修复版；
- 边界守住：qa 门判定标准零改动（`internal/gate/` 未动）、schema 零改动、无新顶层命令、Python 源仓未动；
- 交付文件：`internal/orchestrator/artifacts.go` 6,638 B、`internal/orchestrator/artifacts_test.go` 6,634 B、`internal/cli/goal_artifacts.go` 4,336 B、`internal/cli/goal_artifacts_test.go` 2,858 B、`e2e/run_e2e.sh`（改后）15,136 B、本报告；
- 二进制：`bin/beacon` 16,098,972 B（go.4 为 16,076,832 B）。
