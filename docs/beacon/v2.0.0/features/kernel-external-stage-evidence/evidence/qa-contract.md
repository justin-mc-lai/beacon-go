# QA 交付契约：kernel-external-stage-evidence

每个内核交付至少三份独立 QA verdict，三方由三个不同验收方各自给出，maker 不自签，也不改写他人 verdict。

| 检查面 | 验收方 | verdict 路径 | 必核 |
|--------|--------|--------------|------|
| spec/AC | 验收员 | evidence/qa/spec-ac.md | AC-001 至 AC-004 判据原文未被扩张；每条 AC 两条非软 TC；录制标记字段与 qa 门两条判定路径描述一致；TASK evidence 指向包内真实文件且不自指 |
| 运行时行为 | QA 验收 · agy | evidence/qa/runtime.md | 8 条 TC 按冻结命令实跑，退出码 0；external 与 manual 阶段完成写真实事件；qa 门拒收录制态事件；receive-result 外部完成加已注册 artifact 可作等价证据；streak 与 attach-verifier 行为一致 |
| 负向回归 | QA 验收 · 回归 | evidence/qa/regression.md | 伪造事件字段、placeholder 历史通过路径、未注册或 hash 不符的 artifact、streak 达到 3 后 continue、attach-verifier 单独解除阻断五类 fail-closed；全量 go test ./... -count=1 退出码 0；既有冻结包测试无改写 |

三份 verdict 全部落盘后，才允许进入合入判定。任一份 FAIL，不许降低标准，按失效项返修后重新取得独立 verdict。
