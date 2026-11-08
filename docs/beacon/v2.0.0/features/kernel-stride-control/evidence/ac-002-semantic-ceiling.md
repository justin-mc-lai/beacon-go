# AC-002 证据：数值上限只作安全上限，完成必须有语义 closeout

## 判据（原文）

数值上限只作安全上限，触顶不算完成；完成必须有语义 closeout。

## 源锚点

loopx @ c9c0644，docs/architecture/rfcs/hierarchical-agent-stride-control-v0.md 的 ### 6.1 Semantic boundaries outrank numeric budgets（L330-L337）：ceiling 不是完成规则，不能因为到达最大值就声称成功。

## 验收方式

| TC | Command |
|----|---------|
| TC-003 | `go test ./internal/stride -run TestNumericCeilingIsNotCompletion -count=1` |
| TC-004 | `go test ./internal/stride -run TestSemanticCloseoutRequiredForCompletion -count=1` |

## 执行记录

冻结轮只登记判据与验收命令；实现轮运行上述两条命令后，把实际输出与退出码追加到本小节。
