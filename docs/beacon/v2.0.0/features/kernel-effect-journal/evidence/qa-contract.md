# QA 交付契约：kernel-effect-journal

每个内核交付至少三份独立 QA verdict，三方由三个不同验收方各自给出，maker 不自签，也不改写他人 verdict。

| 检查面 | 验收方 | verdict 路径 | 必核 |
|--------|--------|--------------|------|
| spec/AC | 验收员 | evidence/qa/spec-ac.md | AC-001 至 AC-003 判据原文未被扩张；AC-J1 至 AC-J3 映射一致；每条 AC 至少一条非软 TC；TASK evidence 指向包内真实文件且不自指 |
| 运行时行为 | QA 验收 · agy | evidence/qa/runtime.md | 6 条 TC 按冻结命令实跑，退出码 0；每副作用写 journal；同 key replay 只执行一次；resume 证明清单只含 succeeded 阶段 |
| 负向回归 | QA 验收 · 回归 | evidence/qa/regression.md | 缺 key、重复 replay、failed/不完整记录、缺 journal/损坏 journal 四类 fail-closed；全量 go test ./... -count=1 退出码 0；既有冻结包测试无改写 |

三份 verdict 全部落盘后，才允许进入合入判定。任一份 FAIL，不许降低标准，按失效项返修后重新取得独立 verdict。

