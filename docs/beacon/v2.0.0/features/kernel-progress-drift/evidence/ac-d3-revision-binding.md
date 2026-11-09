# AC-003 证据：回执绑定 goal 固定 revision，绑定到别处不计数并上报 rebind_hint

## 判据（原文）

回执必须绑定 goal 固定 revision，绑定到别处不计数并上报 rebind_hint。

## 源锚点

loopx @ c9c0644，`loopx/capabilities/progress_review/receipt.py` L470-L490：未固定 revision 时 `assist_blocked_reason = "contract_revision_unpinned"`；绑定到其它 revision 或最新 revision 偏离时 `summary["rebind_hint"] = "newer_receipts_under_unpinned_revision"` 且不作为当前证据计数。`loopx/control_plane/work_items/progress_review_policy.py` L57-L70 的 `normalize_progress_review_contract_revision` 负责 revision 规范化（SHA256 hex digest）。

## 验收方式

| TC | Command |
|----|---------|
| TC-005 | `go test ./internal/progressreview -run TestReceiptBindsPinnedGoalRevision -count=1` |
| TC-006 | `go test ./internal/progressreview -run TestForeignRevisionIsNotCountedAndReportsRebindHint -count=1` |

## 执行记录

冻结轮只登记判据与验收命令；实现轮运行上述两条命令后，把实际输出与退出码追加到本小节。
