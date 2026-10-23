---
name: beacon-implement
description: Implementation lanes behind one admission check and one harvest rule. Single, team, and ralph lanes share the same entry gate and result contract.
triggers: 实现, implement, 写代码, ralph, worktree
---

# beacon-implement

1. `beacon implement plan --run-id <id>` shows the lane routing and admission verdict.
2. `beacon implement admit --run-id <id>` is read-only; a blocked admission stops all writes.
3. `beacon implement worktree create|merge|cleanup|status --run-id <id>` isolates parallel work.
   Satellites merge into the main star only; protected paths never merge.
4. `beacon implement delegate-ralph --run-id <id>` hands the stage to the ralph lane.
5. `beacon implement receive-result` harvests artifacts + evidence under one rule.

Rules: no admission, no implement; no evidence, no completion claim.
