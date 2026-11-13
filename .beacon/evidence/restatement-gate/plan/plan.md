# Plan: restatement-gate

mode_id: pln-source
run_id: run_5c0e4bc40869e39e5ae0c70a
feature: restatement-gate
version: v2.0.0
baseline: 4dc8c57 (main, clean)

## 1 已核对的事实（读代码，不凭印象）

| 事实 | 位置 |
|------|------|
| `freezePrecheck` 现有顺序：四件套 → surface 校验 → `EvaluateIntegrity` → intent gate | `internal/orchestrator/truth.go:369-444`（surface 399-417，integrity 418） |
| 首次冻结判定是 `recs.freeze == nil`，与 surfaces 的 legacy waiver 同源 | `internal/orchestrator/truth.go:625` |
| surface 解析/AC 行 token 的正则先例（`^\|\s*(AC-\d{3})\s*\|`、`surface=`） | `internal/gate/surface.go:69,86-...` |
| integrity 只认四件套与 truth 正文结构，不认未知附加文件；`restatement.md` 不会影响四件套判定 | `internal/gate/integrity.go` |
| `truth show --json` 直接序列化 `TruthPackageView` | `internal/cli/truth.go:13-...`、`internal/orchestrator/truth.go:60-...` |
| 现有代码里没有 restatement 门，只有旁注 | `grep -rn restatement --include='*.go'` 仅命中注释 |
| 冻结会重写 frontmatter（status/materials_status），不改正文 | `internal/orchestrator/truth.go:446-...`、`materials.go` |

## 2 决策

1. **解析器放 `internal/gate/restatement.go`**，与 `surface.go` 同层同风格：纯解析、无 I/O 之外依赖，返回 `Restatement{Path, Slug, ConfirmedBy, ConfirmedAt, Hash, BodySHA256, Sections, Matches}`。
2. **正文定义**：frontmatter 结束 `---` 之后的全部字节（保留首部换行），逐字节 sha256，不做 trim/换行归一。
3. **五段匹配**：按行前缀 `## 1` … `## 5`，允许 `## 3.2` 子标题；只要求存在，不要求顺序。
4. **冻结预检插入点**：surface 校验之后、`EvaluateIntegrity` 之前；失败即返回 blocked 视图，沿既有 `freezeBlockedError` 路径非 0 退出。
5. **refreeze 豁免**：复用 `requireSurfaces` 同一个 `recs.freeze == nil` 事实，不新增第二套判定。
6. **AC 追溯**：AC 行内 `restates=<节号>`，支持重复 token；有 restatement 时每行至少要有一个 token，节号解析不到（或缺失）即 `ac_not_traceable_to_restatement`；无 restatement 的老包只 warn。
7. **视图**：`Restatement *RestatementView` + `omitempty`，老包 JSON 不变。
8. **reason codes**：五个常量进 `internal/core/reasons.go`，orchestrator/CLI 只引用常量。

## 3 AC 拆分（编号见 truth.md）

- AC-001 工件与解析器（四键 + 正文 hash + 五段/子标题）
- AC-002 首次冻结四连拒（required / unconfirmed / stale / sections_missing）且顺序在 surface 之后、integrity 之前
- AC-003 首次冻结正例 + refreeze legacy waiver
- AC-004 AC 可追溯（`restates=`；老包只 warn）
- AC-005 `truth show --json` restatement 块
- AC-006 五个 reason code 常量
- AC-007 单测五负例 + 两正例，`go test ./...` 全绿
- AC-008 只加不改、本地 commit、不 push 不安装

## 4 明确不做

- 不改 `dispatch.sh` 与 `--restatement` 自由文本本身（门落在 truth freeze 侧）。
- 不动 truth/design/implement/qa/release 既有语义，不新增顶层命令。
- 不 push、不安装 `~/.local/opt/beacon-go/beacon`、不改版本号与发布说明。

## 5 风险

| 风险 | 缓解 |
|------|------|
| 老包（无 restatement）被新门误伤 | refreeze 走 legacy waiver；AC 追溯对老包只 warn；单测 TC-008 覆盖 |
| hash 定义歧义导致「确认后永远 stale」 | 正文按 `---` 之后原始字节计算，测试同时覆盖一致/改一字 |
| surface 与 restatement 同时失败时顺序错 | 单测断言 surface reason 先出 |

## 6 下一步

next: truth（本稿即 plan 证据；冻结后停手，等放行 implement）
