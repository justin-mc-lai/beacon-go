package gate

import (
	"strings"
	"testing"

	"beacon-go/internal/core"
)

// ---- R2a helpers ----

func findingKinds(fs []Finding) map[string]bool {
	out := map[string]bool{}
	for _, f := range fs {
		out[f.Kind] = true
	}
	return out
}

// intentPkg builds a PackageFiles whose truth satisfies the whole
// intent gate except whatever the caller mutates in.
func intentPkg(truth string) PackageFiles {
	return PackageFiles{
		Slug:  "demo",
		Truth: PackageFile{Path: "features/demo/truth.md", Present: true, Text: truth},
		Tests: PackageFile{Path: "features/demo/tests.md", Present: true, Text: goodTests},
		Tasks: PackageFile{Path: "features/demo/tasks.md", Present: true, Text: "---\nslug: demo\n---\n- [ ] work\n"},
		Evidence: PackageFile{Path: "features/demo/evidence.md", Present: true,
			Text: "# Evidence\n\n| Surface | Authority | Artifact |\n|---|---|---|\n| qa | qa_verdict | artifacts/qa-report.md |\n"},
	}
}

const intentTruthBase = `---
slug: demo
version: v1.6.19
status: draft
language: zh
domain_required: false
package_maturity: filled
intent_confirmed: true
parser_contract: beacon-feature-package-v2
canonical_refs:
  truth: features/demo/truth.md
  tests: features/demo/tests.md
  tasks: features/demo/tasks.md
  evidence: features/demo/evidence.md
---

# Requirement Truth: demo

## 人话

给运维同学一个能离线核对的发布清单，核对完才算完成。

## User Intent Snapshot

` + "```yaml" + `
utterance: "给运维一个离线发布清单"
goal: "离线核对发布清单"
non_goals: "不做在线部署"
constraints: "离线可验证"
scope_mode: focused
` + "```" + `

## Intent Coverage

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | user utterance | truth.md#User-Intent-Snapshot |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 清单文件存在且校验和匹配 |
`

// ---- H1: vague disjunction ----

func TestCheckEVagueBlocksBranchWords(t *testing.T) {
	// Python keeps the LONGEST description per AC across tables,
	// so the vague text must out-length the coverage evidence
	// cell to be the description under review.
	truth := strings.Replace(intentTruthBase,
		"| AC-001 | 清单文件存在且校验和匹配 |",
		"| AC-001 | 清单支持把核对结果导出成文件或者直接打印出来归档保存以便后续审计核查使用 |", 1)
	kinds := findingKinds(CheckIntentGate(intentPkg(truth)))
	if !kinds["ac_vague_disjunction"] {
		t.Fatalf("或者 in an AC must be flagged, got %v", kinds)
	}
	// The clean base must produce no findings at all.
	if got := CheckIntentGate(intentPkg(intentTruthBase)); len(got) != 0 {
		t.Fatalf("clean intent package must pass, got %+v", got)
	}
	// A branch word inside backticks is code, not prose.
	truth2 := strings.Replace(intentTruthBase,
		"| AC-001 | 清单文件存在且校验和匹配 |",
		"| AC-001 | 字段 `mode_a_or_b` 的值必须原样输出到清单文件里面不得改动 |", 1)
	for _, f := range CheckIntentGate(intentPkg(truth2)) {
		if f.Kind == "ac_vague_disjunction" {
			t.Fatalf("backticked code must not fire the vague check: %+v", f)
		}
	}
	// English mirrors fire too.
	truth3 := strings.Replace(intentTruthBase,
		"| AC-001 | 清单文件存在且校验和匹配 |",
		"| AC-001 | the export retries at least three times before it gives up |", 1)
	if !findingKinds(CheckIntentGate(intentPkg(truth3)))["ac_vague_disjunction"] {
		t.Fatal("'at least' in an AC must be flagged")
	}
}

// ---- H2: FSM graph validation ----

const fsmDocGood = `# Truth

## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|----|--------|
| requested | verify | stage_ok | checked | run_verify |
| checked | write | digest_bound | stored | write_bundle |
| stored | load | inputs_changed | stored | recompute |

**终态：** stored

### invalid / illegal

- illegal: requested -> stored
`

