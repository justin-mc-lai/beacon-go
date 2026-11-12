#!/bin/zsh
# Seed 2 (AC-002): same facts twice -> byte-identical output (no clock, no prose).
set -eu
HERE="${0:A:h}"; source "$HERE/lib.sh"; fixture_init
if ! a=$("$BEACON_BIN" goal advise --project "$PROJ" --top 5 --json 2>&1); then
  print -r -- "RESULT: FAIL - goal advise unavailable: $(print -r -- "$a" | head -1)"
  exit 1
fi
b=$("$BEACON_BIN" goal advise --project "$PROJ" --top 5 --json)
[[ "$a" == "$b" ]] || { print -r -- "RESULT: FAIL - output changed between runs"; exit 1; }
print -r -- "RESULT: PASS - advise is deterministic across runs"
