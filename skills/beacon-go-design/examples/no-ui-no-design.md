# 示例：纯后端不插设计

**用户说**：「加一个对账定时任务」

**mode_id**：`ds-check`

**做法**
1. `beacon goal run ...`（不带 `--ux`）
2. `beacon design status --run-id <id>` → required=false
3. 直接进 implement

**产出**：五阶段计划（无 design）

**不要做**：为了"流程完整"硬加设计阶段
