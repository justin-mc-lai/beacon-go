#!/bin/sh
# Read-only kernel maintainer for beacon-go.
#
# Repository-side replacement for the old task that generated a maintenance
# body and dispatched it. This script NEVER fetches, builds, installs,
# dispatches, edits, or evaluates text. It compares the expected hash
# (manifest plus the approval-bound install record) with the target's real
# sha256 through local_release.py status, reports aligned / drifted /
# missing, appends one line on drift, and returns the status code.
#
# Deliberately contains no backticks and no command substitution: body and
# log text are pure data and must never be evaluated.
#
# Exit codes are local_release.py status:
#   0 aligned / 60 drifted / 61 missing / 62 unknown manifest /
#   63 not installed / 68 not usable. Reporting only; no repair.
set -u

BG="${BEACON_REPO:-/Users/apple/Developer/Personal/products/beacon-go}"
STATE="${BEACON_RELEASE_STATE:-$BG/.beacon/release-state}"
TARGET="${BEACON_TARGET:-$HOME/.local/opt/beacon-go/beacon}"
LOG="${BEACON_DRIFT_LOG:-$BG/.beacon/release-drift.log}"

case "${1:-check}" in
  render-body)
    # Emit a maintenance body as data. The file may contain backticks or
    # shell syntax; it is copied verbatim and never evaluated.
    if [ "$#" -lt 2 ]; then
      printf 'usage: %s render-body FILE\n' "$0" >&2
      exit 2
    fi
    cat "$2"
    exit 0
    ;;
  check) ;;
  *)
    printf 'usage: %s [check | render-body FILE]\n' "$0" >&2
    exit 2
    ;;
esac

if [ ! -f "$BG/scripts/local_release.py" ]; then
  printf 'maintainer: local_release.py missing under %s\n' "$BG" >&2
  exit 1
fi

python3 "$BG/scripts/local_release.py" status --state-dir "$STATE" --target "$TARGET"
rc=$?

if [ "$rc" -eq 0 ]; then
  printf 'aligned: %s\n' "$TARGET"
  exit 0
fi

LOG_DIR="${LOG%/*}"
[ -d "$LOG_DIR" ] || mkdir -p "$LOG_DIR"
date '+%Y-%m-%dT%H:%M:%S%z' >> "$LOG"
printf 'drift exit=%s target=%s state=%s\n' "$rc" "$TARGET" "$STATE" >> "$LOG"
printf 'drift: exit=%s target=%s (logged to %s)\n' "$rc" "$TARGET" "$LOG"
exit "$rc"
