#!/bin/zsh
# Seed 1 (AC-001): --accept-current is the explicit heal for a body that
# moved after its change opened.
#
# Baseline (without the flag): plain refreeze refuses with
# revision_authority_drift and the package stays change_pending.
# Green contract: refreeze --accept-current closes the change, the package
# returns to frozen, and the trail records a fresh freeze.
set -eu
HERE="${0:A:h}"
source "$HERE/lib.sh"
fixture_init

# The edit happens BEFORE the change opens and keeps the frontmatter frozen:
# that is exactly the drift the guard must catch (an edit outside the change flow).
sed -i '' 's/^A frozen fixture package used to exercise the truth change cancel path\./A frozen fixture package used to exercise the refreeze heal path./' "$FEATURE_DIR/truth.md"
"$BEACON_BIN" truth change --project "$SEED_PROJECT" --feature demo --by seed --summary "drift heal probe" >/dev/null

set +e
out=$("$BEACON_BIN" truth refreeze --project "$SEED_PROJECT" --feature demo --by seed 2>&1)
rc=$?
set -e
st=$(truth_json | json_field /dev/stdin 'd["status"]')
if [ "$rc" -eq 0 ] || [ "$st" != "change_pending" ]; then
  print -r -- "RESULT: FAIL - plain refreeze wanted a drift refusal (rc=$rc status=$st)"
  exit 1
fi
print -r -- "$out" | grep -q "revision_authority_drift" || { print -r -- "RESULT: FAIL - refusal did not name revision_authority_drift: $out"; exit 1; }

set +e
out2=$("$BEACON_BIN" truth refreeze --project "$SEED_PROJECT" --feature demo --accept-current --by seed 2>&1)
rc2=$?
set -e
st2=$(truth_json | json_field /dev/stdin 'd["status"]')
pending2=$(truth_json | json_field /dev/stdin 'len(d.get("pending_change_ids", []))')
[ "$rc2" -eq 0 ] && [ "$st2" = "frozen" ] && [ "$pending2" -eq 0 ] || {
  print -r -- "RESULT: FAIL - accept-current did not heal (rc=$rc2 status=$st2 pending=$pending2): $out2"
  exit 1
}
[ "$(event_count truth_refrozen)" -ge 1 ] || { print -r -- "RESULT: FAIL - no truth_refrozen event recorded"; exit 1; }
print -r -- "RESULT: PASS - drift refusal kept the package pending; --accept-current healed it to frozen"
