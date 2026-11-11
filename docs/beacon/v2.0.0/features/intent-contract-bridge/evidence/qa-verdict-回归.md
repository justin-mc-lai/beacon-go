# QA 独立验收 · 回归（grok 独立） — intent-contract-bridge

- 判定：**FAIL**（AC-001/002/003 功能与测试全部实测 PASS；FAIL 的唯一来源是硬约束「不 push」已被外部推送打破，代码无需按 AC 返工）
- current_stage：`qa`（`beacon-go goal status --run-id run_8d8efdf2c2a00a1f1b8e3778`：stage=qa / loop_stage=qa_routing / blocked=true / last_block_reason=qa_stage_not_executed）
- 验收时间：2026-11-11（UTC 记录见 qa-gotest-回归.txt）
- 实测 HEAD：`afd5cc7`（派工 HEAD `af6dcd5`；本地领先 origin/main 一笔文档提交）
- 全程只读复验：未改源码/truth/seed/release，未执行 push。仅本目录新增两份 QA 证据文件。

## 一、逐条 AC 实测

| AC | 验证方式 | 实测 | 判定 |
|---|---|---|---|
| AC-001 `truth intent record --briefing` 原子落 `<pkg>/intent.json`（schema=beacon-intent-v1，与 truth.md 同级）+ 有 run 时落 `intent_recorded` | `go test ./internal/cli/ -run TestIntentRecordLandsBriefingInPackageAndRun -count=1 -v`；源码 `internal/cli/truth_intent.go:245`、`internal/orchestrator/intent_contract.go`（`writeFileAtomic`：CreateTemp 同目录 + Chmod + Rename，`:228`/`:249`）；`IntentDocSchema="beacon-intent-v1"`、`intentRelPath` 由 truth 路径同目录推导 | 3 子例全 PASS（无 run 只写文件 / `--run-id` 优先于最新 run / 未确认 briefing 镜像 confirmed=false）；包内现有 `intent.json` 字段与 schema 一致 | **PASS** |
| AC-002 内核驱动执行器缺 confirmed intent 拒建 run，reason=`goal_intent_missing`，run 计数不增；manual/external 放行；空/未确认/坏 JSON 拒 | `go test ./internal/orchestrator/ -run TestKernelDrivenRunWithoutIntentIsRefused -count=1 -v`；源码 `internal/orchestrator/engine.go:479-482` 在写 run 行之前返回 `core.ReasonIntentMissing`；`internal/core/reasons.go:360` = `goal_intent_missing`；`IsKernelDrivenExecutor` 覆盖 recording/subprocess/subprocess:<profile>/ralph/带 Exec 配置 | 10 子例全 PASS：5 种内核驱动模式拒建且 `seedRunCount` 前后相等，manual/external 两控制组放行，空/未确认/坏 JSON 拒建 | **PASS** |
| AC-003 ResetResume 保留 utterance 并写 `goal_resumed` 事件 | `go test ./internal/orchestrator/ -run TestResumeKeepsUtteranceInRunEvent -count=1 -v`；源码 `internal/orchestrator/resume.go:219-250`（事务内一条事件，payload 含 `resume_action`/`utterance_recorded`/`utterance`；空白 utterance 不带 utterance 键）、`intent_contract.go:34` | 1 例 PASS | **PASS** |

## 二、全量与篡改检查

- `go test ./... -count=1` → `EXIT_STATUS=0`，19 个包全 ok，无 FAIL（原始输出见 qa-gotest-回归.txt）。
- seed 未篡改：
  ```
  c1df802782f9352a15868b96f18b1bd98e246b0ceb588cc4bd9619dec588cbb0  internal/cli/intent_contract_seed_test.go
  b1a204f06163851ebf4f6f133a88142b591bdf51d15186c73a0f3d86694450b6  internal/orchestrator/intent_contract_seed_test.go
  git diff --stat 2a61a25 -- <两份 seed> → 空
  ```
- 不碰 release：`git diff --name-only 2a61a25..a9122b2 | grep -i release` → 无。
- 实现后追加提交 `a9122b2..HEAD` 仅动 evidence txt 与包内 intent.json，无 `.go` 改动（`git diff --name-only a9122b2..HEAD | grep '\.go$'` 为空），不影响 AC 证据。

## 三、阻塞项（FAIL 的唯一原因）

「不 push」被打破，且非本机 QA 行为：

```
$ git ls-remote origin refs/heads/main
af6dcd594729bddc3b149e25688174eeb41f4176	refs/heads/main
$ git reflog show origin/main --date=iso | head -1
af6dcd5 refs/remotes/origin/main@{2026-11-11 16:02:45 +0800}: update by push
$ git merge-base --is-ancestor a9122b2 origin/main && echo YES
YES: code commit is on origin/main
```

实现提交 `a9122b2`（作者 `kunqi.lai <goodlive.lkq1128@gmail.com>`，非内核本人）已在 origin/main。truth constraints 与派工 DoD 均写明「不 push」，该硬门禁事实上被外部自动提交/推送进程突破。

同时，派工回执的「HEAD=af6dcd5、工作区干净」已不可复核：实测 HEAD=`afd5cc7`，工作区含多份 QA 证据与 `docs/changes/ops-2026-11-11-rebuild/` 等未跟踪文件。

## 四、结论

- 功能层面：三条 AC 独立实测全绿，证据充分；若人类裁决接受当前远端现状，可据此转 PASS。
- 流程层面：FAIL——需先停掉自动 commit+push 的外部进程，并由人类裁决远端处理（回滚到 `2a61a25` 或接受现状）。我不执行 push/force-push。
