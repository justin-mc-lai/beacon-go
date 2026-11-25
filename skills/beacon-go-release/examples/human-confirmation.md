# 示例：人工确认才完成

**用户说**：「我确认可以发」

**mode_id**：`rel-complete`

**做法**
1. 确认四件事：release_gate / 人工确认 / verifier 已挂 / qa pass
2. `beacon goal attach-verifier --run-id <id> --ref qa-report.txt --source verifier --executor-id checker`
3. `beacon goal complete --run-id <id> --human-confirmed --confirmed-by operator`

**产出**：completed 的 run + 确认事件

**不要做**：自己替用户确认；maker 自己当 verifier