func TestFSMGraphValidation(t *testing.T) {
	p := ParseTruthFSM(fsmDocGood)
	if !p.FiveTuplePresent || len(p.Transitions) != 3 {
		t.Fatalf("projection = %+v", p)
	}
	findings, blocked := ValidateTruthFSM(p)
	if blocked {
		t.Fatalf("gold-shaped FSM must pass, findings=%v", findings)
	}
	for _, f := range findings {
		if f.Severity == "error" {
			t.Fatalf("no error findings expected, got %+v", f)
		}
	}

	// Missing target: an edge to nowhere is an error.
	bad := strings.Replace(fsmDocGood, "| checked | write | digest_bound | stored | write_bundle |",
		"| checked | write | digest_bound |  | write_bundle |", 1)
	_, blocked = ValidateTruthFSM(ParseTruthFSM(bad))
	if !blocked {
		t.Fatal("transition without target must block")
	}

	// Missing illegal section: fail-closed error.
	noIllegal := strings.Split(fsmDocGood, "### invalid")[0]
	findings, blocked = ValidateTruthFSM(ParseTruthFSM(noIllegal))
	if !blocked || !fsmKinds(findings)["illegal_slot_missing"] {
		t.Fatalf("missing illegal section must block with illegal_slot_missing, got %v blocked=%v", findings, blocked)
	}

	// An illegal edge that is also legal is a contradiction.
	contra := strings.Replace(fsmDocGood, "- illegal: requested -> stored",
		"- illegal: requested -> checked", 1)
	findings, blocked = ValidateTruthFSM(ParseTruthFSM(contra))
	if !blocked || !fsmKinds(findings)["illegal_declared_as_legal"] {
		t.Fatalf("illegal==legal must block, got %v blocked=%v", findings, blocked)
	}

	// Undeclared terminal is a warning, never a block: point the
	// terminal declaration at a state that never appears and drop
	// the self-loop so a real reachable end exists undeclared.
	noTerm := strings.Replace(fsmDocGood, "**终态：** stored", "**终态：** archived", 1)
	noTerm = strings.Replace(noTerm, "| stored | load | inputs_changed | stored | recompute |\n", "", 1)
	findings, blocked = ValidateTruthFSM(ParseTruthFSM(noTerm))
	if blocked {
		t.Fatalf("undeclared terminal must warn only, got %v", findings)
	}
	if !fsmKinds(findings)["terminal_undeclared"] {
		t.Fatalf("want terminal_undeclared warning, got %v", findings)
	}
}

func fsmKinds(fs []FSMFinding) map[string]bool {
	out := map[string]bool{}
	for _, f := range fs {
		out[f.Kind] = true
	}
	return out
}

func TestIntentGateFSMGraphBlocks(t *testing.T) {
	truth := strings.Replace(intentTruthBase, "domain_required: false", "domain_required: true", 1)
	truth += `
## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|----|--------|
| idle | start | ok | running | begin |
| running | stop | ok |  | halt |
`
	codes := map[string]bool{}
	for _, f := range CheckIntentGate(intentPkg(truth)) {
		codes[f.ReasonCode] = true
		if f.Kind == "transition_target_missing" && f.ReasonCode != core.ReasonTruthFSMInvalid {
			t.Fatalf("graph finding must carry truth_fsm_invalid, got %+v", f)
		}
	}
	if !codes[core.ReasonTruthFSMInvalid] {
		t.Fatalf("bad FSM graph must fail the intent gate with truth_fsm_invalid, codes=%v", codes)
	}
}

// ---- H4: fill depth ----

func TestFillDepthJourneyAndMetaFSM(t *testing.T) {
	truth := strings.Replace(intentTruthBase, "domain_required: false", "domain_required: true", 1)
	truth += `
## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|----|--------|
| draft | submit | filled | in_review | open_review |
| in_review | approve | signed | frozen | freeze |

### Illegal Transitions

- illegal: draft -> frozen
`
	kinds := findingKinds(CheckIntentGate(intentPkg(truth)))
	if !kinds["journey_missing"] {
		t.Fatalf("business domain without a journey must be flagged, got %v", kinds)
	}
	if !kinds["scaffold_meta_fsm"] {
		t.Fatalf("lifecycle-only FSM must be flagged as meta impersonation, got %v", kinds)
	}
	withJourney := truth + "\n## 用户旅程\n\n1. 运维打开清单核对。\n"
	kinds = findingKinds(CheckIntentGate(intentPkg(withJourney)))
	if kinds["journey_missing"] {
		t.Fatal("a present journey must clear journey_missing")
	}
	if !kinds["scaffold_meta_fsm"] {
		t.Fatal("meta FSM finding must remain until real domain states land")
	}
}

// ---- M5: deferral ledger ----

