# P4c 实施报告：剩余清单终局 + P4 出口（v2.0.0-go.3）

> 日期：2026-10-22 ｜ 版本：v2.0.0-go.3（P4 出口）
> 依据：剩余清单权威来源 `p3b-implementation.md` 第 7 节（v1.6.3 免疫 9 项 + v1.6.4–1.6.6 共 16 项）、`p3c-implementation.md` 第 5/7 节留 P4 清单；逐项处置前均回读 v1 原 truth.md 原文，未凭标题猜。
> 写法：大白话，必要术语一句话解释。

## 1. 做了什么（大白话）

P3 收口时留了一张 25 项的欠账单：有些是旧版修过的毛病还没在 Go 版对上，有些是旧版有、Go 版只做了一半的功能。这一片把 25 项逐项翻出来定了终局：**16 项确认已补齐**（其中 5 项是这一片真写了代码补上的，11 项是逐项核查后确认 Go 的结构本来就覆盖或 P3 时已落地）、**2 项结转 P6**（要等真外部目标到位才能验，写清了前置条件）、**7 项按口径拍板结案**（与已拍板的收敛口径冲突或属不迁的面，逐项写了理由）。没有任何一项留「待定」。

这一片真写的五处代码：

1. **受保护路径守门**（第 20 项）：卫星工位往主工位合并之前，先看清这次合并会带进来哪些文件；只要碰到 `.env`、密钥、支付、数据库迁移这类高危路径，当场拦下并告诉该找人走什么路，不许静默放行。
2. **冻结包正文指纹**（第 6 项）：包冻结时给正文算一个指纹记下来；以后 doctor 对账时重算，对不上就报「冻结后正文被绕过流程改过」——只报，永不替人改回去。
3. **失败模式目录**（第 16 项）：旧版总结的 6 种典型坏法（证据过期、真相漂移、修个没完等）做成受治理的数据进了内核，长跑体检报告会按任务实际犯过的错，自动配上这条坏法的症状、成因和修法。
4. **发布后覆盖清单**（第 4 项）：旧版有个真毛病——发布后给本机 agent 装东西时名单写死、漏了几个 agent 还不报错。Go 版的覆盖名单改成从执行器档案注册表实时生成，注册表里有什么就覆盖什么，结构上不可能再漏。
5. **急停开关**（第 19 项的子项）：项目里放一个 `.beacon/loop/pause-all` 文件，后台司机立刻拒绝启动和推进，等人把文件拿走才恢复。

外加 ACP 探针往前走了一步（见第 4 节），版本号发到 **v2.0.0-go.3**，P4 收口。

## 2. 二十五项终局对账表

状态只有三种：**已补齐**（含结构性等价/原已覆盖的核查确认）、**结转 P6**（依赖外部目标，写明前置）、**按拍板结案**（与口径稿已拍板决定冲突或属不迁面，写明理由）。

### 2.1 v1.6.3 免疫 9 项

