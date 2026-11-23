---
slug: agent-runtime-dual-mode
version: v2.0.0
materials_status: current
---

# Tests: agent-runtime-dual-mode

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./internal/llm/ -run TestLLMClientFactory` | client_factory_uses_key_and_model |
| AC-002 | TC-002 | behavior | `go test ./internal/orchestrator/ -run TestChiefOfStaffIndirectPrompt` | no_code_write_structured_tail |
| AC-003 | TC-003 | behavior | `go test ./internal/orchestrator/ -run TestMultiAgentPipelineRolesAndGate` | roles_isolated_maker_not_checker |
| AC-004 | TC-004 | behavior | `go test ./internal/cli/ -run TestHarnessVerifyJSON` | blocked_without_evidence_passes_with_real_hash |
| AC-005 | TC-005 | behavior | `go test ./desktop/ -run TestSingleSurface` | one_shell_no_web_render_path |
| AC-006 | TC-006 | behavior | `go test ./internal/core/ -run TestFSMTransitionEvidenceGuard` | transition_requires_valid_evidence |
