---
slug: kernel-autotest-shell-tc
version: v2.0.0
status: draft
revision_id: R3-draft
---

# Evidence: kernel-autotest-shell-tc

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| requirement truth | requirement_truth | docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/truth.md | draft | truth |
| acceptance tests | test_truth | docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/tests.md | draft | qa |
| task ledger | support_advisory | docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/tasks.md | draft | plan |
| seed fixture | support_advisory | docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/seeds/lib.sh | present | plan |
| seed 1 shell TC execution | support_advisory | docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/seeds/seed1_shell_tc_executes.sh | red | plan |
| seed 2 failing shell TC | support_advisory | docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/seeds/seed2_failed_shell_tc_no_passed.sh | red | plan |
| seed 3 empty set guard | support_advisory | docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/seeds/seed3_empty_tc_no_passed.sh | green | plan |
| seed 1 canon | support_advisory | docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/evidence/seed1.canon | red | plan |
| seed 2 canon | support_advisory | docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/evidence/seed2.canon | red | plan |
| seed 3 canon | support_advisory | docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/evidence/seed3.canon | green | plan |
| seed 1 raw output | support_advisory | docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/evidence/seed1_shell_tc_executes.out | red | plan |
| seed 2 raw output | support_advisory | docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/evidence/seed2_failed_shell_tc_no_passed.out | red | plan |
| seed 3 raw output | support_advisory | docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/evidence/seed3_empty_tc_no_passed.out | green | plan |
| combined seed log | support_advisory | docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/evidence/plan-seed-red.log | red | plan |
| this index | support_advisory | docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/evidence.md | present | plan |

## Measurement Environment

- Repository: beacon-go, branch main, HEAD 36c40b6.
- Build: fresh beacon binary built from this repo before each seed run; exact command is in the replay section of the tests document.
- Binary: Beacon CLI v2.0.0-go.15.
- Isolation: every seed creates its own store and project with mktemp.
- Production diff: none.

## Seed Results

S1 seed1_shell_tc_executes: exit 1, canon sha256 6ed34ca4e5c8ceb43570bec9edb43321299808fa115ef69d65d56a9eae136db3. Red: total=2 passed=0 failed=2, both rows exit=-1 with executor_command_missing, stdout empty, qa stage_executed=0.

S2 seed2_failed_shell_tc_no_passed: exit 1, canon sha256 18edc88107aeb521a209045c6b79ab27b06bf695a7ee3765035821bb70578c50. Red: exit=-1 instead of the script's 7, stdout and stderr empty.

S3 seed3_empty_tc_no_passed: exit 0, canon sha256 3e0e989c1e408e9be36c246cd3b5825e74cb8b60d12f479d0154620e851d415e. Green guard: empty and blocked-only matrices both return autotest_no_executable_tests with no event and no report.

S2 classification: red. The failing-shell path already refuses qa evidence, but its per-TC evidence is wrong (exit=-1, empty output), so it fails the AC-003 contract.

## File sha256

```text
15f63c3448b7958130430cf8589522f08bb8eac7621a74c35c109b700e8f6dc0  docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/seeds/lib.sh
e6c8350ef5999a13dd78e084d155daeb6e159c6381558f4a716a30a3b495a24f  docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/seeds/seed1_shell_tc_executes.sh
d0b85e6ed6b9c0c7f49701f01c769a0408c6414872dc1c431fb2ef5c71e87b53  docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/seeds/seed2_failed_shell_tc_no_passed.sh
6b0f4ca75c322642542d9df448b643ccb31f22cd1cd49dd664938c0a750a90be  docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/seeds/seed3_empty_tc_no_passed.sh
9a5d5b972b8944593231f3fad95fa183febc004ead8d2d8c1f828a2d42e3655d  docs/beacon/v2.0.0/features/kernel-autotest-shell-tc/evidence/plan-seed-red.log
```

## Red Output (excerpt from the combined log)

```text
seed1: autotest_rc=1 total=2 passed=0 failed=2 tc001_exit=-1 tc001_reason=executor_command_missing qa_stage_executed_count=0 autotest_executed_count=1
seed2: autotest_rc=1 total=1 failed=1 tc001_exit=-1 tc001_reason=executor_command_missing qa_stage_executed_count=0 autotest_executed_count=1
seed3: SEED_RC_empty=1 SEED_QA_empty=0 SEED_AT_empty=0 SEED_REPORT_empty=absent; SEED_RC_blocked=1 SEED_QA_blocked=0 SEED_AT_blocked=0 SEED_REPORT_blocked=absent
```

## Stop State

- Four-piece package present under this feature path.
- Production code untouched: no tracked file changed in beacon-go.
- No goal run created, no tick, no freeze, no push, no tag, no release.
- No QA verdict written by this packaging pass.