| # | 项（原口径一句） | P3b 时点 | 终局 | 落点 / 理由 |
|---|---|---|---|---|
| 1 | ecosystem-discovery-sync-drift：发现只扫旧目录、同步不重算 hash、freeze 后镜像过期 | 部分 | **已补齐**（结构性） | Go 没有 `.machine` 镜像这一层：发现/同步 = 迁移扫描 + doctor 对账，自第一天就按新包路径扫 `features/<slug>/`（P3b 实测源仓 128 包全发现）；freeze 由 truth 命令直接写账本状态+事件，没有第二份会过期的镜像。三处成因在 Go 结构里都不存在，无新代码 |
| 2 | freeze-integrity-drift-repair：完整性校验器自身 4 处漂移 | 已覆盖 | **已补齐** | P3a 已按 AC-101 在 Go 侧重写完整性门并带断裂测试，本片无变更 |
| 3 | gate-truth-model-alignment：release 门对所有版本硬套包闭合 | 已覆盖 | **已补齐** | P3a release 门只查四门最新结论+人工确认，不硬套闭合，本片无变更 |
| 4 | post-release-hook-agent-coverage：hook 名单硬编码 (codex, claude) 漏其余 agent 且静默过期 | 未覆盖 | **已补齐**（本片新代码） | self-heal post 的装机步在计划中给出覆盖名单，名单从 harness 档案注册表实时派生（`postReleaseCoverageDetail`），注册表加档案覆盖自动跟随——第二份硬编码名单在结构上不存在。边界如实：Go 的 post 步按既定口径只出计划+覆盖名单、永不自动执行，装机本体由人确认后在宿主侧执行 |
| 5 | runtime-release-alignment：skills 与 runtime 两面撕裂、release 门不查版本一致 | 部分 | **已补齐**（结构性消解） | Go 是单二进制：版本常量编译进二进制，`--version` 即唯一版本面，没有第二个可撕裂的面；项目 docs 版本是账本里的项目数据、不是 CLI 的版本面。版本面检查已在 self-heal 开跑前检查落地（项目恰有一个当前版本），无新代码 |
| 6 | spec-fingerprint-package-auth-paths：指纹只扫旧目录、忽略 truth 主体，漂移检测失效 | 未覆盖 | **已补齐**（本片新代码） | `core.PackageFingerprint` 只对包自身的 truth.md（状态行归一化）+ tests.md 算指纹，truth 改动必变指纹（旧版两个 Must 全中）；freeze/refreeze 把指纹写入事件作基线；doctor 对账重算并新增 drift 类 `truth_body_drift`（只报、--apply 也不改 truth）。算法是 Go 侧新定并带版本标签 `pkgfp-v1`——旧版「不改算法」约束是针对它自身修复的边界，Go 没有旧指纹可兼容 |
| 7 | truth-format-package-auth-skip：旧格式校验器误套新包 | 已覆盖 | **已补齐** | Go 的完整性检查按包权威口径实现，不存在旧校验器误套路径，本片无变更 |
| 8 | version-resolve-fail-closed：版本解析硬编码回退 v1.6.1、无漂移守卫 | 部分 | **已补齐**（核查确认） | 全仓核查：没有任何版本硬编码回退；两处版本比较实现（跨版本切段、迁移扫描排序）对不可解析输入都报错不降级；解析不到目标版本即 typed error 拒绝。漂移守卫的对应面 = self-heal 版本面检查 + doctor 文件↔账本对账（与第 5 项同源：Go 无双版本面可漂移）。两处实现输入域不同，不为凑「集中」而合并 |
| 9 | workflow-memory-completion-gate：完成真相缺失时软放行 | 已覆盖 | **已补齐** | 结构性覆盖维持：complete 四项齐全才放行且代码根本不读 memory（P2a/P2d 有专门测试），本片无变更 |

### 2.2 v1.6.4–1.6.6 共 16 项

