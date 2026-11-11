#!/bin/zsh
# Seed 3 (AC-003, guard): the strict path keeps working without the flag.
#
# An unmodified body with a valid base still closes through a plain
# `truth refreeze`, and the flag is not required for it.
set -eu
HERE="${0:A:h}"
source "$HERE/lib.sh"
fixture_init

"$BEACON_BIN" truth change --project "$SEED_PROJECT" --feature demo --by seed --summary "strict path probe" >/dev/null
set +e
out=$("$BEACON_BIN" truth refreeze --project "$SEED_PROJECT" --feature demo --by seed 2>&1)
rc=$?
set -e
st=$(truth_json | json_field /dev/stdin 'd["status"]')
pending=$(truth_json | json_field /dev/stdin 'len(d.get("pending_change_ids", []))')
[ "$rc" -eq 0 ] && [ "$st" = "frozen" ] && [ "$pending" -eq 0 ] || {
  print -r -- "RESULT: FAIL - strict refreeze regressed (rc=$rc status=$st pending=$pending): $out"
  exit 1
}
print -r -- "RESULT: PASS (guard) - strict refreeze still works without --accept-current"
