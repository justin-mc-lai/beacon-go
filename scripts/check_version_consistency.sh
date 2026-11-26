#!/usr/bin/env bash
# Version single-source guard.
#
# internal/version/version.go is the only authoritative version. Docs may
# mention historical go.N lines, but the "current release" declarations must
# match the code, and AGENTS.md must not hard-code the current version.

set -euo pipefail

ROOT="${CHECK_VERSION_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"
go_version="$ROOT/internal/version/version.go"
readme="$ROOT/README.md"
readme_zh="$ROOT/README.zh-CN.md"
agents="$ROOT/AGENTS.md"
plugin="$ROOT/.claude-plugin/plugin.json"

fail=0

if [ ! -f "$go_version" ]; then
  echo "version-consistency: missing source $go_version" >&2
  exit 1
fi

version_const_count="$(grep -Ec '^[[:space:]]*const[[:space:]]+Version[[:space:]]*=[[:space:]]*"[^"]+"' "$go_version" || true)"
version_literal_count="$(grep -Eo '"[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?(\+[0-9A-Za-z.]+)?"' "$go_version" | wc -l | tr -d ' ')"
if [ "$version_const_count" -ne 1 ] || [ "$version_literal_count" -ne 1 ]; then
  echo "version-consistency: expected exactly one const Version literal in $go_version, found const=$version_const_count literals=$version_literal_count" >&2
  fail=1
fi

ver="$(awk '/^const Version = "/{gsub(/"/,"",$4); print $4; exit}' "$go_version")"
if [ -z "$ver" ]; then
  echo "version-consistency: empty Version in $go_version" >&2
  exit 1
fi

check_current() {
  local file="$1" pattern="$2" label="$3"
  [ -f "$file" ] || return 0
  local line
  line="$(grep -nE "$pattern" "$file" | head -1 || true)"
  [ -z "$line" ] && return 0
  if ! printf '%s\n' "$line" | grep -Fq "$ver"; then
    echo "version-consistency: $label out of sync with version.go ($ver): $line" >&2
    fail=1
  fi
}

check_current "$readme" 'Current release' "README.md"
check_current "$readme_zh" '当前版本' "README.zh-CN.md"
check_current "$plugin" '"version"[[:space:]]*:' ".claude-plugin/plugin.json"
if [ -f "$agents" ]; then
  if grep -nE '当前版本[：:][[:space:]]*\*{0,2}v?[0-9]+\.[0-9]+\.[0-9]+' "$agents" >&2; then
    echo "version-consistency: AGENTS.md hard-codes a current version; point to 'beacon --version' instead" >&2
    fail=1
  fi
  if grep -nE '当前[[:space:]]*`v2\.0\.0-go\.[0-9]+`' "$agents" >&2; then
    echo "version-consistency: AGENTS.md hard-codes a current version; point to 'beacon --version' instead" >&2
    fail=1
  fi
fi

if [ "$fail" -eq 0 ]; then
  echo "version-consistency: ok ($ver)"
fi
exit "$fail"
