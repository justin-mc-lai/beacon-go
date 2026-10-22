# P3c 实现报告：真集成（P3 收口片）

> 日期：2026-10-22 ｜ 基线：v2.0.0-go.1（P2 出口）+ P3a/P3b ｜ **本片出口发 v2.0.0-go.2**
> 依据：冻结真相 AC-043/044（codex/subprocess 真集成）、AC-045/046（claude/acp）、AC-099（worktree 全模式）、AC-100（ImplementHarness 不许 subprocess-only 冒充）、AC-102（planner_review）、AC-042（AgentBackend/IterationResult 合同）；口径稿 `scope-charter.md`。
> 写法：按用户规则大白话，必要术语一句话解释。诚实边界逐项写明，不冒充。

---

## 1. 做了什么（大白话）

以前 goal 的每一步都是「记账员」在纸上写「这步干完了」（占位记录器）。这片把真干活的接上了：

- **真执行器**：给一条外部命令，goal 真的把它当子进程跑起来干活，输出留档、退出码记账，失败就走原来的重试/卡住流程，绝不吞掉失败装成功。
- **两种风格的 agent 配置（profile）**：codex 风格、claude 风格只是「命令模板 + 一份上下文文件」的规矩，没有任何厂商私有协议写死在代码里；换个同形状的脚本照样跑。
- **ralph 真循环**：外包写代码时，命令一轮一轮真跑，直到它自己喊「干完了」（约定见第 3 节），或失败、或轮数到顶——到顶没喊完就是没干完，不许冒充。
- **真工位**：并行计划真建 git 独立工位（一个湖一个目录一个分支），合并只许卫星往主工位合，冲突就停下来报文件名，清理时有没合并的活默认拒绝删。
- **多人评审真跑**：几个评审命令真跑一遍计划，各自给结论，一个说不行（且说得出理由）就整体不行；评审自己出故障的算无效，永远不当通过。
- **ACP 只做到探针**：能和 ACP 目标握个手问出协议版本，到此为止——结论和缺什么写在第 6 节，没有任何地方写「支持 ACP」。

---

## 2. 逐项落地