| # | 项（原口径一句） | P3b 时点 | 终局 | 落点 / 理由 |
|---|---|---|---|---|
| 10 | program-next-action-admission：program 下一步动作要过准入 | 已覆盖 | **已补齐** | P2c 准入 + P2d program route/admission 已落地，本片无变更 |
| 11 | qa-business-state-machine：从 truth 提取业务状态机生成边界用例 | 部分 | **按拍板结案** | 状态机本体已在内核（loop 12 态+生命周期轴，非法转换 fail-closed）；但「从 truth 自动生成测试用例」是内容生成面，不在口径稿保留的六能力内核内——tests.md 材料归 truth 包内由人/宿主产出（口径稿已把 test-case 顶层文档面并入 truth 包内材料）。不新增生成面 |
| 12 | qa-chrome-extension-e2e：qa 要能验收 Chrome 扩展 | 未覆盖 | **结转 P6**（外部验证项） | 需要能加载扩展的浏览器宿主环境（固定 extension ID、扩展页面、service worker 检测），本机没有此目标；Go 内核只经执行器调度外部 runner，runner 本体在宿主侧。前置：带扩展加载能力的 Chrome/Playwright 宿主 + 一个待验扩展包，到位后以档案模板接入、不改内核结构 |
| 13 | qa-coverage-confidence-chain：十层 QA 之间要有置信度传递、防断链假交付 | 部分 | **按拍板结案** | Go 的 qa 是一道门（证据齐全+引用存活），没有十层结构可供数值置信传递；防断链的内核已由门依赖承担：发布门要 qa 最新 pass、qa 门要证据在盘且哈希一致、complete 要四项齐全——上层结论不可能强于下层证据（P3a 已落地带测试）。数值置信模型不迁 |
| 14 | qa-defect-repair-loop：缺陷修完要强制回归、闭环证据链不断 | 部分 | **已补齐**（等价闭环） | Go 的闭环是结构性的：门/finding 带统一原因码（=缺陷分类）→ tick 的修复路由把任务送回写代码/验收阶段（=修复路由）→ 推进必须重新过验收出口门、complete 只认 qa 门最新 pass（回归是唯一通道、不是可选动作）→ 全程评估与阶段事件即闭环证据链。独立缺陷单实体不另设：卡点记录+门 findings 就是缺陷记录面。无新代码 |
| 15 | qa-methodology-contract：第一性原理+对抗式审查写进行为契约 | 未覆盖 | **按拍板结案** | 这是旧版的散文合同面，口径稿保留清单不含散文合同；其可强制内核已落地：对抗式审查 = 多评审真实现（一人说拦全拦，P3c）+ 独立检查者（干活的不能验收自己，P2a）+ 发布人工确认；第一性原理没有可执行内核面。不把多评审升为发布硬前置：冻结 v2 真相未作此规定，擅改门语义越界 |
| 16 | loop-failure-mode-catalog：已知坏法要成受治理目录供审计消费 | 部分 | **已补齐**（本片新代码） | 旧版 6 个模式以受治理数据进 core（必填字段/严重度集/唯一 id 的校验器照旧版 AC-002），每模式带 Go 原因码映射；消费面 = 长跑体检报告输出 `failure_modes`，按任务实际产生的原因码（卡点记录+事件轨迹+体检问题）配上症状/成因/修法。目录不替代原因码、不产结论（守旧版 Non-Goals） |
| 17 | loop-pattern-registry：loop 模式要成机器可读注册表 | 未覆盖 | **按拍板结案** | Go 的模式集合是代码内静态定义（干法选路表、阶段计划、司机），行为变更走「先补测试」的维护约定；可变注册表与稳定版口径冲突，且入口合同只读视图已能检视当前模式集。不新增注册表面 |
| 18 | loop-readiness-audit：要一个单一就绪评分（L0–L3） | 部分 | **按拍板结案** | 就绪信号已由三个真判定面承担：开跑前自愈检查（上下文+版本面）、长跑体检（单任务健康+本片新增失败模式解释）、gate status（验收/发布正式结论）。口径稿对 doctor 是 slim 口径（只留被自愈和排障真调用的子集）；且其信号清单依赖的模式注册表、token 预算两项已分别结案/结转，拼总分属新设展示面，拒绝新增 |
| 19 | loop-token-budget：loop 成本要可见、超 cap 要停、要有急停开关 | 部分 | **结转 P6**（外部验证项；急停子项本片已补齐） | token 账本要执行器或宿主回报真实用量，Go 内核不在模型回路内、无真实来源，按 P2b 已定口径不编造（预算现以轮次+时长计，已落地）。前置：宿主/档案执行器在阶段证据里回报用量（或 ACP 会话层落地）后，按同一预算结构扩 token 维度。本片补齐其急停开关子项：`.beacon/loop/pause-all` 文件存在时，司机启动与推进都以 `loop_kill_switch_active` 拒绝——文件是人的外部控制信号，内核只读不写、拿走即恢复 |
| 20 | protected-path-denylist：自动写入面不得碰高危路径、命中要拦并指人路 | 未覆盖 | **已补齐**（本片新代码） | core 匹配器五类路径逐字取旧版 global-boundaries §6.1，四种匹配形照 AC-002；写守门落在自动写入主干的唯一通道——卫星工位合并前以三点 diff 取将带入的文件清单，命中即以 `protected_path_denylist` 阻断+事件留痕+显式 human route（AC-003/005）。只读面不阻断（守 AC-004 分工）；计划面的风险 advisory 呈现未另加，守门本体（写面 fail-closed）已落地 |
| 21 | workspace-pool-lease：工位要池化复用+池租约状态 | 部分 | **按拍板结案** | Go 模型是 P3c 定的一任务一卫星工位（隔离优先、全程事件留痕）；租约语义已由司机的任务级租约承担（防双开、过期接管，P2b）。池化复用与一任务一工位模型冲突，且池状态需要账本之外的新状态面，违反「状态只进账本」口径。不迁 |
| 22 | agent-structured-trace：agent 轨迹要结构化可查 | 已覆盖 | **已补齐** | G-17 trace 以 lifecycle_events 为源（P2a），本片无变更 |
| 23 | harness-remediation-sandbox：governor 要能在工位内试错修复、失败自动回滚 | 未覆盖 | **按拍板结案** | Go 的自动修复口径在 P2b 已收敛：自愈只执行有真实落点的最小自动项（修证据清单），其余分计划/人工/需确认三档；overlay 试错是新 harness 子系统，与防长胖和稳定版口径冲突。其「不自动改 canonical」边界在 Go 由「评分只评不改+自愈不动需求与技能正文」承担。不迁 |
| 24 | loop-goal 本体 | 已覆盖 | **已补齐** | P2 全量 G-01..G-22 已落地，本片无变更 |
| 25 | loop-stagnation-detector：停滞要能被检出 | 已覆盖 | **已补齐** | supervisor 心跳超时判 stall 已落地（P2b）；更细的「无新证据重复修复」信号由修复上限（超限进 error_recovery）+ 本片失败模式目录的 infinite-fix-loop 解释面承接，不另设检测面 |

