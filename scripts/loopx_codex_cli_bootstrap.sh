#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GOAL_ID="beacon-go"

cd "$PROJECT_DIR"
export PATH="$HOME/.local/bin:$PATH"

if ! command -v loopx >/dev/null 2>&1; then
  echo "loopx is not on PATH" >&2
  exit 1
fi

loopx doctor >/dev/null

loopx --format json \
  --registry "$PROJECT_DIR/.loopx/registry.json" \
  heartbeat-prompt \
  --goal-id "$GOAL_ID" \
  --runtime-profile codex_cli \
  --thin
