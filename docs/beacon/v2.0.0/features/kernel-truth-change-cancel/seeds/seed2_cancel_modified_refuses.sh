#!/bin/zsh
# Seed 2: a pending change whose truth body moved away from the frozen
# baseline must be REFUSED by cancel, with the package left
# change_pending and the route pointed at refreeze.
#
# Today `--cancel` does not exist, so the call fails for the wrong
# reason (unknown flag) even though the body was edited. Green contract:
# non-zero exit, error naming refreeze, status stays change_pending,
# zero truth_change_cancelled events, and cancel leaves the edited body
# byte-for-byte untouched (no silent revert).
set -eu
HERE="${0:A:h}"
source "$HERE/lib.sh"
fixture_init

OUT="$SEED_TMP/out"
CANON_DIR="${SEED_CANON_DIR:-$OUT}"
mkdir -p "$CANON_DIR"
CANON="$CANON_DIR/seed2.canon"

CHANGE_RC=0
"$BEACON_BIN" truth change --project "$SEED_PROJECT" --feature demo \
  --summary "seed change modified" --by seed --json > "$OUT/change.json" 2> "$OUT/change.err" || CHANGE_RC=$?
CHANGE_DOC=$(json_field "$OUT/change.json" 'd.get("change_doc")' 2>/dev/null || echo missing)

# The author edits the frozen truth body inside the pending change.
printf '\n## Amendment\n\n- pending edit\n' >> "$FEATURE_DIR/truth.md"
EDITED_SHA=$(body_sha)

CANCEL_RC=0
"$BEACON_BIN" truth change --cancel --project "$SEED_PROJECT" --feature demo \
  --by seed > "$OUT/cancel.out" 2> "$OUT/cancel.err" || CANCEL_RC=$?
CANCEL_ERR=$(tr '\n' ' ' < "$OUT/cancel.err")

"$BEACON_BIN" truth show --project "$SEED_PROJECT" --feature demo --json > "$OUT/show.json" 2>/dev/null || true
STATUS=$(json_field "$OUT/show.json" 'd.get("status")' 2>/dev/null || echo parse_error)
PENDING=$(json_field "$OUT/show.json" 'd.get("change_pending")' 2>/dev/null || echo parse_error)
IDS=$(json_field "$OUT/show.json" 'd.get("pending_change_ids") or []' 2>/dev/null || echo parse_error)
CANCEL_EVENTS=$(event_count truth_change_cancelled)
REQUESTED_EVENTS=$(event_count truth_change_requested)
REFROZEN_EVENTS=$(event_count truth_refrozen)
BODY_AFTER=$(body_sha)
BODY_UNTOUCHED=$([ "$BODY_AFTER" = "$EDITED_SHA" ] && echo yes || echo no)
DOC_PRESENT=$([ -f "$SEED_PROJECT/$CHANGE_DOC" ] && echo yes || echo no)

{
  echo "change_rc=${CHANGE_RC}"
  echo "change_pending_at_open=$(json_field "$OUT/change.json" 'd.get("change_pending")' 2>/dev/null || echo parse_error)"
  echo "cancel_rc=${CANCEL_RC}"
  echo "cancel_stderr=${CANCEL_ERR}"
  echo "status=${STATUS}"
  echo "change_pending=${PENDING}"
  echo "pending_ids=${IDS}"
  echo "cancel_events=${CANCEL_EVENTS}"
  echo "requested_events=${REQUESTED_EVENTS}"
  echo "refrozen_events=${REFROZEN_EVENTS}"
  echo "body_untouched=${BODY_UNTOUCHED}"
  echo "change_doc_present=${DOC_PRESENT}"
} > "$CANON"
cat "$CANON"
echo "--- cancel stderr"
cat "$OUT/cancel.err" 2>/dev/null || true
emit_canon "$CANON"

if [ "$CHANGE_RC" -ne 0 ]; then
  echo "RESULT: FAIL - could not open the change: ${CANCEL_ERR}"
  exit 1
fi
if [ "$CANCEL_RC" -eq 0 ]; then
  echo "RESULT: FAIL - cancel accepted a modified body; it must refuse and route to refreeze"
  exit 1
fi
if [[ "$CANCEL_ERR" != *refreeze* ]]; then
  echo "RESULT: FAIL - refusal did not point at refreeze: ${CANCEL_ERR}"
  exit 1
fi
if [ "$STATUS" != "change_pending" ] || [ "$PENDING" != "True" ] || [ "$IDS" = "[]" ]; then
  echo "RESULT: FAIL - refused cancel changed state: status=${STATUS} pending=${PENDING} ids=${IDS}"
  exit 1
fi
if [ "$CANCEL_EVENTS" != "0" ]; then
  echo "RESULT: FAIL - refused cancel appended a cancel event (${CANCEL_EVENTS})"
  exit 1
fi
if [ "$REFROZEN_EVENTS" != "0" ]; then
  echo "RESULT: FAIL - cancel wrote a refreeze event (${REFROZEN_EVENTS})"
  exit 1
fi
if [ "$BODY_UNTOUCHED" != "yes" ]; then
  echo "RESULT: FAIL - cancel modified the edited truth body"
  exit 1
fi
if [ "$DOC_PRESENT" != "yes" ]; then
  echo "RESULT: FAIL - cancel deleted the pending change document"
  exit 1
fi
echo "RESULT: PASS - modified body is refused, change_pending kept, refreeze route named, body untouched"
