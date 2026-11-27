# 历史重放 checkpoint

需求变更：用户确认后经 `chg_8614a7dca58d` 为 pair-and-register 增加 AC-006，
自底向上 refreeze 至 root，快照 `v0.0.1-20261127T112756.678920000Z`。

实现：Red 可显式指定完整祖先 commit；使用 Git tree/blob 原始内容构造临时快照，
仅覆盖同一合同绑定的测试文件。旧源输入 hash、目标 Green commit/input hash 分别记录。
Green 及 verified 均核对目标候选，HEAD 内容不变但提交改变也拒绝。

真实测试：
- 首次 TestCaptureHistoricalReplayRedGreen 在无 --baseline 入口时失败。
- 实现后历史 Red→Green 实跑通过，活动业务文件不变。
- TestCaptureHistoricalReplayCompilationIsNotRed：旧接口编译失败拒绝并登记 rejected。
- TestCaptureReplayUsesRawBlobsAndOverlaysOnlyTests：export-ignore/subst 不改变快照；
  当前业务源码不同仍提取旧业务字节，只覆盖当前测试；快照清理实跑通过。
- TestCaptureHistoricalVerifiedRejectsCommitDrift：历史 Red→Green→verify leaf 成功，
  随后 fixture 空提交（非本仓提交）导致共享 verified 重验拒绝。

独立 reviewer session ses_de33aa261ffeEMeX3tobuB0geZ 发现并复核修复：
archive 导出属性问题、verified 漏当前 HEAD；修复代码通过只读复核。
最后新增 verified 成功/空提交拒绝测试由主执行者实跑，不冒充 reviewer 已复审此增量。

最终 go build、go test ./... -count=1、go vet、版本/依赖检查、diff --check 均通过；
既有 E2E 本机日志保存在临时 opencode/replay-final-e2e.log。
本仓九叶尚未重放登记，仍 frozen；不得把 fixture 成功等同于本仓实际完成。

剩余：本轮源码及需求变更尚未提交固定；可执行 Spec 的 test_files 清单在新增
capture_replay_test.go 后过期，需通过 change/refreeze 重绑定后才能实际采集。
旧基线可能缺少新测试引用的类型/API，造成编译失败；该结果只能拒绝，不得改造旧业务源码制造 Red。
