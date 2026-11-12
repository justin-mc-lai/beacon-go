# Evidence Index: kernel-goal-advise

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v2.0.0/features/kernel-goal-advise/truth.md | frozen | truth |
| tests | test_truth | docs/beacon/v2.0.0/features/kernel-goal-advise/tests.md | frozen | qa |
| tasks | support_advisory | docs/beacon/v2.0.0/features/kernel-goal-advise/tasks.md | present | truth |
| seed 1 | acceptance_truth | docs/beacon/v2.0.0/features/kernel-goal-advise/seeds/seed1_blocked_ranks_first.sh | red on installed binary | qa |
| seed 2 | acceptance_truth | docs/beacon/v2.0.0/features/kernel-goal-advise/seeds/seed2_deterministic.sh | red on installed binary | qa |
| seed 3 | acceptance_truth | docs/beacon/v2.0.0/features/kernel-goal-advise/seeds/seed3_no_invented_facts.sh | red on installed binary | qa |
| red log | support_advisory | docs/beacon/v2.0.0/features/kernel-goal-advise/evidence/red-baseline.log | recorded | qa |

## Reproduction Log

2026-11-11：基线用已安装二进制（不含 advise），新实现用 `go build -o /tmp/beacon-advise ./cmd/beacon`。

| AC | 基线 | 新实现 |
|----|------|--------|
| AC-001 | FAIL：unknown flag --project | PASS：blocked 首项，score=150 |
| AC-002 | FAIL：同上 | PASS：两次输出一致 |
| AC-003 | FAIL：同上 | PASS：事实全部可回溯 |
