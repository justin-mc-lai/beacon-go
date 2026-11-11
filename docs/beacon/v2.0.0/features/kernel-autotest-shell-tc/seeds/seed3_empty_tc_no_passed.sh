#!/bin/zsh
# Seed 3: an empty executable TC set stays refused and writes no passed
# evidence — before and after the shell-TC change.
#
# Measured on HEAD 36c40b6: this guard is ALREADY GREEN. `RunAutoTest`
# returns `autotest_no_executable_tests` before writing any event when no
# row yields a job, so neither `autotest_executed` nor a qa
# `stage_executed` appears. The seed exists to freeze that behaviour so a
# shell-TC change cannot quietly turn an empty matrix into a pass.
#
# It covers two shapes: a matrix with no TC rows at all, and a matrix whose
# only row declares BLOCKED.
set -eu
HERE="${0:A:h}"
source "$HERE/lib.sh"
fixture_init

case_run() {
  local label="$1" run out rc=0
  run=$(new_run)
  tick_to "$run" 3
  out="$SEED_TMP/out/seed3-$label.json"
  "$BEACON_BIN" qa autotest run --run-id "$run" --by qa-seed --json \
    >"$out" 2>"$SEED_TMP/out/seed3-$label.err" || rc=$?
  echo "SEED_RC_${label}=${rc}"
  echo "SEED_QA_${label}=$(qa_stage_executed_count "$run")"
  echo "SEED_AT_${label}=$(autotest_event_count "$run")"
  echo "SEED_REPORT_${label}=$([ -f "$SEED_PROJECT/.beacon/autotest/$run/report.json" ] && echo present || echo absent)"
  echo "SEED_ERR_${label}=$(tr -d '\n' < "$SEED_TMP/out/seed3-$label.err" 2>/dev/null || true)"
}

reset_tests
CANON=$SEED_TMP/out/seed3.canon
: > "$CANON"
echo '# shape=empty-matrix' >> "$CANON"
case_run empty >> "$CANON"
echo '# shape=blocked-only' >> "$CANON"
reset_tests
add_tc AC-001 TC-001 'BLOCKED: needs a human fixture' 'declared blocked'
case_run blocked >> "$CANON"
cat "$CANON"
emit_canon "$CANON"

canon_value() {
  awk -F= -v k="$2" 'index($0, k"=")==1 { print substr($0, length(k)+2) }' "$1"
}

for label in empty blocked; do
  rc=$(canon_value "$CANON" "SEED_RC_${label}")
  qa=$(canon_value "$CANON" "SEED_QA_${label}")
  at=$(canon_value "$CANON" "SEED_AT_${label}")
  report=$(canon_value "$CANON" "SEED_REPORT_${label}")
  err=$(canon_value "$CANON" "SEED_ERR_${label}")
  if [ "$rc" -eq 0 ]; then
    echo "RESULT: FAIL - ${label} TC set exited 0; an empty set must be refused"
    exit 1
  fi
  if [[ "$err" != *autotest_no_executable_tests* ]]; then
    echo "RESULT: FAIL - ${label} TC set did not report autotest_no_executable_tests: ${err}"
    exit 1
  fi
  if [ "$qa" != "0" ] || [ "$at" != "0" ]; then
    echo "RESULT: FAIL - ${label} TC set wrote evidence (qa=${qa}, autotest=${at})"
    exit 1
  fi
  if [ "$report" != "absent" ]; then
    echo "RESULT: FAIL - ${label} TC set wrote an autotest report; nothing ran"
    exit 1
  fi
done
echo "RESULT: PASS (guard) - empty and blocked-only TC sets stay refused with no passed evidence"
