# 示例：一句话跑完整条链

**用户说**：「给订单服务加退款，支持原路返还和幂等回调」

**mode_id**：`loop-goal`

**做法**
1. `beacon truth create --project . --slug order-refund` — 先建包，不写代码
2. `beacon truth ask --feature order-refund --question "退款失败的订单能否重试？" --recommend "不能，必须走人工"`
3. `beacon truth intent ...` + `beacon truth confirm` + `beacon truth freeze`
4. `beacon goal run "加退款" --project . --feature order-refund --executor-id maker`
5. `beacon goal driver run-loop --run-id <id>` — 推到 release gate 自己停

**产出**：`docs/beacon/v2.0.0/features/order-refund/` 四件套 + 一个停在 release gate 的 run

**不要做**：跳过 ask 直接冻；冻完自己写代码再自己说通过
