---
slug: p0-executor-desktop
version: v2.0.0
materials_status: draft
---

# Tests: p0-executor-desktop

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./internal/harness/ -run TestBuiltinProfileFlagsExistInRealCLI -count=1` | profile_flags_match_real_cli（模板每个 `--flag` 能在对应 CLI `--help` 中grep到；且不含真实CLI不认识的 `--context-file`） |
| AC-002 | TC-002 | behavior | `go test ./internal/harness/ -run TestCodexRealIteration -tags=realcli -count=1` | codex_real_iteration_holds（真 codex 二进制 exit_code=0、success=true、agent_name=subprocess:codex-style） |
| AC-003 | TC-003 | behavior | `go test ./internal/harness/ -run TestClaudeRealIteration -tags=realcli -count=1` | claude_real_iteration_holds（真 claude 二进制 exit_code=0、success=true、agent_name=subprocess:claude-style） |
| AC-004 | TC-004 | behavior | `go test ./internal/harness/ -run TestProfilePromptCarriesContextPath -count=1` | context_path_reaches_prompt（解析后的 argv 含 stage-context 绝对路径，agent 可自行读取） |
| AC-005 | TC-005 | behavior | `go test ./internal/harness/ -run TestProfileBackendFailureSurfaces -count=1` | failure_has_reason_code（命令不存在/非零退出/超时均 success=false 且带明确 reason_code，无伪造） |
| AC-006 | TC-006 | behavior | `go test ./internal/agentapi/ -run TestAPIRunAdvanceAndExecutorConfig -count=1` | api_advance_and_config_exposed（Tick/单步driver/执行器配置三接口可调用） |
| AC-007 | TC-007 | behavior | `go test ./desktop/ -run TestDesktopRunControlsPresent -count=1` | desktop_controls_wired（前端含推进控件且绑定 AC-006 接口；无 UI 自动化，本条为静态+绑定级断言） |
| AC-008 | TC-008 | behavior | `go test ./internal/agentapi/ -run TestAPIParksWithoutExecutor -count=1` | park_not_fake_done（无执行器时返回 blocked + `stage_executor_not_registered`，不返回完成态） |

## 真跑口径（用户批准：允许真跑，限定最小代价）

- TC-002 / TC-003 需真调 codex / claude，会产生 API 调用。因此用 build tag `realcli` 隔离：默认 `go test ./...` **不跑**（零消耗），需证据时显式加 `-tags=realcli`。
- 真跑 prompt 取最小代价（纯文本应答类，不改文件、不跑复杂任务），每个执行器**只跑 1 次**。
- 真跑前置由测试自检：二进制不在 PATH 则 skip（不算 pass）——skip 不等于通过，符合 no-fake-delivery。
