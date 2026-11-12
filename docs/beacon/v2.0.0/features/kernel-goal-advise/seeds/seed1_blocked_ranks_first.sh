#!/bin/zsh
# Seed 1 (AC-001): a hard-blocked feature outranks a healthy one.
set -eu
HERE="${0:A:h}"; source "$HERE/lib.sh"; fixture_init
if ! out=$("$BEACON_BIN" goal advise --project "$PROJ" --top 3 --json 2>&1); then
  print -r -- "RESULT: FAIL - goal advise unavailable: $(print -r -- "$out" | head -1)"
  exit 1
fi
first=$(print -r -- "$out" | jq -r '.items[0].feature')
score=$(print -r -- "$out" | jq -r '.items[0].score')
[[ "$first" == "blocked-feature" ]] || { print -r -- "RESULT: FAIL - first=$first want blocked-feature"; exit 1; }
(( score >= 100 )) || { print -r -- "RESULT: FAIL - score=$score want >=100"; exit 1; }
print -r -- "RESULT: PASS - blocked feature ranked first (score=$score)"
