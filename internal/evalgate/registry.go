// Package evalgate is the deterministic, offline kernel regression
// suite frozen by docs/beacon/v2.0.0/features/kernel-eval-gate.
//
// The registry is data: every kernel behaviour the suite protects is a
// Case with a stable ID, a class, a fixture description and the exact
// expectation (exit code, reason code, conformance status, ids the
// failure must name). The assertions live in the package tests; this
// file owns the contract they are checked against.
//
// Three rules keep the suite honest:
//   - Validate refuses an unknown class, a duplicate ID or a case
//     without an expectation, naming the offending case.
//   - Check refuses ANY difference between expectation and observation
//     (including an unparseable observation) and prints
//     case=<ID> expected=... actual=..., never pass and never skip.
//   - Normalize rewrites the volatile parts of a report (run ids and
//     timestamps) so two runs of the same registry are byte-identical.
package evalgate

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Class is the behaviour group a case belongs to.
type Class string

const (
	ClassStageGate     Class = "stage_gate"
	ClassConformance   Class = "conformance"
	ClassRejection     Class = "rejection"
	ClassNoDegradation Class = "no_degradation"
)

// Classes is the frozen set of case classes, in report order.
var Classes = []Class{ClassStageGate, ClassConformance, ClassRejection, ClassNoDegradation}

func (c Class) known() bool {
	for _, k := range Classes {
		if c == k {
			return true
		}
	}
	return false
}

// Expect is one case's declared outcome. ReasonCode/Status empty means
// "none must be produced" (a passing case); Names lists the ids (AC
// ids, TC ids, or AC/TC pairs) the observed detail must mention.
type Expect struct {
	ExitCode       int      `json:"exit_code"`
	ReasonCode     string   `json:"reason_code,omitempty"`
	Status         string   `json:"status,omitempty"`
	Names          []string `json:"names,omitempty"`
	DetailContains []string `json:"detail_contains,omitempty"`
}

// Case is one registry entry.
type Case struct {
	ID      string  `json:"id"`
	Class   Class   `json:"class"`
	Fixture string  `json:"fixture"`
	Expect  *Expect `json:"expect"`
}

// Registry is the frozen 18-case directory from truth.md ("用例目录").
func Registry() []Case {
	exp := func(exit int, status, reason string, names ...string) *Expect {
		return &Expect{ExitCode: exit, Status: status, ReasonCode: reason, Names: names}
	}
	withDetail := func(e *Expect, parts ...string) *Expect {
		e.DetailContains = parts
		return e
	}
	return []Case{
		{ID: "stage_gate/implement_blocked_unfrozen_truth", Class: ClassStageGate,
			Fixture: "truth 未冻结（status: draft），其余事实齐备",
			Expect:  exp(1, "blocked", "stage_exit_truth_not_frozen")},
		{ID: "stage_gate/implement_pass_complete", Class: ClassStageGate,
			Fixture: "truth 冻结，门事实齐备",
			Expect:  exp(0, "pass", "")},
		{ID: "conformance/all_covered", Class: ClassConformance,
			Fixture: "每条 AC 有非软 TC，Evidence 文件存在",
			Expect:  withDetail(exp(0, "pass", ""), "2 AC covered by 2 executable TC")},
		{ID: "conformance/missing_tc", Class: ClassConformance,
			Fixture: "删除一条 AC 的 TC 行",
			Expect:  exp(1, "fail", "traceability_missing_tc", "AC-002")},
		{ID: "conformance/missing_evidence_file", Class: ClassConformance,
			Fixture: "Evidence 指向不存在的文件",
			Expect:  exp(1, "fail", "traceability_missing_evidence", "AC-001/TC-001")},
		{ID: "conformance/evidence_escape", Class: ClassConformance,
			Fixture: "Evidence 路径为 ../ 越界",
			Expect:  exp(1, "fail", "traceability_missing_evidence", "AC-001/TC-001")},
		{ID: "conformance/evidence_column_missing", Class: ClassConformance,
			Fixture: "覆盖矩阵没有 Evidence 列",
			Expect:  exp(1, "fail", "traceability_missing_evidence", "AC-001/TC-001")},
		{ID: "conformance/failclosed_tests_missing", Class: ClassConformance,
			Fixture: "包内没有 tests.md",
			Expect:  exp(1, "fail", "traceability_missing_tc", "AC-001")},
		{ID: "conformance/failclosed_matrix_unparseable", Class: ClassConformance,
			Fixture: "覆盖矩阵无 AC ID 表头",
			Expect:  exp(1, "fail", "traceability_missing_tc", "AC-001")},
		{ID: "conformance/failclosed_no_ac", Class: ClassConformance,
			Fixture: "truth 没有定义 AC",
			Expect:  withDetail(exp(1, "fail", "traceability_missing_tc"), "defines no AC")},
		{ID: "conformance/not_required_compat", Class: ClassConformance,
			Fixture: "未声明 traceability_required 的历史冻结包",
			Expect:  exp(0, "not_required", "")},
		{ID: "rejection/unknown_gate_type", Class: ClassRejection,
			Fixture: "gate evaluate 传入未知 gate type",
			Expect:  exp(1, "", "gate_type_unknown")},
		{ID: "rejection/missing_evaluator", Class: ClassRejection,
			Fixture: "gate evaluate 缺 --evaluator",
			Expect:  exp(1, "", "gate_evaluator_required")},
		{ID: "rejection/unknown_feature", Class: ClassRejection,
			Fixture: "查询不存在的 feature",
			Expect:  exp(1, "", "feature_not_found")},
		{ID: "rejection/unknown_run_conformance", Class: ClassRejection,
			Fixture: "conformance 查询不存在的 run",
			Expect:  exp(1, "", "product_goal_run_not_found")},
		{ID: "no_degradation/expectation_mismatch_detected", Class: ClassNoDegradation,
			Fixture: "注入与真实结果不符的期望",
			Expect:  exp(1, "fail", "")},
		{ID: "no_degradation/deterministic_twice", Class: ClassNoDegradation,
			Fixture: "同一注册表连续跑两次",
			Expect:  exp(0, "pass", "")},
		{ID: "no_degradation/offline_loopback_blocked", Class: ClassNoDegradation,
			Fixture: "出网代理指向 127.0.0.1:1",
			Expect:  exp(0, "pass", "")},
	}
}

