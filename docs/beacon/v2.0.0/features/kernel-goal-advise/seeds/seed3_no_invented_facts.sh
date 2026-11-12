#!/bin/zsh
# Seed 3 (AC-003): every item and blocker exists in the spine facts.
set -eu
HERE="${0:A:h}"; source "$HERE/lib.sh"; fixture_init
if ! adv=$("$BEACON_BIN" goal advise --project "$PROJ" --top 10 --json 2>&1); then
  print -r -- "RESULT: FAIL - goal advise unavailable: $(print -r -- "$adv" | head -1)"
  exit 1
fi
spine=$("$BEACON_BIN" spine list --json --project "$PROJ")
print -r -- "$adv" | jq -e '.schema=="goal-advise.v1" and .version=="advise-v1"' >/dev/null || { print -r -- "RESULT: FAIL - schema/version wrong"; exit 1; }
bad=$(print -r -- "$adv" | jq -r --argjson sp "$(print -r -- "$spine" | jq -c '.snapshots')" '
  [ .items[] as $i
    | ($sp | map(select(.feature_slug==$i.feature)) | .[0]) as $s
    | select($s == null or (($i.blockers - $s.blocking_reason_codes) | length) > 0)
    | $i.feature ] | join(",")')
[[ -z "$bad" ]] || { print -r -- "RESULT: FAIL - invented facts for: $bad"; exit 1; }
print -r -- "RESULT: PASS - every advised item and blocker traces to a spine fact"
