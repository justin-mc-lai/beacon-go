---
slug: delivery-paradigm
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
  truth: docs/beacon/v2.0.0/features/delivery-paradigm/truth.md
  tests: docs/beacon/v2.0.0/features/delivery-paradigm/tests.md
  tasks: docs/beacon/v2.0.0/features/delivery-paradigm/tasks.md
  evidence: docs/beacon/v2.0.0/features/delivery-paradigm/evidence.md
---

# Requirement Truth: delivery-paradigm

## 人话

给 Beacon 内核的使用者与外部房间：renlink 按 beacon-go 交付时，代码和人工验收都过了，内核却停在 plan，并显示“交付了但门没评估”。本包把一次完整交付变成内核能判定并留痕的事实：人工/外部执行器显式可选，阶段出口必须落一条可查询的判定记录，外部房间完成的湖能写回 run，program route 给出可执行的下一步顺序，ux 声明与正文一致，并提供一条 conformance 自检把 truth frozen、每个阶段 gate evaluated、verifier attached、state exported 一次判清。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "justin-mc-lai：renlink 项目按 beacon-go 交付时，代码和人工验收都过了，但内核状态是“交付了但没被内核认定”。下面是实测到的 6 条，请逐条复现、分类并修内核缺陷。"
goal: "beacon-go 能判定并记录一次完整交付（plan→truth→design→implement→qa→release），不再出现“交付了但门没评估”"
non_goals: "不做 agent 运行时；不重写 renlink 业务代码；不把 renlink 的流程缺失算作内核缺陷；不直接改 main；不 push"
constraints: "Go 1.27.3；v2.0.0-go.15；分支 fix/delivery-paradigm；AC-136 真值写入只走 canonical main；AC-009 提案门不回归；验收员独立判定"
scope_mode: lake
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 人工与外部房间执行器怎样表达，才不出现“创建成功、出口才报未注册”？ | 增加显式 `--executor manual» 与 `--executor external»，创建时说明需 `goal tick --by» 记录完成 | answered | 采用显式 manual/external；创建输出与 `goal status» 都写明不自动驱动 |
| Q2 | 阶段出口判定怎样落盘，才能让 `gate status» 与 conformance 读到？ | 离开阶段时把导出判定持久化为 evaluated=true 的记录 | answered | 记录可以落 `gate_results» 表，也可以落 `stage_exit» 表；未评估不允许推进 |
| Q3 | 外部房间完成后用什么入口回写内核？ | `goal receive-result» 增加 `--external»，免除 `--delegation-id» | answered | 采用 `goal receive-result --external --artifact <absolute-path>» |
| Q4 | ux 声明与正文不一致时默认拦还是默认报？ | 默认 audit 警告，显式 `--strict» 时 block | answered | 默认 audit，`--strict» 下 verdict blocked |
| Q5 | conformance 的 state_exported 以什么为准？ | 以 `projects export-state» 产物或 run 的 state_export 记录为准 | answered | 两项任一存在即 pass，均缺失即 fail |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | K1 观察：manual/external 执行器创建后永报未注册 | docs/beacon/v2.0.0/features/delivery-paradigm/evidence/repro-k1-k6.txt |
| AC-002 | K1 期望：创建期直接拒绝并说明 | docs/beacon/v2.0.0/features/delivery-paradigm/evidence/repro-k1-k6.txt |
| AC-003 | K2 观察：阶段在跑、门没评估 | docs/beacon/v2.0.0/features/delivery-paradigm/evidence/repro-k1-k6.txt |
| AC-004 | K3 观察：外部完成没有回写入口 | docs/beacon/v2.0.0/features/delivery-paradigm/evidence/repro-k1-k6.txt |
| AC-005 | K4 观察：route 只给 blocked 与 next batch | docs/beacon/v2.0.0/features/delivery-paradigm/evidence/repro-k1-k6.txt |
| AC-006 | K5 观察：ux_required=false 但正文有 UI 规格章节 | docs/beacon/v2.0.0/features/delivery-paradigm/evidence/repro-k1-k6.txt |
| AC-007 | K6 观察：没有 conformance 自检 | docs/beacon/v2.0.0/features/delivery-paradigm/evidence/repro-k1-k6.txt |
| AC-008 | DoD：端到端测试全过 | docs/beacon/v2.0.0/features/delivery-paradigm/evidence/go-test-main.txt |
| AC-009 | 已冻结行为：新 program 必须走提案门 | docs/beacon/v2.0.0/features/sea-to-lake-proposal/truth.md |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `goal run --executor manual» 与 `goal run --executor external» 均创建成功；创建输出与 `goal status» 明示该执行器不自动驱动、完成需 `goal tick --by <identity>» 记录；这两种执行器的 run 不再报 `stage_executor_not_registered»。 |
| AC-002 | `goal run --executor <unknown>» 以非 0 退出码拒绝创建，reason code 为 `stage_executor_unsupported»，store 中不出现该 run。 |
| AC-003 | 每次成功离开阶段的 `goal tick» 持久化一条 evaluated=true 的出口判定记录，记录可以落 `gate_results» 表，也可以落 `stage_exit» 表；未评估时拒绝推进并 park，reason code 为 `stage_gate_not_evaluated»，`goal status» 显示该阶段未评估。 |
| AC-004 | `goal receive-result --run-id <id> --external --artifact <absolute-path> --summary <text>» 不要求 `--delegation-id»，写入该湖完成事件与 artifact 指针；随后 `goal program route» 显示该湖 completed。 |
| AC-005 | `program route» 输出 `next actions» 段，按拓扑顺序列出每个未完成湖的下一步命令；truth 未冻结的湖出现 `truth freeze» 建议并标注湖 slug。 |
| AC-006 | truth 正文存在 `## UI 规格» 章节且 frontmatter `ux_required: false» 时，`gate integrity» 输出 `truth_ux_requirement_mismatch» finding；默认 verdict 为 audit，`--strict» 时 verdict 为 blocked。 |
| AC-007 | `goal conformance --run-id <id>» 输出 truth_frozen、gates_evaluated、verifier_attached、state_exported 四项 verdict；四项全 pass 时退出码 0，任一项非 pass 时退出码非 0 并列出未过项。 |
| AC-008 | 一个 Go 端到端测试创建 run，依次推进 plan→truth→design→implement→qa→release；plan 出口记录与 truth_freeze、design、implement、qa、release 五个 gate 的最新 verdict 均 evaluated=true；测试退出码 0。 |
| AC-009 | 新 program 必须经 `program propose» 加 `program select»；直接 `program plan» 返回 `lake_proposal_required»；TestProgramPlanRequiresProposal、TestProgramPlanFreezeChange、TestProgramSelectRecordsDecision 原样通过。 |