**计数**：已补齐 16（v1.6.3 段 9 项全部已补齐；v1.6.4–1.6.6 段已补齐 7 项 = #10/#14/#16/#20/#22/#24/#25）｜结转 P6 2 项（#12/#19）｜按拍板结案 7 项（#11/#13/#15/#17/#18/#21/#23）。25 项无一留「待定」。

## 3. 本片补实现明细

| 补什么 | 落点 | 行为要点 |
|---|---|---|
| 受保护路径匹配器+守门 | `internal/core/protected_paths.go`；`internal/orchestrator/worktree.go`（merge 前检查）；`internal/harness/git.go`（`ChangedFiles` 三点 diff） | 五类：environment（`.env`/`.env.*`）、secrets（`secrets/`、`credentials/`、`*.pem`、`*.key`、`*_key*`）、auth、payments、migrations；合并命中 → 事件 `worktree_merge_blocked` + typed error + human route，文件绝不落主工位 |
| 包正文指纹全链 | `internal/core/fingerprint.go`；`internal/orchestrator/truth.go`（freeze/refreeze 事件加 `package_fingerprint`+`fingerprint_alg`）；`internal/migration/doctor.go`（新 drift 类 `truth_body_drift`） | 指纹 = sha256(算法标签‖归一化 truth‖tests)；归一化只动 frontmatter 的 status 行——冻结翻转本身不算漂移、正文动一个字都算；doctor 只报不改，--apply 对正文漂移零动作、不写纠正事件 |
| 失败模式目录+消费 | `internal/core/failure_modes.go`；`internal/orchestrator/supervisor.go`（LongrunDoctor 加 `failure_modes`） | 6 模式字段与旧版目录逐字对齐；校验器查必填/严重度/唯一 id；doctor 按 run 的原因码集合匹配解释，纯解释、不产结论 |
| 发布后覆盖派生 | `internal/orchestrator/selfheal.go`（`postReleaseCoverageDetail`） | post 计划的装机步 Detail 列出覆盖名单，名单 = `harness.BuiltinProfiles()` 实时枚举；测试以注册表全集断言名单完整 |
| loop 急停开关 | `internal/orchestrator/driver.go`（`loopKillSwitch` + Start/RunOnce 两道门）；新原因码 `loop_kill_switch_active` | 只查 `.beacon/loop/pause-all` 是否存在；RunOnce 命中先留 `driver_kill_switch_halt` 事件再拒绝；RunLoop 经 RunOnce 自然停 |
| 新原因码 | `internal/core/reasons.go` | `protected_path_denylist`、`loop_kill_switch_active`，均为旧版冻结原文拼写 |

