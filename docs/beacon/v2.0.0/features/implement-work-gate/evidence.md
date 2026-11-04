---
slug: implement-work-gate
version: v2.0.0
materials_status: draft
---

# Evidence Index: implement-work-gate

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v2.0.0/features/implement-work-gate/truth.md | draft | truth |
| tests | test_truth | docs/beacon/v2.0.0/features/implement-work-gate/tests.md | draft | qa |
| plan | support_advisory | docs/beacon/v2.0.0/features/implement-work-gate/tasks.md | present | truth |

## 证据要求

- AC-001..006：全量 go test 的输出
- 真机：空执行器跑一轮 goal，implement 门必须 blocked + implement_no_work
