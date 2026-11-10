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
| hash manifest | support_advisory | docs/beacon/v2.0.0/features/acceptance-seed-red-test/evidence/sha256.txt | recorded | qa |

## Reproduction Log

Every row below comes from a real command run on 2026-11-11 against beacon-go main @ ea2a566, working tree clean before this package. Full output is in the evidence files named here.

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
| AC-001 | 待填（implement 轮） | 待填（implement 轮） | 待填 |
| AC-002 | 待填（implement 轮） | 待填（implement 轮） | 待填 |
| AC-003 | 待填（implement 轮） | 待填（implement 轮） | 待填 |
