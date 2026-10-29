# 变更记录：桌面端接真账本、删除假数据、收口入口

> 日期：2026-10-28 ｜ 状态：Go 侧已实现并验证，上游冻结包尚未走 truth change
> 触发：2026-10-28 项目审查（README 与真实行为不一致的 6 类问题）

---

## 1. 问题（审查实测，不是推测）

| # | 问题 | 实证 |
|---|---|---|
| 1 | 前端脚本整块语法错误，界面是空壳 | `node --check` 报 `SyntaxError: Unexpected token '}'`（`desktop/frontend/index.html` 第 604 行起有游离 `} else {` 块） |
| 2 | Goal 卡与放行动作是假的 | `executeGoal()` 不发请求，硬编码「QA 100% 通过」「sha256-verified-${slug}-release-gate」；`approveRelease()` 不看响应就弹「已放行」；`/api/action/approve` 只广播事件、返回 `approved` |
| 3 | 桌面端没接内核 | `desktop/main.go` 只 import `llm`、`orchestrator.ChiefOfStaff`、`session`；不碰 `store`/`gate`/`core`，feature 列表靠 walk `docs/beacon` |
| 4 | 三个同名桌面入口 + 两份完全相同的 HTML | `desktop/main.go`（Wails）、`cmd/beacon-desktop`（起 HTTP 开浏览器）、`internal/ui`（Web Console）；两份 `index.html` 各 33,327 字节、逐字节相同 |
| 5 | 会话存储非原子 | JSON 文件 `os.WriteFile` 直写，崩溃即半截；`ListSessions` 静默吞损坏文件 |
| 6 | 验收纪律在自己身上失效 | `tests.md` 的 TC-STREAM-003（`TestAppSessionAPIs`）全仓不存在；`evidence.md` 写 Pending 而 `tasks.md` 三项全 `[x]`；`beacon spine list --project .` 返回 `project_not_registered` |

---

## 2. 变更内容

### 2.1 一个后端：`internal/agentapi`

桌面端（Wails）和 Web 控制台绑定同一个对象，每个方法只实现一次。Web 端不再有各自的 handler 逻辑，只有 `/api/invoke` 一个分发口。这从结构上消灭了「两份实现互相漂移」的成因。

### 2.2 新服务层：`internal/workspace`

只做三件可被 UI 诚实声称的事：

- `Features()` / `Detail(slug)`：从**账本**读需求包；账本里没有时退化为只读磁盘扫描，并明确标 `registered=false`（显示「存在但未纳管」，不冒充已登记）；
- `StartGoal(slug, utterance)`：`EnsureRegistered` → 包已存在则 `adopt`（只登记、不重写正文），不存在才 `CreateTruthPackage` → `CreateRun`，返回真实 `RunView`（含真实 `stage_plan`）；
- `Release(runID)`：`EvaluateGate("release")` 拿真实裁决；只有裁决 `pass` 才调用 `Complete`。**门禁不过时返回 `verdict=blocked` + 真实 reason codes，不报错、不假装成功。**

引擎没有注册执行器时，run 会停在第一个阶段并如实阻塞 —— 这是引擎既有语义（`stage_executor_not_registered`），UI 原样展示。

### 2.3 前端单一来源：`internal/webui`

- 唯一一份 `index.html`，`//go:embed` 后由 Wails `AssetServer` 和 Web 控制台共用；
- 删除 `desktop/frontend/index.html`、`internal/ui/assets/index.html`；
- 脚本整段重写：删除隐藏测试锚点、删除假指纹、删除无条件成功弹窗；
- 新增真实右侧面板（`inspector-pane`）：显示所选需求包的 `truth.md` / `tests.md` / 真实门禁裁决；
- 阶段列表来自 `RunView.stage_plan`，状态只有 `pending/active/done` 三种，且都来自账本。

### 2.4 会话存储：JSON → SQLite

`internal/session` 改为 `~/.beacon/sessions.db`（WAL、`foreign_keys=1`、单连接、写操作单事务）。`AppendMessage` 与 `updated_at` 在同一事务内提交，会话列表不会落后于消息。