## 4. ACP 深化：两步握手探针

探针从只 `initialize` 扩到 `initialize → session/new` 两步（`internal/harness/acp.go` 重构出最小 JSON-RPC 客户端，仍只用标准库、argv 执行）：

- 第一步答了、第二步被拒/不应答/结果里没有 sessionId → 结果如实报 `stopped_at: "session/new"`，`responded: true` 不冒充会话已开；
- 两步全答 → `session_created: true` + 带回 session id；
- `AcpBackend.Iterate` 继续以 `acp_full_backend_not_supported` 明确拒绝；HealthCheck 改为两步都过才算 OK；全表面（结果、CLI 帮助、Detail 文案）继续标 probe，**没有任何一处出现「支持 ACP」的表述**；
- session/prompt 真迭代结转 P6 外部验证项，前置条件不变：需要一个真 ACP agent 二进制（本机没有），到位后补会话层 + 对真目标的集成测试。

真二进制实跑（目标为同形状假 agent 脚本，两步都应答）：

```text
$ beacon goal acp-probe --target /tmp/p4c-smoke/fake-acp.py --json
{
  "probe": true,
  "responded": true,
  "protocol_version": 1,
  "has_agent_capabilities": true,
  "implementation_name": "smoke-agent",
  "session_created": true,
  "session_id": "sess-smoke-42",
  "detail": "initialize + session/new handshake answered (session id returned); session/prompt is NOT implemented (probe only)",
  "full_backend": false
}
```

## 5. 真厂商档案：指向真二进制的配置方法（复核结论，无新代码）

复核 P3c 的档案解析（`exec_resolve.go`）：档案模板本就可由用户配置覆盖，优先级 = 本次调用 > run 记录 > 内置模板。把档案指向真二进制不需要改代码：

```bash
# 建 run 时就把真二进制记在 run 上（之后 tick/driver 不必重复给）
beacon goal run "<一句话>" --project <根> --feature <slug> \
  --executor subprocess:codex-style \
  --exec-cmd /path/to/codex --exec-arg exec --exec-arg --full-auto
# 或在单次推进时覆盖
beacon goal tick --run-id <R> --executor subprocess:claude-style \
  --exec-cmd /path/to/claude --exec-arg -p
```

`--exec-cmd` 给的 argv 整体替代档案模板（`Profile.WithCommand`）；stage-context 文件仍由内核生成并经模板占位/环境变量递给命令。真二进制实测（真 codex/claude 各跑一次多轮）结转 P6 外部验证项——本机没有这两个二进制，本片未做任何「已验」表述。

## 6. P4 收口总表

| 片 | 内容 | 状态 |
|---|---|---|
| P4a | 六能力面收敛（占位全消、truth 真命令组、其余薄面同判）+ truth 包生命周期补真（create/freeze/change/refreeze）+ 清 P2 两笔小欠账（entry-contract、resume-pack） | ✅ 已收口（173 测试时点） |
| P4b | FrozenFact 共享判定修复准入误报 + 统一 skill 评分（一套引擎、只评不改、评分不入任何门） | ✅ 已收口（187 测试时点） |
| P4c | 25 项剩余清单终局（16 补齐/2 结转 P6/7 按拍板结案）+ ACP 两步探针 + 版本 go.3 | ✅ 本片（199 测试时点） |

## 7. 全局剩余清单（只剩 P6）

P4 出口后，冻结 phase 图只剩 P6。清单逐条：

