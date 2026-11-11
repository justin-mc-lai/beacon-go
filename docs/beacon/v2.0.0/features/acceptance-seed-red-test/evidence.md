# Evidence Index: acceptance-seed-red-test

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | frozen | truth |
| tests | test_truth | tests.md | frozen | qa |
| tasks | support_advisory | tasks.md | present | truth |
| build | support_advisory | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/build.txt | exit=0 | qa |
| red seed | acceptance_truth | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/red-ac001.txt | FAIL | qa |
| red seed | acceptance_truth | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/red-ac002.txt | FAIL | qa |
| red seed | acceptance_truth | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/red-ac003.txt | FAIL | qa |
| package red run | acceptance_truth | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/red-orchestrator-pkg.txt | FAIL, only the three seeds | qa |
| green seed | acceptance_truth | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/green-ac001.txt | PASS | qa |
| green seed | acceptance_truth | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/green-ac002.txt | PASS | qa |
| green seed | acceptance_truth | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/green-ac003.txt | PASS | qa |
| same-path kinds red | acceptance_truth | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/red-kind-reuse.txt | FAIL, bypass reproduced | qa |
| same-path kinds green | acceptance_truth | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/green-kind-reuse.txt | PASS | qa |
| CLI lane entry red | acceptance_truth | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/red-cli-lane.txt | FAIL, bypass reproduced on 0fabeb5 | qa |
| CLI lane entry green | acceptance_truth | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/green-cli-lane.txt | PASS, all three entries | qa |
| full green run | acceptance_truth | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/green-suite.txt | exit=0 | qa |
| hash manifest | support_advisory | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/sha256.txt | recorded | qa |

## Reproduction Log

Every row below comes from a real command run on 2026-11-11 against beacon-go main. The red rows were captured at the design baseline ea2a566; the green rows were captured after the plan package was frozen on main @ 8d73258, with the implement changes present in the working tree (uncommitted). Full output is in the evidence files named here.

| AC | Command | Exit | Result |
|----|---------|------|--------|
| build | `go build 全仓` | 0 | no output, full tree compiles |
| AC-001 | `go test ./internal/orchestrator -run TestValidateLakeProposalRejectsProseAcceptanceSeed -count=1 -v` | 1 | FAIL by assertion: prose acceptance_seed accepted |
| AC-002 | `go test ./internal/orchestrator -run TestHarvestRejectsTestOnlyArtifacts -count=1 -v` | 1 | FAIL by assertion: test-only harvest accepted |
| AC-003 | `go test ./internal/orchestrator -run TestHarvestRequiresSeedRedEvidence -count=1 -v` | 1 | FAIL by assertion: harvest without red evidence accepted |
| package | `go test ./internal/orchestrator -count=1` | 1 | FAIL; the only `--- FAIL` lines are the three seeds |

## Seed File

| File | sha256 | Lines |
|------|--------|-------|
| internal/orchestrator/acceptance_seed_test.go | 58a5a998894496caeb8eda6e6ad2c51833e4b4599c857c331ec2812227483cf9 | 122 |

## 转绿证据（implement 轮填写）

| AC | green command exit | green evidence file | independent verifier |
|----|-------------------|---------------------|----------------------|
| AC-001 | 0 | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/green-ac001.txt | 待独立 QA |
| AC-002 | 0 | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/green-ac002.txt | 待独立 QA |
| AC-003 | 0 | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/green-ac003.txt | 待独立 QA |

`go test 全仓 -count=1` 同样退出 0，完整原始输出见 docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/green-suite.txt 。三条 seed 与 helper 未改动，seed 文件 sha256 仍为测试包冻结值。

## 返工证据（implement 轮第 2 次：零产物回报绕过）

独立验收（spec/AC 与负向回归）各自复现：先交只带 `acceptance_test` 的回报被拒，但其产物已落库；再交零产物 done 回报时 `len(in.Artifacts) > 0` 为假，三类规则整段跳过，`Accepted=true`。返工把三类规则改为按本次回报携带的产物判定，并在校验通过后才登记产物；ralph 真实捕获路径改用显式 `CapturesRegistered` 标记豁免，不再由空产物隐式放行。

