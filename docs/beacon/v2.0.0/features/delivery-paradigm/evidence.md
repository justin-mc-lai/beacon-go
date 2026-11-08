# Evidence Index: delivery-paradigm

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | frozen | truth |
| tests | test_truth | tests.md | frozen | qa |
| tasks | support_advisory | tasks.md | present | truth |
| reproduction | support_advisory | repro-k1-k6 (in this evidence directory) | recorded | qa |
| hard gate | support_advisory | go-test-main (in this evidence directory) | recorded | qa |
| freeze | acceptance_truth | freeze-log (in this evidence directory) | recorded | truth |

## Reproduction Log

Every row below comes from a real command run on 2026-11-08 against beacon-go v2.0.0-go.15 and the renlink project. Full output is in repro-k1-k6 under this evidence directory.

| Observation | Command | Result |
|-------------|---------|--------|
| K1 | beacon-go goal status --run-id run_1084fcb4301353f56601ec48 | status blocked, stage plan, reason stage_executor_not_registered |
| K2 | beacon-go gate status --feature l1-parser-core --project . | truth_freeze/design/implement/qa/release all not_evaluated; truth_integrity audit |
| K3 | beacon-go goal receive-result --run-id run_bde3174e4fdc3edd7710c87b | Error: required flag(s) delegation-id not set |
| K4 | beacon-go goal program route --run-id run_1084fcb4301353f56601ec48 | l2-l7 blocked program_truth_not_frozen; only next batch l1-parser-core |
| K5 | rg -n ux_required and UI heading in the l5 truth | ux_required false with a full UI specification chapter |
| K6 | beacon-go goal --help | no conformance command present |

## Root Cause Notes

- K1: the run stores an executor identity only; without an exec command the plan stage has no registered executor and blocks at exit. There is no manual/external executor and no creation-time hint.
- K2: the tick path advances the stage after a derived exit judgement that is never persisted, so the gate list stays not_evaluated for plan/truth/design/implement while qa/release are formal-first.
- K3: receive-result belongs to the ralph delegation loop and hard-requires a delegation id; program complete is sea-level release. An external room has no lake-level writeback.
- K4: program route prints per-lake blocked reasons and one next batch, without an ordered action list that names which truth to freeze first.
- K5: CheckIntegrity cross-references AC, TC, deferrals, fake delivery, evidence artifacts and task bindings. It never compares the ux_required declaration with UI or design chapters in the body.
- K6: no delivery conformance surface exists; projects export-state is optional and not part of any check.

## Hard Gate Log

Main branch before any edit: beacon-go v2.0.0-go.15 at main commit 5ab05f5, clean working tree. The full test suite passed; orchestrator 83.075s, cli 13.185s, harness 10.036s. The transcript is in go-test-main under this evidence directory.

## Freeze Log

The freeze precheck and result are recorded in freeze-log under this evidence directory. Freeze ran on canonical main because AC-136 restricts truth writes to the canonical branch; the resulting package is committed on fix/delivery-paradigm, so main history stays untouched.

## Boundary

This file is an index and read model. It carries no gate verdict, no release readiness and no acceptance authority; those belong to gate evaluations and the independent verifier.