1. **性能门禁 benchmark**（冻结 AC-051..058）：按冻结口径建基准并设门禁。
2. **E2E + evaluator 全矩阵**：六能力 × 执行器的端到端矩阵验收。
3. **外部验证项**（都卡在外部目标，不在内核代码）：
   - ACP session/prompt 真迭代——前置：真 ACP agent 二进制；
   - 真 codex / claude 二进制多轮迭代（AC-043/045 本体）——前置：本机装上真二进制，配置方法见第 5 节；
   - Chrome 扩展 e2e（#12）——前置：带扩展加载能力的浏览器宿主 + 待验扩展包；
   - loop token 账本（#19）——前置：执行器/宿主在阶段证据里回报真实 token 用量的通道。
4. **条件触发项**：AC-077/078 的旧账本库若将来真出现，先探明结构再补导入解析器（P3b 只做了存在性探测，未编解析器）。
5. **非本轨可办**：入口改名 goal 的变更记录已在 Go 侧落账（P3a），上游冻结包的合并待源仓下一次正式变更（源仓只读）。

**P6 之前明确不做**（已结案，不再挂账）：planner_review 并行化（现为顺序执行且如实标注，AC-102 不要求并行）；双轨/loom 面（拍板不迁）；labs 产品分析工具箱（拍板 drop）；第 2 节 7 项按拍板结案的内容面。

## 8. 验证与交付物

- `go build ./...` / `go vet ./...` / `gofmt -l .`（无输出）全净。
- `go test ./... -count=1` **199 个顶层测试全过**（计数口径：源码 `^func Test` 与 `go test -v` 的 `^--- PASS` 行数双向对齐，均为 199），0 失败。本片新增 11 个：core 3（denylist 匹配表、目录校验与消费、指纹三路）、harness 2（ACP 两步成功、session/new 被拒停在第二步）、orchestrator 5（急停拦启动+拦推进、覆盖名单派生、体检失败模式解释、freeze 记录指纹、worktree 合并受保护路径阻断——真 git 仓库实跑）、migration 1（doctor 正文漂移检出且 --apply 不纠正、文件一字未改）。
- **既有测试改动声明：只改 1 处**——`internal/cli/goal_p2d_test.go` 的版本断言 `v2.0.0-go.2` → `v2.0.0-go.3`（理由与 P3 出口同：该断言钉当前发布版本，P4 出口按维护约定升版；断言本体不变）。另 `internal/harness/harness_test.go` 只在文件尾**新增** 2 个测试函数，未动任何既有测试。其余既有测试零改动、零回归。
- 真二进制冒烟：`--version` 实跑 `Beacon CLI v2.0.0-go.3`；ACP 两步探针实跑输出见第 4 节。
- 边界：冻结 schema 九表结构零改动；状态只进 store（急停文件是只读的外部控制信号、内核从不写它）；未新增顶层命令；Python 源仓未动。

**交付物（文件名 + 大小）**：

- 报告：`docs/p4c-implementation.md`（本文件）
- 二进制：`bin/beacon` **16,076,832 字节**（P4b 时点 16,043,120），版本 `v2.0.0-go.3`
- 新增源文件（均 <500 行）：`internal/core/protected_paths.go` 84、`internal/core/failure_modes.go` 150、`internal/core/fingerprint.go` 78、`internal/core/p4c_test.go` 91、`internal/orchestrator/p4c_test.go` 203、`internal/migration/doctor_p4c_test.go` 83
- 改动源文件：`internal/core/reasons.go`（+2 常量）、`internal/harness/acp.go` 305（两步探针重构）、`internal/harness/git.go`（+`ChangedFiles`）、`internal/harness/harness_test.go`（尾加 2 测试）、`internal/orchestrator/worktree.go` 395（merge 守门）、`internal/orchestrator/driver.go` 461（急停两道门）、`internal/orchestrator/selfheal.go` 254（覆盖派生）、`internal/orchestrator/supervisor.go` 335（体检失败模式）、`internal/orchestrator/truth.go` 414（freeze 记录指纹）、`internal/migration/doctor.go` 297（`truth_body_drift`）、`internal/cli/goal_p3c.go` 384（acp-probe 文案与输出）、`internal/cli/root.go`（版本 go.3）、`internal/cli/goal_p2d_test.go`（版本断言，已声明）
