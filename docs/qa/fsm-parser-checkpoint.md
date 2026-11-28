# FSM coverage parser checkpoint

仅本 worktree 的返工 checkpoint；不是需求真源、验收回执或完成账本。

## 两轮审查与实跑 Red

1. 第一轮独立审查 BLOCK：negativeSlotRefs 独立支持 AC_ID/TC_ID，正式 ParseCoverageRows 不支持，未执行的旁路 negative 行可以清除 uncovered；SDD CLI fixture 补齐覆盖后仍因缺合法 TASK 表 blocked。
   - Red：TestFSMNegativeCoverageRejectsNonExecutableSideMatrix 报 `non-executable side matrix cleared uncovered: []`。
   - Red：TestFSMNegativeCoverageIntegrityCLI/sdd 报 `task_dag_invalid: executable TASK table missing or empty`。
   - 修复：移除独立 TC parser，正式 TCRow 携带 slot binding；补合法 SDD TASK fixture。定向、全量及 E2E 当时通过。
2. 第二轮审查 BLOCK：integrity.go 的非法 AC 数据行分支设置 inMatrix=false，后续同 AC 有效 TC 静默消失。上一轮为阻止旁路表头继承矩阵状态，把表头边界错误地泛化到了普通数据行。
   - 本轮 Red：`go test ./internal/gate -run 'TestFSMNegativeCoverage(MalformedDataDoesNotTruncateRequiredTCs|AdjacentSideHeaderEndsMatrix)$' -count=1 -v`，exit 1。blank 和 invalid AC 两个子例均只剩 TC-001，后续同 AC 的 TC-002 丢失；相邻旁路表头用例当时通过。

## 剩余问题与换方法

- 截断问题：空 AC／非法 AC 数据行之后，required TC 集合会被缩小。
- 改用明确的输入事件分类：fence、非表文本、新表头、separator、数据行。
- 只有结构边界（含不受支持的旁路表头）可终止或切换矩阵；普通非法数据行采用跳过策略，保留矩阵状态和后续有效 TC。
- 先新增同 AC 有效 TC 位于 blank/invalid 行后的回归并实跑 Red，再实现；旁路表头反例须继续 Green。
- 本轮只跑定向与格式检查；全量由集成统一执行。不修改其他 lane，不提交。

## 新方法实现

- 显式表头事件检测 AC ID / TC ID 以及不受支持的 AC_ID / TC_ID；后两者只终止旧矩阵，不启用新矩阵。
- 普通非法 AC 行只跳过，保留 inMatrix。相邻无效表头因此不再依赖“所有非法数据都终止”这一错误规则。

## 本轮定向 Green

`go test ./internal/gate ./internal/cli -run 'TestFSMNegativeCoverage|TestParseCoverage|TestCoverageMatrix|TestIntegrityDomainFSMGroup' -count=1 -v`：exit 0；gate 0.342s，cli 0.480s。

blank/invalid 后同 AC 的 TC-002 均保留；相邻与分节旁路表头均不能清除 uncovered；legacy/SDD CLI 缺项均以 uncovered 拒绝，补齐均完整 pass。已对本轮 Go 文件 gofmt；git diff --check exit 0。本轮全量未重跑，等待集成统一验证；独立复审未确认。
