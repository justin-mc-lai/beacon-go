# Freeze Log: kernel-recover-guard-scope

## 冻结前状态

- 仓库：/Users/apple/Developer/Personal/products/beacon-go
- 分支：main
- 冻结前 HEAD：ffdfced12656d46a075ee7400a82d9f5fa847a86
- 包路径：docs/beacon/v2.0.0/features/kernel-recover-guard-scope
- 本轮范围：只写 requirement truth（truth.md、tests.md、tasks.md、evidence.md 与包内证据），零实现代码改动
- 新增内容：git status 只显示本包目录未跟踪，cmd/ 与 internal/ 无改动

## 冻结前检查

- 派单的四条要求与本包 AC-001 至 AC-004 一一对应，无需重编号
- 每条 AC 两条 TC，Command 为可直接运行的 go test 选择器；TC-001 至 TC-008 命名互不重复
- TASK 的 evidence 指向包内已存在的证据文件，无自指
- 源锚点：本仓 main ffdfced 的 complete.go、tick.go、engine.go、exitgates.go、gates_facts.go、core/loopgoal.go、core/reasons.go、cli/goal.go，以及 run_aa702f8da68fe33f23214ae3 的只读 sqlite 事件流；摘录见 source-anchors.md
- Open Questions 4 条全部 answered
- 全量 go test ./... -count=1 已实跑，EXIT=0，17 个包 ok、1 个包无测试文件，耗时 97s，输出见 go-test-all.txt

## 冻结结果

