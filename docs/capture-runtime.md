# Red/Green 进程采集（Go 测试适配器）

实现需求来源：`red-green-capture` / `capture-process` 与 `pair-and-register`。
当前只提供采集及登记，不把 capture accepted 解释为叶子 verified 或发布通过。

## 合同绑定

在节点 freeze 前，通过 `truth node update` 将 Verification.Spec 设置为以下 JSON 字符串：

```json
{"schema":"beacon-capture-command.v1","argv":["go","test",".","-run","^TestBehavior$"],"discover_tests":true,"required_tests":["TestBehavior"]}
```

此 schema 的 Go 选择器模式仅绑定 package、`required_tests` 和 `discover_tests: true`，
调用者无需维护 `test_files`；两者不能同时声明。每次 capture 通过一次
`go list -e -json` 发现当前所选 package 的全部 TestGoFiles/XTestGoFiles，
规范化结果由 preflight、实际测试输入 hash 和结果登记共用，不回写 Verification.Spec。
结果中的 `test_files` 与 `test_sha256` 是本次运行的真实文件快照，不是新增的需求合同。
新增无关测试不改变 Spec 或需求指纹，但会改变快照 hash；Green 必须引用与当前
测试 hash 一致的 Red，不能沿用新增测试前的 Red。

兼容的显式文件模式可省略 `discover_tests`，此时
`test_files` 必须恰好覆盖所选 package 的全部 TestGoFiles/XTestGoFiles。
路径先解析符号链接再检查项目内包含关系，重复及相对/绝对别名拒绝。
Go 适配器支持本地 package、`-run`、`-timeout`；执行器固定追加 `-json -count=1`，
清除继承的 GOFLAGS，GOWORK=off。普通 shell、true/false 明确拒绝。

自定义程序通过 `adapter: custom-json` 选择，argv[0] 必须是项目内 `./` 可执行程序，
且包含在 test_files 的哈希输入中。它向 stdout 写一个完整 JSON：
`{"schema":"beacon-capture-tests.v1","invocation":"<BEACON_CAPTURE_RESULT>","node_id":"<BEACON_CAPTURE_NODE>","ac_id":"<BEACON_CAPTURE_AC>","phase":"<BEACON_CAPTURE_PHASE>","cases":[{"name":"case-name","status":"passed|failed","assertions":1}]}`。
四个绑定值由环境提供；cases 非空、名称唯一、assertions 为正且所有 required_tests 必须执行。
Red 必需用例失败且 exit 1，Green 全通过且 exit 0；skip、错误调用绑定或普通非零均不能通过。
自定义程序是受信项目测试代码，其内部断言质量仍由独立审查判断。

## 调用

```sh
beacon implement capture --project . --version v0.0.1 --node <leaf> --ac <AC> --phase red --by <maker> --timeout-sec 120
# 实现完成后，显式引用上一条返回的 artifact_id：
beacon implement capture --project . --version v0.0.1 --node <leaf> --ac <AC> --phase green --red <artifact-id> --by <maker> --timeout-sec 120
```

命令返回 JSON。`status=accepted` 表示本次阶段符合采集规则：
Red 真实退出 1 且必需用例有观察到的失败；Green 真实退出 0 且用例无失败/skip。
零测试、未执行必需用例、编译失败、包失败但没有用例失败、缓存、取消、超时、输入改变均拒绝。
用例失败的业务原因仍需独立 reviewer 判断，进程事件不证明断言质量。

采集结果包括真实时钟、argv/cwd、退出码、stdout/stderr 路径及 hash、
实际测试集合 hash、source commit（Git 项目）、全项目输入 hash、运行二进制 hash。
输入快照固定排除 `.git`、`.beacon`、`docs/beacon`；需求通过独立合同指纹绑定。
源码符号链接保守拒绝；非 Git fixture 用输入 hash，source_commit 为空。

每次运行创建 `.beacon/captures/attempt-*`，结果和日志使用既有 node-scoped artifact 登记。
`capture_result` 是保留种类，手动 artifacts register 不接受；日志与结果被改写后读取拒绝。
Green 显式加载登记 Red，核对 node/AC/maker/合同/命令/测试 hash；实现源码允许在两阶段之间变化。
失败的实际测试执行也登记 rejected 结果，CLI 返回非零。作用域/合同/输入校验通过后建立 attempt，
package 预检（包括首次 discovery）、进程启动失败、取消或超时也保留 rejected 结果；
`preflight` 记录真实 go-list 命令、退出码、stdout/stderr、超时与进程错误。
预检失败时测试进程未启动，顶层 `exit_code=-1`，同一次调用仅有一个 attempt；
这些 rejected 结果不是 Red，不能被 Green 引用。
完全无效的请求或无法读取的输入仍在建立 attempt 前拒绝。

## 接入 verified

### 历史 Red 重放

