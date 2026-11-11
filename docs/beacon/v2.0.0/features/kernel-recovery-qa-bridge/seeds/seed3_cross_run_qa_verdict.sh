#!/bin/zsh
# Seed 3: qa verdicts must stay bound to their own run.
#
# Today: the exit gate reads gate facts FORMAL-FIRST and falls back to
# feature-level gate rows when the run has none. A passed qa verdict from
# run A (not referenced by any stage-exit event) leaks into a brand new run
# B on the same feature, so B skips its qa gate without any qa evidence.
#
# Green contract: run B, with no qa evidence of its own, must be blocked at
# qa by the qa gate even though run A passed it.
set -eu
HERE="${0:A:h}"
source "$HERE/lib.sh"
fixture_init

# Run A: qa work + a qa tick-out, then a passing qa verdict on A's own run.
RUN_A=$(new_run)
tick_to "$RUN_A" 3
"$BEACON_BIN" qa autotest run --run-id "$RUN_A" --by qa-bot --json >"$SEED_TMP/out/seed3-a-autotest.json" 2>"$SEED_TMP/out/seed3-a-autotest.err" || true
tick_to "$RUN_A" 1
GATE_A=$SEED_TMP/out/seed3-a-gate.json
"$BEACON_BIN" gate evaluate qa --project "$SEED_PROJECT" --feature demo \
  --run "$RUN_A" --evaluator qa-bot --json >"$GATE_A" 2>"$SEED_TMP/out/seed3-a-gate.err" || true
A_VERDICT=$(json_field "$GATE_A" 'd.get("verdict")')

# Run B: fresh run, no qa evidence at all.
RUN_B=$(new_run)
tick_to "$RUN_B" 3
B_EVIDENCE=$(sqlite3 "$BEACON_STORE_PATH" "select count(*) from lifecycle_events where run_id='$RUN_B' and event_type='stage_executed' and json_extract(payload_json,'\$.stage')='qa' and json_extract(payload_json,'\$.placeholder')=0;")
B_TICK=$SEED_TMP/out/seed3-b-tick.json
"$BEACON_BIN" goal tick --run-id "$RUN_B" --by opus --json >"$B_TICK" 2>"$SEED_TMP/out/seed3-b-tick.err" || true
B_STAGE=$(json_field "$B_TICK" 'd.get("current_stage")')
B_BLOCKED=$(json_field "$B_TICK" 'd.get("blocked")')
B_REASON=$(json_field "$B_TICK" 'd.get("reason_code")')

CANON=$SEED_TMP/out/seed3.canon
{
  echo "run_a_qa_verdict=${A_VERDICT}"
  echo "run_b_stage=${B_STAGE}"
  echo "run_b_blocked=${B_BLOCKED}"
  echo "run_b_reason=${B_REASON}"
  echo "run_b_qa_stage_executed_before_exit_tick=${B_EVIDENCE}"
} > "$CANON"
cat "$CANON"
emit_canon "$CANON"

if [[ "$B_STAGE" == "release" && "$B_BLOCKED" != "True" ]]; then
  echo "RESULT: FAIL - run B reached ${B_STAGE} without its own qa evidence, borrowing run A's verdict"
  exit 1
fi
echo "RESULT: PASS - run B stayed at ${B_STAGE} (blocked=${B_BLOCKED}, reason=${B_REASON}) without its own qa evidence"
