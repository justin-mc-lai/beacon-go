# Beacon Go 重构 · P6 验收收尾报告（v2.0.0-go.4）

> 日期：2026-10-22 ｜ 状态：P6 出口（有条件通过——验收工作全部执行完毕，但判据未全绿，缺口逐项列在第 6 节，不藏）
> 版本：`Beacon CLI v2.0.0-go.4`（二进制实跑确认）；`go build/vet/gofmt` 全净；`go test ./... -count=1` **199 个顶层测试全过**（与 P4 出口同数，零回归；唯一既有测试改动 = `goal_p2d_test.go` 版本断言 go.3→go.4，随升版例行改，已声明）

---

## 1. 性能门禁 benchmark（AC-051..058）

产物：[bench/run_bench.sh](../bench/run_bench.sh)（可复跑）+ [bench/results.json](../bench/results.json)（原始 samples 全归档）。种子库 = 源仓经 `migrate v1` 导入的 128 包 / 511 产物指针 / 24 版本真实规模库，特意放在 home 卷（与默认库同卷，btrfs），没有用 tmpfs 美化数字。计时含进程启动开销；读项 n=50、写项 n=30，中位 + nearest-rank p95。阈值为冻结原文数字（原为 Rust 口径，Go 版沿用不改）。

| AC | 测什么 | median | p95 | 阈值 | 结果 | v1 对照（同口径） |
|---|---|---|---|---|---|---|
| AC-051 | CLI 冷启动 `--version` | 3.88ms | 4.70ms | p95<100ms | ✅ PASS | v1 median 529ms / p95 591ms（按同阈值 FAIL） |
| AC-052 | `spine show` 单包 | 9.38ms | 17.61ms | p95<50ms | ✅ PASS | v1 无 spine 面；最接近的 status 命令 p95 约 2027ms（仅参考） |
| AC-053 | 包解析（Go 无 truth-map 命令，按口径稿并入 truth 面，以 `truth show` 为等价口径） | 9.02ms | 11.86ms | p95<200ms | ✅ PASS | v1 原面 truth-map resolve p95 633ms（按同阈值 FAIL） |
| AC-054 | Gate 计算（主数 = `gate evaluate qa` 计算+写结论） | 108.71ms | 132.29ms | p95<50ms | ❌ **FAIL** | v1 无正式评估写库面，无对照 |
| AC-055 | 单文件证据 hash 验证（`spine artifacts`） | 7.13ms | 13.12ms | p95<10ms | ❌ **FAIL**（临界） | 无对照 |
| AC-056 | v1→v2 迁移（128 包单次） | 25.2s | — | <60s | ✅ PASS | — |
| AC-057 | orchestrator 一格调度（`goal tick`，recording 执行器，不含 agent） | 150.26ms | 176.22ms | <100ms | ❌ **FAIL** | 无对照 |
| AC-058 | beacon.db 体积（128 包种子库） | 0.41MB | — | 软上限 500MB | ✅ PASS | — |

**三项 FAIL 的根因（同一条）**：写路径（gate evaluate、tick）每步多次事务落盘，本机 home 卷（btrfs）实测 fsync 约 18ms/次；只读面全部达标（gate status p95 8.35ms、纯 sha256 约 1.3ms），计算本身不是瓶颈。探针期把库放 /tmp（tmpfs）时 tick 约 74ms、迁移约 0.7s，但默认库不在 tmpfs，故不采信。AC-055 为临界抖动：首轮 p95 8.41ms PASS、归档轮 13.12ms FAIL，按归档轮如实记 FAIL。修复方向（本期未做）：写路径事务合并/批处理、WAL 同步模式调优——属实现优化，不动阈值。

读路径总评：Go 版三项可对照指标约为 v1 的 1/50–1/200。

## 2. 端到端全链路 E2E

产物：[e2e/run_e2e.sh](../e2e/run_e2e.sh)（`set -euo pipefail`，退出码 0=全通 / 1=意外失败 / 3=停在已知缺口）+ [e2e/last_run.log](../e2e/last_run.log)（2165 行全文）。剧本 11 步：register → truth create → 填四件套 → freeze → gate truth_freeze → goal run（subprocess 真脚本）→ admit → driver run-loop → gate 四门 → attach-verifier + complete → spine/trace 读回。全程独立临时库与临时 git 项目，默认库未动。

