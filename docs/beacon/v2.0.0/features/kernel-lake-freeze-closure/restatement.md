---
slug: kernel-lake-freeze-closure
confirmed_by: justin (chat 2026-11-19: “调用 /qi-dev-openmausbot 派活 beacon-go 内核群 修复下这个 issue”)
confirmed_at: 2026-11-19T05:20:00Z
hash: d145bb300a949884ccb7dee74a21688f0aaba74dd646ea0caf1343932491feb1
---
## 1 交付形态

内核三处改动：合同指纹与 evidence 分离（`internal/gate/pkgvalidate.go`、`internal/orchestrator/truth.go` 的 surface 指纹计算）、`artifact_refs` 支持 feature 作用域（`internal/orchestrator/artifacts.go` + `internal/cli/goal.go` 的 register 命令）、`truth freeze/refreeze` 预检失败写 feature 维度事件（`internal/orchestrator/truth.go` + 事件 storer）。配套单测；不改产品仓。

## 2 界面与交互

`beacon truth freeze/refreeze`：evidence-only 变更不再报 drift。
`beacon goal artifacts register --feature <slug>`：无 run 也能登记。
`beacon goal trace --feature <slug>`（或 spine 等阅读面）能看到预检失败事件。
既有 JSON 字段只增不改；不新增顶层命令。

## 3 业务流转

### 3.1 全冻

湖 truth 冻结后，实现期补 evidence 不使冻结失效（仍校验 artifact 登记与文件存在）。合同面四件套变更仍走 change→refreeze。

### 3.2 登记

feature 作用域登记优先满足证据引用；run 作用域登记继续有效；两者命中其一即通过。

### 3.3 预检可观测

freeze/refreeze 预检失败（surface、restatement、evidence 未登记等）写 `truth_precheck_failed` 事件，payload 带 feature_id、reason codes、文件与行号。

## 4 边界与不做

不取消 restatement 与 intent 签名；不降低 gate 严格度；不改 release 人工门；不动产品仓与 OpenMausBot；不做自动发布说明。

## 5 歧义与默认值

"evidence-only" 以文件路径集合判定：只改 evidence.md 不算合同面变更；其余四件套任一变化算。
事件写失败不得阻断 freeze 主流程，stderr 告警即可，不静默。
feature 与 run 作用域同时登记同一文件时视为同一登记，spine 标注来源。
