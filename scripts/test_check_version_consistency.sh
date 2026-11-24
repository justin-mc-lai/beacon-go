#!/usr/bin/env bash
# Executable probe for scripts/check_version_consistency.sh.
# Covers: AGENTS hard-coded current version, README hard-coded current release,
# missing version.go, and second version literal in version.go.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SCRIPT="$ROOT/scripts/check_version_consistency.sh"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

pass=0
fail=0

expect_fail() {
  local name="$1" dir="$2" pattern="$3"
  set +e
  local out
  out="$(CHECK_VERSION_ROOT="$dir" bash "$SCRIPT" 2>&1)"
  local rc=$?
  set -e
  if [ "$rc" -eq 0 ]; then
    echo "FAIL[$name]: expected non-zero exit" >&2
    echo "$out" >&2
    fail=$((fail + 1))
    return
  fi
  if ! printf '%s\n' "$out" | grep -Fq "$pattern"; then
    echo "FAIL[$name]: output missing pattern: $pattern" >&2
    echo "$out" >&2
    fail=$((fail + 1))
    return
  fi
  echo "ok[$name]"
  pass=$((pass + 1))
}

expect_ok() {
  local name="$1" dir="$2"
  set +e
  local out
  out="$(CHECK_VERSION_ROOT="$dir" bash "$SCRIPT" 2>&1)"
  local rc=$?
  set -e
  if [ "$rc" -ne 0 ]; then
    echo "FAIL[$name]: expected zero exit" >&2
    echo "$out" >&2
    fail=$((fail + 1))
    return
  fi
  echo "ok[$name]"
  pass=$((pass + 1))
}

mkbase() {
  local dir="$1"
  mkdir -p "$dir/internal/version" "$dir/scripts"
  cat > "$dir/internal/version/version.go" <<'EOF'
package version

const Version = "2.0.0"
EOF
  cat > "$dir/README.md" <<'EOF'
Current release: **v2.0.0** (plain SemVer).
EOF
  cat > "$dir/README.zh-CN.md" <<'EOF'
当前版本：**v2.0.0**（纯 SemVer）。
EOF
  cat > "$dir/AGENTS.md" <<'EOF'
- 版本：以 `beacon --version` 为准；唯一源是 `internal/version/version.go` 的 `Version`，本文不写死版本号。
EOF
}

# Baseline passes.
base="$TMP/base"
mkbase "$base"
expect_ok "baseline" "$base"

# 1) AGENTS hard-codes current version.
case1="$TMP/agents-hardcode"
mkbase "$case1"
cat > "$case1/AGENTS.md" <<'EOF'
当前版本：v9.9.9
EOF
expect_fail "agents-hardcoded-version" "$case1" "AGENTS.md hard-codes a current version"

# 2) README hard-codes a mismatched current release.
case2="$TMP/readme-hardcode"
mkbase "$case2"
cat > "$case2/README.md" <<'EOF'
Current release: v9.9.9
EOF
expect_fail "readme-hardcoded-release" "$case2" "README.md"

# 3) Missing version.go.
case3="$TMP/missing-version"
mkbase "$case3"
rm "$case3/internal/version/version.go"
expect_fail "missing-version-go" "$case3" "missing source"

# 4) Second version literal in version.go.
case4="$TMP/second-literal"
mkbase "$case4"
cat > "$case4/internal/version/version.go" <<'EOF'
package version

const Version = "2.0.0"
const OldVersion = "1.2.3"
EOF
expect_fail "second-version-literal" "$case4" "exactly one const Version literal"

echo "version-consistency-probe: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
