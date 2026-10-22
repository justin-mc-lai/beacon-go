// Truth package skeleton + the one sanctioned document write
// (P4a): the four-piece skeleton shapes rendered at create time,
// distilled from real v1 packages, and setFrontmatterStatus —
// the frontmatter status-line rewrite freeze/refreeze perform.
// Split out of truth.go (the package lifecycle service).
package orchestrator

import (
	"fmt"
	"strings"
)

// ---- create ----

// truthSkeleton renders the four-piece package skeleton. Shapes are
// distilled from real v1 packages (frontmatter keys, the frozen
// coverage-matrix layout AC|TC|Layer|Command|Assertion of the v2
// frozen packages, the tasks ledger and the evidence index) — no
// invented fields. The skeleton is deliberately integrity-clean in
// block mode: AC-001 is defined and covered by a hard TC, the
// evidence index names its surfaces by bare file name (no dangling
// artifact references yet).
func truthSkeleton(slug, version, pkgRel string) map[string]string {
	truth := fmt.Sprintf(`---
slug: %s
version: %s
status: draft
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: draft
materials_status: draft
canonical_refs:
  truth: %s/truth.md
  tests: %s/tests.md
  tasks: %s/tasks.md
  evidence: %s/evidence.md
---

# Requirement Truth: %s

## 人话

（一句话说清这个需求解决什么问题、给谁用、做到什么算完。冻结前由作者补齐。）

## User Intent Snapshot

`+"```yaml"+`
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: ""
`+"```"+`

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 作者写下第一条可验证的完成标准（可被 tests.md 的 TC-001 直接验证） |
`, slug, version, pkgRel, pkgRel, pkgRel, pkgRel, slug)
	tests := fmt.Sprintf(`---
slug: %s
version: %s
materials_status: draft
---

# Tests: %s

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `+"`go test ./... -run TestAC001`"+` | ac_001_holds |
`, slug, version, slug)
	tasks := fmt.Sprintf(`---
slug: %s
version: %s
materials_status: draft
task_source: acceptance_criteria
---

# Tasks: %s

## Task Ledger

- [ ] TASK-001 Implement AC-001 · ac=AC-001
- [ ] TASK-002 Validate package gates (truth review + freeze) · ac=GATES

## Boundary

Checkboxes are AC-bound ledgers. `+"`[x]`"+` is valid only when the bound evidence file exists (completion ≠ self-tick).
`, slug, version, slug)
	evidence := fmt.Sprintf(`# Evidence Index: %s

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | draft | truth |
| tests | test_truth | tests.md | draft | qa |

## Boundary

`+"`evidence.md`"+` is an index/read model. It must not contain final verdict, gate truth, or release readiness authority fields. Surfaces are named by bare file name until real evidence lands; replace with the full project-relative ref once the artifact exists and is registered.
`, slug)
	return map[string]string{
		"truth.md": truth, "tests.md": tests, "tasks.md": tasks, "evidence.md": evidence,
	}
}

// setFrontmatterStatus rewrites ONLY the frontmatter `status:`
// scalar of a markdown document (inserting it after `slug:` when
// absent). Everything outside that one line is preserved
// byte-for-byte. changed=false when the status already matches or
// no parseable frontmatter exists.
func setFrontmatterStatus(text, status string) (string, bool) {
	if !strings.HasPrefix(text, "---\n") {
		return text, false
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return text, false
	}
	fmEnd := 4 + end
	lines := strings.Split(text[4:fmEnd], "\n")
	found := false
	for i, ln := range lines {
		if strings.HasPrefix(ln, "status:") {
			lines[i] = "status: " + status
			found = true
		}
	}
	if !found {
		idx := len(lines)
		for i, ln := range lines {
			if strings.HasPrefix(ln, "slug:") {
				idx = i + 1
				break
			}
		}
		lines = append(lines[:idx], append([]string{"status: " + status}, lines[idx:]...)...)
	}
	out := "---\n" + strings.Join(lines, "\n") + text[fmEnd:]
	return out, out != text
}
