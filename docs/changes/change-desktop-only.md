# 变更记录：只保留桌面端，删除 Web Console

> 日期：2026-10-28 ｜ 决定人：用户（「现在是要有一个桌面端即可，不再有 web ui」）
> 计划：`docs/plan/2026-10-28-desktop-only-plan.md`
> 状态：已落账 —— Go 侧实现、本仓冻结包已按 change → 重写 → refreeze 收口（2026-10-29）；上游只读镜像仍待其正式 gen-change

---

## 1. 决定

Beacon 的用户界面只保留**原生桌面窗口**。浏览器控制台整体删除，不是 deprecated、
不是隐藏开关，而是代码里不存在第二条渲染路径。

理由（计划文档 §2）：

1. 两个壳就是两份漂移源 —— 2026-10-28 修掉的假数据与语法错误，根因正是「两份实现各写一遍」；
2. Web 模式没有真实使用者：`beacon ui` 自 2026-10-28 起默认桌面端，仓库内无脚本/CI 依赖它；
3. 留着要付维护费：每改一次 `agentapi` 都要同步 `dispatch()` 的 switch。

## 2. AC 影响（必须先认账）

| AC | 处置 |
|---|---|
| **AC-AGENT-005**（Single-Binary Web Console） | **dropped-by-charter**：能力不再交付。`truth.md` 正文未改，待上游冻结包走 `truth change` |
| AC-AGENT-004（Headless CLI / JSON） | 保留不变：无头能力在 CLI 与 harness，不依赖浏览器 |
| AC-AGENT-002（参谋长对齐） | 保留；入口从「CLI 或 Web」收敛为「CLI 或桌面端」 |
| AC-DESKTOP-001/002/003 | 保留，且仍由桌面端承担 |

> 边界说明：AC-AGENT-005 的唯一增量是「浏览器里也能看」。这次撤销的正是这条增量，
> 不是「可视化」本身 —— 可视化仍在，只是只有桌面一种形态。

## 3. 删除与搬迁

| 动作 | 路径 |
|---|---|
| 删除 | `internal/ui/`（HTTP 服务、`/api/invoke` 分发、SSE、内嵌页、4 个测试） |
| 删除 | `internal/webui/`（两壳共用 HTML 的中间层，已无第二个消费者） |
| 搬迁 | `internal/webui/index.html` → `desktop/frontend/index.html`，由 `desktop/main.go` 用 `//go:embed frontend` + `fs.Sub` 加载 |
| 搬迁 | `TestShellScriptParses` / `TestShellIsRealNotFabricated` → `desktop/shell_test.go`；新增 `TestSingleSurface` |
| 删除 | `beacon ui --web`、`--port` |
| 删除 | 前端 `fetch('/api/invoke')` 与 `EventSource('/api/events')` 兜底 |

`beacon ui` 现在的唯一职责：按固定顺序找 `beacon-desktop` 二进制，设好
`BEACON_PROJECT_ROOT` 后启动它；找不到就报错并给出构建命令。

## 4. 验收（真实输出）

```
$ grep -rn "api/invoke\|EventSource(\|--web" --include="*.go" --include="*.html" .   # 无命中
$ ls internal/ui internal/webui                                                      # No such file or directory
$ go build ./...                                                                     # 退出 0
$ go vet ./...                                                                       # 退出 0
$ go test ./...                                                                      # 14 个包全绿（358 个 Test 函数）
$ bash e2e/run_e2e.sh                                                                # E2E RESULT: PASS (12s)
$ go build -o bin/beacon-desktop ./desktop                                           # 12 MB
```

`TestSingleSurface` 把「不许再长回来」写成了断言：shell 里出现 `/api/invoke`、
`EventSource(`、`/api/events` 任一字符串即失败；`internal/ui`、`internal/webui`
任一目录重新出现也失败。

## 5. 未做 / 待办

1. **上游 truth change**：本仓冻结包已收口（`agent-runtime-dual-mode`、`agent-desktop-workspace`、
   `agent-session-streaming` 均按 change → 重写 → refreeze 重写为单一桌面形态）；
   源仓只读镜像仍待其正式 `gen-change` 合并本记录。
2. **AC-025 仍未处理**：12 个非测试文件超 500 行。
3. **未提交**：`git commit` 需人工确认。
