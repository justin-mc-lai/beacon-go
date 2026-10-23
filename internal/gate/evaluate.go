// Pure gate evaluators (P3a): facts in, verdict out. No store, no
// filesystem — the orchestrator gate service gathers facts and
// persists results, so the same facts always produce the same
// verdict and the same reason codes (re-evaluation is deterministic).
package gate

import (
	"strings"

	"beacon-go/internal/core"
)

// ---- truth_freeze ----

// TruthFreezeFacts are the store/file facts of the truth_freeze gate.
// The frozen marker's PRIMARY source is the truth document itself
// (package-authoritative model: frontmatter `status: frozen`); the
// lifecycle Freeze snapshot is only a corroborating signal, recorded
// as a non-applicable info item — after freezing, the lifecycle
// legitimately moves on to Implement/QA, so it can never be the
// authority for "is it frozen".
type TruthFreezeFacts struct {
	TruthPresent    bool
	TruthNonEmpty   bool
	TestsPresent    bool
	TestsNonEmpty   bool
	FrozenMarker    bool // truth.md frontmatter status == frozen
	LifecycleFrozen bool // corroboration only: lifecycle at Freeze/Completed
	PendingChange   bool // a change was requested and never refrozen
}

func EvaluateTruthFreeze(f TruthFreezeFacts, evaluator string) Evaluation {
	var checks []CheckItem
	if f.TruthPresent {
		checks = append(checks, okItem("truth_file_present", "truth.md exists"))
	} else {
		checks = append(checks, failItem("truth_file_present", core.ReasonStageExitTruthMissing, "truth.md missing from the feature package"))
	}
	if f.TruthNonEmpty {
		checks = append(checks, okItem("truth_file_nonempty", "truth.md is non-empty"))
	} else {
		checks = append(checks, failItem("truth_file_nonempty", core.ReasonStageExitTruthMissing, "truth.md is empty (scaffold not filled)"))
	}
	if f.TestsPresent {
		checks = append(checks, okItem("tests_file_present", "tests.md exists"))
	} else {
		checks = append(checks, failItem("tests_file_present", core.ReasonStageExitTruthMissing, "tests.md missing from the feature package"))
	}
	if f.TestsNonEmpty {
		checks = append(checks, okItem("tests_file_nonempty", "tests.md is non-empty"))
	} else {
		checks = append(checks, failItem("tests_file_nonempty", core.ReasonStageExitTruthMissing, "tests.md is empty"))
	}
	if f.FrozenMarker {
		checks = append(checks, okItem("frozen_marker", "truth.md frontmatter status=frozen (primary source)"))
	} else {
		checks = append(checks, failItem("frozen_marker", core.ReasonFreezeAckRequired,
			"truth.md frontmatter status is not frozen (fill != self-sign; freeze first)"))
	}
	lcDetail := "lifecycle corroboration: not at Freeze/Completed"
	if f.LifecycleFrozen {
		lcDetail = "lifecycle corroboration: Freeze/Completed snapshot present"
	}
	checks = append(checks, naItem("lifecycle_corroboration", lcDetail))
	if f.PendingChange {
		checks = append(checks, failItem("no_pending_change", core.ReasonTruthChangePending,
			"a truth change was requested but never refrozen"))
	} else {
		checks = append(checks, okItem("no_pending_change", "no unprocessed truth change in the event trail"))
	}
	return finish(GateTruthFreeze, evaluator, checks)
}

// ---- design ----

// DesignFacts are the design-gate facts for one run (the binding is a
// per-run record, P2c). Version match means: the version recorded in
// the run snapshot's binding equals the version in the latest
// design_primitive_bound event — snapshot and audit trail must agree,
// and a binding without a version can never be matched.
type DesignFacts struct {
	HasRun         bool
	Required       bool
	Bound          bool
	BindingSet     string
	BindingVersion string
	EventVersion   string
	HasBoundEvent  bool
}

