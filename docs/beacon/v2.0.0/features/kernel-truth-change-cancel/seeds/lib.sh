#!/bin/zsh
# Shared fixture for the kernel-truth-change-cancel seeds.
# Every seed owns a mktemp store (BEACON_STORE_PATH) and a throwaway
# project, so running a seed never touches ~/.beacon or this repo's
# .beacon/state. The fixture package is frozen through the real
# `beacon truth freeze` path so the change ledger carries a real
# truth_sha256_after baseline.
set -eu

: "${BEACON_BIN:?set BEACON_BIN to a built beacon binary (cd beacon-go && go build -o <path> ./cmd/beacon)}"
if [ ! -x "$BEACON_BIN" ]; then
  echo "BEACON_BIN is not executable: $BEACON_BIN" >&2
  exit 2
fi

SEED_TMP=$(mktemp -d)
export BEACON_STORE_PATH="$SEED_TMP/db.sqlite"
SEED_PROJECT="$SEED_TMP/proj"
FEATURE_DIR="$SEED_PROJECT/docs/beacon/v2.0.0/features/demo"
mkdir -p "$SEED_PROJECT" "$SEED_TMP/out"

write_package() {
  cat > "$FEATURE_DIR/truth.md" <<'DOC'
---
slug: demo
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
canonical_refs:
  truth: docs/beacon/v2.0.0/features/demo/truth.md
  tests: docs/beacon/v2.0.0/features/demo/tests.md
  tasks: docs/beacon/v2.0.0/features/demo/tasks.md
  evidence: docs/beacon/v2.0.0/features/demo/evidence.md
---

# Requirement Truth: demo

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "seed fixture"
goal: "the seed fixture holds"
non_goals: "none"
constraints: "none"
scope_mode: focused
```

## 人话

A frozen fixture package used to exercise the truth change cancel path.

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | the fixture truth body is frozen |
| AC-002 | the fixture ledger records a change |
| AC-003 | the fixture ledger records a second change |
| AC-004 | the fixture package stays on disk |
| AC-005 | the fixture ledger is append only |

## Intent Coverage

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | seed fixture | docs/beacon/v2.0.0/features/demo/evidence/seed1.canon |
| AC-002 | seed fixture | docs/beacon/v2.0.0/features/demo/evidence/seed1.canon |
| AC-003 | seed fixture | docs/beacon/v2.0.0/features/demo/evidence/seed1.canon |
| AC-004 | seed fixture | docs/beacon/v2.0.0/features/demo/evidence/seed1.canon |
| AC-005 | seed fixture | docs/beacon/v2.0.0/features/demo/evidence/seed1.canon |

## Boundary

- fixture only
DOC
  cat > "$FEATURE_DIR/tests.md" <<'DOC'
---
slug: demo
version: v2.0.0
materials_status: current
---

# Tests: demo

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `true` | fixture holds |
| AC-002 | TC-002 | behavior | `true` | fixture holds |
| AC-003 | TC-003 | behavior | `true` | fixture holds |
| AC-004 | TC-004 | behavior | `true` | fixture holds |
| AC-005 | TC-005 | guard | `true` | fixture holds |
DOC
  cat > "$FEATURE_DIR/tasks.md" <<'DOC'
---
slug: demo
version: v2.0.0
---

# Tasks: demo

- [ ] fixture work · ac=AC-001
DOC
  cat > "$FEATURE_DIR/evidence.md" <<'DOC'
---
slug: demo
version: v2.0.0
---

# Evidence: demo

| Surface | Authority | Artifact |
|---|---|---|
| qa | qa_verdict | qa-report.md |
DOC
}

fixture_init() {
  "$BEACON_BIN" projects register --path "$SEED_PROJECT" --name proj \
    --version v2.0.0 --docs-root docs/beacon/v2.0.0 >/dev/null
  "$BEACON_BIN" truth create --project "$SEED_PROJECT" --slug demo --version v2.0.0 >/dev/null
  write_package
  mkdir -p "$FEATURE_DIR/evidence"
  print -r -- 'demo canon line' > "$FEATURE_DIR/evidence/seed1.canon"
  "$BEACON_BIN" truth freeze --project "$SEED_PROJECT" --feature demo --by seed >/dev/null
}

# json_field FILE EXPR prints one python expression over the parsed doc.
json_field() {
  python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(eval(sys.argv[2], {"d": d}))' "$1" "$2"
}

truth_json() {
  "$BEACON_BIN" truth show --project "$SEED_PROJECT" --feature demo --json
}

feature_id() {
  sqlite3 "$BEACON_STORE_PATH" "select id from feature_packages where slug='demo';"
}

event_count() {
  sqlite3 "$BEACON_STORE_PATH" "select count(*) from lifecycle_events where event_type='$1';"
}

last_event_payload() {
  sqlite3 "$BEACON_STORE_PATH" "select payload_json from lifecycle_events where event_type='$1' order by rowid desc limit 1;"
}

payload_field() {
  # payload_field EVENT_TYPE EXPR
  python3 -c 'import json,sys; d=json.loads(sys.argv[1]); print(eval(sys.argv[2], {"d": d}))' "$(last_event_payload "$1")" "$2"
}

body_sha() {
  shasum -a 256 "$FEATURE_DIR/truth.md" | awk '{print $1}'
}

# blank_change_base strips the recorded base fingerprint from the latest
# pending change document. This reproduces a legacy (pre-R1) change record
# whose base body hash was never recorded. The trail itself is left alone:
# the ledger is append-only, and the kernel reads the base from the doc.
blank_change_base() {
  local doc="$1"
  sed -i '' -E 's/^base_truth_sha256:.*/base_truth_sha256:/' "$SEED_PROJECT/$doc"
  # ponytail: the trail is append-only (DB trigger), so the legacy
  # "no base" state is reproduced on the document only; the kernel
  # reads the base from the change doc, never by rewriting events.
}

emit_canon() {
  local file="$1" shown="$1"
  case "$file" in
    "$PWD"/*) shown="${file#"$PWD"/}" ;;
  esac
  echo "canon=$shown"
  echo "sha256=$(shasum -a 256 "$file" | awk '{print $1}')"
}

# normalize_change_ids rewrites the per-run generated change id in a canon
# file to a stable token. The seeds assert on the real id in shell before
# this runs, so coverage is unchanged while the canon bytes and their
# sha256 stay reproducible across runs.
normalize_change_ids() {
  local file="$1" tmp="$1.tmp.$$"
  sed -E 's/chg_[0-9a-f]{12}/chg_<id>/g' "$file" > "$tmp" && mv "$tmp" "$file"
}
