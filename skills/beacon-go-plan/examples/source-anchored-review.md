# 示例：审查必须读源码

**用户说**：「看看这个退款方案有没有问题」

**mode_id**：`pln-source`

**做法**
1. 读 `docs/beacon/*/features/order-refund/truth.md`
2. 读真实代码：`internal/payment/refund.go`、回调入口、幂等键
3. 指出具体行号与缺口，不写"建议加强幂等"这种空话
4. `beacon truth intent ...` 把发现写进意图，而不是只留在聊天里

**产出**：带文件:行号的发现清单

**不要做**：只读文档就下结论；用"应该没问题"收尾
