#!/usr/bin/env bash
# Canonical JSON -> isolated SQLite cache; never repairs repository truth.
set -euo pipefail
exec python3 "$(dirname "$0")/check_self_tree.py" "$@"
