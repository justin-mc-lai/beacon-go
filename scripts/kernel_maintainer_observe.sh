#!/bin/sh
# Read-only kernel maintenance observer.
#
# This is the repository-side replacement for any "generate a maintenance
# body and dispatch it" task. It NEVER builds, installs, dispatches,
# edits, or evaluates text: a maintenance body is data, and generating
# one must not run commands (a backtick or $(...) inside a body that is
# later quoted must never be substituted).
#
# It only reports: repo commit/dirty state and the installed binary's
# release status (via local_release.py status, which never mutates).
set -eu

BG="${BEACON_REPO:-$HOME/Developer/Personal/products/beacon-go}"
STATE="${BEACON_RELEASE_STATE:-$BG/.beacon/release-state}"
BIN="${BEACON_BIN:-$HOME/.local/opt/beacon-go/beacon}"

printf 'repo: %s\n' "$BG"
git -C "$BG" rev-parse HEAD
git -C "$BG" rev-parse --abbrev-ref HEAD
git -C "$BG" status --porcelain

set +e
python3 "$BG/scripts/local_release.py" status --state-dir "$STATE" --target "$BIN"
rc=$?
set -e
printf 'release_status_exit: %s\n' "$rc"
# 0 aligned / 60 drifted / 61 missing / 62 unknown manifest /
# 63 not installed / 68 not usable. Reporting only; no repair.
exit 0
