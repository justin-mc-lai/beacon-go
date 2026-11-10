# Evidence Index: acceptance-seed-red-test

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | draft | truth |
| tests | test_truth | tests.md | draft | qa |
| tasks | support_advisory | tasks.md | present | truth |
| build | support_advisory | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/build.txt | exit=0 | qa |
| red seed | acceptance_truth | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/red-ac001.txt | FAIL | qa |
| red seed | acceptance_truth | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/red-ac002.txt | FAIL | qa |
| red seed | acceptance_truth | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/red-ac003.txt | FAIL | qa |
| package red run | acceptance_truth | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/red-orchestrator-pkg.txt | FAIL, only the three seeds | qa |
| green seed | acceptance_truth | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/green-ac001.txt | PASS | qa |
| green seed | acceptance_truth | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/green-ac002.txt | PASS | qa |
| green seed | acceptance_truth | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/green-ac003.txt | PASS | qa |
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
| AC-001 | 0 | evidence/green-ac001.txt | 待独立 QA |
| AC-002 | 0 | evidence/green-ac002.txt | 待独立 QA |
| AC-003 | 0 | evidence/green-ac003.txt | 待独立 QA |

`go test ./... -count=1` 同样退出 0，完整原始输出见 evidence/green-suite.txt。三条 seed 与 helper 未改动，seed 文件 sha256 仍为测试包冻结值。