func EvaluateDesign(f DesignFacts, evaluator string) Evaluation {
	if !f.HasRun {
		return finish(GateDesign, evaluator, []CheckItem{
			failItem("run_present", core.ReasonGateRunRequired,
				"the design binding is a per-run record; no goal run exists for this feature (pass --run or create one)"),
		})
	}
	if !f.Required {
		return finish(GateDesign, evaluator, []CheckItem{
			naItem("design_required", "run requires no design (no UX signal); design gate not applicable"),
		})
	}
	checks := []CheckItem{okItem("design_required", "run requires design (UX)")}
	if f.Bound {
		checks = append(checks, okItem("binding_present", "design primitives bound: "+f.BindingSet))
	} else {
		checks = append(checks, failItem("binding_present", core.ReasonDesignPrimitiveNotBound,
			"UX run has no design primitive binding; bind before implementing"))
		return finish(GateDesign, evaluator, checks)
	}
	switch {
	case f.BindingVersion == "":
		checks = append(checks, failItem("binding_version_match", core.ReasonDesignBindingVersionMiss,
			"binding carries no version; an unversioned binding cannot be matched"))
	case f.HasBoundEvent && f.EventVersion != f.BindingVersion:
		checks = append(checks, failItem("binding_version_match", core.ReasonDesignBindingVersionMism,
			"snapshot binding version "+f.BindingVersion+" != design_primitive_bound event version "+f.EventVersion))
	default:
		checks = append(checks, okItem("binding_version_match", "binding version "+f.BindingVersion+" consistent with the binding event"))
	}
	return finish(GateDesign, evaluator, checks)
}

// ---- implement ----

// ImplementFacts carry the shared admission checker's outcome (G-04).
// The gate adds NO judgement of its own: Items are the admission
// items mapped 1:1, and the verdict is pass iff admission allowed.
type ImplementFacts struct {
	HasRun         bool
	AllowImplement bool
	HeadlineCode   string
	Items          []CheckItem
}

func EvaluateImplement(f ImplementFacts, evaluator string) Evaluation {
	if !f.HasRun {
		return finish(GateImplement, evaluator, []CheckItem{
			failItem("run_present", core.ReasonGateRunRequired,
				"implement admission is per-run; no goal run exists for this feature (pass --run or create one)"),
		})
	}
	ev := finish(GateImplement, evaluator, f.Items)
	// Consistency guard: the mapped items and the admission headline
	// must agree; if they ever drift, the headline wins (fail closed).
	if ev.Verdict == VerdictPass && !f.AllowImplement {
		ev.Verdict = VerdictBlocked
		if len(ev.ReasonCodes) == 0 && f.HeadlineCode != "" {
			ev.ReasonCodes = []string{f.HeadlineCode}
		}
	}
	return ev
}

// ---- qa ----

// StaleRef is one QA artifact pointer whose blob no longer matches
// (file missing / unreadable / hash mismatch — the spine staleness
// vocabulary of AC-010/AC-019).
type StaleRef struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Reason string `json:"reason"` // file_missing | file_unreadable | hash_mismatch
}

// QAFacts are the QA-evidence facts. The qa gate judges ONLY whether
// the evidence is complete and its references are live: QA-kind
// artifact pointers exist, a qa stage execution is on record, and
// every QA artifact's file exists with a matching hash. It never runs
// tests itself.
type QAFacts struct {
	QAArtifactCount int
	QAStageExecuted bool
	StaleArtifacts  []StaleRef
}

