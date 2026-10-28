# Feature Truth: agent-desktop-workspace

> **Feature Slug**: `agent-desktop-workspace`  
> **Version**: `v2.0.0`  
> **Status**: `FROZEN` (2026-10-28)  
> **Authority**: Resolver-selected Markdown Truth Package

---

## 人话 (Plain Language L0)

为 Beacon Go 打造真正闭环的智能体协同桌面工作台（兼具 Web 与 Wails 跨平台桌面端能力）。
彻底告别单列聊天框，借鉴 CopilotKit OpenMuse 与 Fei2411/DS-Bot 的顶尖交互设计：
采用专业的三栏布局（左侧作战群与组织总览、中间参谋长协作/动态步骤进度树/人工审批卡片、右侧 Truth.md/代码Diff/机器验资证据面板）。
数据流全面打通底层真实 Go 后端，实时展示当前仓库受管的 Feature 清单、Truth 内容以及 FSM 有限状态机执行步骤，并提供 Wails 桌面端脚手架，实现独立桌面应用交付。

---

## 核心承诺与验收标准 (Acceptance Criteria L1)

### AC-DESKTOP-001: 三栏交互工作台（OpenMuse + DS-Bot 交互形态）
* **Given**: 用户启动 Beacon UI 或桌面端。
* **When**: 渲染工作台主界面。
* **Then**: 
  1. **左侧栏（Navigation & Rooms）**：展示组织角色总览（Chief of Staff / Architect / Coder / QA Verifier）以及作战室清单（如 `#默认作战室`、`#功能研发`）；
  2. **中间栏（Main Interaction & Step Plan）**：展示与参谋长、Worker 智能体的协同对话，嵌入 OpenMuse 风格的动态步骤计划树（`Intent -> Truth -> Worktree -> Verify -> Gate`）及人工审批卡片；
  3. **右侧栏（Workspace Context & Truth Viewer）**：提供多 Tab 切换，实时展示已选 Feature 的 `truth.md`、当前 Git 代码 Diff 以及 QA 机器验资证据（`evidence.json`）。

### AC-DESKTOP-002: 真实后端数据流绑定 (Live Features & Markdown Content)
* **Given**: 前端请求 `/api/workspace/features` 与 `/api/workspace/feature?slug=<slug>`。
* **When**: 后端接收请求。
* **Then**: 
  1. 自动扫描当前工作区 `docs/beacon/<ver>/features/`，返回真实的 Feature 列表与其 FSM 状态；
  2. 支持按 Slug 提取完整的 `truth.md`、`tests.md`、`evidence.md` 内容与机器签名摘要并渲染于右侧面板。

### AC-DESKTOP-003: Wails 桌面应用工程化集成 (Desktop App Scaffold)
* **Given**: 项目引入 Wails 桌面化规范。
* **When**: 检查桌面端构建骨架。
* **Then**: 
  1. 提供 `wails.json` 桌面应用配置定义；
  2. 在 `cmd/beacon-desktop` 或独立桌面入口提供与系统原生 Webview 绑定的初始化入口；
  3. 将 Go 后端的能力（Features 获取、Chat 对齐、Gate 审批）作为结构化绑定接口暴露，支持编译为原生应用。
