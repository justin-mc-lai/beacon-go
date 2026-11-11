#!/bin/zsh
# Seed 2 (AC-002): --accept-current never invents work.
#
# With no pending change the flag must be a clean refusal ("no pending
# change"), not a silent freeze, and it must not write a second freeze
# event for an already frozen package.
set -eu
HERE="${0:A:h}"
source "$HERE/lib.sh"
fixture_init

before=$(event_count truth_frozen)$(event_count truth_refrozen)
set +e
out=$("$BEACON_BIN" truth refreeze --project "$SEED_PROJECT" --feature demo --accept-current --by seed 2>&1)
rc=$?
set -e
after=$(event_count truth_frozen)$(event_count truth_refrozen)
st=$(truth_json | json_field /dev/stdin 'd["status"]')

if [ "$rc" -eq 0 ]; then
  print -r -- "RESULT: FAIL - accept-current without a pending change exited 0: $out"
  exit 1
fi
print -r -- "$out" | grep -qi "no pending change" || { print -r -- "RESULT: FAIL - refusal did not say no pending change: $out"; exit 1; }
[ "$st" = "frozen" ] || { print -r -- "RESULT: FAIL - status changed to $st"; exit 1; }
[ "$before" = "$after" ] || { print -r -- "RESULT: FAIL - freeze events written without a pending change ($before -> $after)"; exit 1; }
print -r -- "RESULT: PASS - accept-current refuses without a pending change and writes no freeze event"
