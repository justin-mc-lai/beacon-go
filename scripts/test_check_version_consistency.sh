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
  shift 3
  set +e
  local out
  out="$(CHECK_VERSION_ROOT="$dir" bash "$SCRIPT" "$@" 2>&1)"
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
  shift 2
  set +e
  local out
  out="$(CHECK_VERSION_ROOT="$dir" bash "$SCRIPT" "$@" 2>&1)"
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

release="$TMP/release"
mkbase "$release"
cat > "$release/CHANGELOG.md" <<'EOF'
# Changelog
## [Unreleased]
- Work in progress.
## [2.0.0] — baseline
- Shipped the kernel.
## [1.0.0]
- Previous release.
EOF
cat > "$release/notes.md" <<'EOF'
# Release v2.0.0
- Shipped the kernel; upgrades from v1.0.0 are supported.
EOF
expect_ok "explicit-candidate" "$release" --tag v2.0.0 --release-notes notes.md
expect_ok "absolute-notes-alias" "$release" --tag v2.0.0 --release-notes "$release/./notes.md"
expect_fail "historical-tag-is-not-candidate" "$release" "candidate tag" --tag v3.0.2 --release-notes notes.md
expect_fail "tag-needs-prefix" "$release" "candidate tag" --tag 2.0.0 --release-notes notes.md
expect_fail "blank-tag" "$release" "candidate tag" --tag ' ' --release-notes notes.md
expect_fail "empty-tag" "$release" "candidate tag" --tag '' --release-notes notes.md
expect_fail "missing-tag-scope" "$release" "requires both" --release-notes notes.md
expect_fail "missing-notes-scope" "$release" "requires both" --tag v2.0.0
expect_fail "empty-notes-path" "$release" "release notes" --tag v2.0.0 --release-notes ''
expect_fail "missing-notes-file" "$release" "release notes" --tag v2.0.0 --release-notes absent.md
expect_fail "duplicate-tag-argument" "$release" "duplicate option" --tag v2.0.0 --tag v2.0.0 --release-notes notes.md
expect_fail "duplicate-notes-alias" "$release" "duplicate option" --tag v2.0.0 --release-notes notes.md --release-notes "$release/notes.md"
expect_fail "unknown-argument" "$release" "unknown option" --latest
expect_fail "missing-argument-value" "$release" "requires a value" --tag

printf '# Release v9.9.9\n\nThe prose mentions v2.0.0, but this is another release.\n' > "$release/notes.md"
expect_fail "reverse-notes-description" "$release" "release notes heading" --tag v2.0.0 --release-notes notes.md
printf '# Release v2.0.0\n\n' > "$release/notes.md"
expect_fail "empty-notes-body" "$release" "release notes body" --tag v2.0.0 --release-notes notes.md
printf '# Release v2.0.0\n\nTODO\n' > "$release/notes.md"
expect_fail "stub-notes-body" "$release" "release notes body" --tag v2.0.0 --release-notes notes.md
printf '# Release v2.0.0\n- Shipped.\n# Release v2.0.0\n- Duplicate.\n' > "$release/notes.md"
expect_fail "duplicate-notes-heading" "$release" "release notes heading" --tag v2.0.0 --release-notes notes.md
printf '<!-- # Release v2.0.0 -->\nThis is only a comment.\n' > "$release/notes.md"
expect_fail "comment-is-not-title" "$release" "release notes heading" --tag v2.0.0 --release-notes notes.md
printf '```markdown\n# Release v2.0.0\n```\nThis is only an example.\n' > "$release/notes.md"
expect_fail "example-is-not-title" "$release" "release notes heading" --tag v2.0.0 --release-notes notes.md
printf '# Release v2.0.0\n<!-- TODO: add actual release notes -->\n' > "$release/notes.md"
expect_fail "comment-is-not-body" "$release" "release notes body" --tag v2.0.0 --release-notes notes.md
printf '# Release v2.0.0\n- Shipped.\n' > "$release/notes.md"
printf '## [9.9.9]\n- Mentions 2.0.0 in prose only.\n' > "$release/CHANGELOG.md"
expect_fail "reverse-changelog-description" "$release" "CHANGELOG entry" --tag v2.0.0 --release-notes notes.md
printf '## [2.0.0]\n## [1.0.0]\n- Old entry.\n' > "$release/CHANGELOG.md"
expect_fail "empty-changelog-entry" "$release" "CHANGELOG body" --tag v2.0.0 --release-notes notes.md
printf '## [2.0.0]\n- First.\n## [2.0.0]\n- Duplicate.\n' > "$release/CHANGELOG.md"
expect_fail "duplicate-changelog-entry" "$release" "CHANGELOG entry" --tag v2.0.0 --release-notes notes.md
rm "$release/CHANGELOG.md"
expect_fail "missing-changelog" "$release" "CHANGELOG" --tag v2.0.0 --release-notes notes.md

git -C "$base" init -q
empty_tree="$(git -C "$base" mktree </dev/null)"
historical_commit="$(printf 'Historical fixture\n' | git -C "$base" -c user.name=Probe -c user.email=probe@example.invalid -c commit.gpgsign=false commit-tree "$empty_tree")"
git -C "$base" update-ref refs/tags/v3.0.2 "$historical_commit"
expect_ok "daily-check-ignores-historical-tags" "$base"

echo "version-consistency-probe: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
