# 计划：只保留桌面端，删除 Web Console

> 模式：`pln-review`（范围收敛审查）｜日期：2026-10-28
> 决定人：用户（原话「现在是要有一个桌面端即可，不再有 web ui」）
> 状态：**已执行（2026-10-28）**，验收结果见 `docs/changes/change-desktop-only.md`

---

## 1. 一句话

Beacon 的用户界面**只留一个**：原生桌面窗口。`beacon ui --web`、`/api/invoke`、SSE 推流、
浏览器兜底全部删除，代码里不再存在第二条渲染路径。

## 2. 为什么现在删（而不是留着当调试口）

1. **留着就是两份漂移源。** 今天刚修完的正是「Web 副本与桌面副本各写一遍」导致的假数据与语法错误。
   只要 `/api/*` 还在，下一个人就会再写一遍 handler。
2. **它没有真实用户。** `beacon ui` 自 2026-10-28 起默认启动桌面端，Web 模式只有 `--web` 能进；
   仓库里没有任何脚本、文档、CI 依赖它。
3. **留着要付维护费。** 每次改 `agentapi` 都要同步 `dispatch()` 的 switch，且这条路径没有产品价值。

## 3. 影响面（必须先认账）

| 项 | 现状 | 处置 |
|---|---|---|
| **AC-AGENT-005**（单二进制 Web Console，冻结） | 有实现、有测试 | **dropped-by-charter**：能力不再交付。需上游冻结包走一次 `truth change` 落账；本仓先记 `docs/changes/` |
| AC-AGENT-004（headless CLI/JSON） | 完整保留 | 不受影响：无头能力在 CLI 与 `harness`，不在浏览器 |
| AC-DESKTOP-001/002/003 | 桌面三栏 + 真实账本 | 不受影响 |
| AC-STREAM-001..004 | 会话 + 流式 | 流式改为**只走 Wails 事件**，删 SSE 兜底 |

> 判断依据：AC-AGENT-004 已经覆盖「无头可调用」；AC-AGENT-005 的唯一增量是「浏览器里也能看」，
> 这条增量被本次决定撤销。

## 4. 删除清单

| 路径 | 说明 |
|---|---|
| `internal/ui/server.go`、`internal/ui/server_test.go` | HTTP 服务、`/api/invoke` 分发、SSE、内嵌页 |
| `internal/webui/`（包） | 仅为「两壳共用一份 HTML」而存在；只剩一个壳后没有存在理由 |
| `beacon ui --web` / `--port` | CLI 只保留启动桌面 |
| 前端 `fetch('/api/invoke')` 兜底、`EventSource('/api/events')` 兜底 | 只认 Wails 绑定与 Wails 事件 |

## 5. 迁移清单（不是删掉了事）

| 资产 | 新位置 |
|---|---|
| `internal/webui/index.html` | `desktop/frontend/index.html`（`//go:embed`，唯一消费者就是桌面端） |
| `TestShellScriptParses` / `TestSingleShellCopy` / `TestShellIsRealNotFabricated` | 移到 `desktop/` 包，跟着资产走 |
| `beacon ui` 的二进制查找逻辑 | 保留（`$BEACON_DESKTOP_BIN` → 同级 → PATH → `./bin/`） |

## 6. 验收标准

1. `grep -r "api/invoke\|EventSource\|--web" --include="*.go" --include="*.html"` 无命中（文档除外）；
2. `internal/ui`、`internal/webui` 目录不存在；
3. `go build ./... && go vet ./... && go test ./...` 全绿；
4. `bash e2e/run_e2e.sh` PASS（内核链路不受 UI 影响）；
5. 桌面二进制能构建，`beacon ui` 找不到二进制时给出构建命令。

## 7. 风险与回退

- **风险**：远程/无 GUI 环境再也看不到界面。缓解：AC-AGENT-004 的无头 JSON 面仍在；
  真要回退，`git revert` 这一个提交即可恢复（本次是纯删除 + 搬迁，无数据迁移）。
- **不做的**：不删除 `internal/agentapi`（桌面端唯一后端），不删除 `internal/workspace`（真账本绑定）。

## 8. 执行顺序

1. 搬 HTML 到 `desktop/frontend/`，`desktop/main.go` 改 embed；
2. 搬三个 shell 测试到 `desktop/`；
3. 删 `internal/ui`、`internal/webui`；
4. 改 `beacon ui`（去 `--web`/`--port`）；
5. 前端删两条兜底路径；
6. README / 变更记录 / 任务账本同步；
7. 跑第 6 节五条验收。
