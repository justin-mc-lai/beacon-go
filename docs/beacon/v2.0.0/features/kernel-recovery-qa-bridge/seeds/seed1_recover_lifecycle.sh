#!/bin/zsh
# Seed 1: goal recover must restore the lifecycle snapshot.
#
# Today: goal block writes feature_lifecycle = Implement/Blocked and
# goal recover clears only the run's blocked/streak fields. The
# snapshot stays Blocked, so the next tick at implement hits
# exitgates.go's Implement+Blocked guard forever.
#
# Green contract: after recover the lifecycle is no longer Blocked and
# the next implement tick advances to qa.
set -eu
HERE="${0:A:h}"
source "$HERE/lib.sh"
fixture_init

RUN=$(new_run)
tick_to "$RUN" 2   # plan -> truth -> implement (active, not executed)

"$BEACON_BIN" goal block --run-id "$RUN" --reason-code evidence_gap >/dev/null 2>&1 || true
AFTER_BLOCK=$(lifecycle_state)

"$BEACON_BIN" goal recover --run-id "$RUN" --by opus --note "cause resolved" >/dev/null 2>&1 || true
AFTER_RECOVER=$(lifecycle_state)

TICK=$SEED_TMP/out/seed1-tick.json
"$BEACON_BIN" goal tick --run-id "$RUN" --by opus --json >"$TICK" 2>"$SEED_TMP/out/seed1-tick.err" || true
TICK_STAGE=$(json_field "$TICK" 'd["current_stage"]')
TICK_BLOCKED=$(json_field "$TICK" 'd.get("blocked")')
TICK_REASON=$(json_field "$TICK" 'd.get("reason_code")')

CANON=$SEED_TMP/out/seed1.canon
{
  echo "after_block=${AFTER_BLOCK}"
  echo "after_recover=${AFTER_RECOVER}"
  echo "tick_stage=${TICK_STAGE}"
  echo "tick_blocked=${TICK_BLOCKED}"
  echo "tick_reason=${TICK_REASON}"
} > "$CANON"
cat "$CANON"
emit_canon "$CANON"

if [[ "${AFTER_RECOVER}" == *"/Blocked" ]]; then
  echo "RESULT: FAIL - recover cleared the run but lifecycle stayed ${AFTER_RECOVER}"
  exit 1
fi
if [[ "${TICK_REASON}" == "stage_exit_implement_blocked" ]]; then
  echo "RESULT: FAIL - implement exit gate is still self-locked by the stale Blocked snapshot"
  exit 1
fi
echo "RESULT: PASS - recover restored the lifecycle snapshot and implement advanced to ${TICK_STAGE}"

