#!/bin/zsh
# Shared fixture for the kernel-recovery-qa-bridge seeds.
# Every seed builds its own isolated store (BEACON_STORE_PATH) and a
# throwaway project under a temp dir, so running a seed never touches
# the global ~/.beacon store or this repository's .beacon/state.
set -eu

: "${BEACON_BIN:?set BEACON_BIN to a built beacon binary (go build -o <path> ./cmd/beacon)}"
if [ ! -x "$BEACON_BIN" ]; then
  echo "BEACON_BIN is not executable: $BEACON_BIN" >&2
  exit 2
fi

SEED_TMP=$(mktemp -d)
export BEACON_STORE_PATH="$SEED_TMP/db.sqlite"
SEED_PROJECT="$SEED_TMP/proj"
mkdir -p "$SEED_PROJECT" "$SEED_TMP/out"

fixture_init() {
  "$BEACON_BIN" projects register --path "$SEED_PROJECT" --name proj \
    --version v2.0.0 --docs-root docs/beacon/v2.0.0 >/dev/null
  "$BEACON_BIN" truth create --project "$SEED_PROJECT" --slug demo --version v2.0.0 >/dev/null
  local d="$SEED_PROJECT/docs/beacon/v2.0.0/features/demo"
  cat > "$d/truth.md" <<'DOC'
---
slug: demo
version: v2.0.0
status: frozen
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: filled
materials_status: current
intent_confirmed: true
canonical_refs:
  truth: docs/beacon/v2.0.0/features/demo/truth.md
  tests: docs/beacon/v2.0.0/features/demo/tests.md
  tasks: docs/beacon/v2.0.0/features/demo/tasks.md
  evidence: docs/beacon/v2.0.0/features/demo/evidence.md
---

# Requirement Truth: demo

## 人话

A seed fixture.

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "seed fixture"
goal: "seed fixture holds"
non_goals: "none"
constraints: "none"
scope_mode: focused
```

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | the seed fixture holds |

## Boundary

- fixture only
DOC
  cat > "$d/tests.md" <<'DOC'
---
slug: demo
version: v2.0.0
materials_status: current
---

# Tests: demo

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | unit | `true` | exit0 |
DOC
  cat > "$d/tasks.md" <<'DOC'
---
slug: demo
version: v2.0.0
---

# Tasks: demo

- [ ] fixture work · ac=AC-001
DOC
  cat > "$d/evidence.md" <<'DOC'
---
slug: demo
version: v2.0.0
---

# Evidence: demo

| Surface | Authority | Artifact |
|---|---|---|
| qa | qa_verdict | qa-report.md |

(no artifacts referenced yet)
DOC
}

# new_run prints the run id of a freshly created manual goal run.
new_run() {
  "$BEACON_BIN" goal run --project "$SEED_PROJECT" -f demo \
    --executor manual --executor-id opus "seed fixture" --json \
    | python3 -c 'import json,sys; print(json.load(sys.stdin)["run_id"])'
}

# tick_to advances a manual run N stages (plan -> truth -> implement -> qa ...).
tick_to() {
  local run="$1" n="$2" i
  for i in $(seq 1 "$n"); do
    "$BEACON_BIN" goal tick --run-id "$run" --by opus >/dev/null 2>&1 || true
  done
}

lifecycle_state() {
  sqlite3 "$BEACON_STORE_PATH" "select stage || '/' || status from feature_lifecycle;"
}

# json_field FILE EXPR prints one python expression over the parsed doc.
json_field() {
  python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(eval(sys.argv[2], {"d": d}))' "$1" "$2"
}

qa_stage_executed_count() {
  sqlite3 "$BEACON_STORE_PATH" "select count(*) from lifecycle_events where event_type='stage_executed' and json_extract(payload_json,'\$.stage')='qa' and json_extract(payload_json,'\$.placeholder')=0;"
}

emit_canon() {
  local file="$1"
  echo "canon=$file"
  echo "sha256=$(shasum -a 256 "$file" | awk '{print $1}')"
}
