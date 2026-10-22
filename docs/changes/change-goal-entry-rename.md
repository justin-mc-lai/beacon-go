# 变更记录：用户入口 loop → goal（Go 轨）

> 变更编号：`change-goal-entry-rename`
> 日期：2026-10-22（P3a 落账）
> 类型：入口命名变更（gen-change 要素齐备的变更记录；**不是**对上游冻结包的 gen-change 执行，见文末状态）
> 决策来源：用户 2026-10-21 拍板，口径稿 `docs/scope-charter.md` 第 6 节第 5 条，选 A（全按推荐）。
> 受影响冻结包：`beacon-v2-rust-kernel-redesign`（truth.md，status: frozen，revision: r19-scenario-completeness-s01-s19），
> 位于只读镜像 `~/workspace/repos/beacon/docs/beacon/archive/v2.0.0/features/beacon-v2-rust-kernel-redesign/`。

---

## 1. 变更内容

- Go 轨的用户入口命令统一叫 **`goal`**（`beacon goal …`，23 个子命令沿旧版用户习惯挂在其下），不叫冻结设计里的 `loop`。
- `beacon loop` 只保留为**指针命令**：打印一行指向 `beacon goal` 的提示，无任何自身行为、无别名逻辑（防长胖：不养第二套入口）。
- 六个一等能力在 Go 轨的对外说法随之写成：**goal / truth / design / implement / qa-evidence / release**。
- 不变的：内部包名仍为 `orchestrator`（core/store/orchestrator/gate/cli/harness 不改名）；Loop Engineering 三层 Loop（Coding Loop / Developer Feedback Loop / Real-world Loop）是**概念名**，不是入口名，不受本变更影响；`loop-goal` 状态机、spine `loop_layer` 等内部术语不改名。

## 2. 理由

1. 用户习惯在 `goal` 上：旧版 23 个子命令全挂在 goal 底下，全员改口成本高且无收益（口径稿第 6 节第 5 条原案）。
2. 冻结设计写 `loop` 是能力分类名，不是用户已形成的肌肉记忆；Go 轨是新二进制、没有存量 `loop` 命令用户。
3. 只留指针不留别名行为，避免旧版「别名越留越多、入口从 37 条长到 77 条」的覆辙（口径稿防长胖条款）。

## 3. 影响的冻结条目（逐条）

冻结包 `truth.md`（r19）中与 `loop` 入口命名直接相关的条目：

| 条目 | 冻结原文要点 | 本变更的影响 |
|---|---|---|
| 概述（crate 图） | `beacon-cli -> Rust 主 CLI：loop / truth / design / implement / qa-evidence / release` | Go 轨 CLI 面第一能力名读作 `goal`；crate/包结构不变 |
| User Intent 锚点（truth.md 第 35 行） | 「六个一等能力（loop / truth / design / implement / qa-evidence / release）」 | Go 轨对外能力清单第一项读作 `goal`；锚点的其余语义（双轨、评分、禁止静默 fallback）不变 |
| Alignment Surface（AC-109 引文行） | 「六个一等能力 kernel surface：loop / truth / design / implement / qa-evidence / release（AC-109）」 | 同上，随 AC-109 一并调整 |
| **AC-059** | v2 host-visible harness surface 只暴露 `loop`、`truth`、`design`、`implement`、`qa-evidence`、`release` 六个一等能力 | Go 轨 surface 清单以 `goal` 替代 `loop` 这一 id；「只许六个、无第七能力」的约束不变 |
| **AC-063** | `beacon harness list --version v2.0.0 --json` 只返回六个一等能力及 contract metadata | Go 轨等价断言（TC-063 的 `map(.id)==["loop","truth","design","implement","qa-evidence","release"]`）第一项读作 `"goal"` |
| **AC-109** | v2 public kernel surface 明确定义并只允许六个一等能力：loop / truth / design / implement / qa-evidence / release | 与 AC-059 同口径调整；P4 做 harness list 时按本记录执行 |
| AC-064 | spine/projects/workflow/truth-map/implement/qa/release/run 子命令可解析 | 已逐条核对：该条未点名 `loop` 子命令，**不受影响** |
| AC-105–108 | 以 loop 为概念名描述长任务行为（`beacon-v2 run`、stop_when、feedback checkpoint、三层 Loop 可观测） | 行为语义**不受影响**；这些条目里的 loop 是概念/机制名，不是被改名的用户入口。AC-108 的 spine `loop_layer` 字段名保持不变 |

冻结包 `tests.md` 侧对应 TC：TC-059 / TC-063 / TC-109 的断言枚举了 `"loop"` id —— Go 轨行为级测试按本记录以 `"goal"` 为期望值（P4 harness surface 落地时执行）；其余 TC 不受影响。

## 4. Go 侧 AC 清单的随之调整

- Go 侧对 AC-059 / AC-063 / AC-109 的符合性读法：六能力 id 清单 = `["goal", "truth", "design", "implement", "qa-evidence", "release"]`；计数与排他性（无第七能力）与冻结原文一致。
- 现行代码已按此执行：`internal/cli/root.go` 中 `loop` 为指针命令（打印指向 `beacon goal`），`goal` 命令组承载全部子命令；`internal/harness` 的能力清单内部 id 仍写作 `loop`（内部概念名，不对外），由 root.go 做入口映射 —— P4 若把 harness list 变成对外 JSON，输出 id 必须按本记录写作 `goal`。
- 本变更**不改变**任何行为型 AC（状态机、门禁、证据、存储合同 AC-001..058、AC-065..104、AC-110..173 全部原样有效）。

## 5. 状态

- **已在 Go 侧落账**：本记录 + `docs/scope-charter.md` 第 6 节第 5 条决定记录 + 代码注释指向本记录。
- **上游冻结包未改**：冻结包位于只读镜像仓（`~/workspace/repos/beacon/`，镜像纪律为只读），本记录**没有**对上游 truth.md/tests.md 执行 gen-change / gen-refreeze，也不得声称已经执行。
- **待合并**：源仓下一次正式变更（gen-change → gen-refreeze）时，把本记录作为变更输入合并进冻结包（预期动作：truth.md 概述/User Intent/AC-059/AC-063/AC-109 与 tests.md TC-059/TC-063/TC-109 的 `loop` id 改写为 `goal` 并注明 Go 轨先行落账），合并前 Go 轨以上户口径为准。
