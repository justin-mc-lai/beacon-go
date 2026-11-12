#!/bin/zsh
# Shared fixture for the kernel-goal-advise seeds: one throwaway project with
# one blocked feature and one running feature. Isolated store, no global state.
set -eu
: "${BEACON_BIN:?set BEACON_BIN to a beacon binary}"
[[ -x "$BEACON_BIN" ]] || { echo "BEACON_BIN not executable: $BEACON_BIN"; exit 2; }

TMP=$(mktemp -d)
export BEACON_STORE_PATH="$TMP/store.db"
PROJ="$TMP/proj"
mkdir -p "$PROJ"

make_feature() { # slug
  local slug=$1
  "$BEACON_BIN" truth create --project "$PROJ" --slug "$slug" --version v2.0.0 >/dev/null
  sed -i '' 's/^status: draft$/status: frozen/' "$PROJ/docs/beacon/v2.0.0/features/$slug/truth.md"
  "$BEACON_BIN" gate evaluate truth_freeze --feature "$slug" --project "$PROJ" --evaluator seed >/dev/null
}

fixture_init() {
  "$BEACON_BIN" projects register --path "$PROJ" --name proj --version v2.0.0 --docs-root docs/beacon/v2.0.0 >/dev/null
  make_feature blocked-feature
  make_feature fresh-feature
  BLOCKED_RUN=$("$BEACON_BIN" goal run --project "$PROJ" -f blocked-feature --executor manual --executor-id seed "blocked one" --json | jq -r '.run_id')
  FRESH_RUN=$("$BEACON_BIN" goal run --project "$PROJ" -f fresh-feature --executor manual --executor-id seed "fresh one" --json | jq -r '.run_id')
  "$BEACON_BIN" goal block --run-id "$BLOCKED_RUN" --reason-code manual_block >/dev/null 2>&1 || true
}
