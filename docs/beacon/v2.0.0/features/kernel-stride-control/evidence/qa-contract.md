# QA 交付契约：kernel-stride-control

每个内核交付至少三份独立 QA verdict，三方由三个不同验收方各自给出，maker 不自签，也不改写他人 verdict。

| 检查面 | 验收方 | verdict 路径 | 必核 |
|--------|--------|--------------|------|
| spec/AC | 验收员 | evidence/qa/spec-ac.md | AC-001 至 AC-004 判据原文未被扩张；AC-S1 至 AC-S4 映射一致；每条 AC 至少一条非软 TC；TASK evidence 指向包内真实文件且不自指 |
| 运行时行为 | QA 验收 · agy | evidence/qa/runtime.md | 8 条 TC 按冻结命令实跑，退出码 0；stride 记录字段完整；触顶判未完成；缺 closeout 判未完成；内层越权被拒；外层缺 settlement 被阻断；唤醒不计进展 |
| 负向回归 | QA 验收 · 回归 | evidence/qa/regression.md | 越权与缺 settlement 的负向路径 fail-closed；重放同一 turn 得到同一记录；全量 go test ./... -count=1 退出码 0；既有冻结包的测试无改写 |

三份 verdict 全部落盘后，才允许进入合入判定。任一份 FAIL，不许降低标准，按失效项返修后重新取得独立 verdict。
