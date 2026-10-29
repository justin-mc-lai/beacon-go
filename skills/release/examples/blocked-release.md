# 示例：门禁没过不放行

**用户说**：「发了吧」

**mode_id**：`rel-check`

**做法**
1. `beacon release evaluate --run-id <id> --evaluator operator`
2. 返回 `verdict=blocked` + `qa_gate_not_passed`
3. 把原因原样报给用户，给出修复路径

**产出**：blocked 裁决 + 具名原因

**不要做**：找 `--force`；说"基本没问题"
