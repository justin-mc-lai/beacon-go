# 示例：中途断了怎么接

**用户说**：「昨天那个退款任务继续」

**mode_id**：`loop-goal`

**做法**
1. `beacon spine show --project . --feature order-refund` — 先看账本，不看聊天记录
2. `beacon goal resume-pack --run-id <id>` — 拿恢复包（下一步 + 已完成子步）
3. `beacon goal resume --run-id <id>` — 从断点续跑，不从头来

**产出**：续跑后的 run 状态 + 一条 `host_interrupted` 之后的事件

**不要做**：重开一个新 run；凭记忆复述进度
