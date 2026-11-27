# 严格交付：运行时证据与采集器需求提案

状态：用户于 2026-11-27 确认范围，已通过 change/apply/freeze 入树；本提案不是权威副本或交付证据。
需求权威仍为 canonical intent tree。本文件用于走 change / plan apply / freeze，
不得直接修改冻结 node JSON 或把本提案视为已冻结。

## 用户意图

将六点防假交付协议同时落入 skill、SDD 与运行时；
让缺 Red/Green、独立审查、任务核销的状态迁移失败；
由确定性工具实际运行测试、采集退出码与日志，避免靠手填 JSON 声称执行。

## 已有实现与缺口

- 公共 skill 协议及分发检查已在工作区，未作为本提案的冻结需求。
- delivery_receipt.go 已检查外部 JSON、日志 hash、时间顺序、独立回执和叶子 AC 核销。
- MarkVerified / rollup / deliver / 树阶段检查已有重验调用。
- 以上只证明数据结构与引用完整性，不证明测试进程确曾运行。
- 当前 test_sha256 只是调用者填写的字符串，尚未读取实际测试文件验证。
- 现有 v1 JSON 可手工伪造；不能用它作为新采集器的来源证明。
- 无树 legacy 路径、真实身份认证、恶意宿主防护尚未闭环。

## 需求树落点

在现有 acceptance binding 模块（n_3c1f1d8ec396）下新增两个 feature。
现有两个 feature 的需求正文不变；模块变更通过唯一 change 入口进行。
所有新增节点先冻结需求，不标 implemented / verified / delivered。

### Feature A：strict-delivery-contract

六个叶子对应用户六点，每个叶子有 AC 与可执行验收命令。

| 叶子 | 需求及验收边界 |
|---|---|
| plan-first | 计划明确完成定义、范围、任务依赖、适用检查；宿主确认后写产品代码；不增加 design 阶段。当前文本守卫与将来工具权限拦截分开声明。 |
| red-green | 同一规范测试集合、命令与合同上的 Red 后才接受 Green；缺失、不适用、超时、取消及测试文件变化有明确拒绝或后续变更流程。 |
| blind-review | 独立回执绑定候选、需求、测试证据；自审和缺回执拒绝。身份字符串比对不宣称身份认证或上下文隔离证明。 |
| evidence-gate | 证据登记与状态迁移复用一个校验入口；hash pass 不扩大为行为、树或发布 pass；退出非零保留错误与状态。 |
| external-ledger | 任务定义来自当前合同，执行事实来自既有回执，不新增双账本；空集合、漏项、重复、failed/pending、过期候选拒绝。 |
| deterministic-tools | skill 使用真实 CLI 与脚本签名；验证脚本保留退出码与日志；合同完整性检查不冒充运行时验收。 |

Feature A 的回归入口为 `go test ./skills ./internal/cli ./internal/intenttree ./internal/orchestrator -count=1`。
任务叶子的命令需在树提案中具体绑定到行为测试，不以零测试通过冒充验收。

### Feature B：red-green-capture

#### 叶子 B1：capture-process

- AC-001：宿主通过 implement 组的一个采集命令提交显式项目、版本、叶子、AC、阶段及测试调用；Go 服务直接启动进程，不调用 LLM/agent，不经隐式 shell 拼接。
- AC-002：命令从当前确认的测试定义或被合同绑定的执行配置解析；不能随意把 `false` / `true` 声称为 Red / Green 行为测试。
- AC-003：服务读取实际测试文件集合并计算 hash，记录规范 cwd、argv、开始结束时间、真实 exit code、stdout/stderr；调用者不能传入 exit code、时间或日志内容覆盖系统结果。
- AC-004：运行前后核对合同及测试输入；超时、取消、启动失败、输入在运行中改变均保留失败证据并非零退出。
- AC-005：Go 测试解析机器结果，拒绝零测试、全部 skip、无关编译/环境失败冒充 Red。自定义测试程序需有明确机器结果协议，普通非零退出不足以证明 Red。

#### 叶子 B2：pair-and-register

