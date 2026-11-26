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

parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: draft
materials_status: draft
intent_confirmed: false
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
goal: ""
non_goals: ""
constraints: ""
scope_mode: focused
`+"```"+`

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 用一句话说清：谁、什么问题、什么算完成？ | 先写目标与非目标，再写一条可验证的完成标准 | open | |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | (fill: 用户原话/目标/约束) | (fill: 证据路径、conversation:) |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 作者写下第一条可验证的完成标准（可被 tests.md 的 TC-001 直接验证） |

## FSM 五元组

Only required when frontmatter domain_required: true. Fill the
five-tuple, declare terminal states (终态： ...), and add an
Illegal Transitions section below listing forbidden moves
(- illegal: from -> to). The graph is machine-validated at
freeze: a missing target, a missing event, or a missing illegal
section blocks.

| State | Event | Guard | To | Action |
|-------|-------|-------|----|--------|

## Parity Matrix

Only required when scope_mode: full_parity. List the source capability,
whether Beacon covers it, and the evidence.

| Capability | Covered | Evidence |
|------------|---------|----------|
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

// truthSkeletonSDD renders the two-file SDD contract: truth.md is the
// Spec (intent, subagent plan, FSM, AC/TC matrix) and tasks.md is the
// DAG. tests.md / evidence.md / restatement.md are not created.
func truthSkeletonSDD(slug, version, pkgRel string) map[string]string {
	truth := fmt.Sprintf(`---
slug: %[1]s
version: %[2]s
status: draft
language: zh
domain_required: false

parser_contract: beacon-feature-package-v2
package_contract: truth-tasks
package_maturity: scaffold
materials_status: draft
traceability_required: true
intent_confirmed: false
package_author: ""
surfaces: [cli]
canonical_refs:
  truth: %[3]s/truth.md
  tasks: %[3]s/tasks.md
---

# Requirement Truth: %[1]s

## 人话

待补齐：用户、场景、可观察结果。

## User Intent Snapshot

`+"```yaml"+`
utterance: ""
goal: ""
non_goals: ""
constraints: ""
scope_mode: focused
`+"```"+`

## Open Questions

| Q ID | Question | Recommendation | Status | Answer |
|---|---|---|---|---|
| Q1 | 用户要获得什么可验证结果，哪些工作明确不做？ | 依据原话和已有代码回答 | open | |
| Q2 | 本需求是否有领域状态与安全拒绝路径？ | 明确设置 fsm_required 与 negative_required 并给出依据 | open | |

## Grounding

| Source | Reference | Supports |
|---|---|---|

## Assumptions

记录需要确认的假设与确认依据；没有假设时明确写无。

## Intent Coverage

| AC ID | Intent Source | Evidence |
|---|---|---|

## Acceptance Criteria

| AC ID | Criterion |
|---|---|

## Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion | Evidence | Required Tests |
|---|---|---|---|---|---|---|

## FSM

有状态需求填写 From、To、Trigger、Guard、Side effect 表与转移覆盖；无状态需求设置 fsm_required: false 并说明依据。不要套用需求包生命周期。

## Transition Coverage

| From | To | Trigger | TC ID |
|---|---|---|---|

## 未做

明确写出本包边界。
`, slug, version, pkgRel)
	tasks := fmt.Sprintf(`---
slug: %s
version: %s
materials_status: draft
task_source: acceptance_criteria
package_contract: truth-tasks
---

# Tasks: %s

## Task DAG

| TASK | Action | Depends | AC | Command | Verify | Evidence | Auto |
|---|---|---|---|---|---|---|---|

## Task Ledger

待补齐：每个任务绑定 AC；完成状态以执行回执和证据为准。
`, slug, version, slug)
	return map[string]string{"truth.md": truth, "tasks.md": tasks}
}
