# 示例：有界面才插设计阶段

**用户说**：「加一个退款进度页」

**mode_id**：`ds-check`

**做法**
1. `beacon goal run "加退款进度页" --project . --feature refund-ui --ux` — 显式声明需要 UX
2. 阶段计划里出现 `design`
3. `beacon design status --run-id <id>` → required=true, bound=false

**产出**：带 design 阶段的 run

**不要做**：靠猜"这个大概要设计"；无界面需求也加 design
