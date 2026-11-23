# Evidence Index: qa-surface-e2e

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | frozen | truth |
| tests | test_truth | tests.md | frozen | qa |
| tasks | support_advisory | tasks.md | present | truth |

## 当前源码事实（HEAD 29073f0）

| 事实 | 位置 | 观察 |
|------|------|------|
| 项目类型只有四值 | qastrategy.go:30 | 没有 desktop-app |
| Electron 先落 web-app | qastrategy.go:324 | InferProjectType 的顺序是 skill、web-app、api-service、sdk-lib |
| runner 只有五档 | qastrategy.go:134 | ego-browser 与 agent-desktop 落到 playwright |
| 登记白名单缺三类证据 | artifacts.go:50 | 没有 a11y_tree / browser_trace / e2e_report |
| qa 门只有三项检查 | evaluate.go:258 | 注释原话 "It never runs tests itself" |
| qa 事实不读 autotest 与 surface | gates_facts.go:193 | 只枚举 QA-kind artifact 与 stage_executed |
| frontmatter 只收集标量 | integrity.go:108 | surfaces 列表没有消费者 |
| 兼容性矩阵只有四支 | qastrategy.go:351 | 没有 desktop-app 列 |

## Reproduction Log

本阶段未运行 seed，也未跑 `go test`：派单要求 truth 未冻结前不写实现、不跑实现验证。上表全部来自对 HEAD 29073f0 源码的直接阅读与行号定位，命令为 `git rev-parse HEAD` 与定点读文件。

实现阶段必须补齐 tests.md 的 TC-SUR-001 到 TC-SUR-018，并在真实包夹具上落一份 PASS 与一份 FAIL 的 evaluate JSON。

## Boundary

本文件是索引与只读事实表，不承载最终 verdict、门禁真值或发布就绪判断。证据产物落地后再替换为已登记的全路径。

