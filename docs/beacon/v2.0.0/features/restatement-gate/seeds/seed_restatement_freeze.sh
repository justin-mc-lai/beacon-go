#!/bin/zsh
# Seed: the restatement gate in front of a FIRST truth freeze, on the
# real binary. Five negatives, two positives, plus the ordering rule
# (surface failures must be reported before restatement failures).
#
#   missing file            -> restatement_required
#   empty confirmed_by      -> restatement_unconfirmed
#   hash drift              -> restatement_stale
#   missing section 5       -> restatement_sections_missing
#   AC restates=9           -> ac_not_traceable_to_restatement
#   surface + restatement   -> truth_surface_required first
#   valid restatement       -> frozen
#   legacy refreeze waiver  -> frozen + warning, no block
set -eu
HERE="${0:A:h}"
source "$HERE/lib.sh"
project_init

new_feature() { # new_feature SLUG
  "$BEACON_BIN" truth create --project "$SEED_PROJECT" --slug "$1" --version v2.0.0 >/dev/null
}

# blocked LABEL SLUG EXPECTED_REASON
blocked() {
  local label=$1 slug=$2 want=$3 out="$SEED_TMP/out/$1.json" codes
  freeze "$slug" "$out"
  codes=$(python3 - "$out" "$want" <<'PY'
import json,sys
try:
    d=json.load(open(sys.argv[1]))
except Exception as e:
    print("UNPARSEABLE:%s" % e); raise SystemExit
got=d.get("block_reasons") or []
print("MATCH" if sys.argv[2] in got else "MISSING:"+",".join(got))
PY
)
  if [ "$FREEZE_RC" -ne 0 ] && [ "$codes" = MATCH ]; then
    pass "$label freeze_rc=$FREEZE_RC reason=$want"
  else
    flunk "$label freeze_rc=$FREEZE_RC want=$want got=$codes"
  fi
}

# --- negative 1: no restatement file at all ---
new_feature missing
write_pkg missing 1
blocked missing missing restatement_required

# --- negative 2: a restatement nobody signed ---
new_feature unconfirmed
write_pkg unconfirmed 1
write_restatement "$D" ""
blocked unconfirmed unconfirmed restatement_unconfirmed

# --- negative 3: one character edited after confirmation ---
new_feature stale
write_pkg stale 1
body=$(restatement_body)
write_restatement "$D" justin "$body " "$(sha_of "$body")"
blocked stale stale restatement_stale

# --- negative 4: section 5 missing ---
new_feature partial
write_pkg partial 1
write_restatement "$D" justin "$(restatement_body 1,2,3,4)"
blocked partial partial restatement_sections_missing

# --- negative 5: an AC pointing at a section that does not exist ---
new_feature dangling
write_pkg dangling 9
write_restatement "$D" justin
blocked dangling dangling ac_not_traceable_to_restatement

# --- ordering: surface failure wins over restatement failure ---
new_feature order
write_pkg order 1 nosurface
freeze order "$SEED_TMP/out/order.json"
order_head=$(json_field "$SEED_TMP/out/order.json" 'd["block_reasons"][0]' 2>/dev/null || echo missing)
order_has_rest=$(python3 - "$SEED_TMP/out/order.json" <<'PY'
import json,sys
print("yes" if "restatement_required" in (json.load(open(sys.argv[1])).get("block_reasons") or []) else "no")
PY
)
if [ "$FREEZE_RC" -ne 0 ] && [ "$order_head" = truth_surface_required ] && [ "$order_has_rest" = no ]; then
  pass "order freeze_rc=$FREEZE_RC first=truth_surface_required restatement_reported=no"
else
  flunk "order freeze_rc=$FREEZE_RC first=$order_head restatement_reported=$order_has_rest"
fi

# --- positive 1: parse + first freeze pass ---
new_feature good
write_pkg good 1
write_restatement "$D" justin
freeze good "$SEED_TMP/out/good.json"
good_state=$(python3 - "$SEED_TMP/out/good.json" <<'PY'
import json,sys
d=json.load(open(sys.argv[1]))
r=d.get("restatement") or {}
print("%s frozen=%s matches=%s by=%s blocked=%s" % (d.get("status"), d.get("frozen"), r.get("matches"), r.get("confirmed_by"), bool(d.get("blocked"))))
PY
)
if [ "$FREEZE_RC" -eq 0 ] && [ "$good_state" = "frozen frozen=True matches=True by=justin blocked=False" ]; then
  pass "good_first_freeze $good_state"
else
  flunk "good_first_freeze freeze_rc=$FREEZE_RC $good_state"
fi

# --- positive 2: a legacy package refreezes without a restatement ---
new_feature legacy
write_pkg legacy 1
write_restatement "$D" justin
freeze legacy "$SEED_TMP/out/legacy-first.json"
rm "$D/restatement.md"
"$BEACON_BIN" truth change --project "$SEED_PROJECT" --feature legacy \
  --version v2.0.0 --summary "legacy refreeze" --by seed >/dev/null
legacy_rc=0
"$BEACON_BIN" truth refreeze --project "$SEED_PROJECT" --feature legacy \
  --version v2.0.0 --by seed --json >"$SEED_TMP/out/legacy.json" 2>"$SEED_TMP/out/legacy.err" || legacy_rc=$?
legacy_state=$(python3 - "$SEED_TMP/out/legacy.json" <<'PY'
import json,sys
d=json.load(open(sys.argv[1]))
w=d.get("warnings") or []
print("%s frozen=%s warnings=%d restatement=%s" % (d.get("status"), d.get("frozen"), len(w), "present" if d.get("restatement") else "omitted"))
PY
)
if [ "$legacy_rc" -eq 0 ] && [ "$legacy_state" = "frozen frozen=True warnings=1 restatement=omitted" ]; then
  pass "legacy_refreeze_waiver $legacy_state"
else
  flunk "legacy_refreeze_waiver refreeze_rc=$legacy_rc $legacy_state"
fi

# --- truth show --json carries the restatement block (AC-005) ---
"$BEACON_BIN" truth show --project "$SEED_PROJECT" --feature good --version v2.0.0 --json \
  >"$SEED_TMP/out/show.json" 2>"$SEED_TMP/out/show.err" || true
show_state=$(python3 - "$SEED_TMP/out/show.json" <<'PY'
import json,sys
d=json.load(open(sys.argv[1])); r=d.get("restatement") or {}
print("path=%s by=%s keys=%s matches=%s" % (bool(r.get("path")), bool(r.get("confirmed_by")), ",".join(sorted(r)), r.get("matches")))
PY
)
if [ "$show_state" = "path=True by=True keys=confirmed_at,confirmed_by,hash,matches,path matches=True" ]; then
  pass "truth_show_restatement_block $show_state"
else
  flunk "truth_show_restatement_block $show_state"
fi

echo "---"
for f in missing unconfirmed stale partial dangling order good; do
  rc=$(cat "$SEED_TMP/out/$f.json.rc" 2>/dev/null || echo "?")
  python3 - "$SEED_TMP/out/$f.json" "$f" "$rc" <<'PY'
import json,sys
d=json.load(open(sys.argv[1]))
r=d.get("restatement") or {}
print("%s rc=%s status=%s blocked=%s reasons=%s restatement=%s" % (
    sys.argv[2], sys.argv[3], d.get("status"), bool(d.get("blocked")),
    ",".join(d.get("block_reasons") or []) or "-",
    "present" if r else "omitted"))
PY
done
echo "seed_tmp=$SEED_TMP"
echo "checks_passed=$PASS checks_failed=$FAILED"
[ "$FAILED" -eq 0 ]
