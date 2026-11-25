#!/usr/bin/env bash
# Kernel dependency gate: removed agent/LLM surfaces must not re-enter
# the beacon binary. Fails closed on an import or an API-key literal.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
deps="$(go list -deps ./cmd/beacon)"
fail=0
for pkg in internal/llm internal/httpapi internal/agentapi internal/workspace; do
  if printf '%s\n' "$deps" | grep -qx "beacon-go/$pkg"; then
    echo "kernel-deps: $pkg is in the beacon binary" >&2
    fail=1
  fi
done
for f in internal/cli/goal_program.go internal/cli/goal_program_integrated.go; do
  if [ -e "$f" ]; then
    echo "kernel-deps: sea/lake write entry still present: $f" >&2
    fail=1
  fi
done
for f in internal/orchestrator/exec_subprocess.go internal/orchestrator/exec_ralph.go internal/orchestrator/exec_delegate.go; do
  if [ -e "$f" ]; then
    echo "kernel-deps: built-in executor file still present: $f" >&2
    fail=1
  fi
done
if grep -rnE 'ANTHROPIC_API_KEY|OPENAI_API_KEY' internal/cli internal/orchestrator internal/gate internal/store internal/core internal/intenttree --include='*.go' | grep -v '_test.go' >/dev/null; then
  echo "kernel-deps: API key literal found in kernel source" >&2
  fail=1
fi
if [ "$fail" -eq 0 ]; then
  echo "kernel-deps: ok (no removed surfaces in the binary)"
fi
exit "$fail"
