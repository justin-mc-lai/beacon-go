#!/bin/zsh
# Seed 1: cancelling a pending change whose body still equals the frozen
# baseline closes it and returns the package to frozen.
#
# Today `beacon truth change` has no --cancel flag, so the cancel call
# fails with `unknown flag: --cancel` and the package stays
# change_pending. Green contract: exit 0, status frozen,
# change_pending=false, one appended truth_change_cancelled event, and
# the truth body bytes unchanged.
#
# Case B covers the legacy record: a pending change whose base body hash
# was never recorded is closable only with --accept-current; without the
# flag the cancel refuses and the package stays change_pending.
set -eu
HERE="${0:A:h}"
source "$HERE/lib.sh"
fixture_init

BASE_SHA=$(body_sha)
OUT="$SEED_TMP/out"
CANON_DIR="${SEED_CANON_DIR:-$OUT}"
mkdir -p "$CANON_DIR"
CANON="$CANON_DIR/seed1.canon"

# ---- case A: identical body, base fingerprint present ----
CHANGE_A_RC=0
"$BEACON_BIN" truth change --project "$SEED_PROJECT" --feature demo \
  --summary "seed change A" --by seed --json > "$OUT/a-change.json" 2> "$OUT/a-change.err" || CHANGE_A_RC=$?
A_PENDING=$(json_field "$OUT/a-change.json" 'd.get("change_pending")' 2>/dev/null || echo parse_error)
A_CHANGE_ID=$(json_field "$OUT/a-change.json" 'd.get("change_id")' 2>/dev/null || echo missing)

C1_RC=0
"$BEACON_BIN" truth change --cancel --project "$SEED_PROJECT" --feature demo \
  --by seed > "$OUT/a-cancel.out" 2> "$OUT/a-cancel.err" || C1_RC=$?
"$BEACON_BIN" truth show --project "$SEED_PROJECT" --feature demo --json > "$OUT/a-show.json" 2>/dev/null || true
C1_STATUS=$(json_field "$OUT/a-show.json" 'd.get("status")' 2>/dev/null || echo parse_error)
C1_PENDING=$(json_field "$OUT/a-show.json" 'd.get("change_pending")' 2>/dev/null || echo parse_error)
C1_IDS=$(json_field "$OUT/a-show.json" 'd.get("pending_change_ids") or []' 2>/dev/null || echo parse_error)
C1_CANCEL_EVENTS=$(event_count truth_change_cancelled)
C1_REQUESTED_EVENTS=$(event_count truth_change_requested)
C1_BODY_UNCHANGED=$([ "$(body_sha)" = "$BASE_SHA" ] && echo yes || echo no)
C1_OUT=$(tr '\n' ' ' < "$OUT/a-cancel.out")
C1_ERR=$(tr '\n' ' ' < "$OUT/a-cancel.err")

# ---- case B: legacy change with no recorded base fingerprint ----
CHANGE_B_RC=0
"$BEACON_BIN" truth change --project "$SEED_PROJECT" --feature demo \
  --summary "seed change B" --by seed --json > "$OUT/b-change.json" 2> "$OUT/b-change.err" || CHANGE_B_RC=$?
B_CHANGE_DOC=$(json_field "$OUT/b-change.json" 'd.get("change_doc")' 2>/dev/null || echo missing)
B_BLANKED=no
if [ "$CHANGE_B_RC" -eq 0 ] && [ "$B_CHANGE_DOC" != "missing" ] && [ "$B_CHANGE_DOC" != "None" ] && [ -f "$SEED_PROJECT/$B_CHANGE_DOC" ]; then
  blank_change_base "$B_CHANGE_DOC"
  B_BLANKED=yes
fi

C2_NOREFUSE_RC=0
"$BEACON_BIN" truth change --cancel --project "$SEED_PROJECT" --feature demo \
  --by seed > "$OUT/b-norefuse.out" 2> "$OUT/b-norefuse.err" || C2_NOREFUSE_RC=$?
C2_NOREFUSE_ERR=$(tr '\n' ' ' < "$OUT/b-norefuse.err")
"$BEACON_BIN" truth show --project "$SEED_PROJECT" --feature demo --json > "$OUT/b-after-refusal.json" 2>/dev/null || true
C2_STATUS_REFUSED=$(json_field "$OUT/b-after-refusal.json" 'd.get("status")' 2>/dev/null || echo parse_error)

C2_ACCEPT_RC=0
"$BEACON_BIN" truth change --cancel --accept-current --project "$SEED_PROJECT" --feature demo \
  --by seed > "$OUT/b-accept.out" 2> "$OUT/b-accept.err" || C2_ACCEPT_RC=$?
"$BEACON_BIN" truth show --project "$SEED_PROJECT" --feature demo --json > "$OUT/b-show.json" 2>/dev/null || true
C2_STATUS=$(json_field "$OUT/b-show.json" 'd.get("status")' 2>/dev/null || echo parse_error)
C2_PENDING=$(json_field "$OUT/b-show.json" 'd.get("change_pending")' 2>/dev/null || echo parse_error)
C2_CANCEL_EVENTS=$(event_count truth_change_cancelled)
C2_REQUESTED_EVENTS=$(event_count truth_change_requested)
C2_ACCEPTED=$(payload_field truth_change_cancelled 'd.get("accepted_current", False)' 2>/dev/null || echo parse_error)
C2_BASELINE_MISSING=$(payload_field truth_change_cancelled 'd.get("baseline_missing", False)' 2>/dev/null || echo parse_error)
C2_BODY_UNCHANGED=$([ "$(body_sha)" = "$BASE_SHA" ] && echo yes || echo no)

