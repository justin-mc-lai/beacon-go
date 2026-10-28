# 2026 Go 社区 Agent & LLM 底座生态调研简报 (Intel Radar)

> **调研发起**: `/qi-dev-intel-radar`  
> **核心目标**: 评估 Go 社区主流 Agent 框架与多提供商 (LLM Provider) 底座，为 Beacon Go 演进提供选型与架构依据。

---

## 一、调研背景与生态格局判断 (2025-2026)

在过去，AI Agent 与 LLM 生态严重向 Python 倾斜（LangChain, AutoGen, CrewAI），Go 更多被当作基础设施（MCP 代理、模型网关、Ollama 底座）。  
但从 **2025 到 2026 年**，Go 凭借**单二进制分发、强类型约束、高并发低延迟 (Goroutines)**，在企业级生产系统与桌面客户端（如 Wails、CLI）中迎来爆发。生态格局已完成洗牌：

| 框架 / 库 | 发起方 / 维护者 | 核心定位与特色 | 当前生态地位 (2026) |
|---|---|---|---|
| **CloudWeGo Eino** | 字节跳动 (ByteDance / TikTok) | 专为 Go 设计的高性能 LLM 应用与 Agent 框架，含完整 ADK、图编排与 Human-in-the-Loop | 工业界主流旗舰（强烈推荐借鉴） |
| **tRPC-Agent-Go** | 腾讯 (tRPC Group) | 原生面向微服务生产环境的 Agent 框架，支持状态图 (Graph)、多会话隔离、MCP 与 OpenTelemetry | 腾讯生产级多 Agent 底座 |
| **Microsoft Agent Framework (Go)** | 微软 (Microsoft) | AutoGen / Semantic Kernel 思想的 Go 官方演进，强在跨平台与企业工作流编排 | 企业级新兴标准 |
| **LangChainGo** | Travis Cline (`tmc/langchaingo`) | 早期 Python LangChain 的 Go 直译版，提供 OpenAI/Claude/Ollama/Gemini 基础抽象 | 维护趋于停滞，新项目不建议重度绑定 |
| **go-openai / official SDK** | `sashabaranov` / `openai/openai-go` | 零依赖或官方直连的底层客户端，纯做 API 管道 | 稳健的 Provider 基础通讯库 |

---

## 二、头部项目关键能力深度对比

### 1. 字节跳动 CloudWeGo Eino (`github.com/cloudwego/eino`)
* **核心优势**:
  - **组件化抽象**: 标准化定义 `ChatModel`、`Embedding`、`Retriever`、`ToolsNode`，对 Go 接口友好。
  - **ADK (Agent Development Kit)**: 内置 `ChatModelAgent`（ReAct 推理流）、多 Agent 协作网络、上下文管理器与人工干预 (Human-in-the-loop Interrupt/Resume)。
  - **多 Provider 生态 (`eino-ext`)**: 官方拆包维护 OpenAI、Claude、Doubao/Ark、Ollama 适配器，解耦干净。
* **对 Beacon 的启示**:
  - Beacon 的参谋长（Chief of Staff）与多角色协同（Architect ➔ Coder ➔ QA）机制，与 Eino 的 ADK 协作拓扑完全契合；
  - 其中断与恢复（Interrupt & Resume）设计正是 Beacon 状态机跃迁（FSM Gate）所需要的。

### 2. 腾讯 tRPC-Agent-Go (`github.com/trpc-group/trpc-agent-go`)
* **核心优势**:
  - **状态驱动图工作流 (Graph Workflow)**: 深度参考 LangGraph，原生支持多轮条件分支、并行 Fan-out 与检查点 Checkpoint。
  - **会话与内存管理 (Session & Memory)**: 内置隔离的长短期记忆与上下文管理者。
  - **天然适配 MCP**: 原生接入 `tRPC-MCP-Go` 与 OpenTelemetry 链路追踪。
* **对 Beacon 的启示**:
  - 极其契合桌面端需要“左侧会话看板 + 步骤状态机检查点”的视觉要求。

---

## 三、针对 Beacon 架构演进的落地方案

针对用户明确指出的 **“缺乏类似 DS-Bot / OpenMuse 的 Session 上下文展示看板”** 以及 **“输入框应移至底部”**，架构规划如下：

1. **底层 Provider 抽象**:
   - 保持当前轻量 OpenAI-Compatible 网关（兼容本地 OpenCodex、DeepSeek 与三方转接），按需参考 Eino 的 `ChatModel` 接口解耦。
2. **三栏式 / 抽屉式 OpenMuse 原生交互布局 (Light Desktop)**:
   - **左侧 (Sidebar)**: **Session 上下文与需求历史看板**（类似 DS-Bot 的多轮任务列表、当前生效的 Truth 指纹、运行中的 Agent 角色状态徽章）。
   - **中间主区域 (Canvas / Cards Flow)**: 极简生成式卡片流（Plan 方案剖析卡 ➔ Goal 机器验证闭环卡）。
   - **底部固定 (Sticky Bottom Input Pill)**: OpenMuse 风格底置输入框，带有自动伸缩 Textarea、快捷提示与 Plan/Goal 发送药丸按钮。