| # | 项 | 落地 | 关键文件 |
|---|---|---|---|
| 1 | Subprocess 执行器后端（AC-044） | 后端三件套：`harness.SubprocessBackend` 实现 AC-042 合同（iterate/health_check/abort/name）；编排侧 `SubprocessExecutor` 实现 StageExecutor。命令以**参数数组**执行（`exec.CommandContext`，永远不拼 shell 字符串），超时杀**整个进程组**（只杀直接子进程会留下孙子进程堵住输出管道——实测踩过并修了）；stdout/stderr 落盘为产物文件登记 `artifact_refs`（真 sha256+大小）；`stage_executed` 事件带**真实退出码**；非 0 → failed 进 P2a 的 repair/blocker；命令不存在/不可执行/未配置 → typed error fail-closed（`executor_command_missing` / `executor_command_not_executable` / `executor_not_configured`），且**未配置时绝不回退到 recording 装真** | `internal/harness/subprocess.go`、`internal/orchestrator/exec_subprocess.go` |
| 2 | Agent profile 执行器（ImplementHarness 面，AC-100/045 形状） | `Profile{名字+命令模板}`，模板占位 `{context}/{prompt}/{workdir}/{iteration}/{stage}`；内置 `codex-style`、`claude-style` 两份**纯模板**（默认 argv 是 `codex exec …` / `claude -p …` 加 `--context-file {context}`，无厂商协议代码）。执行前真写 **stage-context 文件**（需求包路径、stage plan、implement 时的准入结论、验收/回滚规矩），路径经 `{context}` + `$BEACON_CONTEXT_FILE` 双路传给命令；cwd 与绑定工位对齐（第 4 项）。这正是它与裸 subprocess 的区别：有上下文文件、有工位对齐，所以裸 subprocess 不能冒充 harness。`goal tick/driver --executor` 支持 `subprocess:<profile>`；未配置命令时明确报 `executor_not_configured` | `internal/harness/profile.go`、`internal/orchestrator/exec_subprocess.go`、`internal/orchestrator/exec_resolve.go` |
| 3 | Ralph 真循环（G-10，AC-043 多轮形状） | `harness.RalphBackend.RunLoop` + `RalphExecutor`：每轮传 iteration 号、`$BEACON_CONTEXT_FILE` 指向上一轮结果文件（prev-result.json）、轮间刷新 `.beacon-ralph/state.json`；**每轮**写真实 `stage_executed` 事件 + 一个 store checkpoint（`ralph-iteration-N`）；`goal delegate-ralph` 在配置了 `--ralph-cmd` 时走真循环（delegation 与一切输出 `simulated=false`），收活仍走 P2c 的 `ReceiveRalphResult`（产物+证据齐全才收，不合格照拒——P2c 的拒绝测试原样有效，本片另有真循环全程收活测试）；模拟路径 `--simulate` 原样保留且处处标 simulated | `internal/harness/ralph.go`、`internal/orchestrator/exec_ralph.go`、`internal/orchestrator/exec_delegate.go` |
| 4 | Git worktree 真集成（AC-099） | 三种模式全落：`worktree`（池模式：每湖 `git worktree add -b beacon-run/<run-id>/<lake>`，目录在 `<项目>/.beacon-worktrees/<lake>`）、`run-branch`（根目录切到 `beacon-run/<run-id>` 分支）、`current-branch`（根目录当前分支记账）；`current-branch + --isolated-worktree` 非法组合 fail-closed（`worktree_invalid_combination`）。合并只许 satellite→main_star（反向/卫星互合在调 git 前拒绝，`worktree_merge_direction_forbidden`）；冲突 → `git merge --abort` 停住并报冲突文件（`worktree_merge_conflict`），不强推不自动解；主工位脏默认拒绝建工位（`worktree_main_dirty`），`--allow-dirty` 例外且记进创建事件；清理拒绝未提交/未合并的卫星（`worktree_unmerged_changes`），`--discard` 才删且记进移除事件；非 git 仓库 fail-closed。CLI：`goal worktree create/merge/cleanup/status` | `internal/harness/git.go`、`internal/orchestrator/worktree.go` |
| 5 | planner_review 最小真实现（AC-102） | `goal planner-review`：N 个评审位（`--reviewer-cmd` 可重复 = N 条 lane，或 `goal run --reviewer-cmd` 记录一条）各跑一个真 profile 子进程，须在 stdout 打印一行 JSON 结论；聚合：任一 block 且理由非空 → 总评 block（`planner_review_gate_blocked`）；block 无理由 = 无效（不是否决也不是通过）；超时/非 0/无可解析结论 = 无效（`planner_reviewer_invalid`），**无效永远不当 pass**；零有效 lane → 不许 pass。输出 AC-102 同构字段（`reviewer_lanes`/`findings`/`synthesis`/`execution_mode`/`fallback_reason`/`runtime_evidence_only=true`）；聚合写 `gate_results` 一行（`gate_type=planner_review`）+ 事件 + 合成产物登记 `artifact_refs`。**诚实标注**：评审是顺序跑的，`execution_mode=multi_agent_sequential`，不声称 parallel | `internal/orchestrator/planner_review.go` |
| 6 | ACP（AC-046） | **结论：探针级，不算完成 AC-046**。实测成本后做 stdio JSON-RPC `initialize` 握手探针（无新依赖，标准库实现）：发一条 initialize、读回响应、报 protocolVersion / agentCapabilities / agentInfo。`AcpBackend.Iterate` 明确拒绝（`acp_full_backend_not_supported`），CLI 叫 `goal acp-probe`，help 与输出处处写 probe。缺什么、为何列 P4 见第 6 节 | `internal/harness/acp.go` |

配置来源（任务书二选一，本片选定并贯穿全部命令）：**CLI 参数 + run 记录**。`goal run --exec-cmd/--ralph-cmd/--profile/--reviewer-cmd` 把 argv 配置记进 run 快照（`runs.metadata_json.exec`，不新增表）；后续 tick/driver/delegate/planner-review 不带命令时回落到 run 记录，本次调用带了则本次优先。`--exec-cmd` 的值是**一个程序路径**，其余参数走可重复的 `--exec-arg`——没有「一个字符串拼整条命令」的入口，注入面从配置层就关死。执行器选择在 `Engine.ResolveExecutorFor` 一处收口（selector 语法：`recording` | `subprocess` | `subprocess:<profile>` | `ralph`，未知 selector 拒绝）。