{
  echo "change_a_rc=${CHANGE_A_RC}"
  echo "change_a_pending=${A_PENDING}"
  echo "change_a_id=${A_CHANGE_ID}"
  echo "c1_cancel_rc=${C1_RC}"
  echo "c1_status=${C1_STATUS}"
  echo "c1_change_pending=${C1_PENDING}"
  echo "c1_pending_ids=${C1_IDS}"
  echo "c1_cancel_events=${C1_CANCEL_EVENTS}"
  echo "c1_requested_events=${C1_REQUESTED_EVENTS}"
  echo "c1_body_unchanged=${C1_BODY_UNCHANGED}"
  echo "c1_stdout=${C1_OUT}"
  echo "c1_stderr=${C1_ERR}"
  echo "change_b_rc=${CHANGE_B_RC}"
  echo "change_b_doc=${B_CHANGE_DOC}"
  echo "change_b_base_blanked=${B_BLANKED}"
  echo "c2_norefuse_rc=${C2_NOREFUSE_RC}"
  echo "c2_norefuse_err=${C2_NOREFUSE_ERR}"
  echo "c2_status_after_refusal=${C2_STATUS_REFUSED}"
  echo "c2_accept_rc=${C2_ACCEPT_RC}"
  echo "c2_status=${C2_STATUS}"
  echo "c2_change_pending=${C2_PENDING}"
  echo "c2_cancel_events=${C2_CANCEL_EVENTS}"
  echo "c2_requested_events=${C2_REQUESTED_EVENTS}"
  echo "c2_accepted_current=${C2_ACCEPTED}"
  echo "c2_baseline_missing=${C2_BASELINE_MISSING}"
  echo "c2_body_unchanged=${C2_BODY_UNCHANGED}"
} > "$CANON"
cat "$CANON"
echo "--- a-cancel stderr"
cat "$OUT/a-cancel.err" 2>/dev/null || true
echo "--- b-norefuse stderr"
cat "$OUT/b-norefuse.err" 2>/dev/null || true
echo "--- b-accept stderr"
cat "$OUT/b-accept.err" 2>/dev/null || true
emit_canon "$CANON"

if [ "$CHANGE_A_RC" -ne 0 ] || [ "$A_PENDING" != "True" ]; then
  echo "RESULT: FAIL - case A could not open a change (rc=${CHANGE_A_RC}, pending=${A_PENDING})"
  exit 1
fi
if [ "$C1_RC" -ne 0 ]; then
  echo "RESULT: FAIL - cancel --cancel exited ${C1_RC}: ${C1_ERR}"
  exit 1
fi
if [ "$C1_STATUS" != "frozen" ] || [ "$C1_PENDING" != "False" ] || [ "$C1_IDS" != "[]" ]; then
  echo "RESULT: FAIL - after cancel status=${C1_STATUS} pending=${C1_PENDING} ids=${C1_IDS} (want frozen/False/[])"
  exit 1
fi
if [ "$C1_CANCEL_EVENTS" != "1" ] || [ "$C1_REQUESTED_EVENTS" != "1" ]; then
  echo "RESULT: FAIL - expected 1 requested + 1 cancelled event, got requested=${C1_REQUESTED_EVENTS} cancelled=${C1_CANCEL_EVENTS}"
  exit 1
fi
if [ "$C1_BODY_UNCHANGED" != "yes" ]; then
  echo "RESULT: FAIL - cancel edited the truth body"
  exit 1
fi
if [ "$C2_NOREFUSE_RC" -eq 0 ]; then
  echo "RESULT: FAIL - cancel closed a legacy change without --accept-current"
  exit 1
fi
if [[ "$C2_NOREFUSE_ERR" != *--accept-current* ]]; then
  echo "RESULT: FAIL - refusal did not point at --accept-current: ${C2_NOREFUSE_ERR}"
  exit 1
fi
if [ "$C2_STATUS_REFUSED" != "change_pending" ]; then
  echo "RESULT: FAIL - refused legacy cancel left status=${C2_STATUS_REFUSED} (want change_pending)"
  exit 1
fi
if [ "$C2_ACCEPT_RC" -ne 0 ]; then
  echo "RESULT: FAIL - cancel --accept-current exited ${C2_ACCEPT_RC}: $(tr '\n' ' ' < "$OUT/b-accept.err")"
  exit 1
fi
if [ "$C2_STATUS" != "frozen" ] || [ "$C2_PENDING" != "False" ]; then
  echo "RESULT: FAIL - after accept-current status=${C2_STATUS} pending=${C2_PENDING} (want frozen/False)"
  exit 1
fi
if [ "$C2_CANCEL_EVENTS" != "2" ] || [ "$C2_REQUESTED_EVENTS" != "2" ]; then
  echo "RESULT: FAIL - expected 2 requested + 2 cancelled events, got requested=${C2_REQUESTED_EVENTS} cancelled=${C2_CANCEL_EVENTS}"
  exit 1
fi
if [ "$C2_ACCEPTED" != "True" ] || [ "$C2_BASELINE_MISSING" != "True" ]; then
  echo "RESULT: FAIL - accept-current event accepted=${C2_ACCEPTED} baseline_missing=${C2_BASELINE_MISSING} (want True/True)"
  exit 1
fi
if [ "$C2_BODY_UNCHANGED" != "yes" ]; then
  echo "RESULT: FAIL - accept-current edited the truth body"
  exit 1
fi
echo "RESULT: PASS - identical body cancels to frozen; legacy change needs and accepts --accept-current"
