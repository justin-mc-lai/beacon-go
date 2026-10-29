# 外部条件依赖表（deferred-external 全集）

> 日期：2026-10-25 ｜ 仓：`beacon-go` ｜ HEAD：`d9326f4`
> 用途：把「不是代码没写，而是外部条件没到位」的验收项集中成一张可复跑的表。每条都给出前置条件、验证命令、阻塞对象、优先级与责任角色，避免对外引用时口径漂移。
> 计数口径：**AC 级 deferred-external 12 条**（来自 `docs/ac-matrix.md` 逐行分类列）＋**外部验证项 4 条**（来自 `docs/p6-implementation.md` §4）。2026-10-29 已把纯代码缺口 AC-092（迁移备份）、AC-095（benchmark 回退门禁）、AC-127/AC-144（route examples）落成代码并用测试收口，从本表移除；AC-105 的 SIGINT 部分已落地，整条仍因厂商二进制多轮留表。AC-091 已于 2026-10-25 改成 tested。

## 优先级口径

| 级别 | 含义 |
|---|---|
| P1 | 造成不可逆数据损失，或直接阻断当前交付 |
| P2 | 阻断对外可复现的验收口径（CI 证据、benchmark 门禁、厂商真集成、迁移样本、选轨面） |
| P3 | 未来能力或条件触发项；外部样本/工具出现前无动作 |

## A. AC 级 deferred-external（16 条）

| AC | 摘要 | 前置条件 | 验证命令 | 阻塞对象 | 优先级 | 责任角色 |
|---|---|---|---|---|---|---|
| AC-009 | 禁止 loop-runs.db 作 v2 权威，v1 ledger 须导入后只读归档 | 真实 `loop-runs.db` 出现以探明 schema | `find . -name 'loop-runs.db' -o -name 'runtime-ledger.sqlite3'`（当前无输出） | 旧账本导入的权威边界结论 | P3 | Beacon 研发 |
| AC-039 | v1 `evidence-signature-chain.json` 可迁移为 v2 记录 | 真实 v1 签名链文件样本，以定形态与字段 | `grep -rl "evidence-signature-chain" internal/`（当前无输出，仅 docs 提及） | 历史证据链迁移 | P2 | Beacon 研发 |
| AC-043 | CodexBackend 真多轮集成（同一 run ≥2 次 iterate） | 已登录的 codex 二进制 + 真多轮集成证据入库 | `codex --version; codex login status`；`grep -rn "func Test.*Codex" --include=*_test.go . \| wc -l`（当前 0） | 厂商本体多轮能力声明 | P2 | 用户（账号）+ Beacon 研发 |
| AC-045 | ClaudeBackend 完成 1 次真集成迭代（health_check 与 abort 有证据） | 已安装并登录的 claude 二进制 + 集成证据入库 | `claude --version`；`grep -rn "func Test.*Claude" --include=*_test.go . \| wc -l`（当前 0） | 厂商本体单轮能力声明 | P2 | 用户（登录）+ Beacon 研发 |
| AC-046 | AcpBackend 完成 1 次真集成迭代 | 真实 ACP agent 目标（session/prompt 迭代路径） | `grep -rn "acp_full_backend_not_supported" internal/`（当前仅探针级拒绝） | ACP 从探针级升为真后端 | P2 | 用户（装 ACP agent）+ Beacon 研发 |
| AC-077 | migration `v1_runtime_ledger` 自 `runtime-ledger.sqlite3` 导入 | 真实 `runtime-ledger.sqlite3` 出现 | `test -f ~/.beacon/runtime-ledger/runtime-ledger.sqlite3; echo $?`（当前 1） | v1 运行态账本导入正确性 | P2 | Beacon 研发 |
| AC-078 | migration `v1_loop_ledger` 扫描各项目 `loop-runs.db` 一次性导入 | 真实 `loop-runs.db` 出现 | `grep -n "LedgerSource\|scanLedgers" internal/migration/scan.go`（仅探测存在性） | v1 长跑账本导入正确性 | P2 | Beacon 研发 |
| AC-111 | truth pre-freeze 生成 `intent_snapshot.v1`（七字段） | 字段集与落盘/落库形态先定（Go 侧无该产物） | `grep -rn "intent_snapshot" internal/`（当前仅 gate/orchestrator 内部快照，非 v1 产物） | 意图快照链路 | P3 | Beacon 产品 |
| AC-112 | 每轮 plan/truth/review 保存 prompt snapshot hash 并只消费 latest accepted | 同 AC-111 前置 | `grep -rn "decision_packet\|prompt_snapshot" internal/ \| wc -l`（当前 0） | 提示快照账本 | P3 | Beacon 产品 |
| AC-114 | loop 启动须读 `decision_packet.v1`，缺失/过期不得进 implement | 同 AC-111 前置 | `grep -rn "decision_packet" internal/ \| wc -l`（当前 0） | implement 准入的第五/第九项 | P3 | Beacon 产品 |
| AC-105 | 真 codex/claude 二进制多轮 | 厂商模型可用 + 多轮集成证据入库（信号处理与进程内 loop 已落地，见矩阵备注） | `go test ./internal/harness/ -run RealIteration -tags=realcli -v` | 厂商本体多轮能力声明 | P2 | 用户（模型）+ Beacon 研发 |
| AC-157 | 双轨安装与 PATH 选轨：不得静默升降级，CI 三矩阵 | 选轨面拍板（独立名或 `--runtime` 旗标）+ CI 三矩阵 | `grep -rn "RuntimeTrack\|runtime_track_unspecified" internal/ \| wc -l`（当前 0） | 安装期选轨语义 | P2 | 用户（拍板）+ Beacon 研发 |