## Evidence Map (K1-K6)

| Observation | Reproduction | Classification |
|-------------|--------------|----------------|
| K1 执行器出口才报未注册 | `goal status --run-id run_1084fcb4301353f56601ec48» 返回 stage=plan blocked reason=stage_executor_not_registered | kernel defect |
| K2 阶段推进但门没评估 | `gate status --feature l1-parser-core» 五门 not_evaluated，仅 truth_integrity audit | kernel defect |
| K3 外部完成无回写入口 | `goal program --help» 无 complete-lake；`goal receive-result» 强制 `--delegation-id» | kernel defect |
| K4 route 无排序建议 | `goal program route» 仅列 blocked 与 next batch | kernel defect |
| K5 ux 声明与正文不一致 | l5 truth `ux_required: false» 且正文有 `## UI 规格说明书» | kernel defect |
| K6 无交付范式自检 | `goal --help» 无 conformance 命令 | kernel defect |

调用方流程缺失只有一条：renlink 在群里推进阶段，没有调用 `gate evaluate»、`goal tick»、`receive-result»，所以内核状态停在 plan。这条记入 context，不改内核语义。

## Non-Goals

- 不重写 renlink 的业务代码与四件套。
- 不改变 release 必须人工确认与独立 verifier 的既有语义。
- 不把 `.beacon/» 运行时状态库直接提交进仓库。
- 不回溯修改历史冻结包。

## Deferral Ledger

| Capability | Item | Reason | Status | Approved By |
|------------|------|--------|--------|-------------|
| none | none | none | dropped | n/a |

