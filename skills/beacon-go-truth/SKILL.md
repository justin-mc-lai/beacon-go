---
name: beacon-go-truth
description: Form the current iteration's canonical intent tree, deliver HTML for review and freeze approved scope. Use for 冻需求, truth, freeze, refreeze or 需求真相; route subsequent node changes through change.
---

# beacon-truth

truth 将已澄清的本轮范围形成唯一 canonical 树，交 HTML 审阅并冻结。
plan 的调查与候选结构是输入，不是冻结需求或实现证据。
对齐链为 `ask -> sign -> freeze`：只问尚未解决的目标取舍，签名绑定当前审阅内容，
冻结以实际节点回执为准。

## Workflow

1. 读取目标、范围取舍、来源与既有树。宿主合并已有节点归属，保留 source refs；
   只就真实目标冲突问用户一个问题并给建议。
2. 首次建树使用 `beacon plan init`；宿主生成 proposal，使用
   `beacon plan expand` 校验、`beacon plan apply` 落盘。
   这些是 CLI 工具命名，不倒推阶段。已有树后续修改先交 change。
3. 形成 `intent -> module -> feature -> touchpoint`；叶子含可观察 AC 与可执行验证。
   宿主整理 module_count、coverage_manifest、依赖、来源和未知项。
4. HTML 必交：`beacon truth tree --project <root> --version <v> --format html`。
   检查真实文件，交 `html_path`、`source_hash` 和本轮范围供用户审阅批准。
   修改后重新生成；旧 HTML、空清单或文件名不证明审阅通过。
5. 用户批准当前内容后，按内核缺项补齐，使用
   `beacon truth node freeze --project <root> --version <v> --node <id> --json`
   自底向上冻结本轮范围并记录真实回执。缺项就停在 truth，不用文档 status 代替回执。
6. 将指纹、baseline 与本轮 delta 交 goal/implement；后续所有节点变更交 change。

## Version = delivery line

首轮文档路径可为 `v0.0.1`；后续 `beacon projects bump` 按用户批准的迭代推进。
版本号纯 `x.y.z`，路径可能使用 `v` 前缀；runtime 版本以二进制报告为准，
不再维护另一条 runtime 版本轴。逻辑一棵树持续成长，详见 core 的 baseline/delta 合同。

## Legacy package compatibility

旧 `features/<slug>/` Markdown 是兼容材料或树的派生正文，不是第二需求权威。
已有包可用 `truth ask/answer/intent/confirm`、`truth freeze/refreeze` 的兼容检查；
SDD 的 truth.md/tasks.md、FSM/AC/TC、semantic-review 回执仍按已有内核要求处理。
包变更使用 `beacon change legacy`。包冻结不等于 canonical 树冻结或本轮完成。

## Route examples

Worked routes (examples are not requirement authority):

- `examples/freeze-readiness.md` — chain `full-delivery`: canonical formation → HTML review → node freeze.
- `examples/change-and-refreeze.md` — chain `change-refreeze`: route later edits to change.
- `examples/drift-check.md` — chain `change-refreeze`: distinguish hash drift from delivery verification.

## HARD GATE

- 完整读取 `../beacon-go-verify/references/strict-delivery.md`，绑定适用的 AC/TC 与证据。
- canonical JSON 是唯一权威；SQLite 是可重建索引，HTML 是派生审阅视图。
- 用户只决定目标、取舍、批准；宿主承担节点组织、来源映射、命令与技术缺项处理。
- 内核不调 LLM、不跑 agent；签名绑定当前内容，证据追加不改变合同指纹。
- HTML 与批准绑定本轮内容；freeze 不代表 hash verified、gate pass、tree verified 或 version ready。

## Load before acting

Read `references/core.md` completely before the first command.

## Output contract (minimum)

- `mode_id`: `tr-ask|tr-sign|tr-freeze|tr-change|tr-drift`
- `run_id`, `current_stage`, `existing_artifacts`, `next_action`, `owner`: 实读当前 run，缺失写未确认
- `tree_path`, `html_path`, `source_hash`, 本轮范围、baseline/delta、冻结指纹和批准回执
- `evidence_refs`: 实际命令输出；`next`: `change|goal|implement|verify|stop`
- `plain_summary`: 一句话说明实际状态

## Refuse (no fake delivery)

1. 未读已有树或来源就声称范围完整。
2. HTML 缺失、陈旧或 source_hash 与审阅内容不符却请求冻结。
3. 用户未批准或未知项影响本轮合同却代答。
4. 将文档 status、签名或包冻结冒充树冻结回执。
5. 后续修改绕过 change 或直接手改 canonical JSON。
6. 将 future draft 混入本轮范围或把本轮完成扩大为全树已验证。