func TestDeferralLedgerGate(t *testing.T) {
	truth := strings.Replace(intentTruthBase, "scope_mode: focused", "scope_mode: full_parity", 1)
	truth = strings.Replace(truth, `utterance: "给运维一个离线发布清单"`,
		`utterance: "完整复刻旧清单，先做 mvp 砍掉导出"`, 1)
	truth += "\n## Parity Matrix\n\n| Capability | Covered | Evidence |\n|------------|---------|----------|\n| export | yes | plan |\n"
	kinds := findingKinds(CheckIntentGate(intentPkg(truth)))
	if !kinds["unapproved_scope_reduction"] {
		t.Fatalf("full_parity + deferral wording without a ledger must block, got %v", kinds)
	}
	truth += `
## Deferral Ledger

| Capability | Item | Reason | Status | Approved By |
|------------|------|--------|--------|-------------|
| export | 导出功能推迟到下一版 | 用户确认先上核心清单 | approved | lai |
`
	kinds = findingKinds(CheckIntentGate(intentPkg(truth)))
	if kinds["unapproved_scope_reduction"] {
		t.Fatal("an approved ledger entry must clear the deferral block")
	}
	// An unapproved row does not disposition anything.
	truth2 := strings.Replace(truth, "| approved | lai |", "| pending | lai |", 1)
	if !findingKinds(CheckIntentGate(intentPkg(truth2)))["unapproved_scope_reduction"] {
		t.Fatal("a pending ledger row must not clear the deferral block")
	}
}

// ---- M6: scope vocabulary + derivation ----

func TestScopeModeVocabularyAndDerivation(t *testing.T) {
	for _, ok := range []string{"full_parity", "mvp", "patch", "brownfield_takeover", "research", "lake", "ocean", "focused", " Full_Parity "} {
		if !ValidScopeMode(ok) {
			t.Fatalf("%q must be a valid scope mode", ok)
		}
	}
	for _, bad := range []string{"", "banana", "explore", "full"} {
		if ValidScopeMode(bad) {
			t.Fatalf("%q must not be a valid scope mode", bad)
		}
	}
	cases := map[string]string{
		"完整复刻旧系统全部能力":  "full_parity",
		"先做个 mvp 上线":   "mvp",
		"修复登录页崩溃":      "patch",
		"整包底座重构，多模块一起": "ocean",
		"给设置页加一个开关":    "focused",
		"hotfix 线上问题":  "patch",
	}
	for utterance, want := range cases {
		if got := DeriveScopeMode(utterance); got != want {
			t.Fatalf("DeriveScopeMode(%q) = %q, want %q", utterance, got, want)
		}
	}
	// A garbage hand-written mode is an intent-gate finding.
	truth := strings.Replace(intentTruthBase, "scope_mode: focused", "scope_mode: banana", 1)
	codes := map[string]bool{}
	for _, f := range CheckIntentGate(intentPkg(truth)) {
		codes[f.ReasonCode] = true
	}
	if !codes[core.ReasonScopeModeInvalid] {
		t.Fatalf("garbage scope_mode must fail closed, codes=%v", codes)
	}
}

// ---- H3: integrity groups ----

func TestIntegrityClarityAndDeterminism(t *testing.T) {
	in := baseIntegrityInput()
	in.Pkg.Truth.Text = goodTruth + "\n发布日期待确认。\n"
	rep := CheckIntegrity(in, IntegrityModeBlock)
	kinds := findingKinds(rep.Findings)
	if !kinds["inline_marker"] {
		t.Fatalf("待确认 line must be a clarity finding, got %v", kinds)
	}
	if !kinds["dc_placeholder_chinese_daiqueren"] {
		t.Fatalf("待确认 line must be a determinism finding, got %v", kinds)
	}
	if rep.Verdict != VerdictBlocked {
		t.Fatal("placeholder findings must block")
	}

	// Hollow template alone: reported as warn, verdict stays pass.
	in2 := baseIntegrityInput()
	in2.Pkg.Truth.Text = goodTruth + "\n系统具有良好的扩展性。\n"
	rep2 := CheckIntegrity(in2, IntegrityModeBlock)
	if rep2.Verdict != VerdictPass {
		t.Fatalf("hollow-only package must pass, got %+v", rep2)
	}
	found := false
	for _, f := range rep2.Findings {
		if f.Kind == "dc_hollow_hollow_good" && f.Severity == "warn" {
			found = true
		}
	}
	if !found {
		t.Fatalf("hollow template must be reported as warn, got %+v", rep2.Findings)
	}

	// Unresolved section items are clarity findings.
	in3 := baseIntegrityInput()
	in3.Pkg.Truth.Text = goodTruth + "\n## 待澄清\n\n- 这个口径还没定\n"
	if !findingKinds(CheckIntegrity(in3, IntegrityModeBlock).Findings)["unresolved_section_item"] {
		t.Fatal("items under an unresolved heading must be clarity findings")
	}
}

