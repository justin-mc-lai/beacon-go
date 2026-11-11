#!/bin/zsh
# Seed 3 (AC-HTTP-007..008): no write surface on the read-only HTTP face.
# Self-contained: no shared fixture, no EXIT trap, one server, plain curl.
# NB: the loop variable must NOT be named `path` — in zsh `path` is tied to
# $PATH, so assigning it wipes the command search path.
set -eu
: "${BEACON_BIN:?set BEACON_BIN to a built Go kernel}"
[[ -x "$BEACON_BIN" ]] || { echo "BEACON_BIN not executable: $BEACON_BIN"; exit 2; }

TMP=$(mktemp -d); PROJ=$TMP/proj; mkdir -p "$PROJ"
export BEACON_STORE_PATH=$TMP/store.db
PORT=$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()')
BASE="http://127.0.0.1:$PORT"
OUT=$TMP/out; mkdir -p "$OUT"

"$BEACON_BIN" projects register --path "$PROJ" --name proj --version v2.0.0 --docs-root docs/beacon/v2.0.0 >/dev/null
"$BEACON_BIN" truth create --project "$PROJ" --slug demo --version v2.0.0 >/dev/null
sed -i '' 's/^status: draft$/status: frozen/' "$PROJ/docs/beacon/v2.0.0/features/demo/truth.md"
"$BEACON_BIN" gate evaluate truth_freeze --feature demo --evaluator seed --project "$PROJ" >/dev/null
RUN_A=$("$BEACON_BIN" goal run --project "$PROJ" -f demo --executor manual --executor-id seed "fixture a" --json | jq -r '.run_id')

"$BEACON_BIN" goal serve --http --addr "127.0.0.1:$PORT" >"$OUT/serve.log" 2>&1 &
SPID=$!
cleanup() { kill "$SPID" 2>/dev/null || true; }
trap cleanup EXIT
for i in {1..50}; do
  [[ "$(curl -s -o /dev/null -m 1 -w '%{http_code}' "$BASE/health" || true)" == "200" ]] && break
  sleep 0.1
done

fail=0
note() { print -r -- "FAIL: $*"; fail=$((fail + 1)); }
status_of() { curl -sS -m 3 -o "$OUT/body.json" -w '%{http_code}' -X "$1" "$2" 2>/dev/null || print -r -- 000; }

for p in /health /spine/runs "/spine/gates?feature=demo" "/runs/$RUN_A" /features/demo; do
  for m in POST PUT PATCH DELETE; do
    code=$(status_of "$m" "$BASE$p")
    [[ "$code" == "405" ]] || note "$m $p -> want 405 got $code"
    grep -q '"code":"method_not_allowed"' "$OUT/body.json" || note "$m $p body missing method_not_allowed"
  done
done

for p in /release /publish /complete "/runs/$RUN_A/release" "/runs/$RUN_A/complete" /features/demo/publish; do
  for m in GET POST; do
    code=$(status_of "$m" "$BASE$p")
    [[ "$code" == "404" ]] || note "$m $p -> want 404 got $code"
    grep -q '"code":"not_found"' "$OUT/body.json" || note "$m $p body missing not_found"
  done
done

before=$(sqlite3 "$BEACON_STORE_PATH" "select (select count(*) from runs)||'/'||(select count(*) from gate_results)||'/'||(select count(*) from lifecycle_events);")
after=$(sqlite3 "$BEACON_STORE_PATH" "select (select count(*) from runs)||'/'||(select count(*) from gate_results)||'/'||(select count(*) from lifecycle_events);")
[[ "$before" == "$after" ]] || note "store rows changed by requests ($before -> $after)"

cat >"$OUT/seed3.canon" <<CANON
seed=seed3_no_write_surface
non_get=405
write_paths=404
rows_before=$before
rows_after=$after
failures=$fail
CANON
cat "$OUT/seed3.canon"
if (( fail == 0 )); then print -r -- "RESULT: PASS - no write surface; 20 non-GET 405 and 12 release/publish 404"; else print -r -- "RESULT: FAIL - $fail failure(s)"; fi
