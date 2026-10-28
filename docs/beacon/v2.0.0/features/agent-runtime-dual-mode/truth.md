# Feature Truth: agent-runtime-dual-mode

> **Feature Slug**: `agent-runtime-dual-mode`  
> **Version**: `v2.0.0`  
> **Status**: `FROZEN` (2026-10-27)  
> **Authority**: Resolver-selected Markdown Truth Package

---

## 人话 (Plain Language L0)

让 Beacon Go 进化为一个真正的多智能体协作与开发操作系统。
它既能在 Cursor / Claude Code 等外部 Agent 中作为技能（Skill/CLI）被随时无头调用，也能在本地单命令拉起可视化控制台（Web Console）。
系统内置参谋长（Chief of Staff）做需求对齐和任务路由，专职 Agent（Architect / Coder / QA）通过有限状态机与 git worktree 隔离执行；支持 BYOK（自带 OpenAI / Anthropic / Gemini / DeepSeek 密钥），并以确定性的机器杠杆（`beacon harness verify`）与证据链（Evidence）做不可篡改的交付验收。

---

## 业务背景与问题 (Context & Problem)

当前 Beacon Go 已经具备强大的需求真相追踪、有限状态机（FSM）与工件账本，但在实际应用中仍存在三个核心鸿沟：
1. **外部使用与自主执行割裂**：目前作为静态约束框架，外部 Agent 执行任务时容易出现注意力漂移；
2. **缺乏多模型驱动与参谋长协调（Jobs, not Tasks）**：缺乏开箱即用的多 Agent 角色分工（参谋长、架构师、码农、质检员）；
3. **缺乏直观统一的监控控制台**：多 Agent 并发长程任务缺乏开箱即用的轻量可视化与一键审批门禁。

---

## 核心承诺与验收标准 (Acceptance Criteria L1)

### AC-AGENT-001: BYOK 多提供商配置与模型网关
* **Given**: 用户在 `beacon.config.yaml` 或系统环境变量中配置了模型提供商与 API Key。
* **When**: 系统执行 Agent 任务时。
* **Then**: `internal/llm` 抽象网关能根据角色映射（Role-to-Model），成功调用 OpenAI 兼容接口（如 DeepSeek/Qwen）、Anthropic 或 Google Gemini 接口，支持上下文对话与流式调用；缺失有效 Key 时安全降维报错并熔断。

### AC-AGENT-002: 参谋长（Chief of Staff）与间接意图对齐
* **Given**: 用户通过命令行 `beacon goal "<prompt>"` 或 Web 界面输入高层需求。
* **When**: 参谋长 Agent 接获输入。
* **Then**: 严禁直接动代码；必须先复述需求真相、识别隐性边界，并生成待确认简报；人类确认后自动拆解为 feature slug 并分配专职任务。

### AC-AGENT-003: 专职 Agent 阵列与物理隔离流水线
* **Given**: 任务进入实现阶段。
* **When**: Orchestrator 调度 Worker Agent。
* **Then**: 
  1. `Architect Agent` 负责撰写 `truth.md` 与 `tests.md`；
  2. `Coder Agent` 在独立的 `git worktree` 沙盒内编写业务代码并提交 Commit；
  3. `QA Agent` 拥有全新独立 Context，执行 `beacon harness verify` 收集机器证据；严格遵守“Maker 不自证”。

### AC-AGENT-004: 双模运行之无头模式（Headless Skill & CLI Harness）
* **Given**: 运行在 Cursor、Claude Code 等外部 Agent 宿主环境中。
* **When**: 宿主调用 `beacon` 命令或执行 `beacon harness verify --slug=<slug> --json`。
* **Then**: 输出符合 RFC 规范的机器可解析 JSON 结构体与明确退出码，不依赖 Web 界面也能作为极简工具杠杆（Build the Lever）供宿主调用。

### AC-AGENT-005: 双模运行之独立可视化模式（Single-Binary Web Console）
* **Given**: 用户在终端执行 `beacon ui [--port 4178]`。
* **When**: 命令执行。
* **Then**: Go 后端启动轻量 HTTP 服务器并自动打开浏览器控制台：
  1. 界面通过 Go `embed` 打包内置前端静态文件，零外部 Node.js 运行时依赖；
  2. 可视化看板展示 FSM 状态机流转进度；
  3. 提供参谋长交互抽屉，以及高危动作与最终发布的 Human Approve 卡片；
  4. 支持 SSE 实时推流后端 Agent 执行日志。

### AC-AGENT-006: 状态机防跳步与 Evidence 机器签名
* **Given**: 任务试图从 `Implement` 跃迁至 `QA` 或 `Released`。
* **When**: FSM 状态机执行 `Transition`。
* **Then**: 必须校验 `docs/beacon/<ver>/features/<slug>/evidence.json` 的有效哈希，未通过 `beacon harness verify` 验资的迁移一律被拦截并返回 `reason_code=evidence_verification_failed`。

---

## 边界与防线 (Boundaries & Safety)

1. **不可替代人类审批**：任何生产合并、公开发布、数据库迁移，必须经人类点击 Approve 或 CLI 显式放行。
2. **Token 配额熔断 (Turn Budget)**：单 Feature 超过最大设定轮次（默认 10 轮）或发生 3 次连续自愈失败，立刻置为 `Blocked` 并通知人类。
3. **Change Check 成本守卫**：后台监控与轮询前置执行 10ms 哈希比对，无变动直接退出，杜绝空转刷 Token。
