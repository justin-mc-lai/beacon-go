# Tests Truth: agent-desktop-workspace

> **Feature Slug**: `agent-desktop-workspace`  
> **Version**: `v2.0.0`

---

## 覆盖矩阵 (Coverage Matrix)

| 用例 ID | 对应 AC | 测试描述 | 检验命令 / 断言 |
|---|---|---|---|
| **TC-DESKTOP-001** | AC-DESKTOP-001 | 验证三栏前端模板完整包含 Rooms、Step Plan 与 Inspector | `go test -run TestConsoleThreeColumnLayout ./internal/ui/...` 验证 HTML 模板包含三栏结构与对应容器 |
| **TC-DESKTOP-002** | AC-DESKTOP-002 | 验证后端真实 Feature 扫描与详情读取 API | `go test -run TestWorkspaceFeaturesAPI ./internal/ui/...` 请求 `/api/workspace/features` 返回真实 feature 列表与 truth 正文 |
| **TC-DESKTOP-003** | AC-DESKTOP-003 | 验证 Wails 桌面端配置规范与入口编译 | `go test -run TestWailsConfigValid ./internal/ui/...` 校验 `wails.json` 结构完整合法 |
