---
slug: intent-contract-bridge
version: v2.0.0
status: frozen
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: draft
materials_status: current
intent_confirmed: true
canonical_refs:
  truth: docs/beacon/v2.0.0/features/intent-contract-bridge/truth.md
  tests: docs/beacon/v2.0.0/features/intent-contract-bridge/tests.md
  tasks: docs/beacon/v2.0.0/features/intent-contract-bridge/tasks.md
  evidence: docs/beacon/v2.0.0/features/intent-contract-bridge/evidence.md
---

# Requirement Truth: intent-contract-bridge

## 人话

意图对齐目前只存在于一次 LLM 调用的返回值里：AlignIntentStream 把结构化简报返回给调用方就结束了，包内没有 intent.json，run 里也没有任何意图记录；于是 goal run 可以带着一个自动执行器跑起来，而内核根本不知道这次交付要对齐什么。goal resume --utterance 也把这段补充说明直接丢掉（resume.go 里 `_ = utterance`）。本包把“意图”变成内核可读、可查、可拒跑的契约：结构化简报落进包内 intent.json 并写进 run 事件；没有已确认意图的 run 不允许内核自动执行；resume 的 utterance 必须落事件。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "justin-mc-lai：让意图对齐真正落地：AlignIntentStream 的结构化结果写进包内 intent.json 与 run；goal run 无 intent 时禁止指定 executor 自动跑；resume 不再丢弃 utterance 并写入 run event"
goal: "意图对齐的结构化结果成为包内与 run 内的持久事实，并能阻止无意图的自动执行"
non_goals: "不改 AlignIntentStream 的 LLM 提示词与返回结构；不重写 resume 的决策表；不改 release/human gate 语义；不把 intent.json 当作 truth.md 的替代"
constraints: "Go 1.27.3；v2.0.0；分支 fix/intent-contract-bridge；truth 未冻结前不写业务代码；maker != checker；不 push"
scope_mode: "lake"
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 结构化简报落到哪里，路径怎样在包迁移后仍可解析？ | 与 truth.md 同目录的 intent.json | answered | 路径 = filepath.Dir(Feature.TruthPath) 下的 intent.json；不写死版本目录，包目录整体移动后仍成立 |
| Q2 | intent.json 用谁的形状？ | 直接包住 orchestrator.Briefing | answered | schema=beacon-intent-v1 + 包元数据 + briefing（原样 Briefing 字段：original_prompt/restated_intent/identified_risks/boundaries/proposed_slugs/confirmed/degraded） |
| Q3 | “写入 run” 用元数据还是事件？ | run 事件 | answered | 追加事件 intent_recorded，payload 带 intent_path/intent_sha256/restated_intent/risks/boundaries/proposed_slugs/confirmed；goal trace 可读 |
| Q4 | “没有 intent 不许自动跑” 的边界：哪些执行器算自动？ | 内核自行驱动阶段的执行器 | answered | recording、subprocess、subprocess:<profile>、ralph，以及带 --exec 命令配置的 run；manual/external 由人显式 tick，不受限 |
| Q5 | intent 存在的判定标准？ | 文件存在且已确认 | answered | <pkg>/intent.json 可解析、confirmed=true、briefing.restated_intent 非空。只有可解析的 utterance 不算 intent |
| Q6 | 没给 --run-id 时 intent 事件挂哪个 run？ | 该 feature 最近创建的 run | answered | 用 --run-id 显式指定优先；否则取该 feature 最新 run；都没有时只落文件，输出 run_id 为空并说明 |
| Q7 | 写入顺序与失败语义？ | 先文件后事件 | answered | 先原子写 intent.json（临时文件 + rename），成功后追加 run 事件；事件失败则命令非 0，文件保留（可重跑幂等覆盖） |
| Q8 | 旧 fixture 用自动执行器且没有 intent，会不会被新门挡死？ | 允许一并补 fixture intent | answered | 现有测试 fixture 可新增 intent.json 种子；不得为放行而放宽三条 seed 断言 |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | outcome A：结构化结果写进包内 intent.json 与 run | docs/beacon/v2.0.0/features/intent-contract-bridge/evidence/red-ac001.txt |
| AC-002 | outcome B：无 intent 时指定 executor 自动跑必须被拒绝 | docs/beacon/v2.0.0/features/intent-contract-bridge/evidence/red-ac002.txt |
| AC-003 | outcome C：resume 不丢弃 utterance 并写 run event | docs/beacon/v2.0.0/features/intent-contract-bridge/evidence/red-ac003.txt |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `beacon truth intent record --project <root> --feature <slug> --briefing <path> [--run-id <id>] [--by <who>] [--json]` 退出码 0，并在 <pkg>/intent.json（与 truth.md 同目录）写出可解析 JSON：schema=beacon-intent-v1、feature_slug、version、truth_path（项目相对）、recorded_at（RFC3339 UTC）、recorded_by、confirmed（镜像 briefing.confirmed）、briefing（原样保留 AlignIntentStream 的 original_prompt/restated_intent/identified_risks/boundaries/proposed_slugs/confirmed/degraded）。同一命令在该 feature 存在 run 时追加事件 intent_recorded（--run-id 优先，否则取最近 run），payload 含 intent_path（项目相对）、intent_sha256（等于 intent.json 字节的 sha256）、briefing_sha256、restated_intent、risks、boundaries、proposed_slugs、confirmed、recorded_by；随后 `beacon goal trace --run-id <id> --json` 的事件流里能看到该事件且 sha256 与文件一致。没有 run 时只落文件，输出 run_id 为空且不报错。 |
| AC-002 | `Engine.CreateRun` 在“内核自行驱动”的执行器下（recording、subprocess、subprocess:<profile>、ralph、携带 Exec 命令配置；这一枚举即内核自动执行的完整判定集合），若目标包没有已确认 intent（AC-001 的判定标准），必须在写入任何 run 行之前拒绝：返回 reason code goal_intent_missing，错误文案包含 truth intent record 的修复指引，Runs().ListByFeature 不新增 run。同一参数在包内已有 confirmed=true 的 intent.json 时创建成功，current_stage=plan。manual/external 执行器在没有 intent 时仍可创建（它们不自动驱动）。 |
| AC-003 | `beacon goal resume --run-id <id> --utterance <text>` 即使不进入 replay_substep 也追加事件 goal_resumed，payload 含 resume_action、utterance（与传入文本逐字一致）、utterance_recorded=true、stage、loop_stage；空 utterance 时该事件仍追加，payload utterance_recorded=false 且不写 utterance 键。resume 不修改原 goal_run_created 事件里的 utterance。Engine.Trace(runID) 能读到上述事件。 |

