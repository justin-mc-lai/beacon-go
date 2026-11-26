# 示例：改了正文但没走变更

**用户说**：「我直接把冻结的正文改了，怎么没生效？」

**mode_id**：`ch-legacy`

**做法**
1. `beacon truth drift --feature <slug> --json` — 确认哪些 AC 被移动、指纹是否失配
2. 不要试图把正文改回去假装没发生；登记真实变更：`beacon change legacy --feature <slug> --summary "..." --by <id>`
3. 让编辑后的正文成为授权变更的内容，再 `beacon truth refreeze --feature <slug>`
4. 再跑一次 drift，确认 AC 与指纹对齐

**产出**：drift 报告 + change id + 新指纹

**不要做**：直接编辑冻结正文让它"看起来对"；把 drift 结果当成已评估而不实际核对
