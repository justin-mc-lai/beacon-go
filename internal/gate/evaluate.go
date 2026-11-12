// Pure gate evaluators (P3a): facts in, verdict out. No store, no
// filesystem — the orchestrator gate service gathers facts and
// persists results, so the same facts always produce the same
// verdict and the same reason codes (re-evaluation is deterministic).
package gate

import (
	"strconv"
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

// CheckImplementWorkRecorded is the implement exit's work-evidence
// item (implement-work-gate AC-001): at the stage exit the bound work
// tree must be able to prove that the stage moved something.
const CheckImplementWorkRecorded = "implement_work_recorded"

// ImplementWorkFacts carry the exit-side work evidence of one
// implement stage (implement-work-gate AC-001..006). Every field is
// gathered by READ-ONLY git probes plus store reads: the worktree is
// never written, no commit is created and no run state is mutated, so
// re-evaluating the same stage yields the same conclusion (AC-006).
type ImplementWorkFacts struct {
	// GateEnabled is the implement_work_gate config (default on).
	// false = the item is not_applicable with implement_work_gate_off.
	GateEnabled bool
	// EmptyAllowed: the run explicitly declared the empty stage
	// (implement_empty_allowed lifecycle event) — not_applicable
	// with implement_empty_allowed (AC-003).
	EmptyAllowed bool
	// Unverified: the work tree could not be read at all (missing
	// directory, not a git repository, git failure) or the run has
	// no consumable baseline commit to compare HEAD against. Never a
	// failure and never a pass: not_applicable with
	// implement_work_unverified (AC-004).
	Unverified       bool
	UnverifiedDetail string
	// Dirty: the bound worktree carries uncommitted changes.
	Dirty      bool
	DirtyCount int
	// BaselineCommit is the run feature's consumable baseline
	// (truth_baseline_commit); HeadCommit is the live HEAD of the
	// bound worktree. HeadMoved means HEAD no longer equals the
	// recorded baseline.
	BaselineCommit string
	HeadCommit     string
	HeadMoved      bool
}

// shortHash trims a commit hash for item details (git's own calendar).
func shortHash(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	if s == "" {
		return "(none)"
	}
	return s
}

// implementWorkItem is the implement exit's work-evidence judgement:
// pass when the work tree is dirty or its HEAD moved past the recorded
// baseline, fail when the stage provably changed nothing, and
// not_applicable when the question cannot be answered (AC-001..004).
func implementWorkItem(w ImplementWorkFacts) CheckItem {
	switch {
	case !w.GateEnabled:
		return naItemCode(CheckImplementWorkRecorded, core.ReasonImplementWorkGateOff,
			"implement_work_gate is off: the exit's work evidence is not judged")
	case w.EmptyAllowed:
		return naItemCode(CheckImplementWorkRecorded, core.ReasonImplementEmptyAllowed,
			"the run declares an explicitly empty implement stage (implement_empty_allowed event): no work evidence required")
	case w.Unverified:
		return naItemCode(CheckImplementWorkRecorded, core.ReasonImplementWorkUnverified,
			"work tree unreadable ("+w.UnverifiedDetail+"): the exit's work evidence is not judged")
	case w.Dirty:
		return okItem(CheckImplementWorkRecorded,
			"work tree carries "+strconv.Itoa(w.DirtyCount)+" uncommitted change(s)")
	case w.HeadMoved:
		return okItem(CheckImplementWorkRecorded,
			"HEAD "+shortHash(w.HeadCommit)+" moved past baseline "+shortHash(w.BaselineCommit))
	}
	return failItem(CheckImplementWorkRecorded, core.ReasonImplementNoWork,
		"HEAD is still at baseline "+shortHash(w.BaselineCommit)+" and the work tree is clean: "+
			"the implement stage recorded no work (declare an intentional empty stage with --allow-empty)")
}

// ImplementFacts carry the shared admission checker's outcome (G-04)
// plus the exit's work evidence. The gate adds no admission judgement
// of its own: Items are the admission items mapped 1:1. The ONE
// addition is the work-evidence item (AC-001), fed by Work.
type ImplementFacts struct {
	HasRun         bool
	AllowImplement bool
	HeadlineCode   string
	Items          []CheckItem
	Work           ImplementWorkFacts
}

func EvaluateImplement(f ImplementFacts, evaluator string) Evaluation {
	if !f.HasRun {
		return finish(GateImplement, evaluator, []CheckItem{
			failItem("run_present", core.ReasonGateRunRequired,
				"implement admission is per-run; no goal run exists for this feature (pass --run or create one)"),
		})
	}
	checks := make([]CheckItem, 0, len(f.Items)+1)
	checks = append(checks, f.Items...)
	checks = append(checks, implementWorkItem(f.Work))
	ev := finish(GateImplement, evaluator, checks)
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
	// qa-surface-e2e: the two fail-closed doors. The gate still never
	// runs tests; it now refuses to pass without evidence that some
	// autotest record exists and that every declared surface has its
	// own kind of proof.
	AutotestRecorded bool
	AutotestDetail   string
	// Surfaces is the requirement truth's `surfaces` list (empty when
	// the package never declared one).
	Surfaces []string
	// SurfaceStrict is `surface_strict: true` on the frozen package.
	// false keeps the legacy waiver: the two new checks only warn.
	SurfaceStrict bool
	// LiveKindCounts counts live (hash-matching) artifacts per kind;
	// SurfaceEvidence maps a declared surface to the kinds registered.
	SurfaceEvidence map[string]map[string]bool
}

// qaSurfaceEvidenceItem is one declared surface's proof requirement.
func qaSurfaceEvidenceItem(f QAFacts) CheckItem {
	if !f.SurfaceStrict {
		return warnItem("qa_surface_evidence", core.ReasonLegacyWaiverSurface,
			"surface_strict is false (legacy package): declared-surface evidence is only warned about")
	}
	if len(f.Surfaces) == 0 {
		return failItem("qa_surface_evidence", "qa_surface_evidence_miss:surfaces_undeclared",
			"surface_strict is true but the truth declares no surfaces")
	}
	var missing []string
	for _, surface := range f.Surfaces {
		want := SurfaceEvidenceKind(surface)
		if want == "" {
			missing = append(missing, SurfaceEvidenceReason(surface))
			continue
		}
		kinds := f.SurfaceEvidence[surface]
		if !kinds[want] {
			missing = append(missing, SurfaceEvidenceReason(surface))
		}
	}
	if len(missing) == 0 {
		return okItem("qa_surface_evidence", "every declared surface has its evidence kind")
	}
	return failItem("qa_surface_evidence", missing[0],
		"missing surface evidence: "+strings.Join(missing, ", "))
}

func qaAutotestItem(f QAFacts) CheckItem {
	if !f.SurfaceStrict {
		return warnItem("qa_autotest_recorded", core.ReasonLegacyWaiverSurface,
			"surface_strict is false (legacy package): autotest evidence is only warned about")
	}
	if f.AutotestRecorded {
		detail := f.AutotestDetail
		if detail == "" {
			detail = "autotest evidence on record"
		}
		return okItem("qa_autotest_recorded", detail)
	}
	return failItem("qa_autotest_recorded", core.ReasonQAAutotestRecordedMissing,
		"no .beacon/autotest/<run-id>/ evidence and no registered autotest artifact for this feature's runs")
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
	checks = append(checks, qaAutotestItem(f), qaSurfaceEvidenceItem(f))
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
