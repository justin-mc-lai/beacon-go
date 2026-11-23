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
ux_required: false
parser_contract: beacon-feature-package-v2
package_contract: truth-tasks
package_maturity: filled
materials_status: current
traceability_required: true
intent_confirmed: false
surfaces: [cli]
canonical_refs:
  truth: %[3]s/truth.md
  tasks: %[3]s/tasks.md
---

# Requirement Truth: %[1]s

## 人话

写清谁在什么场景下完成什么，做到什么算完成。

## User Intent Snapshot

`+"```yaml"+`
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: ""
goal: "写清这个包要达成的结果"
non_goals: "写清本包明确不做的范围"
constraints: "写清必须遵守的约束"
scope_mode: lake
`+"```"+`

## Open Questions

| Q ID | Question | Recommendation | Status | Answer |
|---|---|---|---|---|
| Q1 | 谁在什么场景下完成什么？ | 先写目标与非目标 | answered | 目标与非目标已写入 Intent Snapshot |

## Intent Coverage

| AC ID | Intent Source | Evidence |
|---|---|---|
| AC-001 | 目标：主体流程可达完成 | evidence/ac-001.txt |
| AC-002 | 约束：非法转移必须拒绝 | evidence/ac-002.txt |
| AC-003 | 目标：依赖失败可观测 | evidence/ac-003.txt |
| AC-004 | 约束：恢复必须幂等 | evidence/ac-004.txt |
| AC-005 | 目标：完成门不可绕过 | evidence/ac-005.txt |

## Subagent Plan

| 阶段 | 角色 | 产出 |
|---|---|---|
| intake | business-analyst | 问题清单与边界 |
| spec | product-manager | AC 与未做清单 |
| spec | domain-fsm-architect | 状态、转移、非法槽位 |
| spec | security-engineer | 支付与权限边界 |
| spec | devops-engineer | 环境与回滚 |
| spec | microservices-architect | 服务与契约 |
| spec | ui-designer | 页面与交互状态 |
| spec | database-administrator | 表结构与迁移 |
| spec | integration-specialist | 外部接口与回调 |
| test | test-automator | 每条 AC 的正负用例 |
| plan | task-distributor | 任务 DAG |
| review | architect-reviewer | 独立评审结论 |

## FSM

`+"```mermaid"+`
stateDiagram-v2
  [*] --> Draft
  Draft --> Active
  Active --> Blocked
  Blocked --> Active
  Active --> Done
  Done --> [*]
`+"```"+`

| From | To | Trigger | Guard | Side effect |
|---|---|---|---|---|
| Draft | Active | submit | fields complete | record submitted_at |
| Active | Blocked | dependency fails | retry budget left | record reason |
| Blocked | Active | dependency recovers | none | clear reason |
| Active | Done | accept | checks pass | record accepted_at |

## Acceptance Criteria

| AC ID | Criterion |
|---|---|
| AC-001 | 主体流程在合法输入下到达完成状态。 |
| AC-002 | 非法转移被拒绝且状态不变。 |
| AC-003 | 依赖失败进入阻塞并记录原因。 |
| AC-004 | 依赖恢复后从阻塞回到进行中。 |
| AC-005 | 完成动作只在检查通过后发生。 |

## Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion | Evidence |
|---|---|---|---|---|---|
| AC-001 | TC-001 | behavior | `+"`go test ./... -run TestAC001 -count=1`"+` | happy path reaches done | evidence/ac-001.txt |
| AC-001 | TC-002 | negative | `+"`go test ./... -run TestAC001Reject -count=1`"+` | invalid input rejected | evidence/ac-001-neg.txt |
| AC-001 | TC-003 | negative | `+"`go test ./... -run TestAC001Retry -count=1`"+` | retry is bounded | evidence/ac-001-retry.txt |
| AC-002 | TC-004 | behavior | `+"`go test ./... -run TestAC002 -count=1`"+` | illegal slot refused | evidence/ac-002.txt |
| AC-002 | TC-005 | negative | `+"`go test ./... -run TestAC002Noop -count=1`"+` | state unchanged | evidence/ac-002-noop.txt |
| AC-002 | TC-006 | negative | `+"`go test ./... -run TestAC002Force -count=1`"+` | forced transition refused | evidence/ac-002-force.txt |
| AC-003 | TC-007 | behavior | `+"`go test ./... -run TestAC003 -count=1`"+` | blocked state recorded | evidence/ac-003.txt |
| AC-003 | TC-008 | negative | `+"`go test ./... -run TestAC003NoBudget -count=1`"+` | no retry left blocks | evidence/ac-003-budget.txt |
| AC-003 | TC-009 | negative | `+"`go test ./... -run TestAC003NoReason -count=1`"+` | missing reason refused | evidence/ac-003-reason.txt |
| AC-004 | TC-010 | behavior | `+"`go test ./... -run TestAC004 -count=1`"+` | recovery resumes | evidence/ac-004.txt |
| AC-004 | TC-011 | negative | `+"`go test ./... -run TestAC004Stale -count=1`"+` | stale recovery refused | evidence/ac-004-stale.txt |
| AC-004 | TC-012 | negative | `+"`go test ./... -run TestAC004Double -count=1`"+` | double resume is idempotent | evidence/ac-004-double.txt |
| AC-005 | TC-013 | behavior | `+"`go test ./... -run TestAC005 -count=1`"+` | accept after checks | evidence/ac-005.txt |
| AC-005 | TC-014 | negative | `+"`go test ./... -run TestAC005Early -count=1`"+` | accept before checks refused | evidence/ac-005-early.txt |
| AC-005 | TC-015 | negative | `+"`go test ./... -run TestAC005FailedCheck -count=1`"+` | failed check blocks accept | evidence/ac-005-fail.txt |

## 未做

| 项 | 原因 |
|---|---|
| 旧包迁移 | 本版只保证兼容读取 |
`, slug, version, pkgRel)

	tasks := fmt.Sprintf(`---
slug: %[1]s
version: %[2]s
materials_status: draft
task_source: acceptance_criteria
package_contract: truth-tasks
---

# Tasks: %[1]s

## Task DAG

| TASK | 动作 | 依赖 | AC | Evidence |
|---|---|---|---|---|
| TASK-001 | 落状态与转移实现 | - | AC-001, AC-002 | evidence/ac-001.txt |
| TASK-002 | 落阻塞与恢复 | TASK-001 | AC-003, AC-004 | evidence/ac-003.txt |
| TASK-003 | 落完成门与回归 | TASK-002 | AC-005 | evidence/ac-005.txt |

## Task Ledger

- [ ] TASK-001 落状态与转移实现 · ac=AC-001,AC-002 · evidence=evidence/ac-001.txt
- [ ] TASK-002 落阻塞与恢复 · ac=AC-003,AC-004 · evidence=evidence/ac-003.txt
- [ ] TASK-003 落完成门与回归 · ac=AC-005 · evidence=evidence/ac-005.txt

## Boundary

任务只按 DAG 顺序执行。`+"`[x]`"+` 只在该行 Evidence 文件存在时勾选。
`, slug, version)

	return map[string]string{"truth.md": truth, "tasks.md": tasks}
}
