#!/bin/zsh
# Seed 2: a failing shell TC must fail with its real exit code and must not
# write passed qa evidence.
#
# Today: the bare project-relative script path never reaches the script, so
# the harness reports exit=-1/`executor_command_missing` instead of the
# script's own exit 7, and stdout/stderr stay empty. The suite is refused,
# but the per-TC evidence is wrong — an operator cannot tell what the TC
# actually did.
#
# Green contract: the script runs through its interpreter, the result row
# carries exit_code=7 with the script's stdout/stderr on disk, and the
# aggregate still writes no qa stage_executed.
set -eu
HERE="${0:A:h}"
source "$HERE/lib.sh"
fixture_init

RUN=$(new_run)
tick_to "$RUN" 3

mkdir -p "$SEED_PROJECT/scripts"
printf '#!/bin/sh\necho partial-out\necho boom-err >&2\nexit 7\n' > "$SEED_PROJECT/scripts/fail_shell.sh"
chmod 644 "$SEED_PROJECT/scripts/fail_shell.sh"

reset_tests
add_tc AC-001 TC-001 'scripts/fail_shell.sh' 'exits 7 and keeps its own output'

AUTOTEST=$SEED_TMP/out/seed2-autotest.json
AT_RC=0
"$BEACON_BIN" qa autotest run --run-id "$RUN" --by qa-seed --json \
  >"$AUTOTEST" 2>"$SEED_TMP/out/seed2-autotest.err" || AT_RC=$?

AT_TOTAL=$(json_field "$AUTOTEST" 'd.get("total")' 2>/dev/null || echo parse_error)
AT_FAILED=$(json_field "$AUTOTEST" 'd.get("failed")' 2>/dev/null || echo parse_error)
TC_STATUS=$(json_field "$AUTOTEST" 'd["results"][0]["status"]' 2>/dev/null || echo missing)
TC_EXIT=$(json_field "$AUTOTEST" 'd["results"][0].get("exit_code")' 2>/dev/null || echo missing)
TC_REASON=$(json_field "$AUTOTEST" 'd["results"][0].get("reason_code")' 2>/dev/null || echo missing)
TC_OUT=$(cat "$(tc_file "$RUN" TC-001)/stdout.log" 2>/dev/null | tr -d '\n' || true)
TC_ERR=$(cat "$(tc_file "$RUN" TC-001)/stderr.log" 2>/dev/null | tr -d '\n' || true)
QA_EVIDENCE=$(qa_stage_executed_count "$RUN")
AT_EVENTS=$(autotest_event_count "$RUN")

CANON=$SEED_TMP/out/seed2.canon
{
  echo "autotest_rc=${AT_RC}"
  echo "autotest_total=${AT_TOTAL}"
  echo "autotest_failed=${AT_FAILED}"
  echo "tc001_status=${TC_STATUS}"
  echo "tc001_exit=${TC_EXIT}"
  echo "tc001_reason=${TC_REASON}"
  echo "tc001_stdout=${TC_OUT}"
  echo "tc001_stderr=${TC_ERR}"
  echo "qa_stage_executed_count=${QA_EVIDENCE}"
  echo "autotest_executed_count=${AT_EVENTS}"
} > "$CANON"
cat "$CANON"
emit_canon "$CANON"

if [ "$AT_RC" -eq 0 ]; then
  echo "RESULT: FAIL - a failing shell TC must not exit 0"
  exit 1
fi
if [ "$AT_TOTAL" != "1" ] || [ "$AT_FAILED" != "1" ] || [ "$TC_STATUS" != "failed" ]; then
  echo "RESULT: FAIL - expected total=1 failed=1 status=failed, got total=${AT_TOTAL} failed=${AT_FAILED} status=${TC_STATUS}"
  exit 1
fi
if [ "$TC_EXIT" != "7" ]; then
  echo "RESULT: FAIL - expected the script's exit 7, got exit=${TC_EXIT} reason=${TC_REASON}"
  exit 1
fi
if [[ "$TC_OUT" != *partial-out* ]] || [[ "$TC_ERR" != *boom-err* ]]; then
  echo "RESULT: FAIL - stdout/stderr evidence missing: stdout='${TC_OUT}' stderr='${TC_ERR}'"
  exit 1
fi
if [ "$QA_EVIDENCE" != "0" ]; then
  echo "RESULT: FAIL - a failing shell TC wrote ${QA_EVIDENCE} qa stage_executed event(s)"
  exit 1
fi
if [ "$AT_EVENTS" != "1" ]; then
  echo "RESULT: FAIL - expected 1 audit autotest_executed event, got ${AT_EVENTS}"
  exit 1
fi
echo "RESULT: PASS - failing shell TC kept exit 7 and its output, and wrote no passed qa evidence"