- intent confirm：~/.local/opt/beacon-go/beacon truth confirm --project . --feature kernel-recover-guard-scope --by "BeaconGo 内核"，exit 0，2026-11-09T11:53:19.858111Z，ready=true，intent_confirmed=true；写入 truth_intent_confirmed 事件
- 冻结命令：~/.local/opt/beacon-go/beacon truth freeze --project . --feature kernel-recover-guard-scope --by "BeaconGo 内核"
- 尝试 1：exit 1，2026-11-09T11:53:23Z。precheck 拦截 2 条 finding，均为 dc_placeholder_chinese_dai_x_queren（规则 待.+确认），位置 truth.md 第 78、79 行的人类门禁原因表「语义」列，写法「等待人类确认」「等待人类执行完成动作」。修法：该两格改成「需要人类确认才能退出」「人类执行完成动作后收尾」，只改语义描述措辞，AC 判据、原因码与清除动作未动
- 尝试 2：exit 0，2026-11-09T11:53:32.725983Z truth_integrity_evaluated verdict=pass findings=0，2026-11-09T11:53:32.788588Z 写入 truth_frozen
- 冻结时间：2026-11-09T11:53:32Z（19:53:32 +08:00）
- truth 状态：frozen
- change_pending：false
- branch_check：ok（current=main，canonical=main）
- lifecycle：Freeze / Completed
- truth SHA256 演进：draft 初稿 d96ef22887536479a377b5ec4ea92156c4aebfe9d058610dae79a49b03e3daaa（intent confirm 后）→ 措辞修正后 10d2e42bf303193d4bf8ac778fbb4ad955a7c9e7d966c7038fd1ee0f749a77f9（freeze 事件 truth_sha256_before）→ 冻结后 fdc99c155b0516539657ac30fda0a95c65c0d4762a0f6df37c2eafa1c6939e27（freeze 事件 truth_sha256_after）
- package_fingerprint：e8fec0159a88c4da3ec02d61ff44c7c0e03f936fe3f2ceb1b68203283a82bc57（pkgfp-v1）
- surface_fingerprint：08de87517f9e4b3f2d0c5aaee876f3d04d96f66a9a627b503ff1c3b1344640ff（pkgsurface-v1）
- requirement_fingerprints（reqfp-v1）：AC-001 9752f2d1f658680b14edfb5d6d45f22eac1908c1c68db06814fecf981d6d4d9f；AC-002 7e9f35f562d2b91c2cbfc3e8e86ecf53080157b45aadfda009af51fada8a380a；AC-003 dcd242a871ef52749c9968fff85a25ee8faec97e139bdec6f2a450fd416eb977；AC-004 810186c6a6c64d3c27084f6b32e13d50eb98b9a9ef707db77e5b34d77c94a758
- baseline_status：uncommitted（冻结瞬间未提交）
- drift：clean=true，body_drift=false，frozen_truth_sha256 与 current_truth_sha256 同为 fdc99c15...e27，open_questions=0，base_diverged=false
 - 回归：go test ./... -count=1 exit 0，2026-11-09T11:51:17Z 起跑，耗时 97s，17 个包 ok、1 个包无测试文件，零 FAIL，输出见 go-test-all.txt

 ## 变更与再冻结（spec/AC QA 返修 · 2026-11-09 20:33 +0800）

 - 触发：spec/AC 面独立 verdict FAIL（验收员，bind 分支 `21047a4`），阻塞项一条——第二次人类确认只写 `human_gate_cleared` 不写 `release_confirmed`，status 重建把已完成 run 报成 blocked（AC-002/AC-004 实质违反）。
 - 实现修复：`83e4c8d fix(orchestrator): record every human release confirmation`，`complete.go` 去掉 `writeConfirmation := !m.ReleaseHumanConfirmed` 门槛，每次确认都在同一事务写 `release_confirmed`；仅在清除有效人类门禁时追加 `human_gate_cleared`。`engine.go` 的 `effectiveBlockView` 未改。
 - 新增 TC-009 `TestRepeatedHumanConfirmationAfterReblockKeepsStatusUnblocked`：6 步复现（首次确认 → 挂 verifier → 换身份 tick 重新阻断 → 二次确认 → complete → 读 status），断言 `completed / blocked=false / last_block_reason 空 / run_status=Succeeded`，`release_confirmed` 与 `human_gate_cleared` 各 2 条。修复前 EXIT=1，修复后 EXIT=0。
 - change 命令：`~/.local/opt/beacon-go/beacon truth change --project . --feature kernel-recover-guard-scope --by "BeaconGo 内核" --summary "spec/AC QA 返修：补 TC-009 覆盖重复人类确认后 status 不得误报 blocked"`，exit 0，2026-11-09T12:33:06.356518Z，change_id `chg_5d96b6d33709`，materials_marked_stale = tests.md、tasks.md
 - 变更内容：tests.md 增 TC-009 行（AC-002 主映射，命令可直接跑）并把回归段从 8 条改为 9 条；tasks.md TASK-008 文案 8 条改 9 条；truth.md 未改（AC 原文不动；AC-002 本来就要求写 `release_confirmed`）；changes/chg_5d96b6d33709.md 记录本次请求
 - refreeze 命令：`~/.local/opt/beacon-go/beacon truth refreeze --project . --feature kernel-recover-guard-scope --by "BeaconGo 内核"`，exit 0；truth_integrity_evaluated verdict=pass findings=0（2026-11-09T12:33:18.253992Z，gate_142e738b046cbf8323f92f49）；truth_refrozen 2026-11-09T12:33:18.327329Z
 - truth SHA256：before `fdc99c155b0516539657ac30fda0a95c65c0d4762a0f6df37c2eafa1c6939e27` → after `fdc99c155b0516539657ac30fda0a95c65c0d4762a0f6df37c2eafa1c6939e27`（正文零改动）
 - package_fingerprint：`e8fec0159a88c4da3ec02d61ff44c7c0e03f936fe3f2ceb1b68203283a82bc57` → `ceb53b42862cb32bc0af8edf80d43eeed122afc95e96f4bae4bbb1af787f57cd`；surface_fingerprint：`08de87517f9e4b3f2d0c5aaee876f3d04d96f66a9a627b503ff1c3b1344640ff` → `edaf77b37508019893b6b1bab31d189f1947fc5e32413c2386814b1add6ea0a5`；四条 requirement_fingerprints 未变
 - change_ac_diff：added/changed/removed 全为空；status=frozen，change_pending=false，branch_check=ok（current=main，canonical=main）
 - drift：`beacon truth drift` clean=true，body_drift=false，frozen 与 current truth SHA 同为 `fdc99c15...39e27`，open_questions=0，base_diverged=false
 - 9 条 TC 逐条 `-count=1 -v`：全部 `--- PASS`，逐条退出码 0（2026-11-09 20:34:23–20:34:32 +0800）
 - 全量回归：`go test ./... -count=1` EXIT=0（2026-11-09 20:37:32 起跑，20:39:08 结束，96s，17 包 ok、1 包无测试文件），全文覆盖 evidence/go-test-all.txt
 - 基线：返修的实现与文档在分支 `feat/kernel-recover-guard-scope`，未 merge、未 push、未重装；`main` 仍 `ffdfced`

