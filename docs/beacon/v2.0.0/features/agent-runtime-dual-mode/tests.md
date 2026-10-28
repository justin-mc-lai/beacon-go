# Tests Truth: agent-runtime-dual-mode

> **Feature Slug**: `agent-runtime-dual-mode`  
> **Version**: `v2.0.0`  
> **Authority**: Coverage truth mapping each Acceptance Criteria

---

## 覆盖矩阵 (Coverage Matrix)

| 用例 ID | 对应 AC | 测试描述 | 检验命令 / 断言 |
|---|---|---|---|
| **TC-AGENT-001** | AC-AGENT-001 | LLM Client 工厂与 BYOK 接口封装 | `go test -run TestLLMClientFactory ./internal/llm/...` 断言各 Provider Client 正确初始化与请求序列化 |
| **TC-AGENT-002** | AC-AGENT-002 | 参谋长需求对齐与 Indirect Prompt 格式校验 | `go test -run TestChiefOfStaffIndirectPrompt ./internal/orchestrator/...` 断言输出包含需求事实、风险边界与确认卡片 |
| **TC-AGENT-003** | AC-AGENT-003 | 多 Agent 隔离流水线与状态流转 | `go test -run TestMultiAgentPipeline ./internal/orchestrator/...` 断言 Architect/Coder/QA 独立上下文与 worktree 绑定 |
| **TC-AGENT-004** | AC-AGENT-004 | 无头 CLI Harness 机器输出 | `go test -run TestHarnessVerifyJSON ./internal/cli/...` 验证 `beacon harness verify --json` 产出结构化验证摘要 |
| **TC-AGENT-005** | AC-AGENT-005 | 单二进制内嵌 UI 服务与 API 探针 | `go test -run TestEmbeddedUIServer ./internal/cli/...` 验证 HTTP 探针 `/api/health` 与 `/api/runs` 返回 200 OK |
| **TC-AGENT-006** | AC-AGENT-006 | 状态机验资签名防跳步拦截 | `go test -run TestFSMTransitionEvidenceGuard ./internal/core/...` 验证无证据哈希时拦截迁移至 Released |