func TestIntegrityExtensionScan(t *testing.T) {
	in := baseIntegrityInput()
	in.Pkg.Truth.Text = strings.Replace(goodTruth, "status: frozen",
		"status: frozen\nscope_mode: ocean", 1) + "\nAC-001 的验收先占位。\n"
	rep := CheckIntegrity(in, IntegrityModeBlock)
	kinds := findingKinds(rep.Findings)
	for _, want := range []string{"placeholder_ac_language", "alignment_surface_missing", "user_intent_missing"} {
		if !kinds[want] {
			t.Fatalf("missing extension finding %s in %v", want, kinds)
		}
	}
}

func TestIntegrityDomainFSMGroup(t *testing.T) {
	// A domain section appears but the authority surface is
	// incomplete: domain findings fire.
	in := baseIntegrityInput()
	in.Pkg.Truth.Text = goodTruth + "\n## Illegal Transitions\n\n- illegal: a -> b\n"
	rep := CheckIntegrity(in, IntegrityModeBlock)
	kinds := findingKinds(rep.Findings)
	for _, want := range []string{"domain_model_missing", "entity_precedence_missing", "domain_fsm_missing", "illegal_transition_slot_missing"} {
		if !kinds[want] {
			t.Fatalf("missing domain finding %s in %v", want, kinds)
		}
	}

	// A complete domain surface passes the group. Python binds
	// ACs only from AC-section tables whose description names a
	// state/entity/path verb — bold-line ACs have empty
	// descriptions there and can never bind (verified live), so
	// the fixture carries a binding AC table.
	in2 := baseIntegrityInput()
	in2.Pkg.Truth.Text = goodTruth + `
## Domain Model

| Entity | Key | Precedence |
|--------|-----|------------|
| Order | id | root |

## Domain FSM — Order

| To | From | Guard |
|----|------|-------|
| placed | draft | paid |

## Illegal Transitions

- illegal: placed -> draft

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | order draft becomes placed when paid |
`
	in2.Pkg.Tests.Text = goodTests + "\nillegal transitions covered by TC-001.\n"
	rep2 := CheckIntegrity(in2, IntegrityModeBlock)
	for _, f := range rep2.Findings {
		if strings.HasPrefix(f.Kind, "domain_") || f.Kind == "illegal_transition_slot_missing" ||
			f.Kind == "entity_precedence_missing" || f.Kind == "orphan_ac_without_fsm_path" {
			t.Fatalf("complete domain surface must not produce domain findings, got %+v", f)
		}
	}
}

func TestPackageValidationGroup(t *testing.T) {
	in := baseIntegrityInput()
	// Missing a required frontmatter key.
	in.Pkg.Truth.Text = strings.Replace(goodTruth, "language: en\n", "", 1)
	// Evidence authority outside the vocabulary.
	in.Pkg.Evidence.Text = "# Evidence\n\n| Surface | Authority |\n|---|---|\n| qa | vibes |\n"
	// Tasks without a checkbox ledger.
	in.Pkg.Tasks.Text = "---\nslug: demo\n---\nno tasks yet\n"
	// A test row without a command.
	in.Pkg.Tests.Text = "---\nslug: demo\n---\n| AC ID | TC ID | Layer | Command | Assertion |\n|-------|-------|-------|---------|-----------|\n| AC-001 | TC-001 | unit |  | holds |\n| AC-002 | TC-002 | unit | `go test ./...` | holds |\n"
	rep := CheckIntegrity(in, IntegrityModeBlock)
	details := map[string]bool{}
	for _, f := range rep.Findings {
		if f.Kind == "feature_package_validation" {
			details[f.Detail] = true
		}
	}
	for _, want := range []string{
		"frontmatter_missing_language", "evidence_invalid_authority:vibes",
		"checkbox_ledger_missing", "test_command_missing_or_blocked",
	} {
		if !details[want] {
			t.Fatalf("missing validation diagnostic %s in %v", want, details)
		}
	}
	if rep.Verdict != VerdictBlocked {
		t.Fatal("validation errors must block")
	}
}