对已经实现的工作，允许 `--phase red --baseline <完整40位祖先commit>`。
仅支持 Go 适配器；系统通过 Git ls-tree/cat-file 读取原始 tree/blob 字节，
不使用会受 export-ignore/export-subst 影响的 archive。在临时目录提取（拒绝特殊文件和符号链接），
只覆盖本次选择器发现的测试文件（显式文件模式使用其校验后的集合），保留旧业务源码。
当前支持 same effective test set：覆盖后的历史执行使用与当前快照相同的测试文件集合。
采用保守拒绝边界：baseline 任意额外 `_test.go` 不在当前集合中时，执行前返回
`capture_replay_test_set_unsupported`，包括删除/改名的旧测试、未选 package 的测试
及被 build tag 排除的测试；不会让旧测试混入实际执行，也不自动删除历史测试。
测试必须实际运行且按同一机器事件规则
产生预期用例失败；缺接口导致的编译失败不算 Red。
不会 checkout、修改或删除活动工作区的正确实现。

成功构造的重放结果额外记录 baseline_commit、replay_candidate、replay_target_sha256 和 execution_root，
以及 `replay_overlay`（覆盖的当前测试集合）、`replay_deleted`（当前支持范围内恒为空数组）。
verified 复核覆盖集合与实际 test_files 相同且删除集合为空。
source_commit 指旧源码基线，source_sha256 指覆盖相同测试后的实际旧输入；
replay_candidate/input hash 指计划配对的当前 Green。
临时快照运行后清理，原始 stdout/stderr 和结果仍在本项目 captures 下登记。
Green 必须引用此 Red，且当前 HEAD 与输入 hash 仍等于重放目标；verified 也复核此绑定。
HEAD 本身、缩写、无效/非祖先 commit、Green 上携带 baseline 均拒绝。
这是真实历史执行，不是把旧终端日志补写为 capture；不保证所有已实现功能均有可用旧基线。

`verify leaf` 的每个 DeliveryRun 现在必须提供 `capture_id`；该 ID 必须指向当前节点
账本中的保留种类 `capture_result`，不能由手动 `artifacts register` 生成。
系统从采集结果读取真实命令、退出码、时间、测试 hash 和日志 hash，逐项与 envelope 比较。
Green 必须显式引用同一对中的 Red，当前源码输入与实际测试文件 hash 必须仍匹配最终 Green。

独立 reviewer JSON 增加 `source_sha256`（最终 Green 的 source_sha256）与
`captures`（`{"<AC ID>":"<Green artifact ID>"}`）。它与原有节点/需求/二进制/commit
绑定一起检查；所有当前 AC 都要核销，不能靠 envelope 自报 done。
review 时间必须晚于 Green，verifier 不能是 maker；身份仍是外部声明，非认证机制。

复制到仓内 evidence 的 stdout 必须与已登记 capture 的 stdout hash 一致。
已登记 capture 的原始结果与日志也必须保留；丢失本机账本或捕获文件会拒绝验证，
仅提交 envelope 不提供可移植的采集来源重建。跨机导入尚未提供。

同一共享校验用于 MarkVerified、rollup、deliver、goal 树阶段与直接 gate 评估；
verify audit 也重验 capture 来源。旧 v1 字段即使完整，缺 capture ID 仍返回 delivery_capture_required。

## 覆盖边界

- 采集器不自动构造或批准独立审查回执；宿主按上述 schema 输出 envelope，系统只接受与采集事实完全一致的字段。
- 当前登记是本机账本事实，不提供抵御任意改写账本/可执行程序的身份认证。
- 两个真实 Go 测试进程通过 `.beacon` 屏障后分别失败的并发用例，证明独立 attempt 与登记 ID；不以时间戳或两个 active 状态冒充并发。
- artifact DAO 对保留结果的刷新使用 SQL 条件拒绝，避免仅依赖上层先查后写。
- 不把本页、合成 fixture 或 package 检查通过当作需求叶子已验收。

## 同类扫描

| 入口 | 共享实现 | 拒绝探针 |
|---|---|---|
| implement capture | Engine.CaptureTest | TestCaptureRealProcessAndTestHash、TestCaptureExplicitRedPair |
| 配置/测试文件解析 | normalizeCaptureCommand/captureTestHash | TestCaptureRejectsUnboundCommand、TestCaptureRejectsDuplicateAliasAndEmptyTests |
| 运行时选择器/唯一预检 | discoverCaptureTests/Engine.CaptureTest | TestCaptureSelectorDiscoversFilesWithoutContractEdit、TestCaptureSelectorInterruptedDiscoveryRetainsAttempt、TestCaptureSelectorOneDiscoveryAndDegenerateInputs |
| 历史测试集合 | prepareCaptureReplay | TestCaptureSelectorReplayRejectsHistoricalExtraTests（删除/改名拒绝）、TestCaptureHistoricalReplayRedGreen（成功链路） |
| 进程执行与结果 | harness.RunCommand/captureGoVerdict | TestCaptureRejectsZeroSkipAndEnvironmentFailure、TestCaptureTimeoutCancellationAndMutation |
| Red 读取 | Engine.ReadCapture | TestCapturePairRejectsChangedTestsAndStaleLogs |
| 保留登记种类 | registerArtifactScopedCaptured | 手动 capture_result 登记拒绝测试 |
| verified 与重验 | ValidateCaptureDelivery/ReadCaptureEvidence | TestCaptureRejectsHandwrittenDelivery、TestCaptureDeliveryRefusesCandidateAndReferenceForgery |
| 真实采集到树迁移成功 | 现有 CLI fixture 使用真实进程 capture | TestVerifyLeafAndParentRollup、TestTruthNodeDeliverRequiresVerifiedChildren |
