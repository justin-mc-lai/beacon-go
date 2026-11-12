---
slug: qa-surface-e2e
version: v2.0.0
status: frozen
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: filled
materials_status: current
intent_confirmed: true
canonical_refs:
  truth: docs/beacon/v2.0.0/features/qa-surface-e2e/truth.md
  tests: docs/beacon/v2.0.0/features/qa-surface-e2e/tests.md
  tasks: docs/beacon/v2.0.0/features/qa-surface-e2e/tasks.md
  evidence: docs/beacon/v2.0.0/features/qa-surface-e2e/evidence.md
---

# Requirement Truth: qa surface e2e (R1-draft)

## 人话

qa 门现在只核对三件事：有没有 QA 产物指针、有没有 qa stage 执行记录、产物哈希是否匹配；它从不执行测试，也没有任何“界面”维度。一个 Electron 应用可以全程只跑 Node 脚本、零次界面验收，qa 照样 PASS。本 feature 在 requirement truth 里增加 `surfaces` 声明（api / web / desktop / cli），把 `desktop-app` 变成 qa9 的一等项目类型，让 ui-runner 认 `ego-browser` 与 `agent-desktop` 两个 runner，收口 `a11y_tree` / `browser_trace` / `e2e_report` 三类桌面与浏览器证据，并让 `EvaluateQA` 对“没有 autotest 记录”和“声明面缺证据”fail-closed；已冻结的老包在 `surface_strict: false` 下只降级为 warn，不回溯判 FAIL。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "qa 门只查产物指针、stage 记录和哈希，不跑测试也没有界面维度；给 truth 加 surfaces 声明、加 desktop-app、加 ego-browser/agent-desktop runner、加 a11y_tree/browser_trace/e2e_report 证据类型，并让 EvaluateQA 对 autotest 与 surface 证据 fail-closed"
goal: "beacon-go 的 qa 面在 requirement truth 中显式声明 api/web/desktop/cli surface；desktop-app 可被 InferProjectType 识别；ego-browser 与 agent-desktop 生成 qae2e 命令；QA 证据类型带扩展名校验；EvaluateQA 缺 autotest 记录或缺失声明面证据时 FAIL，老包 surface_strict=false 只 warn"
non_goals: "不实现 qae2e 执行器；不重写 qastrategy.go 矩阵结构；不改 truth/design/implement/release 四门语义；不回溯判历史包 FAIL；不 push；不安装到 ~/.local/opt/beacon-go"
constraints: "Go 内核；只加 qa 面；新增行为必须有单测；go test ./... 全绿；commit 只到本地；不安装"
scope_mode: focused
```

## 起点现场

- 单一真源为 beacon-go 仓库 `/Users/apple/Developer/Personal/products/beacon-go`，HEAD `29073f0`，开工时 `git status --short` 为空。
- 本 feature 包位于 `docs/beacon/v2.0.0/features/qa-surface-e2e/`；本阶段只写这四个文档，不碰生产代码。
- 相关实现位置：`internal/gate/qastrategy.go`（项目类型、矩阵、命令预设）、`internal/gate/evaluate.go`（EvaluateQA）、`internal/orchestrator/gates_facts.go`（evalQA）、`internal/orchestrator/artifacts.go`（登记白名单）、`internal/gate/integrity.go`（frontmatter 解析）、`internal/orchestrator/truth.go`（freeze precheck）。
- 本包自身不带 `surfaces` 键：现有 freeze 的 frontmatter 解析不识别该键，按派单纯度先记「本包 surface 声明待实现后回填」，实现落地后由 change/refreeze 回填并实测 strict。

## 当前行为（HEAD 29073f0，源码与实测）

| ID | 实测事实 |
|----|----------|
| F1 | `QAProjectTypes` 只有 `ai-agent-skill / web-app / api-service / sdk-lib` 四个值，没有 `desktop-app`（qastrategy.go:30）。 |
| F2 | `InferProjectType` 的判断顺序是 skill → web-app → api-service → sdk-lib；带 `package.json` 的 Electron 项目会先落进 `web-app`（qastrategy.go:324）。 |
| F3 | `DefaultCommandForQAType` 的 functional 分支只认 playwright / cypress / browser-use / agent-browser / midscene；`ego-browser` 与 `agent-desktop` 落到 playwright 默认命令（qastrategy.go:134）。compatibility 分支同样只有这五档。 |
| F4 | `registerableKinds` 只有 qa / scorecard / test_result / test_results / evidence / implement_artifact / report / log / artifact，`ValidRegisterKind` 只额外放行 `qa_*`；没有 `a11y_tree` / `browser_trace` / `e2e_report`（artifacts.go:50）。 |
| F5 | `EvaluateQA` 只产生 `qa_artifacts_present`、`qa_stage_executed`、`qa_artifact_refs_valid` 三项检查，源码注释原话是 "It never runs tests itself"（evaluate.go:258）。 |
| F6 | `evalQA` 只枚举 QA-kind artifact 与 `stage_executed` 事件，不读取 `.beacon/autotest/<run-id>/`，也不读取 truth 的 surface 声明（gates_facts.go:193）。 |
| F7 | truth frontmatter 解析 `ParseFrontmatter` 只收集标量键值，`surfaces` 这种列表没有消费者；freeze precheck 只查四件套完整性、integrity 与 intent gate（truth.go:353）。 |
| F8 | `truth show` 的视图没有 surfaces / surface_strict 字段；`gate status` 没有 `surface_undeclared` 警告。 |
| F9 | qa9 兼容性矩阵 `QAApplicability` 只有 ai-agent-skill / web-app / api-service / sdk-lib 四支，desktop 项目没有专属列（qastrategy.go:351）。 |

## Scope

### In scope

- truth frontmatter 增加 `surfaces` 列表与 `surface_strict` 布尔；AC 行支持 `surface=` 列并在缺省时继承。
- `truth freeze` 校验 surfaces 非空、枚举合法、strict 时每条 AC 的 surface 落在声明列表内；新冻包自动 strict。
- 老包兼容：缺 surfaces 的已冻结包不回溯判 FAIL，只由 `gate status` 给一条 `surface_undeclared` warn。
- `QAProjectTypes` 增加 `desktop-app`；`InferProjectType` 识别 electron / tauri / appId / entitlements 并优先于 web-app。
- `QAApplicability` 增加 desktop-app 列：unit/interface/smoke/functional/regression/exception required，integration/compatibility conditional，contract na。
- `DefaultCommandForQAType` 的 functional 与 compatibility 分支认 `ego-browser` 与 `agent-desktop`，生成 qae2e 命令。
- artifact kind 白名单增加 `a11y_tree` / `browser_trace` / `e2e_report`，并在登记时校验扩展名。
- `EvaluateQA` 增加 `qa_autotest_recorded` 与 `qa_surface_evidence` 两条 fail-closed 检查；`surface_strict: false` 时降级为 `legacy_waiver_surface` warn。

### Out of scope

- 实现 `qae2e` 执行器本身（由 `qi-dev-qa-e2e` 另一张票提供）；内核只认 runner 名并生成命令。
- 重写 qastrategy.go 的矩阵结构；只在既有表里加列与加项。
- 修改 truth/design/implement/release 四门的既有语义。
- 对历史已冻结包做回溯 refreeze 或把缺 surface 判成 FAIL。
- push origin、发布版本号、安装到 `~/.local/opt/beacon-go`。

## Execution Contract

### surface 声明

feature 的 `truth.md` frontmatter 增加两个键：

```yaml
surfaces: [api, web]        # 多选，枚举 api | web | desktop | cli
surface_strict: true        # 新冻结自动置 true；老包缺省 false
```

AC 表格每行可增加一个 `surface=<value>` 列；未写该列的 AC 继承 `surfaces` 列表。解析后的 surface 集合必须同时出现在 `truth show --json` 与冻结记录 payload 中，供 qa 门读取。

### freeze 校验

| 检查 | 条件 | 结果 |
|------|------|------|
| surfaces 非空 | `surfaces` 缺失或为空数组 | 拒绝，reason `truth_surface_required` |
| 取值合法 | 任一值不在 api / web / desktop / cli 内 | 拒绝，reason `truth_surface_invalid` |
| AC 归属 | `surface_strict: true` 且某条 AC 的 surface 不在 surfaces 列表内 | 拒绝，reason `truth_ac_surface_undeclared` |
| 自动 strict | 新 freeze 且未显式写 `surface_strict` | 冻结记录写 `surface_strict: true` |

老包兼容：已冻结包不改写、不 refreeze、不判 FAIL；`gate status` 对缺 `surfaces` 的包追加一条 `surface_undeclared` warn，包仍显示 frozen。显式写 `surface_strict: false` 的包在 qa 门只走 legacy waiver。

### desktop-app 项目类型

`QAProjectTypes` 增加 `desktop-app`。`InferProjectType` 在 `ai-agent-skill` 之后、`web-app` 之前识别以下信号：`package.json` 的 dependencies 或 devDependencies 含 `electron`；存在 `tauri.conf.json` 或 `src-tauri/` 目录；`package.json` 含 `"build"\s*:\s*\{\s*"appId"`；项目内存在 `*.entitlements` 文件。命中任一信号即返回 `desktop-app`。

`QAApplicability("desktop-app")` 返回：

| QA 类型 | 适用性 |
|---------|--------|
| unit / interface / smoke / functional / regression / exception | required |
| integration / compatibility | conditional |
| contract | na |

与 web-app 的差异写在代码注释里：desktop-app 的 functional 验收走原生窗口与无障碍树，contract 不适用；web-app 的 contract 为 conditional，compatibility 为 required。

### e2e runner 命令预设

functional 与 compatibility 两个分支新增：

| runner | 生成命令 |
|--------|----------|
| `ego-browser` | `qae2e run --surface web --spec .beacon/e2e/<feature>.json` |
| `agent-desktop` | `qae2e run --surface desktop --spec .beacon/e2e/<feature>.json` |

其他 runner 名保持既有分支；未知 runner 继续落 playwright 默认命令，不报错。

### artifact kind 与扩展名

QA 类 artifact 的 kind 增加 `a11y_tree`、`browser_trace`、`e2e_report`，三者都计入 `qaKind()` 的 QA 证据集合。登记时按扩展名校验：

| kind | 允许扩展名 |
|------|------------|
| `a11y_tree` | `.json` |
| `browser_trace` | `.zip`、`.json` |
| `e2e_report` | `.json`、`.md` |

扩展名不匹配时注册失败，reason `artifact_kind_extension_not_allowed`，不写 artifact_refs，不追加 `artifact_registered` 事件。

### EvaluateQA 两条硬门

`QAFacts` 增加 autotest 记录、声明面列表、strict 标记与每个面的证据命中信息。`EvaluateQA` 在既有三项检查之后追加：

| 检查 | 规则 | strict 缺失结果 |
|------|------|-----------------|
| `qa_autotest_recorded` | 该 run 存在非空 `.beacon/autotest/<run-id>/` 目录，或登记了路径位于该目录下的 autotest artifact | FAIL，reason `qa_autotest_recorded_missing` |
| `qa_surface_evidence` | truth 声明的每个面都有对应证据：api→`e2e_report`、web→`browser_trace`、desktop→`a11y_tree`、cli→`e2e_report` | FAIL，reason `qa_surface_evidence_miss:<surface>` |

`surface_strict: true` 且 `surfaces` 为空时按 `qa_surface_evidence_miss:surfaces_undeclared` 失败关闭。任一 FAIL 让 verdict 为 fail；warn 不影响 verdict。

legacy waiver：`surface_strict: false` 时上述两条检查只产生 warn，severity 为 warn，reason code 为 `legacy_waiver_surface`，不出现 fail reason code。

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 新冻包的 truth frontmatter 声明 `surfaces: [api, web]` 时 `truth freeze` 退出 0，`truth show --json` 返回 `surface_strict=true` 与同一列表；声明 `surfaces: [api, grpc]` 时 freeze 退出非 0 且 reason 为 `truth_surface_invalid`；声明空列表时 reason 为 `truth_surface_required`。 |
| AC-002 | `surface_strict: true` 的包中，某条 AC 行写 `surface=desktop` 而 surfaces 为 `[api, web]` 时 freeze 退出非 0 且 reason 为 `truth_ac_surface_undeclared`；AC 行不写 surface 时该 AC 继承 surfaces 列表，并出现在 `truth show --json` 的 AC surface 字段。 |
| AC-003 | 已冻结且 frontmatter 缺 `surfaces` 的老包不做回溯改写：`truth show` 仍为 frozen，`gate status --json` 只追加一条 `surface_undeclared` 警告，该包的 truth 哈希不变。 |
| AC-004 | Electron 夹具目录（package.json 依赖 electron 且含 `"build":{"appId"`）调用 `InferProjectType` 返回 `desktop-app`；`src-tauri/tauri.conf.json` 夹具与含 `*.entitlements` 文件的夹具同样返回 `desktop-app`；同时带 web 前端标记的 Electron 夹具仍返回 `desktop-app`。 |
| AC-005 | `QAProjectTypes` 包含 `desktop-app`；`QAApplicability("desktop-app")` 返回 unit/interface/smoke/functional/regression/exception 为 required，integration 与 compatibility 为 conditional，contract 为 na；Electron 夹具的 `beacon qa strategy --json` 的 `project_type` 为 `desktop-app`。 |
| AC-006 | `beacon qa strategy --types functional --ui-runner ego-browser --json` 的 functional preset 等于 `qae2e run --surface web --spec .beacon/e2e/<feature>.json`；`--ui-runner agent-desktop` 等于 `qae2e run --surface desktop --spec .beacon/e2e/<feature>.json`；同一 runner 在 compatibility 分支生成同一条 qae2e 命令。 |
| AC-007 | `RegisterArtifact` 接受 `a11y_tree` 的 `.json`、`browser_trace` 的 `.zip` 与 `.json`、`e2e_report` 的 `.json` 与 `.md`；`a11y_tree` 传 `.txt`、`browser_trace` 传 `.md`、`e2e_report` 传 `.zip` 时注册失败，reason 为 `artifact_kind_extension_not_allowed`，不写入 artifact_refs，不追加 `artifact_registered` 事件。 |
| AC-008 | `surface_strict: true` 且 `surfaces: [api, desktop]` 的包，在只有 `e2e_report` 证据时 `beacon gate evaluate qa --json` 返回 fail，reason 含 `qa_surface_evidence_miss:desktop`；补登 `a11y_tree` 后同一夹具返回 pass；web 面缺 `browser_trace` 时 reason 为 `qa_surface_evidence_miss:web`。 |
| AC-009 | `surface_strict: true` 的包，在 qa stage 已执行且有 QA 产物、但没有 `.beacon/autotest/<run-id>/` 目录且没有登记 autotest 产物时，`gate evaluate qa` 返回 fail 且 reason 为 `qa_autotest_recorded_missing`；该 run 目录存在且非空时同一夹具返回 pass。 |
| AC-010 | `surface_strict: false` 的包在 AC-008 与 AC-009 的同一缺失输入下 `gate evaluate qa` 返回 pass，`qa_autotest_recorded` 与 `qa_surface_evidence` 为 warn，warn 的 reason code 为 `legacy_waiver_surface`，不出现任何 fail reason code。 |

## Intent Coverage

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | conversation:派单第1项 | conversation:派单1 |
| AC-002 | conversation:派单第1项 | conversation:派单1 |
| AC-003 | conversation:派单第1项 | conversation:派单1 |
| AC-004 | conversation:派单第2项 | conversation:派单2 |
| AC-005 | conversation:派单第2项 | conversation:派单2 |
| AC-006 | conversation:派单第3项 | conversation:派单3 |
| AC-007 | conversation:派单第4项 | conversation:派单4 |
| AC-008 | conversation:派单第5项 | conversation:派单5 |
| AC-009 | conversation:派单第5项 | conversation:派单5 |
| AC-010 | conversation:派单第5项 | conversation:派单5 |

## Non-negotiable constraints（不得放宽）

| # | Constraint | 现有兜底校验点 |
|---|---|---|
| N1 | 不改 truth/design/implement/release 四门既有语义，只加 qa 面 | 既有门禁单测 |
| N2 | 不重写 qastrategy.go 矩阵结构，只加列与加项 | qastrategy 既有单测 |
| N3 | 新增行为必须有单测；`go test ./...` 全绿 | 仓库 CI 口径 |
| N4 | 新增栏位与证据只读；老包不回溯判 FAIL | 冻结基线 + legacy waiver |
| N5 | commit 只到本地，不 push origin；不安装到 `~/.local/opt/beacon-go` | 派单硬约束 |

## Stop Line

本阶段只到 truth 打包与 freeze：四件套齐全，不写业务代码、不跑 `go test`、不写 QA verdict。实现阶段只允许改 `internal/**` 与 `cmd/**`，并保持四条约束（N1 / N2 / N3 / N5）。