边界说明：会话历史是 UI 状态，不是 run/gate 的运行态权威，因此**不塞进内核九表账本**（AC-067..075 是冻结契约）。要合并进主账本必须对冻结包走一次 truth change，本次不做。

### 2.5 入口收口

- 删除 `cmd/beacon-desktop`（伪桌面：起 HTTP 再开浏览器）；
- `beacon ui`：默认启动原生 Wails 窗口；`--web` 用同一个二进制起内嵌控制台（保住 AC-AGENT-005），`--port` 只在 `--web` 下有效；
- 二进制查找顺序固定：`$BEACON_DESKTOP_BIN` → CLI 同级目录 → `PATH` → `./bin/`，找不到就报错并给出构建命令。

> **2026-10-28 追加决定**：用户决定只保留桌面端，`--web` / `--port` / `internal/ui` /
> `internal/webui` 已整体删除，AC-AGENT-005 转为 dropped-by-charter。
> 本节关于 `internal/webui` 与 `--web` 的描述作废，以 `docs/changes/change-desktop-only.md` 为准。

### 2.6 不再编造模型没说的内容

`ChiefOfStaff` 改为要求模型在回答末尾给出 `{"slug","risks","boundaries"}` 的 JSON 尾块：

- 解析成功 → 用模型给的风险/边界/ slug；
- 解析失败 → 风险与边界留空，slug 由话语派生（ASCII 安全），并标记 `degraded`，UI 明示「模型未返回结构化字段，不编造」；
- 删除原先写死的两条 risks + 两条 boundaries 常量（此前每张方案卡都声称同样的两条不变量）。

---

## 3. 影响的 AC

| AC | 影响 |
|---|---|
| AC-DESKTOP-001 | 三栏成立，且右栏与阶段列表是真实数据（不再是隐藏空 div） |
| AC-DESKTOP-002 | Feature 列表/正文改从账本 + 磁盘读，状态不再一律写 `Frozen` |
| AC-DESKTOP-003 | 保留 `wails.json` 与 Wails 入口，删除重复的伪桌面入口 |
| AC-STREAM-001..004 | 存储换 SQLite；补上 TC-STREAM-003（`TestAppSessionAPIs`） |
| AC-AGENT-002 | 简报只含模型实际说出的风险/边界 |
| AC-AGENT-005 | `beacon ui --web` 保留单二进制控制台 |
| AC-AGENT-006 | 放行动作改为真门禁评估，未验资不放行 |
| AC-025 | **未处理**：仍有 12 个非测试文件 >500 行 |

---

## 4. 验证（真实输出）

```
$ go build ./...          # 无输出，退出 0
$ go vet ./...            # 无输出，退出 0
$ go test ./...           # 15 个包全部 ok（orchestrator 64.4s）
$ bash e2e/run_e2e.sh     # E2E RESULT: PASS — full chain completed (12s)
$ node --check <inline>   # 退出 0
```

真二进制控制台实测（临时项目 `/tmp/beacon-ui-check`）：

```
GET /                      -> 200
GET /api/health            -> {"status":"ok", ..., "project":{"root":"/tmp/beacon-ui-check","registered":false}}
POST StartGoal             -> run_id=run_d6e0ad68d12105804cfc93ef, stage_plan[0]=plan/active
POST ReleaseGoal           -> {"verdict":"blocked","reason_codes":["gate_not_evaluated",
                               "release_requires_human_confirmation"],"completed":false}
```

最后一条是关键：旧版本这里会弹「已正式放行发布」；现在它如实报告门禁没过。

---

## 5. 未覆盖边界（如实列出）

1. **桌面端不代替 agent 干活**：没有注册执行器时，run 停在 `plan/active`，不会自动推进。这是引擎语义，不是本次遗漏。
2. **上游冻结包未改**：本记录是 Go 侧变更，`docs/beacon/v2.0.0/features/*` 的 `truth.md` 正文未动。
3. **证据仍非 harness 生成**：`.beacon/evidence/**` 是历史手工文件，本次只补了真实测试，未接入 `beacon harness verify` 自动产证。
4. **AC-025 未处理**：12 个文件仍超 500 行。
5. **未提交**：按仓库约定，`git commit` 需人工确认。
