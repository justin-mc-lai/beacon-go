---
slug: acceptance-seed-red-test
version: v2.0.0
materials_status: current
---

# Tests: acceptance-seed-red-test

红测试文件：internal/orchestrator/acceptance_seed_test.go
seed 文件 sha256：58a5a998894496caeb8eda6e6ad2c51833e4b4599c857c331ec2812227483cf9
起手基线：main @ ea2a566，三条命令当前全部 FAIL，失败原因均为断言（未被拒绝/被错误接受），不是编译错误。

| AC ID | TC ID | Layer | Command | Expected | Seed sha256 |
|-------|-------|-------|---------|----------|-------------|
| AC-001 | TC-001 | behavior | `go test ./internal/orchestrator -run TestValidateLakeProposalRejectsProseAcceptanceSeed -count=1` | FAIL：proposal 接受了散文 seed | `58a5a998894496caeb8eda6e6ad2c51833e4b4599c857c331ec2812227483cf9` |
| AC-002 | TC-002 | behavior | `go test ./internal/orchestrator -run TestHarvestRejectsTestOnlyArtifacts -count=1` | FAIL：harvest 接受了只带测试产物的回报 | `58a5a998894496caeb8eda6e6ad2c51833e4b4599c857c331ec2812227483cf9` |
| AC-003 | TC-003 | behavior | `go test ./internal/orchestrator -run TestHarvestRequiresSeedRedEvidence -count=1` | FAIL：harvest 接受了没有 red 基线证据的回报 | `58a5a998894496caeb8eda6e6ad2c51833e4b4599c857c331ec2812227483cf9` |

## 种子命令规范

每条 acceptance_seed 必须是一条可复制执行的 `go test ./<pkg> -run <TestName> -count=1` 命令，TestName 指向 tests.md 中该 AC 的 TC。散文、自然语言、截图、人工步骤都不再算种子。命令的项目根是该包所在仓库根，cwd 固定为 beacon-go 仓库根。

## 红记录（plan 阶段实跑）

| AC | Evidence | Exit | 失败点 |
|----|----------|------|--------|
| AC-001 | evidence/red-ac001.txt | 1 | `proposal accepted prose acceptance_seed "a visitor joins the waitlist"` |
| AC-002 | evidence/red-ac002.txt | 1 | `harvest accepted a test-only result ... Accepted:true` |
| AC-003 | evidence/red-ac003.txt | 1 | `harvest accepted a result without acceptance_seed_red evidence ... Accepted:true` |

包级复跑 evidence/red-orchestrator-pkg.txt：`go test ./internal/orchestrator -count=1` 退出 1，`--- FAIL` 仅这三条，其余用例通过。

## 转绿规则

implement 阶段同一命令必须 PASS，且不得修改三条测试的断言与 helper 语义；允许新增生产逻辑与更新受影响的旧 fixture。合入前另由独立 QA 复跑并对比 evidence 中的 red 输出。