## B. 外部验证项（p6-implementation §4，4 条）

| 项 | 前置条件 | 验证命令 | 阻塞对象 | 优先级 | 责任角色 | 对应 AC |
|---|---|---|---|---|---|---|
| ACP 真迭代（session/prompt） | 装一个真 ACP agent 并指向迭代路径 | `grep -rn "acp_full_backend_not_supported" internal/` | ACP 从两步探针升为真迭代 | P2 | 用户 + Beacon 研发 | AC-046 |
| 真 codex / claude 二进制多轮 | 账号可用 + 多轮集成证据入库 | `codex login status; claude --version` | 厂商执行器本体已验的说法 | P2 | 用户 + Beacon 研发 | AC-043 / AC-045 / AC-105 |
| Chrome 扩展 e2e | 带扩展加载能力的浏览器宿主 + 扩展包 | `command -v google-chrome \|\| command -v chromium`（当前 MISSING） | 扩展面 e2e | P3 | 用户 + Beacon 测试 | 矩阵无对应行 |
| loop token 账本 | 执行器/宿主在阶段证据里回报真实 token 用量 | `grep -rn "TokenUsage" internal/`（字段存在，子进程恒 0） | 真实成本归集 | P3 | 用户（回报通道）+ Beacon 研发 | AC-120 的结转项 |

## C. 与 `docs/ac-matrix.md` 的双向对齐

计数复跑（应输出 `171` 行与四类分布）：

```bash
awk -F'|' '/^\| AC-[0-9]/{s=$4; gsub(/ /,"",s); c[s]++; n++} END{for(k in c) printf "%s %d\n", k, c[k]; print "ROWS", n}' docs/ac-matrix.md
# 实测：deferred-external 16 ｜ dropped-by-charter 9 ｜ implemented 64 ｜ tested 82 ｜ ROWS 171
```

本表 A 段的 12 条与矩阵分类列**逐条对应、无增无减**：

```
AC-009 AC-039 AC-043 AC-045 AC-046 AC-077 AC-078
AC-105 AC-111 AC-112 AC-114 AC-157
```

### C.1 矩阵备注已过期（需 Beacon 产品复核，本表未改矩阵）

以下四行描述的外部条件在实测中已变化，本表按**现状**写、矩阵按**原备注**写，两边暂时不一致。改分类属 truth 语义，留给 Beacon 产品定夺：

| AC | 矩阵原备注 | 现状实测 | 结论 |
|---|---|---|---|
| AC-091 | 「仓内无任何 CI 配置」 | 已结案：run `40172116739`（head `7bf5a018`）两个平台 test job 全绿，windows-advisory 失败但为 `continue-on-error` 非门禁 | 分类已改为 tested，行已移出本表 A 段 |
| AC-043 / AC-045 | 「codex 未登录、claude 未安装」 | `codex-cli 0.156.1` 已登录（API key）；`claude 2.1.281` 已安装 | 前置已满足，缺口只剩多轮集成证据 |
| AC-125 / AC-127 / AC-144 | 「SKILL.md 未随二进制打包，skills/ 仍是占位」 | `skills/skills.go` 有 `//go:embed goal plan truth design implement qa release`，7 个 SKILL.md 在盘（`find skills -name '*.md' \| wc -l` = 15） | 嵌入已完成，缺口只剩 route examples |

另：`docs/p6-implementation.md` §6 第 4 条仍写「拓扑集群暂置 dropped」，但矩阵 AC-171..173 现为 tested（S3 已落地，见 `docs/stdflow-s3-implementation.md`），该句亦过期。

## D. 边界

- 本表只记录外部条件，不判定 AC 的最终分类；分类权威仍是 `docs/ac-matrix.md`。
- 验证命令均为本机实跑过的原命令，输出随环境变化；P1 项（AC-092）不依赖外部条件，是纯代码缺口，应最先补。
- 4 条外部验证项与 16 条 AC 有 3 条重叠（见 B 段「对应 AC」列），统计总数 20 = 16 + 4，引用时不要相加成 16 + 4 个独立缺口。
