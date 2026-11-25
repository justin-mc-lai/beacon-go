#!/usr/bin/env bash
# Badminton dogfood: greenfield tree, host expansion, freeze/implement/
# verify/rollup, subtree change, snapshots and HTML.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BEACON="${BEACON_BIN:-$ROOT/bin/beacon}"
exec python3 "$ROOT/e2e/dogfood_badminton.py" "$BEACON"
