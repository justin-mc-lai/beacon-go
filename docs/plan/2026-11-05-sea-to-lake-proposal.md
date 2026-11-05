# 计划：一句话 → AI 提案拆湖 → 人多选确认（不漂移）

> 模式：`pln-brainstorm`（第一性）+ `pln-program`（海拆湖）
> 状态：计划，未冻结；冻结后以 `docs/beacon/<version>/features/<slug>/truth.md` 为准
> 人话：AI 不再“瞎拆”，而是给出 3–7 个可选的湖（每个湖说清用户能得到什么、值多少钱、依赖谁、多大），你多选确认后才进入 goal 交付。

## 1. 第一性原理

| 不可再分的事实 | 推论 |
|---|---|
| 内核不知道你的商业优先级 | **AI 提案，人决定**；内核只校验与记录 |
| 开放式“帮我拆一下”必然漂移 | 选项必须**有界**（3–7 个），每个带用户结果与验收起点 |
| 没有用户结果的功能不是湖 | 每个湖必须有 `outcome`（用户能做什么）和 `acceptance_seed` |
| 商用不是“能跑”，是“能运营、可衡量、可部署、可维护” | 每个湖必须过**商用五项**，至少一句话说明 |
| 人没确认就冻结 = 偷跑 | `select` 记录人工多选；`freeze` 才锁图；未选中的湖永不执行 |
| 提案本身也要可追溯 | 保存 proposal + selected/rejected + 人 + 时间；拒绝过的湖不再重复提 |

## 2. 现状（源码事实）

- `truth decompose` 只能把一句需求拆成**一个** feature 包；`--lakes` 只是标签（`internal/orchestrator/intent_decomposition.go`、`internal/cli/truth.go`）。
- `goal program plan` 要你**手写** `--lake` 列表；`ProgramPlanInput.Lakes` 只有 `slug/goal/depends_on/touches/parity_subset/status`（`internal/orchestrator/meta_p2d.go:16`）。
- 已有 `feature-graph.json` 图校验：必填键、重复 slug、自依赖、未知依赖、环（`internal/orchestrator/program_file.go`、`program.go:ValidateProgramGraph`）。
- `program drive --until-done` / `program complete` 已经能执行并一次签（go.13）。
- 缺口：**没有“海 → 候选湖提案 + 人多选”的层**。

## 3. 设计：Propose → Present → Multi-select → Confirm → Plan/Freeze

```text
一句话（海）
   │
   ▼
AI 提案（3–7 个候选湖，带商用五项 + 依赖 + 验收起点）
   │  program propose（内核校验并记录 proposal）
   ▼
人看表 → 多选（可改名/合并/拆分/加湖）
   │  program select --lake a --lake c --by <name>
   ▼
选中集合 → 草稿 feature 包 + 依赖图
   │  program plan（记录）→ program freeze（人确认后锁图）
   ▼
program admit → drive --until-done → program complete（一次签）
```

### 3.1 AI 的职责（宿主/agent）

- 只基于用户原话、约束、已有代码/资料提案；**不编事实**。
- 输出 3–7 个候选湖；超过 7 个 → 先分版本/阶段，不硬塞。
- 每个湖必须给：
  - `outcome`：用户做完这个湖能做什么（一句人话）。
  - `value`：商业价值（为什么现在做）。
  - `metric`：怎么衡量成功（一个可观察指标）。
  - `operable`：上线后怎么运营/支持（一句话）。
  - `depends_on`：依赖哪些湖。
  - `size`：S/M/L。
  - `acceptance_seed`：验收起点（一句可验证的话）。
  - `out_of_scope`：明确不做什么。
  - `questions`：不确定的点（未知→问题，不硬编）。
- 必须给 `not_proposed`：故意不放进首批的湖 + 原因。

### 3.2 内核的职责（beacon-go）

- `program propose`：校验 `lake-proposal.v1` 的结构、3–7 个湖、slug 唯一、DAG 合法、每个湖有必填字段；记录到 host run（metadata + event），不创建包、不冻结。
- `program select`：记录人选中的湖、拒绝的湖、`by`、时间；拒绝未知 lake id；**不自动 freeze**。
- 选中后：为每个选中湖建**draft** feature 包（四件套骨架，绿项目自动 `v0.0.1`），形成 `program plan`；仍不冻结。
- `program freeze`：只有人确认后才锁图；之后只能 `program change`。
- `program drive`：只跑选中的湖；未选中的湖不存在、不执行。

