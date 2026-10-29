# 示例：评分不改技能

**用户说**：「给这个技能打个分」

**mode_id**：`qa-score`

**做法**
1. `beacon qa score goal --project . --feature order-refund --evaluator verifier`
2. 看结构分与 evals 项；effectiveness 没人评就显示 not scored
3. 按建议人工改技能，再重打分

**产出**：scorecard（含 evals 检查项）+ 历史记录

**不要做**：用 `--apply` 让工具改 SKILL.md（会被拒绝）