## 写入时机与复核点

| 事实 | 时机 | 位置 | 复核命令 |
|------|------|------|----------|
| 结构化意图 | 调用方拿到 AlignIntentStream 的 Briefing 后，由 truth intent record 落盘 | <pkg>/intent.json（先临时文件后 rename） | cat <pkg>/intent.json；sha256sum 与事件里的 intent_sha256 对比 |
| run 侧记录 | intent.json 落盘成功后，同一命令追加事件 | lifecycle_events.event_type=intent_recorded | beacon goal trace --run-id <id> --json |
| 拒跑判定 | CreateRun 校验执行器之后、写 run 行之前 | 返回值 reason code goal_intent_missing | beacon goal run ... 的退出码与错误文本 |
| resume 补充说明 | Engine.Resume 每次调用 | lifecycle_events.event_type=goal_resumed | beacon goal trace --run-id <id> --json |

## 边界

- intent.json 是意图的机器可读副本，truth.md 仍是需求真值的唯一权威；不因为 intent.json 存在而跳过 truth freeze。
- 不改变 AlignIntentStream 的提示词、模型或返回结构；只负责把它已经返回的结构化结果落地。
- 不改变 decideResume 的决策表；只补事件与 utterance 持久化。
- 不放开 human release gate：无 intent 只影响自动执行，人工/外部执行器路径照旧。
- 事件 payload 不扩写调用方没给的字段。

## 兼容性

现有 fixture（如 deliverParadigmRun）直接用 recording 执行器建 run 且没有 intent.json；实现轮需在 fixture 种子中补一份已确认 intent，或为这些用例显式改用 manual。三条 seed 断言不得放宽。
