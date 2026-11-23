---
slug: qa-surface-e2e
version: v2.0.0
materials_status: current
---

# Tests: qa-surface-e2e

所有命令都在 beacon-go 仓库根执行。本 feature 的测试分两层：`internal/gate` 覆盖 surface 声明、项目类型、矩阵、runner 命令与 EvaluateQA；`internal/orchestrator` 覆盖 freeze/status 写读、artifact 登记与真实 qa 门夹具。

## TC 矩阵

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-SUR-001 | behavior | `go test ./internal/gate -run TestTruthSurfaceFreezeValid -count=1` | freeze 退出 0；show --json 的 surfaces 与输入一致；surface_strict 为 true |
| AC-001 | TC-SUR-002 | negative | `go test ./internal/gate -run TestTruthSurfaceFreezeRejectsInvalid -count=1` | freeze 非 0；reason 为 truth_surface_invalid；包仍为 draft |
| AC-001 | TC-SUR-003 | negative | `go test ./internal/gate -run TestTruthSurfaceFreezeRejectsEmpty -count=1` | freeze 非 0；reason 为 truth_surface_required；包仍为 draft |
| AC-002 | TC-SUR-004 | negative | `go test ./internal/gate -run TestTruthACSurfaceUndeclared -count=1` | AC 的 surface=desktop 不在 [api, web] 内时 freeze 非 0；reason 为 truth_ac_surface_undeclared |
| AC-002 | TC-SUR-005 | behavior | `go test ./internal/gate -run TestTruthACSurfaceInheritance -count=1` | 无 surface 列的 AC 继承 [api, web]；show --json 的 ac_surfaces 与冻结记录一致 |
| AC-003 | TC-SUR-006 | guard | `go test ./internal/orchestrator -run TestGateStatusSurfaceUndeclaredWarn -count=1` | 老包仍为 frozen；truth sha256 不变；gate status 只追加一条 surface_undeclared warn |
| AC-004 | TC-SUR-007 | behavior | `go test ./internal/gate -run TestInferDesktopProjectType -count=1` | electron、tauri、appId、entitlements 四种夹具都返回 desktop-app |
| AC-004 | TC-SUR-008 | guard | `go test ./internal/gate -run TestInferDesktopProjectTypeBeatsWeb -count=1` | 同时带 web 前端标记的 Electron 夹具仍返回 desktop-app |
| AC-005 | TC-SUR-009 | behavior | `go test ./internal/gate -run TestDesktopApplicabilityMatrix -count=1` | 六类 required、两类 conditional、contract na；与 web-app 的差异只在 contract 与 compatibility |
| AC-005 | TC-SUR-010 | behavior | `go test ./internal/gate -run TestQAStrategyDesktopProject -count=1` | Electron 夹具的 strategy.project_type 为 desktop-app，presets 含 functional |
| AC-006 | TC-SUR-011 | behavior | `go test ./internal/gate -run TestE2ERunnerPresetsFunctional -count=1` | ego-browser 与 agent-desktop 的 functional preset 精确等于两条 qae2e 命令 |
| AC-006 | TC-SUR-012 | behavior | `go test ./internal/gate -run TestE2ERunnerPresetsCompatibility -count=1` | compatibility 分支生成同一组 web 与 desktop qae2e 命令 |
| AC-007 | TC-SUR-013 | behavior | `go test ./internal/orchestrator -run TestArtifactSurfaceKindAccepts -count=1` | a11y_tree 的 json、browser_trace 的 zip 与 json、e2e_report 的 json 与 md 均登记成功 |
| AC-007 | TC-SUR-014 | negative | `go test ./internal/orchestrator -run TestArtifactSurfaceKindRejects -count=1` | 三种错扩展名都失败；reason 为 artifact_kind_extension_not_allowed；artifact_refs 与 artifact_registered 事件均未增加 |
| AC-008 | TC-SUR-015 | negative | `go test ./internal/gate -run TestEvaluateQASurfaceEvidenceGate -count=1` | 缺 desktop 时 fail 且 reason 含 qa_surface_evidence_miss:desktop；补 a11y_tree 后 pass；缺 web 时 reason 为 qa_surface_evidence_miss:web |
| AC-009 | TC-SUR-016 | negative | `go test ./internal/gate -run TestEvaluateQAAutotestGate -count=1` | 无 autotest 记录时 fail 且 reason 为 qa_autotest_recorded_missing；目录非空时 pass |
| AC-010 | TC-SUR-017 | guard | `go test ./internal/gate -run TestEvaluateQALegacySurfaceWaiver -count=1` | 同一缺失输入下 verdict 为 pass；两条检查为 warn；warn reason 为 legacy_waiver_surface；无 fail reason |
| AC-008 | TC-SUR-018 | behavior | `go test ./internal/orchestrator -run TestSurfaceEvaluateRealPackage -count=1` | 用隔离 store 造真实包跑 evaluate，分别落一份 PASS 与一份 FAIL 的 JSON，字段与 reason 可复核 |

## 当前实测

本阶段只到 truth freeze，测试函数尚未落地，不把 `-run` 的零匹配结果计为通过。当前可证的源码事实见 truth.md 的 F1-F9：EvaluateQA 只有三项检查（evaluate.go:258），QAProjectTypes 只有四值（qastrategy.go:30），functional/compatibility 只认五个 runner（qastrategy.go:134），登记白名单没有三类新证据（artifacts.go:50）。

## 负向断言

- 非法 surface 值不得落盘为 frozen；失败后包仍为 draft，truth 正文一个字节不改。
- AC 的 surface 不在 surfaces 内时不得靠缺省继承掩盖；strict 包必须拒绝。
- 老包缺 surfaces 不得被改写、不得被 refreeze、不得被判 FAIL；只允许 gate status 的 warn。
- `a11y_tree` 的 `.txt`、`browser_trace` 的 `.md`、`e2e_report` 的 `.zip` 必须在写 artifact_refs 之前被拒绝，且不追加 `artifact_registered`。
- strict 包缺 autotest 记录或缺声明面证据时必须 fail；legacy waiver 只降 warn，不得让任何 fail reason code 出现。
- 不新增 qae2e 执行器实现；runner 只生成命令字符串。

## 修复后行为契约

| 场景 | verdict | 关键 reason |
|------|---------|-------------|
| strict 包，autotest 与各面证据齐全 | pass | 无新增 fail reason |
| strict 包，无 autotest 记录 | fail | qa_autotest_recorded_missing |
| strict 包，声明 desktop 但只有 e2e_report | fail | qa_surface_evidence_miss:desktop |
| strict 包，声明 web 但只有 a11y_tree | fail | qa_surface_evidence_miss:web |
| legacy 包（surface_strict: false），同上缺失 | pass | 仅 legacy_waiver_surface warn |
| 老冻包缺 surfaces | frozen 不变 | gate status 一条 surface_undeclared warn |