**结果：1–9 步全通，停在第 10 步，退出码 3。** 关键实测：冻结一次过预检；driver 4 轮真 subprocess 执行（plan/truth/implement/qa 各一次 `stage_executed`）推到 release_gate 并停在 awaiting_human；design 门以 not-applicable pass、implement 门 pass、release 门如预期 blocked（缺人工确认）。第 10 步 attach-verifier 成功，但 **complete 被拒：`qa_gate_not_passed`**。

根因是既有实现缺口（非脚本问题）：qa 阶段真执行了、报告文件真落盘，但 **CLI 面没有任何机制能给一个 goal run 登记带正确哈希的 QA 类产物指针**——执行器只登记固定 kind 的捕获（stdout/stderr 等），唯一的产物登记面 `goal receive-result` 属 implement 委托路径、且存入的 ContentHash 为空，必然被判 stale。于是正式 qa 门判 `stage_exit_qa_evidence_missing` blocked，全链无法经正式门走完。脚本已按未来兼容写好：补上 QA 产物登记面后同一脚本无需改动即可跑到退出 0。**E2E 判据：未全绿。**

## 3. AC 全矩阵（evaluator 终表）

终表：[ac-matrix.md](ac-matrix.md)（171 行逐条，分片原文 [part1](ac-matrix-part1.md) / [part2](ac-matrix-part2.md) 保留备查）。统计与逐行加总一致（自查通过）：

| 分类 | 条数 | 说明 |
|---|---|---|
| tested | 71 | 有行为级测试或 benchmark 实际断言（含 AC-051..058 的 bench 归档，3 项为「已测且 FAIL」） |
| implemented | 67 | 代码已实现、无专门测试（落点逐行写明） |
| deferred-external | 21 | 依赖外部目标，前置条件逐行写明（见第 4 节） |
| dropped-by-charter | 12 | 按口径稿拍板不迁（双轨叙事、labs 等，依据逐行写明） |
| **合计** | **171** | AC-001..173，缺 145/146（TC 编号） |

诚实注记：分片中有 61 条标「待复核」（分类者拿不准、按保守档落位），终表原样保留标记；其中 AC-136..142（标准流集群）与 AC-171..173（GitScopeGraph/topology）标在 dropped 一档但依据是「口径稿模型未包含」而非明文 drop，本报告在第 6 节把它们作为真实缺口单列，不当作已结案。

## 4. 外部验证项终局（逐项实测前置条件，2026-10-22 本机）

| 项 | 前置实测 | 终局 |
|---|---|---|
| ACP 真迭代（session/prompt） | 本机无任何 ACP agent 二进制（claude-code-acp / codex-acp / zed 均 NOT FOUND） | **待外部条件**：装一个真 ACP agent 并指向 `acp-probe` 之外的迭代路径；探针现到 initialize→session/new 两步 |
| 真 codex / claude 二进制多轮 | `codex` 在本机（codex-cli 0.164.0-alpha.9）但 `codex login status` = Not logged in；`claude` 未安装 | **待外部条件**：codex 需用户账号登录授权、claude 需安装+登录；profile 指向方法已在 P4c 报告第 5 节，形状级替身验证已过，不冒充本体已验 |
| Chrome 扩展 e2e | PATH 内无 chrome/chromium， 无待验扩展包 | **待外部条件**：带扩展加载能力的浏览器宿主 + 扩展包 |
| loop token 账本 | harness 中 Subprocess 后端按设计 `token_usage=0`（子进程无用量回报通道） | **待外部条件**：执行器/宿主在阶段证据里回报真实 token 用量的通道出现后才可建账 |

四项全部结案为「已知的未竟项」，无一冒充完成。

## 5. 整个重构总收口（P0–P6）

| 期 | 一句话 |
|---|---|
| Phase 0 | 盘点 77 命令、立 Python 行为基线、Go 骨架与 SQLite 选型（纯 Go 单二进制） |
| P0 | core 类型层（双状态机轴、typed error、证据哈希链）+ store 九表六 DAO， writer-track 隔离 |
| P1 | spine 查询面 + projects 注册表（DB-first，只读） |
| P2（go.1） | goal 22 条工作流全量落地：引擎/长跑看护/外包并行/编排扩展，首个稳定版 |
| P3（go.2） | 四门正式评估写库、v1 数据幂等迁移 + doctor 对账、真集成（subprocess/ralph/worktree/多评审） |
| P4（go.3） | truth 能力补真、FrozenFact 共享判定、统一评分（只评不改）、25 项剩余终局处置 |
| P6（go.4） | 本期：benchmark 门禁建成（5/8 达标）、AC 全矩阵终局、E2E 剧本建成（停步第 10 步）、外部验证逐项实测结案 |

