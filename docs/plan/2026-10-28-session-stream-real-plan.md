# 深度复盘与需求细化规划：为什么当前形态被判定为“磨洋工”及彻底解决路线 (Beacon Plan)

> **模式**: `pln-review` + `pln-program`  
> **审查对象**: Beacon Agent Desktop Workspace (Wails 原生桌面端)  
> **核心痛点**: 用户明确批评：“没有聊天 LLM SSE 会话记录，Session 也是静态写死的，做得很磨洋工”。  
> **原则**: **停止表象改动，深挖架构根因，进行严密的工程级需求细拆，绝不在 Plan 阶段偷跑假交付。**

---

## 一、根因复盘：为什么会造成“磨洋工、浮于表面”的体感？

复盘前几轮交付，存在三大关键短视与假闭环问题：

1. **“UI 先行、数据写死”的 Demo 心态（Fake/Static Session）**：
   - 之前在左侧侧边栏中直接硬编码了写死的“微信退款幂等方案”、“用户积分商城重构”静态 HTML `div`；
   - 用户点击切换没有任何实际数据流动，也没有读取本地真实磁盘持久化的 Session/Thread；
   - **后果**：给用户极强的“做静态页面糊弄人”的感受。

2. **缺少真正的流式打字与上下文历史（Missing SSE & Message Turns）**：
   - 仅用一个一次性的 POST `/api/chat` 同步返回一个 JSON 静态卡片；
   - 没有实现类似 ChatGPT / OpenMuse 的 **Server-Sent Events (SSE) 逐字流式打字**；
   - 缺乏像 DS-Bot 一样的 `Thread -> Messages[] (User / Assistant / Tool Calls)` 上下文记录回显机制，刷新或新建之后毫无记忆与回放能力。

3. **需求拆解不够结构化（缺少细粒度的任务分解表）**：
   - 将“做个桌面端”当作一个大黑盒，没有细拆成：**持久化会话存储层 (Store)** ➔ **SSE 流式通讯协议层 (Streaming)** ➔ **Wails 原生事件派发 (Event Emitter)** ➔ **前端状态机驱动组件 (React/Pure JS Component)**。

---

## 二、真实完整闭环的目标架构 (Parity Target)

参考 **CopilotKit OpenMuse** 与 **Fei2411/DS-Bot** 的工业级标准，系统必须具备以下四个物理支柱：

```
┌────────────────────────────────────────────────────────────────────────────────────────────────┐
│                               Beacon Agent Desktop 运行时架构                                  │
├────────────────────────────────┬───────────────────────────────┬───────────────────────────────┤
│ 1. 真实数据持久化 (SQLite/FS)   │ 2. SSE / Wails 事件流 (Engine) │ 3. 动态响应式前端 (UI View)    │
│                                │                               │                               │
│ • ~/.beacon/sessions.db        │ • Wails Runtime: EventsEmit   │ • 动态读取 Session 列表       │
│ • Session {ID, Title, Created} │ • Chunk 级别 Token 流式输出   │ • 逐字打字机动画 (Streaming)  │
│ • Message {SessionID, Role,    │ • 实时工具/门禁进度通知       │ • 历史 Message 恢复渲染       │
│     Content, StepCards, Stamp} │   (Planning -> Coding -> QA)  │ • 真实新建/切换/删除会话      │
└────────────────────────────────┴───────────────────────────────┴───────────────────────────────┘
```

---

## 三、细化需求拆解与原子任务表 (Work Breakdown Structure)

我们将完整需求细拆为 4 个不可跳过的阶段，每一项都有机器验证指标：

### 阶段 1：底层会话与消息历史持久化层 (Session & Store Layer)
* **任务 1.1 (数据模型定义)**：
  - 定义 `Session` 实体：`ID (UUID)`, `Title`, `CreatedAt`, `UpdatedAt`, `Status (active|archived)`；
  - 定义 `ChatMessage` 实体：`ID`, `SessionID`, `Role (user|chief|architect|coder|qa|system)`, `Content`, `CardType (none|plan_brief|goal_status)`, `CardPayload (JSON)`, `CreatedAt`。
* **任务 1.2 (存储引擎实现)**：
  - 在 `internal/store/` 扩展支持 `SessionStore`，将数据保存在本地 SQLite（或结构化 JSON 文件），支持 `ListSessions()`, `CreateSession()`, `GetSessionMessages(sessionID)`, `AppendMessage()`。
* **验证标准**：通过 Go 单元测试 `TestSessionStore_CreateAndReplay`，验证会话持久化与读回 100% 通过。

---

### 阶段 2：SSE 逐字流式打字与 Wails 原生事件桥接 (Streaming Bridge)
* **任务 2.1 (LLM 客户端流式改造)**：
  - 在 `internal/llm/` 中将当前一次性 `Complete()` 演进为支持流式的 `StreamComplete(ctx, req, chunkCallback)`；
  - 支持从 OpenAI-Compatible 网关（如 OpenCodex / DeepSeek）接收 `data: {"choices":[{"delta":{"content":"..."}}]}` 并逐块推送。
* **任务 2.2 (桌面端事件广播桥接)**：
  - 在 `desktop/main.go` 中引入 Wails 原生事件 `wails.EventsEmit(ctx, "llm:stream:chunk", payload)`；
  - 同时支持内置 Web 模式下的 `/api/sessions/{id}/stream` SSE 长连接。
* **验证标准**：端到端模拟测试收到不低于 5 个增量 Token chunk，并且客户端能拼成完整段落。

---

### 阶段 3：前端完全动态化（彻底告别静态写死）
* **任务 3.1 (会话列表动态生命周期)**：
  - 启动时立即调用 `GetSessions()` 异步拉取真实列表渲染在左侧；
  - 点击 `+ 新建` 真正写入数据库并高亮新会话；
  - 点击左侧任意一条，调用 `GetSessionMessages(id)`，清空画布并精确还原历史聊天消息与生成式卡片。
* **任务 3.2 (底部输入框与流式打字机交互)**：
  - 用户敲击回车，在主视图先插入一条 `User` 消息气泡并滚动到底部；
  - 随即插入一个流式接收容器，监听 `llm:stream:chunk`，实现类似 OpenMuse 的渐进式打字效果；
  - 生成完毕后，将解析出的 Plan/Goal 生成式卡片吸附在消息下方，并持久化保存入库。
* **验证标准**：用户刷新桌面窗口或重启进程后，历史聊天与卡片全部原样恢复。

---

### 阶段 4：Beacon 状态机与多 Agent 执行联动
* **任务 4.1 (Plan ➔ Goal 步骤卡片持久化绑定)**：
  - 用户在 Card 1 点击 `[确认方案，执行 Goal]` 时，在数据库中记录当前会话跃迁事件；
  - 自动触发后端 `Truth.md 冻结 ➔ Worktree 沙盒 ➔ QA 机器验资`，并将步骤进度通过事件实时推送到当前 Card 2 中。
* **验证标准**：多会话间进度互不干扰，支持后台并发。

---

## 四、自查反思与纪律承诺

1. **为什么此前在执行时让人感到“磨洋工”**：
   - 把精力过度放在了 CSS 调色和静态 HTML 容器拼接上，误以为把界面画得像 OpenMuse 就完成了；
   - 忽视了软件工程最核心的**数据闭环（状态流动与持久存储）**。
2. **纠偏承诺**：
   - 本次分析严格停留在 `plan` 规划阶段，输出上述清晰可落地的 4 个阶段；
   - 等待您对上述细拆方案确认后，再启动 `/beacon-goal implement` 或相关实现门禁，绝不再用静态假数据冒充功能完成。
