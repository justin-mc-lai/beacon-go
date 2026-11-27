#!/usr/bin/env bash
# Version single-source guard.
#
# internal/version/version.go is the only authoritative version. Docs may
# mention historical go.N lines, but the "current release" declarations must
# match the code, and AGENTS.md must not hard-code the current version.

set -euo pipefail

candidate_tag=""
release_notes=""
tag_set=0
notes_set=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --tag|--release-notes)
      option="$1"
      if [ "$#" -lt 2 ] || [[ "$2" == --* ]]; then
        echo "version-consistency: $option requires a value" >&2
        exit 1
      fi
      if [ "$option" = --tag ]; then
        if [ "$tag_set" -eq 1 ]; then
          echo "version-consistency: duplicate option $option" >&2
          exit 1
        fi
        candidate_tag="$2"
        tag_set=1
      else
        if [ "$notes_set" -eq 1 ]; then
          echo "version-consistency: duplicate option $option" >&2
          exit 1
        fi
        release_notes="$2"
        notes_set=1
      fi
      shift 2
      ;;
    --help|-h)
      echo 'Usage: check_version_consistency.sh [--tag vX.Y.Z --release-notes PATH]'
      echo 'Default: source/docs checks only; no implicit Git tag selection.'
      echo 'Candidate: also check exact tag, CHANGELOG entry and release notes heading/body.'
      echo 'PATH is absolute or relative to CHECK_VERSION_ROOT (default: repository root).'
      echo 'Notes title: # [Release |Beacon Go ]vX.Y.Z; this checks content, not publication.'
      exit 0
      ;;
    *)
      echo "version-consistency: unknown option $1" >&2
      exit 1
      ;;
  esac
done
if [ "$tag_set" -ne "$notes_set" ]; then
  echo 'version-consistency: candidate scope requires both --tag and --release-notes' >&2
  exit 1
fi

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

if [ "$tag_set" -eq 1 ]; then
  if ! python3 - "$ROOT" "$ver" "$candidate_tag" "$release_notes" <<'PY'
import pathlib
import re
import sys

root, version, tag, notes_path = sys.argv[1:]

def reject(message):
    sys.exit("version-consistency: " + message)

if not re.fullmatch(r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)", version):
    reject("candidate tag requires a plain SemVer source version")
if tag != "v" + version:
    reject("candidate tag %r does not match version.go (v%s)" % (tag, version))

def read(path, label):
    try:
        return path.read_text(encoding="utf-8")
    except (OSError, UnicodeError, ValueError) as exc:
        reject("%s cannot be read: %s" % (label, exc))

def markdown_lines(text):
    lines = []
    fence = None
    for line in re.sub(r"<!--.*?-->", "", text, flags=re.S).splitlines():
        marker = re.match(r"^\s{0,3}(`{3,}|~{3,})", line)
        if marker:
            token = marker[1]
            if fence is None:
                fence = token
            elif token[0] == fence[0] and len(token) >= len(fence):
                fence = None
            continue
        if fence is None:
            lines.append(line)
    return lines

def require_body(lines, label):
    body = "\n".join(line for line in lines if not re.match(r"^\s*#", line))
    body = re.sub(r"[\s*`_>#|\-]+", "", body)
    if not body or re.fullmatch(r"(?i:TODO|TBD|placeholder|generatedstub)|待补充|待填写", body):
        reject(label + " body is empty or a generated stub")

changelog = markdown_lines(read(pathlib.Path(root) / "CHANGELOG.md", "CHANGELOG"))
entries = [index for index, line in enumerate(changelog)
           if re.match(r"^## \[" + re.escape(version) + r"\](?:\s.*)?$", line)]
if len(entries) != 1:
    reject("CHANGELOG entry [%s] must occur exactly once (found %d)" % (version, len(entries)))
start = entries[0] + 1
end = next((index for index in range(start, len(changelog))
            if re.match(r"^#{1,2}\s", changelog[index])), len(changelog))
require_body(changelog[start:end], "CHANGELOG")

if not notes_path.strip():
    reject("release notes path is empty")
notes_file = pathlib.Path(notes_path)
if not notes_file.is_absolute():
    notes_file = pathlib.Path(root) / notes_file
notes = markdown_lines(read(notes_file, "release notes"))
headings = [line for line in notes if re.match(r"^#\s", line)]
title = r"# (?:Release |Beacon Go )?v?" + re.escape(version) + r"(?:\s+[—–-]\s+.+)?"
if len(headings) != 1 or not re.fullmatch(title, headings[0]):
    reject("release notes heading must identify only v%s exactly once" % version)
require_body(notes[notes.index(headings[0]) + 1:], "release notes")
print("version-consistency: candidate ok (%s, CHANGELOG, %s)" % (tag, notes_file))
PY
  then
    fail=1
  fi
fi

if [ "$fail" -eq 0 ]; then
  echo "version-consistency: ok ($ver)"
fi
exit "$fail"
