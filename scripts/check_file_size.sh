#!/usr/bin/env bash
# AC-025 guard: every non-test Go file over 500 lines must be listed in
# docs/adr/ADR-0001-file-size-exceptions.md (split, or ADR exception).
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ADR="$ROOT/docs/adr/ADR-0001-file-size-exceptions.md"
[ -f "$ADR" ] || { echo "missing $ADR" >&2; exit 1; }
fail=0
while read -r lines file; do
  rel="${file#"$ROOT"/}"
  [ "$lines" -gt 500 ] || continue
  if ! grep -qF "\`$rel\`" "$ADR"; then
    echo "over 500 lines without an ADR exception: $rel ($lines)" >&2
    fail=1
  fi
done < <(find "$ROOT/internal" "$ROOT/cmd" "$ROOT/desktop" "$ROOT/skills" \
           -name '*.go' ! -name '*_test.go' -exec wc -l {} + | awk '$2!="total"{print $1, $2}')
if [ "$fail" -eq 0 ]; then echo "file-size guard: ok"; fi
exit "$fail"
