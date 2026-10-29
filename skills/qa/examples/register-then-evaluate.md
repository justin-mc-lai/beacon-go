# 示例：先登记证据再判

**用户说**：「验收一下」

**mode_id**：`qa-gate`

**做法**
1. 跑测试，把输出落文件：`go test ./... > qa-report.txt`
2. `beacon goal artifacts register --run-id <id> --kind qa_report --path qa-report.txt --by verifier`
3. `beacon qa evaluate --run-id <id> --evaluator verifier`
4. `beacon gate status --project . --feature order-refund`

**产出**：带 sha256 的登记记录 + 门禁裁决

**不要做**：不登记就 evaluate；拿聊天记录当证据
