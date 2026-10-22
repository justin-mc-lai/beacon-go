// Package gate is the Go counterpart of the frozen v2 design's beacon-gate
// crate: truth_freeze / design / implement / qa / release gate evaluations
// plus the Truth Integrity Gate (AC-101). Every evaluation is persisted to
// gate_results with reason_codes bound to the FSM (AC-013, AC-028, D-05).
//
// P3a split of labour: this package owns the JUDGEMENT — pure evaluators
// over fact structs (evaluate.go) and the package cross-reference checks
// (integrity.go). Fact gathering from the store and persistence
// (gate_results + lifecycle_events) live in the orchestrator's gate
// service, which is also where the one shared implement-admission
// checker lives — the implement gate wraps that checker, it never
// re-implements it.
package gate

// GateType enumerates the gate axes (truth.md Diagram Truth Layer,
// extended with the design gate and the integrity gate of AC-101).
type GateType string

const (
	GateTruthFreeze    GateType = "truth_freeze"
	GateDesign         GateType = "design"
	GateImplement      GateType = "implement"
	GateQA             GateType = "qa"
	GateRelease        GateType = "release"
	GateTruthIntegrity GateType = "truth_integrity"
)

// OrderedGateTypes lists every gate type the `gate status` surface
// reports, in pipeline order (integrity last: it stands apart from
// the four stage gates + release composition).
var OrderedGateTypes = []GateType{
	GateTruthFreeze, GateDesign, GateImplement, GateQA, GateRelease, GateTruthIntegrity,
}

// Known reports whether t is one of the evaluable gate types.
func (t GateType) Known() bool {
	switch t {
	case GateTruthFreeze, GateDesign, GateImplement, GateQA, GateRelease, GateTruthIntegrity:
		return true
	}
	return false
}

// Verdict is a gate outcome. Blocked must always carry reason codes (AC-028).
type Verdict string

const (
	VerdictPass    Verdict = "pass"
	VerdictFail    Verdict = "fail"
	VerdictBlocked Verdict = "blocked"
	// VerdictAudit is the Truth Integrity Gate's non-blocking outcome
	// for already-frozen historical packages (AC-101: audit_only does
	// not block). Findings still ride along in the evaluation.
	VerdictAudit Verdict = "audit"
)

// Result is one gate evaluation (persisted shape; store wiring lands in P0/P3).
type Result struct {
	GateType    GateType `json:"gate_type"`
	Verdict     Verdict  `json:"verdict"`
	ReasonCodes []string `json:"reason_codes,omitempty"`
}

// CheckItem is one per-item verdict inside an evaluation: every gate
// reports item by item so a block never hides which fact failed.
type CheckItem struct {
	Check      string `json:"check"`
	OK         bool   `json:"ok"`
	Applicable bool   `json:"applicable"`
	ReasonCode string `json:"reason_code,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

// Evaluation is one formal gate evaluation: the persisted verdict plus
// the full per-item trail and the evaluator identity (an evaluation
// without an evaluator is refused before it runs — see the
// orchestrator gate service).
type Evaluation struct {
	GateType     GateType    `json:"gate_type"`
	Verdict      Verdict     `json:"verdict"`
	ReasonCodes  []string    `json:"reason_codes,omitempty"`
	Checks       []CheckItem `json:"checks,omitempty"`
	Evaluator    string      `json:"evaluator"`
	FeatureID    string      `json:"feature_id"`
	FeatureSlug  string      `json:"feature_slug,omitempty"`
	RunID        string      `json:"run_id,omitempty"`
	GateResultID string      `json:"gate_result_id,omitempty"`
	EvaluatedAt  string      `json:"evaluated_at,omitempty"`
}

// finish assembles an Evaluation from its check items: verdict is
// pass iff every applicable item is OK; reason codes are the failing
// items' codes, de-duplicated, in check order (deterministic — the
// same facts always yield the same verdict and the same codes).
func finish(gt GateType, evaluator string, checks []CheckItem) Evaluation {
	ev := Evaluation{GateType: gt, Evaluator: evaluator, Checks: checks, Verdict: VerdictPass}
	seen := map[string]bool{}
	for _, c := range checks {
		if !c.Applicable || c.OK {
			continue
		}
		ev.Verdict = VerdictBlocked
		if c.ReasonCode != "" && !seen[c.ReasonCode] {
			seen[c.ReasonCode] = true
			ev.ReasonCodes = append(ev.ReasonCodes, c.ReasonCode)
		}
	}
	return ev
}

func okItem(check, detail string) CheckItem {
	return CheckItem{Check: check, OK: true, Applicable: true, Detail: detail}
}

func naItem(check, detail string) CheckItem {
	return CheckItem{Check: check, OK: true, Applicable: false, Detail: detail}
}

func failItem(check, code, detail string) CheckItem {
	return CheckItem{Check: check, Applicable: true, ReasonCode: code, Detail: detail}
}