- AC-001：Green 显式引用 Red capture ID；不得查隐式 latest；同一叶子/AC、合同、命令、测试集合和 maker 才能配对。
- AC-002：Red 非预期失败或 Green 非零不能关闭任务；Green 绑定实现后的 source commit/input hash，不能把 Red 的旧源码当作最终候选。
- AC-003：每次采集生成唯一目录与不可覆盖的结果、日志，登记至现有 scoped artifact 入口；失败重试追加历史，不覆盖上一份。
- AC-004：校验器读取已登记采集结果，重算所有引用 hash，与实际测试集合/候选匹配；手填 v1 JSON、借用他节点、缺登记、篡改、重复 ID 或别名绕过均被拒绝。
- AC-005：同一采集记录保持一个权威写入口；派生 delivery envelope 引用 capture ID/hash，不另维护独立任务状态。

#### 叶子 B3：transition-integration

- AC-001：叶子 verified、父级 rollup、delivered、verify/release evaluate 和 goal tick/complete 通过共享校验消费有效采集配对；CLI 直接入口与服务调用结论一致。
- AC-002：独立审查回执绑定最终 Green 候选与 capture refs；任何任务未核销、缺独立回执、输入过期或日志失效均阻止成功迁移。
- AC-003：验证前读取并拒绝非法迁移；拒绝不得推进节点状态或产生成功登记，失败证据允许追加。
- AC-004：fixture 与 E2E 同批更新，实跑完整成功链及各类拒绝；既有 legacy E2E 通过不能代替新增采集链通过。
- AC-005：旧式外部 v1 回执的适用范围和迁移明确记录；不能静默降级放行，也不能把无树 legacy 当成严格模式已覆盖。

Feature B 新增测试入口拟为 `go test ./internal/cli ./internal/intenttree -run 'TestCapture|TestDeliveryReceipt' -count=1`。
实现前先补可执行失败用例；当前不存在的测试不声称已执行或通过。

## 状态机

每个叶子/AC：未采集 → Red 已采集 → Green 已采集 → 独立审查完成 → 可核销。
失败、取消、过期留在阻塞态；重新采集保留旧尝试。Green 不得跳过 Red，
review 不得先于最终 Green；核销不等于发布批准。

## 微步实现顺序

1. 需求入树并冻结；写采集器拒绝用例，记录真实 Red。
2. 实现实际进程采集、机器结果解析与实际测试 hash。
3. 实现显式 capture 配对、唯一结果及 artifact 登记。
4. 将现有运行时校验改为消费已登记 capture，覆盖直接服务和 CLI 入口。
5. 更新 skill / SDD / fixture / E2E，运行完整验收与独立审查。

## 本轮边界

内核不启动 agent，不认证任意外部 reviewer 的真实身份，不承诺抵御能任意改写
源码、可执行文件和账本的恶意本机用户。不新增第八能力，不新增 design 阶段。
测试集合变化必须重做配对；非 TDD 工作的受控不适用合同需另行定义，不能口头豁免。
提交、推送、安装和发布仍需独立授权。

## 冻结操作的影响与授权

现有 acceptance binding 模块已冻结。现有 change API 会将目标模块及全部后代置为 draft，
祖先置为 change_pending；加入两个 feature 后需要自底向上 refreeze。
模块现有两个 feature 与四个叶子目前均 frozen，操作不应改写其需求正文。
执行前已取得用户对上述冻结子树变更的确认；未直接编辑 canonical JSON。

## 入树与冻结记录

- change：`chg_eb09a43ce525`，目标 `n_3c1f1d8ec396`。
- strict-delivery-contract：`n_9e4515fdf99c`，6 个 touchpoint，全部 frozen。
- red-green-capture：`n_4bdf0311e438`，3 个 touchpoint，全部 frozen；TC 保持 planned。
- 原有 4 个叶子与 2 个 feature refreeze 后指纹与变更前一致。
- 顺序：13 个叶子 → 4 个 feature → acceptance binding → root。
- root snapshot：`v0.0.1-20261127T092615.317426000Z`。
- 新增节点共 11 个，仅需求冻结，不标 implemented/verified/delivered。
- 全树审计基线已有 6 项 delivery_receipt_missing（模块外既有 verified 证据）；本次冻结不修补或掩盖这些发现。
