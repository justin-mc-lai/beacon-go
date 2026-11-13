---
slug: restatement-gate
confirmed_by: justin-mc-lai (confirmed 2026-11-13 in chat, scope = restatement-gate plan)
confirmed_at: 2026-11-13T04:39:57Z
hash: 51f4871f72d08fa8ae89f15facdea105c8808f7b1941ca6cf28a196fd53c8694
---
## 1 交付形态

内核 `beacon-go` 新增一个纯解析器（`internal/gate/restatement.go`）、一处冻结前校验（`internal/orchestrator/truth.go` 的 `freezePrecheck`）、一组 reason code 常量和 `truth show` 的一个只读字段。不新增顶层命令，不改 `dispatch.sh`，不引入 design 阶段。

`restatement.md` 与 truth 四件套同目录，frontmatter 四键：`slug`、`confirmed_by`、`confirmed_at`、`hash`；正文固定五段：交付形态、界面与交互、业务流转、边界与不做、歧义与默认值。

单测随实现落地：五个负例 + 两个正例，`go test ./...` 必须全绿。实现完成后只做本地 commit。

## 2 界面与交互

只有 CLI 面，没有网页或桌面改动。

`truth freeze` 首次冻结失败时非 0 退出，`block_reasons` 与 findings 带对应 reason code；成功时输出既有 frozen 视图。

`truth show --json` 增加 `restatement` 块：`path`、`confirmed_by`、`confirmed_at`、`hash`、`matches`；`matches` 只表示正文现算 hash 与 frontmatter 一致。没有 restatement.md 的老包省略该块，其余 JSON 字段不变。

五个 reason code 是稳定的契约字符串，内核代码只引用常量。

## 3 业务流转

## 3.1 首次冻结

顺序固定：四件套非空 → surface 声明 → restatement 校验 → `EvaluateIntegrity` → intent gate。

restatement 四类拒绝：文件不存在 → `restatement_required`；`confirmed_by` 为空 → `restatement_unconfirmed`；现算正文 sha256 与 frontmatter `hash` 不一致 → `restatement_stale`；五段标题缺失 → `restatement_sections_missing`。

AC 表格行可写 `restates=<节号>`；节号必须落在五段或子标题里（如 `3.2`），找不到或缺失 → `ac_not_traceable_to_restatement`。

## 3.2 重新冻结（refreeze）

已经冻结过的包沿用 legacy waiver：refreeze 不强制 restatement，没有该文件也照旧通过，只记 warning；surfaces 的规则保持不变。

## 3.3 老包读取

没有 restatement 的老包在 `truth show` 与 AC 追溯中都不报错、不补文件、不升级成 blocked，只 warn。

## 4 边界与不做

不改 `dispatch.sh` 的 `--restatement` 自由文本本身；门只落在 truth freeze 侧。

不动 truth/design/implement/qa/release 的既有语义，不新增顶层命令，不改发布流程。

不 push、不安装 `~/.local/opt/beacon-go/beacon`、不改版本号与对外发布说明；这些由人复核后决定。

## 5 歧义与默认值

正文 hash 按 frontmatter 结束 `---` 之后的原始字节计算，不做 trim、不做换行归一；确认后改一个字即 stale。

五个二级标题按前缀匹配，允许 `## 3.2` 子标题；只要求存在，不要求顺序。

`confirmed_by` 非空即视为已确认，格式不限（姓名、账号、代理均可）；`confirmed_at` 只展示，不参与校验。

有 restatement 时每条 AC 至少要有一个 `restates=`；缺失按不可追溯处理。老包没有 restatement 时不校验。