**工作目录口径**：执行器 cwd = run 绑定的**活跃真实工位**（自己那个湖的卫星），没有真工位时 = 项目根。计划里的卫星路径（P2c plan ref）本身不算真工位——建了才算。这样 AC-099 的「harness/agent project-root 与 worktree cwd 对齐」有真测试盯着（工位里跑 pwd 的捕获输出就是卫星路径）。

---

## 3. Ralph 完成约定（写死在这里）

一轮算「干完」当且仅当：**命令退出码为 0，且 stdout 里有一行单独是 `RALPH_DONE`**（行首尾空白容忍；`RALPH_DONE-ish`、`echo RALPH_DONE` 这类子串不算）。其他结局：非 0 退出 → 循环立即以失败结束（`executor_exit_nonzero`，不静默重试）；超时 → `executor_timed_out`；跑满上限（默认 8 轮，`--ralph-max-iterations` 可改）没等到标记 → 未完成（`ralph_max_iterations_reached`），stage 判 failed 走 repair/预算，**绝不报 done**。

---

## 4. 实跑样例（真二进制，/tmp/p3c-smoke，独立临时库）

环境：真 git 项目（已提交）+ 冻结 truth 包；feature 行按既往冒烟惯例以 sqlite3 播种（feature 包创建 UI 属 P4 truth 面）。脚本均为真脚本。

```
$ beacon projects register --path /tmp/p3c-smoke/proj --name smoke --version v2.0.0 --docs-root docs
$ beacon goal run "ship it for real" --project .../proj --feature feat-a --executor-id host-1 \
    --exec-cmd .../agent.sh --ralph-cmd .../ralph.sh --reviewer-cmd .../reviewer.sh --json
run=run_5c4b215014b4d61f7858ac5e

$ beacon goal tick --run-id $RID --executor subprocess   ×2   # 真 agent.sh 执行 plan、truth 两步
truth     truth_routing      blocked False
implement implement_routing  blocked False

$ beacon goal trace --run-id $RID   # stage_executed 摘录（真实退出码）
stage_executed plan      exit 0 real True executor subprocess:agent.sh
stage_executed truth     exit 0 real True executor subprocess:agent.sh
stage_executed implement exit 0 real True executor ralph:ralph.sh     # 第 1 轮
stage_executed implement exit 0 real True executor ralph:ralph.sh     # 第 2 轮（RALPH_DONE）

$ beacon goal delegate-ralph --run-id $RID --json        # 真循环，2 轮后收活
status returned simulated False

$ beacon goal planner-review --run-id $RID --json
verdict pass mode multi_agent_sequential lanes [('codex-style', 'pass')]

$ beacon goal parallel-plan --run-id $RID --lake feat-a --lake lake-b
mode main_star_satellite_pool planned_only True
$ beacon goal worktree create --run-id $RID --json       # 真建工位
feat-a active beacon-run/run_5c4b215014b4d61f7858ac5e/feat-a /tmp/p3c-smoke/proj/.beacon-worktrees/feat-a
lake-b active beacon-run/run_5c4b215014b4d61f7858ac5e/lake-b /tmp/p3c-smoke/proj/.beacon-worktrees/lake-b
# 卫星里真提交后：
$ beacon goal worktree merge --run-id $RID --lake feat-a --json
feat-a merged / lake-b active；主工位上 merged.txt 出现

$ beacon goal acp-probe --target .../fake-acp.py --json  # 真握手
{"probe": true, "responded": true, "protocol_version": 1,
 "has_agent_capabilities": true, "implementation_name": "fake-agent",
 "full_backend": false,
 "detail": "initialize handshake answered; session/new and session/prompt are NOT implemented (probe only)"}

$ beacon --version
Beacon CLI v2.0.0-go.2
```

---

## 5. P3 收口总表

