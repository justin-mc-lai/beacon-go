---
slug: acceptance-binding-tc-acceptance-binding
version: v0.0.1
status: frozen
package_maturity: complete
source: tree
tree_node_id: n_12c7f50ea594
parser_contract: beacon-feature-package-v2
truth_source_model: tree_derived
---
## 问题
现有 QA 门（internal/gate/evaluate.go 的 EvaluateQA）只产出 qa_artifacts_present、qa_stage_executed、qa_artifact_refs_valid 等整体 checks，一条 TC 从 tests.md 定义到 AutoTest report 再到 gate verdict 没有可追溯的逐条判定。结果是 gate 只给出整体 pass/blocked，无法回答「TC-001 到底过没过」，也无法保证 gate pass 仅当全部 TC passed。

## 目标用户
写 tests.md 并把 TC 绑定到 FSM 转移或 illegal slot 的规格作者，以及读取 gate evaluate qa --json 结果做放行决策的 QA / 发布验证者。

## 场景
一个 feature 定义 TC-001 到 TC-N，每条 TC 声明它验证的 FSM legal path 或 illegal slot。执行 autotest 后生成逐 TC evidence 与 report；作者期望 report 里的 TC 顺序与 tests.md 一致，并期望 gate evaluate qa 的 JSON checks 中逐条出现 TC-001:pass 这类条目，全部 TC passed 时 gate 才 pass。

## 范围
本 feature 冻结「每条 TC 显式绑定 FSM 转移或 illegal slot，且 gate 输出 per-TC verdict 而非整体 pass/blocked」这一需求边界，覆盖定义→report→gate 的可追溯链。现有落点为 internal/orchestrator/autotest.go 的 Engine.RunAutoTest 已逐 TC 写入 evidence/result 并生成 AutoTestReport，而 internal/gate/evaluate.go 的 EvaluateQA 仍是整体 checks。per-TC 的 gate verdict、checks 中 TC-001:pass 形式的条目、以及「gate pass 仅当全部 TC passed」的判定，当前未实现，本包冻结的是需求定义，不是实现状态。

## 非目标
不定义每个 illegal slot 必须有 negative TC 的覆盖规则（由兄弟 feature fsm-illegal-coverage 冻结），也不负责 module 级准入或跨 module 调度。不改动 AutoTest 引擎的执行与证据落盘机制本身。

## 验收标准
`beacon gate evaluate qa --run-id <id> --evaluator x --json | jq -e '.checks'` 的 checks 中应出现 TC-001:pass 形式的逐条条目，AutoTest report 的 TC 顺序应与 tests.md 一致，且 gate pass 仅当全部 TC passed。串联验证的验收命令为 `beacon goal autotest --run-id <id> --by x && beacon gate evaluate qa --run-id <id> --evaluator x`，该命令当前尚不存在，属待实现能力；现有 autotest 能力在 verify autotest run 下。其中 per-TC verdict 与逐条 checks 输出也属待实现能力。
