# 示例：急修快车道

**用户说**：「线上支付挂了，先修」

**mode_id**：`rel-check`

**做法**
1. `beacon goal hotfix-plan --run-id <id> --from v2.0.0`
2. 计划里写清从哪个版本修、怎么回滚
3. 仍走 qa 验资，只是不排全流程

**产出**：hotfix 计划 + 回滚步骤

**不要做**：跳过 qa；不写回滚