// Validate refuses a registry the suite cannot trust: an unknown class,
// a duplicate ID, a missing expectation, or an ID that does not carry
// its class prefix. Every error names the offending case.
func Validate(cases []Case) error {
	seen := map[string]bool{}
	for _, c := range cases {
		if strings.TrimSpace(c.ID) == "" {
			return fmt.Errorf("case=<empty>: a case must carry a stable ID")
		}
		if !c.Class.known() {
			return fmt.Errorf("case=%s: unknown class %q (want one of %v)", c.ID, c.Class, Classes)
		}
		if !strings.HasPrefix(c.ID, string(c.Class)+"/") {
			return fmt.Errorf("case=%s: id must start with %q", c.ID, string(c.Class)+"/")
		}
		if seen[c.ID] {
			return fmt.Errorf("case=%s: duplicate ID", c.ID)
		}
		seen[c.ID] = true
		if c.Expect == nil {
			return fmt.Errorf("case=%s: missing expectation (exit code / reason code / status)", c.ID)
		}
	}
	for _, k := range Classes {
		found := false
		for _, c := range cases {
			if c.Class == k {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("case=<registry>: class %q has no named case", k)
		}
	}
	return nil
}

// Observation is what one case actually produced.
type Observation struct {
	ExitCode   int      `json:"exit_code"`
	ReasonCode string   `json:"reason_code,omitempty"`
	Status     string   `json:"status,omitempty"`
	Names      []string `json:"names,omitempty"`
	Detail     string   `json:"detail,omitempty"`
	// Unparseable marks an observation the case could not read as a
	// verdict at all. It always fails, never skips.
	Unparseable bool `json:"unparseable,omitempty"`
}

// HasName reports whether the observation names one identifier.
func (o Observation) HasName(id string) bool {
	for _, n := range o.Names {
		if n == id {
			return true
		}
	}
	return false
}

// Check compares one observation with its case. Any difference — wrong
// exit code, wrong reason code, wrong status, a required id missing, or
// an unparseable observation — is an error carrying
// case=<ID> expected=... actual=...; the suite turns this error into a
// non-zero exit, never into a pass or a skip.
func Check(c Case, o Observation) error {
	if c.Expect == nil {
		return fmt.Errorf("case=%s: missing expectation", c.ID)
	}
	want := describeExpect(*c.Expect)
	got := describeObservation(o)
	if o.Unparseable {
		return fmt.Errorf("case=%s expected=%s actual=unparseable-output", c.ID, want)
	}
	var problems []string
	if o.ExitCode != c.Expect.ExitCode {
		problems = append(problems, fmt.Sprintf("exit_code want %d got %d", c.Expect.ExitCode, o.ExitCode))
	}
	if c.Expect.ReasonCode != o.ReasonCode {
		problems = append(problems, fmt.Sprintf("reason_code want %q got %q", c.Expect.ReasonCode, o.ReasonCode))
	}
	if c.Expect.Status != o.Status {
		problems = append(problems, fmt.Sprintf("status want %q got %q", c.Expect.Status, o.Status))
	}
	for _, name := range c.Expect.Names {
		if !o.HasName(name) {
			problems = append(problems, fmt.Sprintf("detail must name %q", name))
		}
	}
	for _, part := range c.Expect.DetailContains {
		if !strings.Contains(o.Detail, part) {
			problems = append(problems, fmt.Sprintf("detail must contain %q", part))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("case=%s expected=%s actual=%s (%s)", c.ID, want, got, strings.Join(problems, "; "))
	}
	return nil
}

func describeExpect(e Expect) string {
	parts := []string{fmt.Sprintf("exit=%d", e.ExitCode)}
	if e.Status != "" {
		parts = append(parts, "status="+e.Status)
	}
	if e.ReasonCode != "" {
		parts = append(parts, "reason="+e.ReasonCode)
	}
	if len(e.Names) > 0 {
		parts = append(parts, "names="+strings.Join(e.Names, "|"))
	}
	if len(e.DetailContains) > 0 {
		parts = append(parts, "detail~"+strings.Join(e.DetailContains, "&"))
	}
	return strings.Join(parts, ",")
}

func describeObservation(o Observation) string {
	parts := []string{fmt.Sprintf("exit=%d", o.ExitCode)}
	if o.Status != "" {
		parts = append(parts, "status="+o.Status)
	}
	if o.ReasonCode != "" {
		parts = append(parts, "reason="+o.ReasonCode)
	}
	if len(o.Names) > 0 {
		parts = append(parts, "names="+strings.Join(o.Names, "|"))
	}
	return strings.Join(parts, ",")
}

// ReportEntry is one case's expected/actual pair plus the wall clock it
// was observed at (volatile, removed by Normalize).
type ReportEntry struct {
	CaseID     string      `json:"case_id"`
	Class      Class       `json:"class"`
	Expected   Expect      `json:"expected"`
	Actual     Observation `json:"actual"`
	ObservedAt string      `json:"observed_at"`
}

// Report is the suite's machine-readable result, in registry order.
type Report []ReportEntry

var (
	// A run id is store.newID("run_") = "run_" + hex digest; the
	// length bound keeps words like run_not_found out of the rewrite.
	volatileRunID = regexp.MustCompile(`run_[0-9a-f]{16,}`)
	volatileTime  = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})`)
	volatilePath  = regexp.MustCompile(`/[^\s,;()"']+`)
)

// Normalize returns the canonical bytes of a report: entries sorted by
// case ID and every run id / timestamp replaced by a stable token. Two
// runs of the same registry must normalize to identical bytes.
func Normalize(r Report) []byte {
	sorted := make(Report, len(r))
	copy(sorted, r)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].CaseID < sorted[j].CaseID })
	for i := range sorted {
		sorted[i].ObservedAt = "<observed_at>"
		sorted[i].Actual.Detail = normalizeText(sorted[i].Actual.Detail)
		for j := range sorted[i].Actual.Names {
			sorted[i].Actual.Names[j] = normalizeText(sorted[i].Actual.Names[j])
		}
	}
	raw, err := json.Marshal(sorted)
	if err != nil {
		return []byte("normalize-error: " + err.Error())
	}
	return raw
}

func normalizeText(s string) string {
	s = volatileRunID.ReplaceAllString(s, "<run-id>")
	s = volatilePath.ReplaceAllString(s, "<path>")
	return volatileTime.ReplaceAllString(s, "<timestamp>")
}
