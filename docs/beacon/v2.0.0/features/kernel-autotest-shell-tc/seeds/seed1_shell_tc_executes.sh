#!/bin/zsh
# Seed 1: a shell TC recorded as a project-relative script path must run.
#
# Today: `qa autotest run` hands argv[0] straight to exec.LookPath, which
# resolves against the CLI's working directory, and then exec.CommandContext
# requires the execute bit. A tests.md row such as `scripts/ok_shell.sh`
# (no execute bit, shebang present) therefore fails with
# `executor_command_missing`, and no qa evidence is written.
#
# Green contract: a row whose first word is an existing project-relative
# shell script (.sh/.zsh, or a file whose first line is a supported
# shebang) runs through that interpreter; per-TC exit code and stdout/stderr
# are recorded, and an all-pass suite writes real qa stage_executed.
set -eu
HERE="${0:A:h}"
source "$HERE/lib.sh"
fixture_init

RUN=$(new_run)
tick_to "$RUN" 3   # plan -> truth -> implement -> qa (active)

mkdir -p "$SEED_PROJECT/scripts"
printf '#!/bin/sh\necho hello-from-shell\n' > "$SEED_PROJECT/scripts/ok_shell.sh"
printf '#!/bin/zsh\necho hello-shebang\n' > "$SEED_PROJECT/scripts/check-noext"
chmod 644 "$SEED_PROJECT/scripts/ok_shell.sh" "$SEED_PROJECT/scripts/check-noext"

reset_tests
add_tc AC-001 TC-001 'scripts/ok_shell.sh' 'runs through /bin/sh and prints hello-from-shell'
add_tc AC-002 TC-002 'scripts/check-noext' 'runs through its shebang and prints hello-shebang'

AUTOTEST=$SEED_TMP/out/seed1-autotest.json
AT_RC=0
"$BEACON_BIN" qa autotest run --run-id "$RUN" --by qa-seed --json \
  >"$AUTOTEST" 2>"$SEED_TMP/out/seed1-autotest.err" || AT_RC=$?

AT_TOTAL=$(json_field "$AUTOTEST" 'd.get("total")' 2>/dev/null || echo parse_error)
AT_PASSED=$(json_field "$AUTOTEST" 'd.get("passed")' 2>/dev/null || echo parse_error)
AT_FAILED=$(json_field "$AUTOTEST" 'd.get("failed")' 2>/dev/null || echo parse_error)

TC1_STATUS=$(json_field "$AUTOTEST" 'd["results"][0]["status"]' 2>/dev/null || echo missing)
TC1_EXIT=$(json_field "$AUTOTEST" 'd["results"][0].get("exit_code")' 2>/dev/null || echo missing)
TC1_REASON=$(json_field "$AUTOTEST" 'd["results"][0].get("reason_code")' 2>/dev/null || echo missing)
TC2_STATUS=$(json_field "$AUTOTEST" 'd["results"][1]["status"]' 2>/dev/null || echo missing)
TC2_EXIT=$(json_field "$AUTOTEST" 'd["results"][1].get("exit_code")' 2>/dev/null || echo missing)

TC1_OUT=$(cat "$(tc_file "$RUN" TC-001)/stdout.log" 2>/dev/null | tr -d '\n' || true)
TC2_OUT=$(cat "$(tc_file "$RUN" TC-002)/stdout.log" 2>/dev/null | tr -d '\n' || true)
QA_EVIDENCE=$(qa_stage_executed_count "$RUN")
AT_EVENTS=$(autotest_event_count "$RUN")

CANON=$SEED_TMP/out/seed1.canon
{
  echo "autotest_rc=${AT_RC}"
  echo "autotest_total=${AT_TOTAL}"
  echo "autotest_passed=${AT_PASSED}"
  echo "autotest_failed=${AT_FAILED}"
  echo "tc001_status=${TC1_STATUS}"
  echo "tc001_exit=${TC1_EXIT}"
  echo "tc001_reason=${TC1_REASON}"
  echo "tc001_stdout=${TC1_OUT}"
  echo "tc002_status=${TC2_STATUS}"
  echo "tc002_exit=${TC2_EXIT}"
  echo "tc002_stdout=${TC2_OUT}"
  echo "qa_stage_executed_count=${QA_EVIDENCE}"
  echo "autotest_executed_count=${AT_EVENTS}"
} > "$CANON"
cat "$CANON"
echo "--- stderr"
cat "$SEED_TMP/out/seed1-autotest.err" 2>/dev/null || true
emit_canon "$CANON"

if [ "$AT_RC" -ne 0 ]; then
  echo "RESULT: FAIL - qa autotest run exited ${AT_RC}; shell TC rows did not execute"
  exit 1
fi
if [ "$AT_TOTAL" != "2" ] || [ "$AT_PASSED" != "2" ] || [ "$AT_FAILED" != "0" ]; then
  echo "RESULT: FAIL - expected total=2 passed=2 failed=0, got total=${AT_TOTAL} passed=${AT_PASSED} failed=${AT_FAILED}"
  exit 1
fi
if [ "$TC1_STATUS" != "passed" ] || [ "$TC1_EXIT" != "0" ]; then
  echo "RESULT: FAIL - TC-001 status=${TC1_STATUS} exit=${TC1_EXIT} (want passed/0)"
  exit 1
fi
if [ "$TC2_STATUS" != "passed" ] || [ "$TC2_EXIT" != "0" ]; then
  echo "RESULT: FAIL - TC-002 status=${TC2_STATUS} exit=${TC2_EXIT} (want passed/0)"
  exit 1
fi
if [[ "$TC1_OUT" != *hello-from-shell* ]]; then
  echo "RESULT: FAIL - TC-001 stdout missing hello-from-shell: '${TC1_OUT}'"
  exit 1
fi
if [[ "$TC2_OUT" != *hello-shebang* ]]; then
  echo "RESULT: FAIL - TC-002 stdout missing hello-shebang: '${TC2_OUT}'"
  exit 1
fi
if [ "$QA_EVIDENCE" != "1" ]; then
  echo "RESULT: FAIL - expected 1 real qa stage_executed for this run, got ${QA_EVIDENCE}"
  exit 1
fi
echo "RESULT: PASS - both shell TC forms executed, exit codes and stdout recorded, qa evidence written"
