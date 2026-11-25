# 示例：没有执行器时如实停住

**用户说**：「把这个需求跑完」

**mode_id**：`loop-goal`

**做法**
1. `beacon goal run ... --executor-id operator` — 建 run
2. `beacon goal tick --run-id <id>` → 返回 `stage_executor_not_registered`
3. 如实报告：`stage=plan, blocked=stage_executor_not_registered`，并给出下一步（接执行器或手工推进）

**产出**：一个停在 `plan/active` 的 run + 明确的 reason code

**不要做**：手写"已完成"；把停在 plan 说成"正在推进"
