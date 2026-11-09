# P2 源锚点：kernel-progress-drift

- 上游仓库：https://github.com/loopx-project/loopx
- 基线 commit：c9c0644be428bbb0ab10726637f5b36602972912（2.3.0 发布窗口 HEAD）
- 锚点清单来源：BeaconGo 研究 2026-11-09 落盘的 loopx-p1-p2-p3-p5-source-anchors.md，本文件只摘 P2 部分
- 主锚点文件：`loopx/capabilities/progress_review/receipt.py`、`loopx/control_plane/work_items/progress_review_policy.py`、`loopx/capabilities/progress_review/README.md`

## AC-001（上游 AC-D1）

- 文件与行号：`receipt.py` L27-L38；`progress_review_policy.py` L16-L23
- 支撑点：回执 schema 固定为 `progress_review_receipt_v0`；模式词表为 `off` / `shadow` / `assist`，默认 `off`。回执字段包含 `event_id`、`evidence_id`、`status`、`run`、`judgments`、`drift_signal`、`question_version`。

## AC-002（上游 AC-D2）

- 文件与行号：`progress_review_policy.py` L19-L55；`receipt.py` L28-L34
- 支撑点：默认 drift 阈值 N=2；只有 `completed` 且 `drift_signal=true` 的回执累计计数并触发既有 replan 义务；状态词表 `completed` / `abstained` / `failed` / `not_evaluated` / `stale`，非 completed 状态永不算已处理。

## AC-003（上游 AC-D3）

- 文件与行号：`receipt.py` L470-L490；`progress_review_policy.py` L57-L70
- 支撑点：回执必须绑定 goal 的固定 revision（SHA256 hex digest）；未固定时 `assist_blocked_reason = "contract_revision_unpinned"`；绑定到其它 revision 或最新 revision 偏离时上报 `summary["rebind_hint"] = "newer_receipts_under_unpinned_revision"`，且该回执不作为当前证据计数。
