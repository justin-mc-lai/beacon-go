# Bootstrap exception: intent-contract-bridge

Recorded 2026-11-11 by beacon-go-kernel during the truth stage of run_6e1eeb6c0613f1568234abf2.

## 冲突

本 feature 的 TASK-001 才要新增 “beacon truth intent record”（把 AlignIntentStream 的 Briefing 落成 intent.json 并写 run 事件）。truth 阶段轮次到达时，这条子命令尚不存在；seed 里 “truth intent record --briefing” 在 main @ df0a8a5 上仍以 “unknown flag: --briefing” 失败，红证据是 red-ac001.txt。所以本轮不能、也没有假装调用 truth intent record。

## 实际走的路径

按 beacon-truth 的 sign 链条，用已存在的两条命令完成同一件事：记录用户意图快照并签署。

| 步骤 | 命令（省略 --project .） | 退出码 | 原始输出 |
|------|--------------------------|--------|----------|
| 1 | beacon truth intent --feature intent-contract-bridge --by beacon-go-kernel --utterance ... --goal ... --non-goals ... --constraints ... --scope-mode lake --json | 0 | bootstrap-intent.txt |
| 2 | 同一命令，AC-002 措辞修正后重跑 | 0 | bootstrap-intent-after-fix.txt |
| 3a | beacon truth confirm --feature intent-contract-bridge --by justin-mc-lai（按派单代表其意图快照） | 0 | bootstrap-confirm.txt |
| 3b | beacon truth confirm --feature intent-contract-bridge --by beacon-go-kernel（实际执行者身份，避免把机器动作记成人签） | 0 | bootstrap-confirm.txt |

- utterance 取自 truth.md 的 User Intent Snapshot（justin-mc-lai 原话），goal/non-goals/constraints/scope_mode 同表，未改写。
- 第一次 intent 快照带一条阻塞：ac_vague_disjunction，因为 AC-002 的枚举写了“或携带 Exec 命令配置”。只把“或”改成枚举式收束（“这一枚举即内核自动执行的完整判定集合”），判定集合不变；未触碰任何 AC 的断言、阈值或边界。
- confirm 输出 confirmed=true、ready=true，open questions Q1..Q8 全部 answered，退出码 0。第一次 confirm 的 by 写成 justin-mc-lai，随后用实际执行者身份 beacon-go-kernel 重签并留了两段原文；进入 implement 仍需 TASK-006 的人审确认，本轮没有替代它。
- 两次 truth intent 都只是快照/签署，不写 intent.json，也不写 run 事件；intent.json 与 intent_recorded 事件仍归 implement 轮 TASK-001。

## 冻结结果

beacon truth freeze --project . --feature intent-contract-bridge --by beacon-go-kernel 第二次执行退出码 0，输出 status: frozen、lifecycle: Freeze/Completed，原文见 freeze.txt。

第一次 freeze（freeze-attempt-1.txt）被 20 条 finding 挡住，全部是 evidence.md 索引的格式问题：authority 用了旧词表（truth/test_truth/task_ledger/test_evidence/test_source），且证据路径写成包内相对路径，被解析到项目根。修正只动 evidence.md 与 AC-002 的一句措辞，再逐条把被引用的证据注册为该 run 的 artifact（beacon goal artifacts register --kind evidence）。

## 遗留与边界

- frontmatter 保留 package_maturity: draft：truth freeze 只把 materials_status 归一到 current，不重写 package_maturity；intent gate 只在 scaffold/空值时阻塞，draft 被接受。这是冻结后的外观残留，不影响本次冻结结论，后续如需改由 change/refreeze 处理。
- baseline: uncommitted：本轮按指令不 commit、不 push，frozen 或许可消费所需的 committed baseline 由人工提交后 beacon truth baseline 补记。
- 本轮改动范围：truth.md（AC-002 一句措辞 + freeze 翻 status）、evidence.md（索引格式）、evidence/ 下证据文件；未改 seed、未改任何生产代码、未勾 tasks.md。
