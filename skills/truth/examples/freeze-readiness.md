# 示例：冻前对齐

**用户说**：「这个需求可以冻了」

**mode_id**：`tr-sign`

**做法**
1. `beacon truth confirm --feature order-refund` → 有未答问题就拒绝
2. 补答：`beacon truth answer --id Q1 --answer "..."`
3. `beacon truth intent ...` 确认每个 AC 都有意图来源
4. `beacon truth freeze --feature order-refund` → 打印指纹

**产出**：冻结指纹 + 冻结事件

**不要做**：跳过 confirm 直接 freeze；手动改 truth.md 里的 status