## 变更与再冻结（负向回归 QA 返修 · 2026-11-09 21:18 +0800）

- 触发：负向回归面独立 verdict FAIL（QA 验收 · 回归，bind `d1f08c4`），阻塞项一条——external run 在 release 被人类门禁拦后由 `receive-result --external` 收尾，`goal status` 由事件流重建把已 done 的 run 重报成 `blocked / Paused`；根因是 `effectiveBlockView` 只认 `release_confirmed`，终止事件 `goal_completed` / `external_result_received` 未参与判定。
- 实现修复：`15c8133 fix(orchestrator): let terminal facts win over historical human-gate blocks`。`engine.go` 的 `effectiveBlockView` 改为终止事实优先：LoopStage 为 done 时直接未阻断；事件流重建时晚于最新人类门禁的 `goal_completed` / `external_result_received` 覆盖门禁。`ReceiveExternalResult` 仍按设计不写 `release_confirmed`。
- 新增 TC-010 `TestExternalReceiveResultAfterHumanGateKeepsStatusCompleted`：release tick 阻断 → `ReceiveExternalResult` 收尾 → status 断言 `completed / blocked=false / last_block_reason 空 / run_status=Succeeded`。修复前同一 TC `EXIT=1`，修复后 `EXIT=0`。
- change 命令：`~/.local/opt/beacon-go/beacon truth change --project . --feature kernel-recover-guard-scope --by "BeaconGo 内核" --summary "负向回归返修：终止事实优先，补 TC-010 覆盖 external receive-result 完成后 status 不得重建 blocked"`，exit 0，2026-11-09T13:17:03Z，change_id `chg_862ad81a9133`，materials_marked_stale = tests.md、tasks.md
- 变更内容：truth.md 的 status 呈现规则补「终止事实优先」一段（AC 判据原文未动）；tests.md 增 TC-010 行（AC-004 负向映射，命令可直接跑）并把回归段 9 条改 10 条；tasks.md TASK-008 文案 9 条改 10 条；changes/chg_862ad81a9133.md 记录本次请求
- AC-137 过程如实记录：本轮 change 在正文编辑之后才注册，首次 refreeze 被 `revision_authority_drift` 拦截（fingerprint at open != frozen body）。按工具给出的恢复路径执行 `beacon truth rebase --change chg_862ad81a9133 --accept-current --by "BeaconGo 内核"`，把变更基准确认为冻结的 `d1f08c4` 正文，exit 0；随后 refreeze exit 0。
- refreeze 命令：`~/.local/opt/beacon-go/beacon truth refreeze --project . --feature kernel-recover-guard-scope --by "BeaconGo 内核"`，exit 0，package 回到 status=frozen、change_pending=false，branch_check=ok（current=main，canonical=main）
- change_ac_diff：added/changed/removed 全为空；四条 requirement_fingerprints 未变（AC 判据原文未改）
- truth SHA256：`fdc99c155b0516539657ac30fda0a95c65c0d4762a0f6df37c2eafa1c6939e27` → `93adff7b5422ac2915c34c600744df908cb44403ff523dcd274045c2e06292d7`（status 规则新增一段）
- package_fingerprint：`ceb53b42862cb32bc0af8edf80d43eeed122afc95e96f4bae4bbb1af787f57cd` → `d91129fcf24c9c09004cd3495b378706f0ae9733e3616267eaf71391e2dba201`；surface_fingerprint：`edaf77b37508019893b6b1bab31d189f1947fc5e32413c2386814b1add6ea0a5` → `3540c8864d6cb79bff09ec90d751377c0a3d6b7d030ef4b16ebbbf05d52e7939`
- drift：`beacon truth drift` exit 0，clean=true，body_drift=false，open_questions=0，base_diverged=false
- 10 条 TC 逐条 `-count=1 -v`：全部 `--- PASS`，逐条退出码 0（2026-11-09 21:18:50–21:19:00 +0800）
- 全量回归：`go test ./... -count=1` EXIT=0（2026-11-09T21:19:06+0800 起跑，21:20:48+0800 结束，17 包 ok、1 包无测试文件），全文覆盖 evidence/go-test-all.txt
- 基线：返修实现与文档在分支 `feat/kernel-recover-guard-scope`，未 merge、未 push、未重装；`main` 仍 `ffdfced`
