# QA 交付契约：kernel-recover-guard-scope

每个内核交付至少三份独立 QA verdict，三方由三个不同验收方各自给出，maker 不自签，也不改写他人 verdict。

| 检查面 | 验收方 | verdict 路径 | 必核 |
|--------|--------|--------------|------|
| spec/AC | 验收员 | evidence/qa/spec-ac.md | AC-001 至 AC-004 判据原文未被扩张；TC-001 至 TC-010 与 AC 映射一致且为执行层断言；人类门禁原因集合是封闭清单且与 AC 引用一致；TASK evidence 指向包内真实文件且不自指 |
| 运行时行为 | QA 验收 · agy | evidence/qa/runtime.md | 10 条 TC 按冻结命令实跑，退出码 0；recover 对五种人类门禁原因拒绝且快照零改动；确认动作独占清除人类门禁并写两类事件；goal_recovered 与 goal_recover_rejected 的 payload 字段齐全 |
| 负向回归 | QA 验收 · 回归 | evidence/qa/regression.md | 快照改写类绕过、空身份、被拒绝的 recover 写 release_confirmed、attach-verifier 单独解除人类门禁阻断、status 把等待人类显示成未阻断、external 收尾后 status 被重建为 blocked 六类 fail-closed；全量 go test ./... -count=1 退出码 0；既有冻结包测试无改写 |

三份 verdict 全部落盘后，才允许进入合入判定。任一份 FAIL，不许降低标准，按失效项返修后重新取得独立 verdict。

## 边界

本包只写 requirement truth。实现轮在独立分支上按 tests.md 的冻结命令落地行为测试与实现；QA 只在各自的临时 worktree 里读与跑，不改实现与 truth。
