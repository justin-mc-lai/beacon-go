# Tasks Ledger: agent-runtime-dual-mode

> **Feature Slug**: `agent-runtime-dual-mode`  
> **Version**: `v2.0.0`

---

## 任务执行清单

- [x] T1: 搭建 `internal/llm` 原生多提供商网关与配置解析器（支持 OpenAI / DeepSeek / Anthropic / Gemini BYOK）
  - `ac=AC-AGENT-001`
  - `evidence=.beacon/evidence/implement/agent-runtime-dual-mode/AC-AGENT-001.json`
- [x] T2: 升级 `internal/harness` 增强 `harness verify` 结构化机器接口与 Evidence 校验器
  - `ac=AC-AGENT-004`
  - `evidence=.beacon/evidence/implement/agent-runtime-dual-mode/AC-AGENT-004.json`
- [x] T3: 在 `internal/core/fsm.go` 强化证据指纹比对与跳步熔断守卫
  - `ac=AC-AGENT-006`
  - `evidence=.beacon/evidence/implement/agent-runtime-dual-mode/AC-AGENT-006.json`
- [x] T4: 在 `internal/orchestrator` 实现参谋长（Chief of Staff）意图对齐与多 Agent 角色流
  - `ac=AC-AGENT-002,AC-AGENT-003`
  - `evidence=.beacon/evidence/implement/agent-runtime-dual-mode/AC-AGENT-002.json`
- [x] T5: 在 `internal/cli/ui.go` 实现 Go 内嵌轻量 Web 控制台服务（`beacon ui`）与状态同步接口
  - `ac=AC-AGENT-005`
  - `evidence=.beacon/evidence/implement/agent-runtime-dual-mode/AC-AGENT-005.json`
