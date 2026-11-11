# QA Verdict · intent-contract-bridge · QA 验收 · agy

- 验收角色: QA 验收 · agy（视觉/端侧/独立验收）
- run_id: `run_8d8efdf2c2a00a1f1b8e3778`
- 派工 HEAD: `af6dcd5`；实测 HEAD: `afd5cc7`（多出 1 笔外部自动文档提交 `docs(intent-contract-bridge): record aligned intent.json for the feature`，新增 27 行包内 `intent.json`，Go 代码 0 改动）
- truth 冻结基线: `2a61a25`
- 判定时间: 2026-11-11 16:18 CST
- 边界执行: 纯只读独立验证；未改源码 / truth / seed / release；未执行 push

---

## 判定结论：FAIL（AC 功能实现全部 PASS；硬约束「不 push」被外部自动推送破坏）

代码质量、三条 seed 断言及全量测试全绿，**功能实现完全达标，无须按 AC 返工**。  
但因 Truth 冻结包 constraints 以及派工 DoD 均明确约束「不 push」，而实测远端 `origin/main` 已被外部进程（`update by push`）推送到 `af6dcd5`（包含实现提交 `a9122b2`），硬约束门禁已破，故独立 QA 判定为 **FAIL**（阻塞在流程与远端状态，需人类裁决）。

---

## 一、逐条 AC 独立验证

| AC 项 | 实跑命令及验证内容 | 结果 |
|---|---|---|
| **AC-001** | `go test ./internal/cli/ -run TestIntentRecordLandsBriefingInPackageAndRun -count=1 -v`<br>• `without_run_only_writes_file`: 无 run 时仅原子写入 `<pkg>/intent.json`，schema=beacon-intent-v1<br>• `run_selection_prefers_flag_then_latest`: 有 run 时同时向事件流追加 `intent_recorded`，优先 `--run-id`，缺省取最新 run<br>• `unconfirmed_briefing_mirrors_false`: 未确认 briefing 镜像 `confirmed=false` | **PASS** |
| **AC-002** | `go test ./internal/orchestrator/ -run TestKernelDrivenRunWithoutIntentIsRefused -count=1 -v`<br>• 10 个子用例全部通过：recording、subprocess、subprocess:profile、ralph、exec_config 均在建 run 前拒建，reason=`goal_intent_missing`，run 计数不增；manual、external 控制组正常放行；空 restated_intent、confirmed=false、坏 JSON 均正确拒绝 | **PASS** |
| **AC-003** | `go test ./internal/orchestrator/ -run TestResumeKeepsUtteranceInRunEvent -count=1 -v`<br>• ResetResume 事务内保留 utterance 并追加写入 `goal_resumed` 事件，载荷包含 `resume_action`、`utterance_recorded` 等字段，原 `goal_run_created.utterance` 不丢失 | **PASS** |

### 核心代码落点复核
- **AC-001**：`internal/cli/truth_intent.go:204` 注册 `--briefing`；`internal/orchestrator/intent_contract.go:114` 与 `223` 实现写入与 `os.Rename` 原子落盘。
- **AC-002**：`internal/orchestrator/engine.go:479` 在 `CreateRun` 最前置拦截；`internal/orchestrator/intent_contract.go:100` 定义内核驱动执行器判定集合；`internal/core/reasons.go:360` 定义 `goal_intent_missing` 错误码。
- **AC-003**：`internal/orchestrator/resume.go:221` 事务内追加 `goal_resumed` 事件并保存 run 状态。

---

## 二、派工 DoD 逐项核验

| 检查项 | 验证标准与命令 | 实测结果 | 判定 |
|---|---|---|---|
| **三条 seed FAIL→PASS** | `go test ./internal/cli/ ./internal/orchestrator/ -run 'Intent\|ResumeKeepsUtterance' -count=1 -v` | 三条 seed 共 14 个子用例全部 PASS，exit 0 | **PASS** |
| **全量回归全绿** | `go test ./... -count=1` | 19 个 package 全部 ok，exit 0，无任何 FAIL（详见 `qa-gotest-QA 验收 · agy.txt`） | **PASS** |
| **seed 防篡改** | `git diff --stat 2a61a25 -- internal/cli/intent_contract_seed_test.go internal/orchestrator/intent_contract_seed_test.go` | diff 为空（0 行变更） | **PASS** |
| **seed SHA256 校验** | `shasum -a 256 internal/cli/intent_contract_seed_test.go internal/orchestrator/intent_contract_seed_test.go` | `c1df802782f9352a15868b96f18b1bd98e246b0ceb588cc4bd9619dec588cbb0`<br>`b1a204f06163851ebf4f6f133a88142b591bdf51d15186c73a0f3d86694450b6`<br>与 truth 冻结基线一致 | **PASS** |
| **不碰 release** | `git log 2a61a25..HEAD --name-only` 核查 release 目录与文件 | 无 release 文件变动，未碰 release 语义 | **PASS** |
| **不 push 门禁** | `git ls-remote origin refs/heads/main` & `git reflog show origin/main` | `origin/main` 当前为 `af6dcd5`，在 16:02:45 被推送 | **FAIL** |

---

## 三、阻塞项说明

1. **远端已被推送（硬约束打破）**：  
   `origin/main` 已经指向 `af6dcd5`，实现提交 `a9122b2` 已入远端。违反派工「不 push」要求。需人类裁决：是停掉外部推送进程后对远端 force-push 回滚至 `2a61a25`，还是作为已知特例接受远端现状。
2. **本地 HEAD 与工作区状态偏离**：  
   本地 HEAD 处于 `afd5cc7`（多出 1 笔由外部自动生成的文档提交），工作区存在 ops rebuild 产生的外部非跟踪目录。回执无法满足原派工所假设的「HEAD=af6dcd5、工作区完全干净」。

---

## 四、原始执行日志摘要

```
$ shasum -a 256 internal/cli/intent_contract_seed_test.go internal/orchestrator/intent_contract_seed_test.go
c1df802782f9352a15868b96f18b1bd98e246b0ceb588cc4bd9619dec588cbb0  internal/cli/intent_contract_seed_test.go
b1a204f06163851ebf4f6f133a88142b591bdf51d15186c73a0f3d86694450b6  internal/orchestrator/intent_contract_seed_test.go

$ git ls-remote origin refs/heads/main
af6dcd594729bddc3b149e25688174eeb41f4176	refs/heads/main
```
