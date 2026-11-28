# 一句话跑完整条链

输入：「给订单服务加退款，支持原路返还和幂等回调」。mode_id=`loop-goal`。
以下是有前置条件的命令模板，不是实际运行记录。

```bash
beacon goal run "加退款，支持原路返还和幂等回调" --project "$PROJECT" --feature "$FEATURE" --executor-id "$HOST"
```

宿主保留返回的 RUN_ID；恢复已有工作时复用当前 run。
plan 调查退款状态机、范围与来源，无需先建树或 HTML；只将真实目标取舍交用户决定。
truth 按其 freeze-readiness 示例形成本轮 canonical 树、交 HTML 审阅批准并冻结。

```bash
beacon goal workflow check --run-id "$RUN_ID"
```

按内核 next/missing 补齐当前阶段缺项；必要 bind/prepare 由宿主完成，
不让用户重复解释技术清单。命令不可用或实际合同仍在 plan 要求 HTML 时，
如实记录内核差异与阻塞，不以本示例宣称已推进。
只有阶段工作真实完成且内核允许时才记录：

```bash
beacon goal tick --run-id "$RUN_ID" --by "$HOST"
```

逐阶段将 implement 交 admitted 宿主（可 worktree），verify 交独立 verifier，
release 等用户批准；tick 本身不执行 agent。
每轮输出 run_id/current_stage/existing_artifacts/next_action/owner 与真实命令回执。
成功：仅报告内核确认的当前阶段与产物；拒绝：缺执行者、缺 HTML/批准、
未独立验证、未发布批准，或凭文档 status 报完成。
