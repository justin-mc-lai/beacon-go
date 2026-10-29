---
change_id: chg_4f1f6fab3ea5
slug: p1-skill-router-acp
rebased_by: lai
rebased_at: 2026-10-28T14:59:02.181191Z
status: pending
created_by: lai
created_at: 2026-10-28T14:57:50.697154Z
base_truth_sha256: ebaa227ebff0584e3ac3cba0400bfdc7d12045ccd2df882cd87a44e101b96f7e
base_package_fingerprint: 504e407a1ea464d36693190cbe3a7e165d98127bec81be4d21c3c9b135ad1ab2
---

# Change Request: AC-007 修订：ACP session/prompt 已实现，Iterate 不再无条件拒绝。AC-007 从'必须返回 acp_full_backend_not_supported'改为'无真实目标时返回明确 reason_code 且不伪造 IterationResult'。AC-006（真实 ACP agent 的一次真迭代）保持未达成，缺真实目标

AC-007 修订：ACP session/prompt 已实现，Iterate 不再无条件拒绝。AC-007 从'必须返回 acp_full_backend_not_supported'改为'无真实目标时返回明确 reason_code 且不伪造 IterationResult'。AC-006（真实 ACP agent 的一次真迭代）保持未达成，缺真实目标

## Route

Edit the package truth (and its tests/tasks/evidence as needed), then close this change with `beacon truth refreeze`. The change stays pending — and the truth_freeze gate stays blocked — until the refreeze lands.

## Rebase Note

合并流程外编辑：tests.md 包路径修正（./internal/skills/ -> ./skills/）与 AC-007 修订，纳入本变更范围
