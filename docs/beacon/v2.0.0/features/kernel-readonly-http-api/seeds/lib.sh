#!/bin/zsh
# Shared harness for the kernel-readonly-http-api seeds.
#
# Every seed drives the Go kernel binary built from ./cmd/beacon via
# BEACON_BIN, creates its own temporary project and store
# (BEACON_STORE_PATH), and prints a canon file plus its sha256.
# No Python CLI is involved; JSON assertions use jq and HTTP probes curl.
#
# Required: BEACON_BIN (go build -o /tmp/beacon-ro-api ./cmd/beacon)
# Also used: jq, curl, lsof, sqlite3, sed, shasum, zsh
set -eu

: "${BEACON_BIN:?set BEACON_BIN to a built Go kernel (go build -o /tmp/beacon-ro-api ./cmd/beacon)}"
if [ ! -x "$BEACON_BIN" ]; then
  echo "BEACON_BIN is not executable: $BEACON_BIN" >&2
  exit 2
fi
for tool in jq curl lsof sqlite3 shasum; do
  command -v "$tool" >/dev/null 2>&1 || { echo "missing tool: $tool" >&2; exit 2 }
done

SEED_TMP=$(mktemp -d)
export BEACON_STORE_PATH="$SEED_TMP/beacon.db.sqlite"
SEED_PROJECT="$SEED_TMP/proj"
SEED_OUT="$SEED_TMP/out"
mkdir -p "$SEED_PROJECT" "$SEED_OUT"
FAILED=0
SERVE_PID=""
SERVE_LOG="$SEED_OUT/serve.log"

goodbye() {
  stop_serve 2>/dev/null || true
}
trap goodbye EXIT INT TERM

ok() { print -r -- "PASS: $*"; }
bad() { print -r -- "FAIL: $*"; FAILED=$((FAILED + 1)); }
check_eq() { # label expected actual
  if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (want=$2 got=$3)"; fi
}
check_jq() { # label file jq-expression
  if jq -e "$3" "$2" >/dev/null 2>&1; then ok "$1"; else bad "$1 (jq failed: $3)"; fi
}
check_keys() { # label file sorted-key-csv
  local got
  got=$(jq -r 'keys_unsorted | sort | join(",")' "$2" 2>/dev/null || print -r -- '<invalid-json>')
  check_eq "$1 keys" "$3" "$got"
}

# fixture_init builds one project with a frozen demo package, two evaluated
# truth_freeze gates and two goal runs (A created before B).
fixture_init() {
  "$BEACON_BIN" projects register --path "$SEED_PROJECT" --name proj \
    --version v2.0.0 --docs-root docs/beacon/v2.0.0 >/dev/null
  "$BEACON_BIN" truth create --project "$SEED_PROJECT" --slug demo --version v2.0.0 >/dev/null
  local d="$SEED_PROJECT/docs/beacon/v2.0.0/features/demo"
  sed -i '' 's/^status: draft$/status: frozen/' "$d/truth.md"
  "$BEACON_BIN" gate evaluate truth_freeze --feature demo --evaluator seed-fixture \
    --project "$SEED_PROJECT" >/dev/null
  sleep 0.05
  "$BEACON_BIN" gate evaluate truth_freeze --feature demo --evaluator seed-fixture-2 \
    --project "$SEED_PROJECT" >/dev/null
  RUN_A=$("$BEACON_BIN" goal run --project "$SEED_PROJECT" -f demo \
    --executor manual --executor-id seed "fixture run a" --json | jq -r '.run_id')
  sleep 0.05
  RUN_B=$("$BEACON_BIN" goal run --project "$SEED_PROJECT" -f demo \
    --executor manual --executor-id seed "fixture run b" --json | jq -r '.run_id')
  [ -n "$RUN_A" ] && [ -n "$RUN_B" ]
}

# pick_port returns a port with no LISTEN socket on this host.
pick_port() {
  local p i
  for i in {1..80}; do
    p=$(( 18000 + (RANDOM % 20000) ))
    if ! lsof -nP -iTCP:"$p" -sTCP:LISTEN >/dev/null 2>&1; then
      print -r -- "$p"
      return 0
    fi
  done
  print -r -- "no free port" >&2
  exit 2
}

# probe_has_listener PORT returns 0 when something answers HTTP on it.
probe_has_listener() { # port
  local code
  code=$(curl -sS -m 1 -o /dev/null -w '%{http_code}' \
    "http://127.0.0.1:$1/health" 2>/dev/null) || return 1
  [ -n "$code" ] && [ "$code" != "000" ]
}

# port_closed PORT returns 0 when no HTTP listener answers on it.
# Method: curl exit 7 (connection refused) and no status code.
port_closed() { # port
  if probe_has_listener "$1"; then return 1; fi
  return 0
}

# http_status URL OUTFILE [curl-extra...] prints the status code; body -> OUTFILE.
http_status() { # url outfile [curl-extra...]
  local url=$1 out=$2
  shift 2
  local code
  code=$(curl -sS -m 3 -o "$out" -w '%{http_code}' "$@" "$url" 2>/dev/null) || code="000"
  [ -n "$code" ] || code="000"
  print -r -- "$code"
}

# start_serve PORT [flags...] starts `goal serve` and waits up to 5s for /health.
# Returns 0 started, 1 process exited early (SERVE_EXIT set), 2 startup timeout.
start_serve() { # port [flags...]
  local port=$1
  shift
  : >"$SERVE_LOG"
  "$BEACON_BIN" goal serve "$@" >"$SERVE_LOG" 2>&1 &
  SERVE_PID=$!
  SERVE_EXIT=""
  local i
  for i in {1..50}; do
    if ! kill -0 "$SERVE_PID" 2>/dev/null; then
      wait "$SERVE_PID" 2>/dev/null || SERVE_EXIT=$?
      SERVE_PID=""
      return 1
    fi
    if probe_has_listener "$port"; then
      return 0
    fi
    sleep 0.1
  done
  return 2
}

stop_serve() {
  if [ -n "${SERVE_PID:-}" ] && kill -0 "$SERVE_PID" 2>/dev/null; then
    kill "$SERVE_PID" 2>/dev/null || true
    local i
    for i in {1..40}; do
      kill -0 "$SERVE_PID" 2>/dev/null || break
      sleep 0.05
    done
    kill -9 "$SERVE_PID" 2>/dev/null || true
    wait "$SERVE_PID" 2>/dev/null || true
  fi
  SERVE_PID=""
}

serve_error_token() { # normalize the first error line into a stable token
  local line
  line=$(grep -m1 -E 'Error|error|unknown|refus|denied' "$SERVE_LOG" 2>/dev/null || true)
  if [ -z "$line" ]; then
    print -r -- none
  elif print -r -- "$line" | grep -qi 'unknown flag'; then
    print -r -- unknown_flag
  elif print -r -- "$line" | grep -qi 'unknown command'; then
    print -r -- unknown_command
  elif print -r -- "$line" | grep -qi 'loopback'; then
    print -r -- non_loopback_refused
  else
    print -r -- "$(print -r -- "$line" | sed -e 's/[^A-Za-z0-9_ -]/_/g' -e 's/ /_/g')"
  fi
}

emit_canon() { # file
  print -r -- "canon=$1"
  print -r -- "sha256=$(shasum -a 256 "$1" | awk '{print $1}')"
}

finish() { # seed-name canon-file
  emit_canon "$2"
  if [ "$FAILED" -ne 0 ]; then
    print -r -- "RESULT: FAIL - $1 ($FAILED assertion(s) failed)"
    exit 1
  fi
  print -r -- "RESULT: PASS - $1"
  exit 0
}
