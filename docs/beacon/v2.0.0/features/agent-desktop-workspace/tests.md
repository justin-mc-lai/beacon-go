---
slug: agent-desktop-workspace
version: v2.0.0
materials_status: current
---

# Tests: agent-desktop-workspace

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./desktop/ -run TestShellScriptParses` | shell_parses_three_panes |
| AC-002 | TC-002 | behavior | `go test ./internal/workspace/ -run TestUnregisteredProjectIsEmptyNotInvented` | empty_not_invented |
| AC-003 | TC-003 | behavior | `go test ./desktop/ -run TestWailsConfigValid` | wails_config_complete |
