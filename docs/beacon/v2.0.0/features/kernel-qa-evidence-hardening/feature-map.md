---
slug: kernel-qa-evidence-hardening
version: v2.0.0
generated_at: 2026-11-21T10:53:27.316357Z
surface_fingerprint: 2d25e07fbe58aa185fb22a5987e8f28426a6e20c1858047e1a5015544eb7e7f6
---

# Feature Map: kernel-qa-evidence-hardening

One-way projection of truth + tests + tasks. It never rewrites truth.

| AC | Entry Point | Verification Command | Status |
|----|-------------|----------------------|--------|
| AC-001 | internal/orchestrator/autotest.go; internal/orchestrator/qa_execution.go | `go test ./internal/cli -run '^TestKernelQAExecutionEvidence$' -count=1 -v` | implemented_pending_independent_qa |
| AC-002 | internal/orchestrator/artifacts.go; internal/orchestrator/qa_evidence.go | `go test ./internal/cli -run '^TestKernelQACandidateBinding$' -count=1 -v` | implemented_pending_independent_qa |
| AC-003 | internal/orchestrator/qa_evidence.go; internal/orchestrator/complete.go | `go test ./internal/cli -run '^TestKernelQAIndependentReview$' -count=1 -v` | implemented_pending_independent_qa |
| AC-004 | internal/orchestrator/gates_eval.go; internal/orchestrator/exitgates.go; internal/orchestrator/complete.go; internal/orchestrator/delivery_paradigm.go; internal/orchestrator/program_evidence.go | `go test ./internal/cli -run '^TestKernelQAUnifiedGate$' -count=1 -v` | implemented_pending_independent_qa |

Implementation bindings above are executor evidence added after the initial projection; frozen truth/tests/tasks are unchanged. Independent acceptance remains a separate manager action.
