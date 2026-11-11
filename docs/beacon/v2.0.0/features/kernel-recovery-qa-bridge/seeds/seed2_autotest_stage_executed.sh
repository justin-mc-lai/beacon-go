#!/bin/zsh
# Seed 2: a passing autotest must satisfy the qa exit gate.
#
# Today: `qa autotest run` records a passing test suite but emits only
# `autotest_executed`, never a qa `stage_executed` with placeholder=false.
# The qa gate reads `qa_stage_executed` from stage_executed facts only, so
# a run that just proved its tests pass is still refused with
# `qa_stage_not_executed` (circular dependency: the gate wants proof the
# stage executed, autotest only writes it after the gate passes).
#
# Green contract: after a passing autotest and no other qa evidence, the qa
# gate reaches pass (or at minimum does not block with qa_stage_not_executed).
set -eu
HERE="${0:A:h}"
source "$HERE/lib.sh"
fixture_init

RUN=$(new_run)
tick_to "$RUN" 3   # plan -> truth -> implement -> qa (active)

AUTOTEST=$SEED_TMP/out/seed2-autotest.json
"$BEACON_BIN" qa autotest run --run-id "$RUN" --by qa-bot --json >"$AUTOTEST" 2>"$SEED_TMP/out/seed2-autotest.err" || true
AT_TOTAL=$(json_field "$AUTOTEST" 'd.get("total")')
AT_PASSED=$(json_field "$AUTOTEST" 'd.get("passed")')
AT_FAILED=$(json_field "$AUTOTEST" 'd.get("failed")')

GATE=$SEED_TMP/out/seed2-gate.json
"$BEACON_BIN" gate evaluate qa --project "$SEED_PROJECT" --feature demo \
  --run "$RUN" --evaluator qa-bot --json >"$GATE" 2>"$SEED_TMP/out/seed2-gate.err" || true
QA_VERDICT=$(json_field "$GATE" 'd.get("verdict")')
QA_REASONS=$(json_field "$GATE" '",".join(d.get("reason_codes") or [])')
QA_CHECKS=$(json_field "$GATE" '",".join("%s=%s" % (c.get("check"), c.get("ok")) for c in (d.get("checks") or []))')
STAGE_EXEC_COUNT=$(qa_stage_executed_count)

CANON=$SEED_TMP/out/seed2.canon
{
  echo "autotest_total=${AT_TOTAL}"
  echo "autotest_passed=${AT_PASSED}"
  echo "autotest_failed=${AT_FAILED}"
  echo "qa_verdict=${QA_VERDICT}"
  echo "qa_reason_codes=${QA_REASONS}"
  echo "qa_checks=${QA_CHECKS}"
  echo "qa_stage_executed_count=${STAGE_EXEC_COUNT}"
} > "$CANON"
cat "$CANON"
emit_canon "$CANON"

if [[ "$QA_VERDICT" == "blocked" && "$QA_REASONS" == *qa_stage_not_executed* ]]; then
  echo "RESULT: FAIL - autotest passed ${AT_PASSED}/${AT_TOTAL} but the qa gate still reports qa_stage_not_executed"
  exit 1
fi
if [[ "$QA_VERDICT" != "pass" ]]; then
  echo "RESULT: FAIL - autotest passed ${AT_PASSED}/${AT_TOTAL} but the qa gate verdict is ${QA_VERDICT}"
  exit 1
fi
echo "RESULT: PASS - autotest produced qa stage evidence and the qa gate passed"