| probe | command | exit | result | evidence |
|-------|---------|------|--------|----------|
| bypass red（改前 197c514+旧实现） | orchestrator 包内两条 bypass 用例（-run 匹配名见 tests 与证据原文） | 1 | 两条均 FAIL：空回报收割了历史 test-only 产物 | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/red-bypass.txt |
| bypass green（返工后） | 同上 | 0 | 两条均 PASS：test-only 被拒后空回报仍被拒（`result_artifacts_missing`），随后完整回报仍可收割 | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/green-bypass.txt |
| CLI lane entry red（父提交 `0fabeb5` + 新用例） | `go test ./internal/orchestrator -run 'TestHarvestLaneEntryRejectsEmptyReturnAfterTestOnlyRejection|TestProbeOldCLILaneEmptyRetryAccepted' -count=1 -v` | 1 | 新用例 FAIL：被拒回报在 `artifact_refs` 残留 `acceptance_test`；探针实测后续空回报 `accepted=true` | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/red-cli-lane.txt |
| CLI lane entry green（当前实现） | `go test ./internal/orchestrator -run 'TestHarvestLaneEntryRejectsEmptyReturnAfterTestOnlyRejection|TestHarvestRejectsEmptyReturnAfterTestOnlyRejection|TestReceiveRejectsEmptyReturnAfterTestOnlyRejection' -count=1 -v` | 0 | 三入口（`HarvestLaneResult` / `Harvest` / `ReceiveRalphResult`）全部 PASS | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/green-cli-lane.txt |
| full green after rework | `go test 全仓 -count=1` | 0 | 全包 ok | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/green-suite.txt |

回归用例集中放在 orchestrator 包内新增的 bypass 回归文件，覆盖 `HarvestLaneResult`（CLI 车道入口）、`ReceiveRalphResult`（ralph 回报）以及共同的 `validateHarvest` 收敛点；CLI 车道入口另有独立命名的用例，并断言“被拒后 `artifact_refs` 为空”。三条既有 seed 的断言、helper 与冻结的 seed 测试文件均未改动。

## 返工证据（implement 轮第 3 次：同一路径多 kind / 空白 kind 绕过）

两方独立验收复现：校验按 `strings.TrimSpace(kind)` 建集合，登记却用未 trim 的 kind；`artifact_refs` 又按 `(run_id, abs_path)` 去重并保留库里首个 kind。于是 `[red=X, test, impl=X]` 校验看到三类齐备、登记后 X 只剩 red，`Accepted=true` 但落库缺 `implement_artifact`。

修复路线：新增 `normalizeArtifacts` 作为唯一规范化入口，`validateHarvest` 的三类判定与 `registerArtifacts` 的登记共用同一份结果；kind 去空白后若非原样（含仅空白 kind）直接拒绝并给 `result_artifact_kind_invalid`，按 `abs_path` 折叠时首次声明胜出。折叠键沿用登记语义：相对路径先用 `filepath.Abs` 解析成绝对路径再比较，所以同一文件的相对/绝对别名也不能冒充两个 kind。先校验后登记的顺序保持。

| probe | command | exit | result | evidence |
|-------|---------|------|--------|----------|
| 同路径 kind red（改为规范化前的第 2 轮状态） | `go test ./internal/orchestrator -run 'TestHarvestRejectsKindReusedOnOnePath|TestReceiveRejectsKindReusedOnOnePath|TestHarvestRejectsUncanonicalArtifactKind|TestHarvestAcceptsThreeSeparateKinds|TestHarvestRejectsRelativeAliasOfSamePath' -count=1 -v` | 1 | 五个拒绝用例均 FAIL，`Accepted:true`（red/impl 同路径、test/impl 同路径、三类同路径、相对/绝对别名、空白 kind）；对照组 PASS | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/red-kind-reuse.txt |
| 同路径 kind green（修复后） | 同上 | 0 | 五个拒绝用例与对照组全部 PASS；被拒回报 `artifacts_after=[]`，正常三类分开路径仍被接受且三类均落库 | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/green-kind-reuse.txt |
| 三条既有 seed + 上轮两条空回报回归 | 同上 run 匹配，9 条用例 | 0 | 9 条全部 PASS | 见 Evidence Index 中 green-ac001..003 与 green-bypass |
| full green after rework | `go test 全仓 -count=1` | 0 | 19 个有测试的包全部 ok，无 FAIL | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/green-suite.txt |

回归用例仍集中在 orchestrator 包内新增的 bypass 回归文件。冻结 seed 测试文件的 sha256 仍为 `58a5a998894496caeb8eda6e6ad2c51833e4b4599c857c331ec2812227483cf9`，`git diff` 为空。
