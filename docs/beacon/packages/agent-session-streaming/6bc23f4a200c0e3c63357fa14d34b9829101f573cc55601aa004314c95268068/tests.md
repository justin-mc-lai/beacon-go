---
slug: agent-session-streaming
version: v2.0.0
materials_status: current
---

# Tests: agent-session-streaming

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./internal/session/ -run TestSessionStore_CRUD` | create_list_append_persist |
| AC-002 | TC-002 | behavior | `go test ./internal/llm/ -run TestStreamCompleteChunks` | at_least_three_chunks |
| AC-003 | TC-003 | behavior | `go test ./desktop/ -run TestShellSessionAndStreamWiring` | shell_binds_sessions_and_stream |
| AC-004 | TC-004 | behavior | `go test ./desktop/ -run TestAppSessionAPIs` | history_replays_after_switch |