| 片 | 内容 | 状态 |
|---|---|---|
| P3a | 四道门正式评估写库（gate_results + 事件）、Truth Integrity Gate（AC-101）、Tick 正式结论优先、入口改名变更登记（Go 侧落账） | ✅ 已收口（128 测试时点） |
| P3b | v1 老数据迁移（幂等、dry-run、映射不了不硬塞）、doctor 投影重建（AC-011）、v1.6.3 免疫 + v1.6.4–1.6.6 对账盘点 | ✅ 已收口（135 测试时点） |
| P3c | subprocess 后端（AC-044）、profile/harness 面（AC-100，AC-045 形状）、ralph 真循环（G-10 / AC-043 多轮形状）、worktree 全模式（AC-099）、planner_review（AC-102） | ✅ 本片 |
| P3c | ACP（AC-046） | ⚠️ **探针级，未完成**：initialize 握手真实现；session 迭代未做，列 P4（第 6 节）。不声称支持 ACP |
| P3c | AC-043/045 的厂商本体（真 codex / 真 claude 二进制多轮） | ⚠️ **形状完成、本体未验**：profile 模板与多轮循环都是真的，但本机没有 codex/claude 二进制，未对厂商本体做过一次真迭代——不声称 AC-043/045 已达成；用同形状脚本的真集成已全覆盖 |

**测试改动声明（既有测试）**：只改了 **1 处**——`internal/cli/goal_p2d_test.go` 的版本断言 `v2.0.0-go.1` → `v2.0.0-go.2`。理由：该断言钉的是「当前发布版本」，P3 出口按维护约定升版（与 P2d 当时把断言从 dev 改到 go.1 同理）；断言本体（--version 必须报当前版）不变。其余既有测试零改动、零回归。

**过程中修的两个真问题**（不是测试迁就实现，是实现修对）：
1. 超时只杀直接子进程：agent 派生的孙子进程（如 `sleep`）会占住输出管道，Run 在超时后仍阻塞到孙子自然结束（实测 5 秒）。修为进程组击杀（Setpgid + group SIGKILL）+ WaitDelay 兜底，超时测试从 5.2s 降到 0.6s 级。
2. 工位容器目录让主工位永远「脏」：卫星建在项目内 `.beacon-worktrees/` 后，主工位 `git status` 恒有未跟踪项，第二次 create 会被自己的脏检查拒绝。修为脏检查以 pathspec 排除 beacon 的两个保留目录（`.beacon-worktrees`、`.beacon`），其余与 git status 完全一致；该口径写在 `harness.MainDirty` 的注释里。

---

## 6. ACP 实测结论（为什么是探针、P4 缺什么）

实测：initialize 握手用标准库（bufio + encoding/json + 子进程 stdio）即可完成，零新依赖，探针已落地并有真测试（假 agent 真应答、垃圾目标不误判、缺命令 fail-closed）。

但 AC-046 要的是**一次真 integration iteration**（session/new + session/prompt 跑完一轮并产出 AC-042 形状结果），这需要：会话生命周期管理（session id、取消通知、权限请求往返——ACP 里 agent 会反向调 client 的 fs/terminal/permission 方法）、把 prompt 结果映射回 IterationResult 的语义、以及一个**真 ACP agent 目标**做集成验证（本机没有）。在没有真目标的情况下把 session 层写出来只能靠自造假目标自证，正是冻结口径禁止的空壳验收。故本片只做探针，P4 候选写明前置条件：先有可用的 ACP agent 目标（或官方参考实现），再补 session 层 + 对真目标的集成测试。

---

## 7. 留 P4 清单

P4 主线（口径稿第 5 节）：六能力面收敛 + 统一评分（skill-score 只评不改）+ truth 命令面补齐。以下为累积欠账与候选，逐项有出处：