func EvaluateQA(f QAFacts, evaluator string) Evaluation {
	var checks []CheckItem
	if f.QAArtifactCount > 0 {
		checks = append(checks, okItem("qa_artifacts_present", "QA artifact pointers recorded"))
	} else {
		checks = append(checks, failItem("qa_artifacts_present", core.ReasonStageExitQAEvidenceMiss,
			"no QA-kind artifact pointers (qa*/scorecard/test_result) across the feature's runs"))
	}
	if f.QAStageExecuted {
		checks = append(checks, okItem("qa_stage_executed", "a qa stage execution is on record (stage_executed)"))
	} else {
		checks = append(checks, failItem("qa_stage_executed", core.ReasonQAStageNotExecuted,
			"no stage_executed evidence for the qa stage"))
	}
	if f.QAArtifactCount == 0 {
		checks = append(checks, naItem("qa_artifact_refs_valid", "no QA artifacts to validate"))
	} else if len(f.StaleArtifacts) == 0 {
		checks = append(checks, okItem("qa_artifact_refs_valid", "every QA artifact file exists and its hash matches"))
	} else {
		first := f.StaleArtifacts[0]
		checks = append(checks, failItem("qa_artifact_refs_valid", core.ReasonQAArtifactStale,
			"stale QA artifact "+first.Path+" ("+first.Reason+")"))
	}
	return finish(GateQA, evaluator, checks)
}

// ---- release ----

// GateFact is the latest recorded verdict of one prerequisite gate.
type GateFact struct {
	Present     bool
	Verdict     string
	ReasonCodes []string
}

// ReleaseFacts compose the release gate: the other four gates' latest
// verdicts must all be pass, and a human confirmation must be on
// record (the P2a口径: run snapshot flag or the release_confirmed
// event). The integrity gate is NOT a release prerequisite — AC-101
// binds it to freeze/refreeze, not to release.
type ReleaseFacts struct {
	TruthFreeze      GateFact
	Design           GateFact
	Implement        GateFact
	QA               GateFact
	DesignApplicable bool
	HumanConfirmed   bool
	HumanConfirmedBy string
	// MaterialsStale names the package surfaces (truth/tests/
	// tasks) whose frontmatter reads materials_status: stale
	// (R2a, audit M7 — the stale half of Python's
	// evaluate_release_truth_materials_gate, fail-closed).
	MaterialsStale []string
}

func releasePrereq(name string, g GateFact) CheckItem {
	if !g.Present {
		return failItem("gate_"+name+"_passed", core.ReasonGateNotEvaluated,
			"gate "+name+" has never been evaluated; run `beacon gate evaluate "+name+"` first")
	}
	if g.Verdict != string(VerdictPass) {
		return failItem("gate_"+name+"_passed", core.ReasonGatePrereqNotPassed,
			"latest "+name+" verdict is "+g.Verdict)
	}
	return okItem("gate_"+name+"_passed", "latest "+name+" verdict is pass")
}

func EvaluateRelease(f ReleaseFacts, evaluator string) Evaluation {
	checks := []CheckItem{releasePrereq("truth_freeze", f.TruthFreeze)}
	if f.DesignApplicable {
		checks = append(checks, releasePrereq("design", f.Design))
	} else {
		checks = append(checks, naItem("gate_design_passed", "no run of this feature requires design"))
	}
	checks = append(checks, releasePrereq("implement", f.Implement), releasePrereq("qa", f.QA))
	if len(f.MaterialsStale) > 0 {
		checks = append(checks, failItem("truth_materials_fresh", core.ReasonMaterialsStale,
			"package materials marked stale after a truth change ("+
				strings.Join(f.MaterialsStale, ", ")+"); refreeze to refresh them before release"))
	} else {
		checks = append(checks, okItem("truth_materials_fresh", "no package material is marked stale"))
	}
	if f.HumanConfirmed {
		detail := "human release confirmation on record"
		if f.HumanConfirmedBy != "" {
			detail += " (by " + f.HumanConfirmedBy + ")"
		}
		checks = append(checks, okItem("human_confirmation", detail))
	} else {
		checks = append(checks, failItem("human_confirmation", core.ReasonReleaseRequiresHuman,
			"no human release confirmation on record for the feature's latest run"))
	}
	return finish(GateRelease, evaluator, checks)
}