### 3.3 提案数据契约（建议 `lake-proposal.v1`）

```jsonc
{
  "schema": "lake-proposal.v1",
  "proposal_id": "lp_...",
  "utterance": "做一个羽毛球比赛管理系统",
  "version": "v0.0.1",
  "commercial_lens": ["user_value", "operable", "measurable", "deployable", "maintainable"],
  "lakes": [
    {
      "slug": "auth",
      "title": "账号与登录",
      "outcome": "新用户能注册/登录并进入自己的首页",
      "value": "可运营的前提：知道谁是谁",
      "metric": "注册→首次进入首页转化率",
      "operable": "后台可封禁/重置账号",
      "depends_on": [],
      "size": "S",
      "acceptance_seed": "新用户 3 分钟内完成注册并进入首页",
      "out_of_scope": ["第三方登录"],
      "questions": ["是否强制手机号验证？"]
    }
  ],
  "not_proposed": [
    { "slug": "payments", "reason": "首批不做收款，先用线下/免费验证需求" }
  ],
  "open_questions": ["首发面向个人还是球馆？"]
}
```

选中后，内核把它折叠成现有 `ProgramPlanInput`（`slug/goal/depends_on`），不新增第二套图模型。

## 4. CLI 面（建议）

```bash
# 1. AI 生成/宿主提交提案；内核校验并记录
beacon-go goal program propose --run-id <host> --file proposal.json --json

# 2. 人看表后多选（可多次调用，合并选择）
beacon-go goal program select --run-id <host> --proposal <id> \
  --lake auth --lake matches --by <name> --json

# 3. 记录为 program plan（draft 包由 select 建好）
beacon-go goal program plan --run-id <host> --from-proposal <id> --freeze

# 4. 正常交付
beacon-go goal program admit --run-id <host>
beacon-go goal program drive --run-id <host> --until-done --executor ... --exec-cmd ...
beacon-go goal program complete --run-id <host> --confirmed-by <name>
```

## 5. 验收标准（草案）

| AC | 标准 |
|---|---|
| AC-001 | `program propose` 接受 `lake-proposal.v1`；湖数 3–7；slug 唯一；DAG 合法 |
| AC-002 | 每个湖必须有 `outcome/value/metric/operable/size/acceptance_seed`；缺一拒绝并给稳定 reason code |
| AC-003 | `not_proposed` 必填；未知/空原因拒绝 |
| AC-004 | `program select` 记录 selected/rejected/by/at；未知 lake id 拒绝；不自动 freeze |
| AC-005 | 选中湖自动建 draft 四件套（绿项目 v0.0.1）；未选中的湖不建、不跑 |
| AC-006 | `program freeze` 只接受已 select 的图；冻结后只能 `program change` |
| AC-007 | `program route` 显示 proposal 来源、selected/rejected 计数、每湖阶段 |
| AC-008 | 事件可追溯：`proposal_created`、`proposal_selected`、`program_frozen` |
| AC-009 | plan skill 写清 propose → 表 → 多选 → confirm → freeze 的流程 |
| AC-010 | 全仓 test/vet/e2e 通过；不自动 release、不自动写代码 |

## 6. 非目标

- 不自动替人做选择（人必须多选确认）。
- 不做通用 PM/项目规划器。
- 不做并行跑湖（另立能力）。
- 不做自动 release（`auto` 仍未启用）。
- 不做 UI；交互在宿主对话 + CLI 记录。

## 7. 待拍板（推荐默认）

| 问题 | 推荐 |
|---|---|
| 提案存哪？ | host run metadata + event（单一快照，无侧文件） |
| `select` 要不要自动建 draft 包？ | 要，只建骨架，不冻结 |
| 最多几个湖？ | 3–7；超过 7 先分版本/阶段 |
| 人怎么选？ | 宿主对话展示表格，`program select --lake` 记录 |
| 拒绝的湖怎么办？ | 记录在 `not_proposed`/`rejected`，不静默重提 |

## 8. 证据

- `internal/orchestrator/program_file.go`（feature-graph 结构/校验）
- `internal/orchestrator/program.go:ValidateProgramGraph`
- `internal/orchestrator/meta_p2d.go:ProgramLakeInput`
- `internal/cli/goal_program.go`（plan/admit/route/drive/complete）
- `internal/orchestrator/intent_decomposition.go` + `truth decompose --lakes`（单包拆解，非多湖）
- `skills/plan/SKILL.md`（pln-program）