1. **ACP 完整后端（AC-046）**：session/new、session/prompt、权限/fs/terminal 反向请求处理、对真 ACP 目标的集成迭代。前置：可用的 ACP agent 目标。本片探针是它的健康检查底座。
2. **AC-043/045 厂商本体验证**：在本机装上真 codex / claude 二进制后，各跑一次真多轮迭代（AC-043 要 ≥2 次 iterate 或触达上限），把证据补进集成测试；profile 模板如与真二进制 argv 不符，只改模板不改代码结构。
3. **P3b 免疫对账未覆盖/部分项**（出处：`docs/p3b-implementation.md` 第 7 节）：post-release hook 的 agent 覆盖循环、spec fingerprint 面、物化（materialize）面、runtime↔docs 版本漂移完整守卫、qa-methodology-contract、loop-pattern-registry、harness-remediation-sandbox、qa-chrome-extension-e2e、AC-077/078 账本库真出现时的导入解析器。protected-path-denylist 本片**已部分落点**（执行器 argv-only + worktree 方向/清理护栏），其「受保护路径清单」本体仍未迁，保留在本项。
4. **planner_review 并行化与 fallback**：v1.6 runtime 有 reviewer lanes 并行 + synthesis fallback；本片是顺序执行 + 无 fallback（`execution_mode` 已如实标注）。P4 若做并行，`execution_mode` 才能改称 parallel。
5. **入口改名 gen-change 的上游合并**：Go 侧变更记录已落账（P3a），源仓冻结包仍待其下一次正式变更时合并（源仓只读，非本轨可办）。
6. **G-01 entry-contract、G-08 resume-pack 独立子命令**：P2 收口时标的部分落地项，功能在现有命令内，独立子面是否补由 P4 六能力面收敛时一并定（受防长胖条款约束，须声明替代谁）。

---

## 8. 验证

- `go build ./...` / `go vet ./...` / `gofmt -l`（无输出）全净。
- `go test ./... -count=1` **159 个顶层测试全过**（计数口径：`go test -v` 的顶层 `--- PASS` 行数，本次实测；本片新增 31：harness 13 + orchestrator 17 + cli 1），0 失败；既有测试除第 5 节声明的 1 处版本断言外零改动、无删除。
- 任务书覆盖点逐项有真测试：subprocess 成功/非 0/超时/命令不存在四路、产物登记与哈希、失败接入 blocker、profile context 文件内容、执行器 cwd 落在真卫星工位、ralph 两轮完成/达上限未完成、delegate 真循环收活全程、无证据收活照拒、worktree 建/合并方向拒绝/冲突停住/脏主工位拒绝/未合并清理拒绝/非 git 拒绝/非法组合拒绝、planner-review 一 block 全 block + invalid 不当 pass + 空理由 block 判无效、ACP 探针真握手 + Iterate 诚实拒绝。
- 真二进制冒烟：第 4 节（独立临时库 + 真 git 项目 + 真脚本），`--version` 实跑 `Beacon CLI v2.0.0-go.2`。
- 边界：冻结 schema 九表结构零改动；状态只进 store（run 快照 + 事件 + checkpoints + gate_results + artifact_refs），项目侧落盘只有证据文件（捕获流、stage-context、worktree）且全部登记指针；Python 源仓未动（`git status` 干净）。

## 9. 交付物（文件名 + 大小）

- 报告：`docs/p3c-implementation.md`（本文件）
- 二进制：`bin/beacon` **15,813,370 字节**（P3b 时点 15,334,991），版本 `v2.0.0-go.2`
- 新增源文件（均 <500 行）：`internal/harness/backend.go` 60、`subprocess.go` 250、`profile.go` 201、`ralph.go` 206、`git.go` 196、`acp.go` 234、`harness_test.go` 245；`internal/orchestrator/exec_subprocess.go` 302、`exec_ralph.go` 98、`exec_delegate.go` 132、`exec_resolve.go` 185、`worktree.go` 367、`planner_review.go` 336、`meta_p3c.go` 102、`p3c_test.go`、`p3c_worktree_test.go`；`internal/cli/goal_p3c.go` 382、`goal_p3c_test.go`
- 改动文件：`internal/core/reasons.go`（P3c 原因码家族 + IsKnownReasonCode 前缀）、`internal/orchestrator/meta.go`（runMeta.Exec）、`meta_p2c.go`（ImplementState 加 Worktrees/PlannerReview）、`engine.go`（CreateRunOptions.Exec）、`orchestrator.go`（包注释同步）、`internal/cli/goal.go`（run 记 Exec、tick 走 ResolveExecutorFor、挂 worktree/planner-review/acp-probe）、`goal_driver.go`（run-once/run-loop 同选路）、`goal_p2c.go`（delegate-ralph 真路径分流）、`root.go`（版本 go.2）、`internal/cli/goal_p2d_test.go`（版本断言，第 5 节已声明）