**最终交付物**：二进制 `bin/beacon` 16,076,832 B（单文件、免运行时）；源码 `internal/{core,store,gate,orchestrator,harness,migration,cli}`；文档 `docs/`（口径稿、P0–P6 各期报告、AC 终表、变更记录）；基准 `bench/`（脚本+结果+种子库）；E2E `e2e/`（脚本+全文日志）。测试 199 个顶层全过。

**怎么用（速览）**
- 装哪：`~/workspace/beacon-go/bin/beacon` 就是全部，拷到任何 linux/amd64 机器可跑；数据在 `~/.beacon/beacon.db`（可用 `BEACON_STORE_PATH` 改位置），需求真相在各项目 `docs/beacon/<版本>/features/<slug>/` 的 Markdown 里。
- 五条最常用：`beacon projects register --path <项目>` 建项目；`beacon truth create/freeze` 建包冻需求；`beacon goal run "一句话" --project <项目>` 起任务；`beacon goal driver run-loop` 长跑推进（到发布门自动停）；`beacon spine show` 看全局状态。
- 与旧版的关系：旧 Python 版原样保留在源仓、只读未动；Go 版是独立轨，旧项目数据用 `beacon migrate v1 --path <旧项目>` 一次性导入（幂等）。两轨不混写（store 有 writer-track 守卫）。

## 6. 已知缺口全集（不报喜的清单）

1. **E2E 未全绿**：QA 产物登记面缺失（第 2 节），正式 qa 门下全链走不完——这是当前第一优先级的真实功能缺口。
2. **benchmark 3 项 FAIL**：AC-054（evaluate 写路径 p95 132ms）、AC-057（tick p95 176ms）、AC-055（临界 13.12ms）；根因写事务 fsync，未做事务合并优化，阈值未动。
3. **标准流集群未实现**（AC-136..142）：canonical branch 校验、truth baseline commit 绑定、refreeze 漂移检测、五类 verification bundle——只有相邻部分落地。
4. **拓扑集群未迁**（AC-171..173）：GitScopeGraph / topology gate 整组未实现；矩阵中暂置 dropped（依据为口径稿模型未含，非明文拍板），应视为待用户定夺的缺口而非结案。
5. **内核快照类未实现**（AC-111..114 等）：intent/prompt snapshot ledger、decision packet 在 Go 侧无落点。
6. **skill 包未嵌入**（AC-125..127）：六个 SKILL.md 未随二进制打包，`skills/` 仍是占位；progressive 加载的例子面未落地。
7. **杂项**：迁移前自动备份不存在（AC-092）；AC-162 golden parity 语料未建；AC-060/157 二进制命名与选轨面仍未拍板；planner_review 为顺序执行（非并行，已如实标注）；`internal/cli/goal.go` 539 行超「单文件 <500 行」自律线（AC-025 观察项）；store_meta 为九表外的内部表（P0 已记豁免待拍板）；AC-077/078 旧账本解析器为条件触发项（样本至今未出现）。
8. **外部未竟四项**（第 4 节）：ACP 真迭代、真厂商二进制、扩展 e2e、token 账本——均待外部条件，非代码内可办。
9. **非本轨可办**：入口改名 goal 的变更记录已在 Go 侧落账，上游冻结包合并待源仓下一次正式变更。

## 7. 验证与交付物

- `go build ./...` / `go vet ./...` / `gofmt -l .`（无输出）/ `go test ./... -count=1`：7 个包全 ok，199 个顶层测试全过、零回归（版本断言 1 处随升版改，已声明；另 `internal/harness/acp.go` 探针 clientInfo 版本字面同步为 2.0.0-go.4 并注明手工同步原因——harness 不能反向 import cli）。
- 文件大小：`bin/beacon` 16,076,832 B；`docs/p6-implementation.md`（本文件）；`docs/ac-matrix.md` 约 56KB；`bench/run_bench.sh` 约 17KB、`bench/results.json` 约 12KB；`e2e/run_e2e.sh` 约 16KB、`e2e/last_run.log` 约 67KB。
- 边界守住：benchmark/E2E 只用临时库与临时目录，默认库（`~/.beacon` 至今未创建）与 Python 源仓未动；未新增顶层命令；schema 未动。
