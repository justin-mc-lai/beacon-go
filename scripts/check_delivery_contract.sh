#!/usr/bin/env bash
# Checks the embedded/distributed delivery contract, not agent execution.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
go test ./skills -run '^TestStrictDelivery' -count=1
