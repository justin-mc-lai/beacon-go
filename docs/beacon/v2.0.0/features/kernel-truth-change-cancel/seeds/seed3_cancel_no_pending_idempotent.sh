#!/bin/zsh
# Seed 3: cancel with no pending change is a successful no-op.
#
# Today `--cancel` does not exist, so the call is an error; after the
# feature lands, exit 0, no stderr, no new event, and the frozen package
# is unchanged. Running it twice proves idempotence.
set -eu
HERE="${0:A:h}"
source "$HERE/lib.sh"
fixture_init

OUT="$SEED_TMP/out"
CANON_DIR="${SEED_CANON_DIR:-$OUT}"
mkdir -p "$CANON_DIR"
CANON="$CANON_DIR/seed3.canon"
BASE_SHA=$(body_sha)
: > "$CANON"

for label in first second; do
  rc=0
  "$BEACON_BIN" truth change --cancel --project "$SEED_PROJECT" --feature demo \
    --by seed > "$OUT/$label.out" 2> "$OUT/$label.err" || rc=$?
  "$BEACON_BIN" truth show --project "$SEED_PROJECT" --feature demo --json > "$OUT/$label-show.json" 2>/dev/null || true
  {
    echo "# run=$label"
    echo "rc_${label}=${rc}"
    echo "stderr_${label}=$(tr '\n' ' ' < "$OUT/$label.err")"
    echo "status_${label}=$(json_field "$OUT/$label-show.json" 'd.get("status")' 2>/dev/null || echo parse_error)"
    echo "change_pending_${label}=$(json_field "$OUT/$label-show.json" 'd.get("change_pending")' 2>/dev/null || echo parse_error)"
    echo "cancel_events_${label}=$(event_count truth_change_cancelled)"
    echo "requested_events_${label}=$(event_count truth_change_requested)"
    echo "body_unchanged_${label}=$([ "$(body_sha)" = "$BASE_SHA" ] && echo yes || echo no)"
  } >> "$CANON"
done
cat "$CANON"
echo "--- first stderr"
cat "$OUT/first.err" 2>/dev/null || true
emit_canon "$CANON"

for label in first second; do
  rc=$(awk -F= -v k="rc_${label}" '$1 == k { print $2 }' "$CANON")
  err=$(awk -F= -v k="stderr_${label}" '$1 == k { sub(/^[^=]*=/, ""); print }' "$CANON")
  st=$(awk -F= -v k="status_${label}" '$1 == k { print $2 }' "$CANON")
  pending=$(awk -F= -v k="change_pending_${label}" '$1 == k { print $2 }' "$CANON")
  events=$(awk -F= -v k="cancel_events_${label}" '$1 == k { print $2 }' "$CANON")
  body=$(awk -F= -v k="body_unchanged_${label}" '$1 == k { print $2 }' "$CANON")
  if [ "$rc" -ne 0 ]; then
    echo "RESULT: FAIL - ${label} no-pending cancel exited ${rc}: ${err}"
    exit 1
  fi
  if [ -n "$err" ]; then
    echo "RESULT: FAIL - ${label} no-pending cancel wrote stderr: ${err}"
    exit 1
  fi
  if [ "$st" != "frozen" ] || [ "$pending" != "False" ]; then
    echo "RESULT: FAIL - ${label} no-pending cancel changed state: status=${st} pending=${pending}"
    exit 1
  fi
  if [ "$events" != "0" ]; then
    echo "RESULT: FAIL - ${label} no-pending cancel appended an event (${events})"
    exit 1
  fi
  if [ "$body" != "yes" ]; then
    echo "RESULT: FAIL - ${label} no-pending cancel edited the truth body"
    exit 1
  fi
done
echo "RESULT: PASS (guard) - no-pending cancel is a successful no-op, twice"
