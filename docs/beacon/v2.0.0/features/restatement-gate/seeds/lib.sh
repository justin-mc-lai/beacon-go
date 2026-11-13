#!/bin/zsh
# Shared fixture for the restatement-gate seeds.
# Every seed builds its own isolated store (BEACON_STORE_PATH) and a
# throwaway project under a temp dir, so running a seed never touches
# the global ~/.beacon store or this repository's .beacon/state.
set -eu

: "${BEACON_BIN:?set BEACON_BIN to a built beacon binary (cd beacon-go && go build -o <path> ./cmd/beacon)}"
if [ ! -x "$BEACON_BIN" ]; then
  echo "BEACON_BIN is not executable: $BEACON_BIN" >&2
  exit 2
fi

SEED_TMP=$(mktemp -d)
export BEACON_STORE_PATH="$SEED_TMP/db.sqlite"
SEED_PROJECT="$SEED_TMP/proj"
DOCS="$SEED_PROJECT/docs/beacon/v2.0.0"
mkdir -p "$SEED_PROJECT" "$SEED_TMP/out"

project_init() {
  "$BEACON_BIN" projects register --path "$SEED_PROJECT" --name proj \
    --version v2.0.0 --docs-root docs/beacon/v2.0.0 >/dev/null
}

# write_pkg SLUG RESTATES [nosurface]
#   RESTATES is the token after `restates=` in the AC row (pass "" to omit it).
#   `nosurface` omits the surfaces declaration, for the ordering scenario.
write_pkg() {
  local slug=$1 restates=$2 nosurface=${3-}
  D="$DOCS/features/$slug"
  mkdir -p "$D"
  local surf="" ac="the fixture holds."
  if [ -z "$nosurface" ]; then
    surf=$'surfaces: [cli]\nsurface_strict: true\n'
  fi
  if [ -n "$restates" ]; then
    ac="$ac restates=$restates"
  fi
  cat > "$D/truth.md" <<DOC
---
slug: $slug
version: v2.0.0
status: draft
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: filled
materials_status: current
intent_confirmed: true
${surf}canonical_refs:
  truth: docs/beacon/v2.0.0/features/$slug/truth.md
  tests: docs/beacon/v2.0.0/features/$slug/tests.md
  tasks: docs/beacon/v2.0.0/features/$slug/tasks.md
  evidence: docs/beacon/v2.0.0/features/$slug/evidence.md
---

# Requirement Truth: $slug

## 人话

seed fixture for the restatement gate.

## User Intent Snapshot

\`\`\`yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "seed fixture"
goal: "the fixture holds"
non_goals: "none"
constraints: "none"
scope_mode: focused
\`\`\`

## Open Questions

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | does the fixture parse? | yes | answered | yes |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | $ac |

## Intent Coverage

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | seed fixture | tests.md TC-001 |
DOC
  cat > "$D/tests.md" <<DOC
---
slug: $slug
version: v2.0.0
materials_status: current
---

# Tests: $slug

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | \`true\` | holds |
DOC
  cat > "$D/tasks.md" <<DOC
---
slug: $slug
version: v2.0.0
---

# Tasks: $slug

- [ ] fixture work · ac=AC-001
DOC
  cat > "$D/evidence.md" <<DOC
---
slug: $slug
version: v2.0.0
---

# Evidence: $slug

| Surface | Authority | Artifact |
|---|---|---|
| qa | qa_verdict | qa-report.md |
DOC
}

# restatement_body [sections]  sections defaults to 1,2,3,4,5.
restatement_body() {
  local s=${1:-1,2,3,4,5} n
  for n in 1 2 3 4 5; do
    case ",$s," in
      *",$n,"*)
        printf '## %s. section %s\n\nbody %s.\n\n' "$n" "$n" "$n"
        if [ "$n" = 3 ]; then
          printf '## 3.1. first freeze\n\nforced.\n\n## 3.2. refreeze\n\nlegacy waiver.\n\n'
        fi
        ;;
    esac
  done
}

sha_of() {
  printf '%s' "$1" | shasum -a 256 | awk '{print $1}'
}

# write_restatement DIR CONFIRMED_BY [BODY] [HASH]
write_restatement() {
  local dir=$1 by=$2 body=${3-} hash=${4-}
  if [ -z "$body" ]; then
    body=$(restatement_body)
  fi
  if [ -z "$hash" ]; then
    hash=$(sha_of "$body")
  fi
  printf -- '---\nslug: %s\nconfirmed_by: %s\nconfirmed_at: 2026-11-13T10:00:00+08:00\nhash: %s\n---\n%s' \
    "$(basename "$dir")" "$by" "$hash" "$body" > "$dir/restatement.md"
}

# freeze SLUG OUT — writes the JSON view to OUT and leaves the exit code in FREEZE_RC.
freeze() {
  local slug=$1 out=$2
  FREEZE_RC=0
  "$BEACON_BIN" truth freeze --project "$SEED_PROJECT" --feature "$slug" \
    --version v2.0.0 --by seed --json >"$out" 2>"$out.err" || FREEZE_RC=$?
  echo "$FREEZE_RC" > "$out.rc"
}

json_field() { # json_field FILE EXPR
  python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(eval(sys.argv[2], {"d": d}))' "$1" "$2"
}

PASS=0
FAILED=0
pass() { PASS=$((PASS + 1)); echo "PASS $*"; }
flunk() { FAILED=$((FAILED + 1)); echo "FAIL $*"; }
