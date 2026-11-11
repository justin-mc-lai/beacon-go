// Stage exit gates (P2a, G-03): the minimum real read-side judgement of
// goal_stage_exit_gates.evaluate_stage_exit, over store data only.
//
// EvaluateStageExit is a pure function: the engine builds an ExitInput
// from the lifecycle snapshot + gate_results + artifact pointers, and
// the function decides. Missing evidence means blocked — never a
// default pass (the release stage is ALWAYS blocked for tick: leaving
// release requires human confirmation via Engine.Complete).
//
// P3a linkage: since formal gate evaluations now exist (gate_results
// written by `beacon gate evaluate`), the qa and release exits are
// FORMAL-FIRST — a recorded verdict for the stage's gate decides,
// and the old derived judgement runs only when no formal verdict
// exists (the verdict then carries Derived=true). truth / design /
// implement keep their derived judgements (their formal records are
// surfaced via FormalVerdict) — see the p3a report for the per-stage
// rationale.
package orchestrator

import (
	"encoding/json"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// Exit-gate reason codes: aliases of the unified core vocabulary
// (core/reasons.go, G-21) — one spelling, owned by core.
const (
	ReasonStageExitOK            = core.ReasonStageExitOK
	ReasonStageExitPlanIncompl   = core.ReasonStageExitPlanIncomplete
	ReasonStageExitTruthNotFroz  = core.ReasonStageExitTruthNotFrozen
	ReasonStageExitTruthMissing  = core.ReasonStageExitTruthMissing
	ReasonStageExitDesignIncompl = core.ReasonStageExitDesignIncompl
	ReasonStageExitImplementBlk  = core.ReasonStageExitImplementBlock
	ReasonStageExitQAEvidence    = core.ReasonStageExitQAEvidenceMiss
	ReasonFreezeAckRequired      = core.ReasonFreezeAckRequired
	ReasonTruthScaffoldNotFilled = "truth_scaffold_not_filled"
)

// GateFact is the latest verdict of one gate type.
type GateFact struct {
	Verdict     string
	ReasonCodes []string
	EvaluatedAt string
	Evaluator   string
	// Source names the attribution of the fact. Run-scoped facts are
	// "run_stage_exit" / "run_formal_gate"; a feature-level row read
	// as a fallback is "feature_gate" and is never treated as this
	// run's own stage-exit evidence.
	Source string
}

// ExitInput is everything an exit decision may consult (DB-first).
type ExitInput struct {
	Stage string
	// HasFeature mirrors the source's "no feature context -> skip the
	// truth package checks" behaviour.
	HasFeature bool
	// TruthFrozen comes from the ONE shared determination
	// (Engine.FrozenFact, frozen_fact.go): pending change -> not
	// frozen; the truth document's frontmatter decides when it
	// speaks; a formal truth_freeze pass corroborates only when
	// the document is silent; the lifecycle snapshot never decides.
	TruthFrozen bool
	// TruthFrozenFormal: the deciding source was the formal
	// truth_freeze gate verdict (FrozenSource formal_gate).
	TruthFrozenFormal bool
	// TruthChangePending: an unrefrozen change request is on the
	// feature's trail (the shared determination's step 1).
	TruthChangePending bool
	// TruthBaselineUncommitted (S1, AC-139): the package is
	// frozen but its body is not a committed baseline on the
	// canonical branch. Only ever set for git projects — non-git
	// projects keep the P4b frontmatter-only caliber.
	TruthBaselineUncommitted bool
	// FrozenSource names the deciding source of the determination
	// (change_pending | frontmatter | formal_gate | none).
	FrozenSource string
	// TruthPackagePresent / TruthDraftPresent distinguish the source's
	// missing-package vs draft-not-frozen branches.
	TruthPackagePresent bool
	TruthDraftPresent   bool
	// Lifecycle snapshot of the feature ("" when none recorded).
	LifecycleStage  string
	LifecycleStatus string
	// LatestGates maps gate_type -> the latest fact THIS RUN produced
	// (stage exits + formal evaluations bound to the run). Another
	// run's verdict on the same feature must not appear here (review
	// B3).
	LatestGates map[string]GateFact
	// FeatureGates maps gate_type -> the feature-level latest fact.
	// Only consulted where the gate is genuinely feature-scoped (the
	// plan veto gates, and the formal QA/release verdicts); every use
	// is labelled with Source="feature_gate" on the verdict.
	FeatureGates map[string]GateFact
	// ArtifactCount is the number of artifact pointers across the
	// feature's runs (QA evidence presence signal).
	ArtifactCount int
	// DesignEvidence: lifecycle at Prototype/Completed or design gate pass.
	DesignEvidence bool
	// DesignEvidenceFormal: a formal design gate pass is (part of)
	// the evidence for DesignEvidence.
	DesignEvidenceFormal bool
}

// ExitVerdict is one exit decision. Derived marks that the decision
// came from the derived judgement rather than a formal gate_results
// verdict (P3a); FormalVerdict carries the deciding (or, for the
// still-derived stages, the latest recorded) formal verdict.
type ExitVerdict struct {
	Allowed bool `json:"allowed"`
	// Evaluated marks that a real judgement was established from the
	// stage's facts. false means no gate covers the stage (or none of
	// its facts could be read): the run must park with
	// stage_gate_not_evaluated instead of advancing on a guess.
	Evaluated     bool   `json:"evaluated"`
	ReasonCode    string `json:"reason_code"`
	ResumeAction  string `json:"resume_action"`
	FillSkill     string `json:"fill_skill,omitempty"`
	Derived       bool   `json:"derived"`
	FormalVerdict string `json:"formal_verdict,omitempty"`
	// FormalSource labels which fact decided (or informed) the
	// formal verdict: run-scoped evidence or the feature-level row.
	FormalSource string `json:"formal_source,omitempty"`
	// FrozenSource (truth exit only): which source of the shared
	// FrozenFact determination decided frozenness.
	FrozenSource string `json:"frozen_source,omitempty"`
}

func firstCode(codes []string, fallback string) string {
	for _, c := range codes {
		if c != "" {
			return c
		}
	}
	return fallback
}

func blockedVerdict(reason, resume, fill string) ExitVerdict {
	// Default is Derived=true; callers that were decided by a formal
	// gate verdict override it (and set FormalVerdict).
	return ExitVerdict{Allowed: false, Evaluated: true, ReasonCode: reason, ResumeAction: resume, FillSkill: fill, Derived: true}
}

func passVerdict() ExitVerdict {
	return ExitVerdict{Allowed: true, Evaluated: true, ReasonCode: ReasonStageExitOK, ResumeAction: core.ResumeNone, Derived: true}
}

// runVerdictOf returns the latest verdict recorded BY THIS RUN for a
// gate type ("" when this run never evaluated it). Design and implement
// exits use only this: a sibling run's pass is not this run's evidence.
func runVerdictOf(in ExitInput, gateType string) string {
	if g, ok := in.LatestGates[gateType]; ok {
		return g.Verdict
	}
	return ""
}

// gateFactOf resolves the fact a feature-scoped decision consults: the
// run's own fact first, then the feature-level latest (explicitly
// labelled feature_gate so the caller can surface the source).
func gateFactOf(in ExitInput, gateType string) (GateFact, bool) {
	if g, ok := in.LatestGates[gateType]; ok {
		return g, true
	}
	if g, ok := in.FeatureGates[gateType]; ok {
		return g, true
	}
	return GateFact{}, false
}

// setFormal records the fact that informed a verdict, with its
// attribution (run-scoped or feature-level).
func setFormal(v *ExitVerdict, in ExitInput, gateType string) {
	if g, ok := gateFactOf(in, gateType); ok {
		v.FormalVerdict, v.FormalSource = g.Verdict, g.Source
	}
}

// setRunFormal is setFormal restricted to facts THIS RUN recorded.
func setRunFormal(v *ExitVerdict, in ExitInput, gateType string) {
	if g, ok := in.LatestGates[gateType]; ok {
		v.FormalVerdict, v.FormalSource = g.Verdict, g.Source
	}
}

// EvaluateStageExit decides whether the current product stage may be
// left. Ported branch-by-branch from evaluate_stage_exit; where the
// source consulted payload/file state, the store-derived ExitInput
// fields above are the documented substitutes (see p2a report).
func EvaluateStageExit(in ExitInput) ExitVerdict {
	switch in.Stage {
	case StagePlan:
		// plan_to_truth_gate / truth_readiness may veto leaving plan.
		for _, gt := range []string{"plan_to_truth", "truth_readiness"} {
			if g, ok := gateFactOf(in, gt); ok && g.Verdict == "blocked" {
				v := blockedVerdict(firstCode(g.ReasonCodes, ReasonStageExitPlanIncompl),
					core.ResumeLoadNextSkill, "beacon-gen-truth")
				v.Derived, v.FormalVerdict = false, g.Verdict
				v.FormalSource = g.Source
				return v
			}
		}
		return passVerdict()
	case StageTruth:
		if !in.HasFeature {
			return passVerdict() // source: skip when no feature context
		}
		if in.TruthChangePending {
			// Shared determination step 1: an unrefrozen change
			// means the frozen text is under authorised edit —
			// the truth stage cannot be left until the refreeze.
			v := blockedVerdict(core.ReasonTruthChangePending, core.ResumeAwaitFreezeAck, "beacon-gen-truth")
			setFormal(&v, in, "truth_freeze")
			v.FrozenSource = FrozenSourceChangePending
			return v
		}
		if in.TruthFrozen && in.TruthBaselineUncommitted {
			// S1 (AC-139): frozen is not enough — on a git project
			// the frozen body must be a COMMITTED baseline on the
			// canonical branch before the truth stage may be left.
			// Non-git projects never set this field (P4b caliber:
			// frontmatter is the authority there).
			v := blockedVerdict(core.ReasonCanonicalRefreezeNotCommitted, core.ResumeAwaitFreezeAck, "beacon-gen-truth")
			setFormal(&v, in, "truth_freeze")
			v.FrozenSource = in.FrozenSource
			return v
		}
		if in.TruthFrozen {
			v := passVerdict()
			// The pass is formal-decided only when the deciding
			// source was a recorded truth_freeze gate verdict
			// (formal_gate); a frontmatter-decided pass is the
			// document read live — still a derivation, marked so.
			v.Derived = !in.TruthFrozenFormal
			setFormal(&v, in, "truth_freeze")
			v.FrozenSource = in.FrozenSource
			return v
		}
		if !in.TruthPackagePresent {
			v := blockedVerdict(ReasonStageExitTruthMissing, core.ResumeLoadNextSkill, "beacon-gen-truth")
			v.FrozenSource = in.FrozenSource
			return v
		}
		if in.TruthDraftPresent {
			v := blockedVerdict(ReasonStageExitTruthNotFroz, core.ResumeAwaitFreezeAck, "beacon-gen-truth")
			v.FrozenSource = in.FrozenSource
			return v
		}
		v := blockedVerdict(ReasonFreezeAckRequired, core.ResumeAwaitFreezeAck, "beacon-gen-truth")
		v.FrozenSource = in.FrozenSource
		return v
	case StageDesign:
		if in.DesignEvidence {
			v := passVerdict()
			v.Derived = !in.DesignEvidenceFormal
			setRunFormal(&v, in, "design")
			return v
		}
		v := blockedVerdict(ReasonStageExitDesignIncompl, core.ResumeLoadNextSkill, "beacon-design")
		setRunFormal(&v, in, "design")
		return v
	case StageImplement:
		if g, ok := in.LatestGates["implement_admission"]; ok && g.Verdict == "blocked" {
			v := blockedVerdict(firstCode(g.ReasonCodes, ReasonStageExitImplementBlk),
				core.ResumeLoadNextSkill, "beacon-implement")
			v.Derived, v.FormalVerdict = false, g.Verdict
			return v
		}
		if in.LifecycleStage == string(core.StageImplement) && in.LifecycleStatus == string(core.StatusBlocked) {
			return blockedVerdict(ReasonStageExitImplementBlk, core.ResumeLoadNextSkill, "beacon-implement")
		}
		return passVerdict()
	case StageQA:
		// FORMAL-FIRST (P3a): a recorded qa verdict decides — pass
		// opens the exit, blocked/fail closes it with the formal
		// reason codes, regardless of raw artifact counts. Only
		// when no qa evaluation exists does the derived judgement
		// (artifact presence) run, marked Derived.
		if g, ok := gateFactOf(in, "qa"); ok {
			if g.Verdict == "pass" {
				v := passVerdict()
				v.Derived, v.FormalVerdict = false, g.Verdict
				v.FormalSource = g.Source
				return v
			}
			v := blockedVerdict(firstCode(g.ReasonCodes, ReasonStageExitQAEvidence),
				core.ResumeLoadNextSkill, "beacon-qa")
			v.Derived, v.FormalVerdict = false, g.Verdict
			v.FormalSource = g.Source
			return v
		}
		if in.ArtifactCount > 0 {
			return passVerdict()
		}
		return blockedVerdict(ReasonStageExitQAEvidence, core.ResumeLoadNextSkill, "beacon-qa")
	case StageRelease:
		// FORMAL-FIRST (P3a) with the human-completion reservation:
		// a formal release verdict is read and surfaced, but tick
		// NEVER exits release — leaving release IS completion, and
		// completion is Engine.Complete's human act (source-exact).
		// A formal pass therefore reports release_awaiting_complete
		// instead of the derived human-confirmation block.
		if g, ok := gateFactOf(in, "release"); ok {
			if g.Verdict == "pass" {
				v := blockedVerdict(core.ReasonReleaseAwaitingComplete, core.ResumeAwaitHumanRelease, "beacon-release")
				v.Derived, v.FormalVerdict = false, g.Verdict
				v.FormalSource = g.Source
				return v
			}
			v := blockedVerdict(firstCode(g.ReasonCodes, core.ReasonReleaseRequiresHuman),
				core.ResumeAwaitHumanRelease, "beacon-release")
			v.Derived, v.FormalVerdict = false, g.Verdict
			v.FormalSource = g.Source
			return v
		}
		// Source-exact: release never auto-exits on tick.
		return blockedVerdict(core.ReasonReleaseRequiresHuman, core.ResumeAwaitHumanRelease, "beacon-release")
	default:
		// Unknown stage: no gate covers it, so there is nothing to
		// evaluate. Fail closed and name the real defect — the run may
		// not advance on a stage the kernel cannot judge.
		return ExitVerdict{Allowed: false, Evaluated: false, ReasonCode: core.ReasonStageGateNotEvaluated,
			ResumeAction: core.ResumeNone, Derived: true}
	}
}

// buildExitInput reads the store facts for one run's current stage.
func (e *Engine) buildExitInput(run store.Run, m *runMeta) (ExitInput, error) {
	in := ExitInput{Stage: m.currentStage(), HasFeature: run.FeatureID != "",
		LatestGates: map[string]GateFact{}, FeatureGates: map[string]GateFact{}}
	feat, err := e.store.Features().GetByID(run.FeatureID)
	if err == nil {
		in.TruthPackagePresent = feat.TruthPath != ""
		in.TruthDraftPresent = feat.Status == "draft"
	}
	// Run-scoped facts first (review B3): only this run's own exit
	// judgements and run-bound formal evaluations. Sibling runs of the
	// same feature never leak in here.
	byGate, _, err := e.runGateFacts(run.ID)
	if err != nil {
		return in, err
	}
	for gt, rec := range byGate {
		in.LatestGates[gt] = GateFact{
			Verdict: rec.Verdict, ReasonCodes: rec.ReasonCodes,
			EvaluatedAt: rec.EvaluatedAt, Evaluator: rec.Evaluator, Source: rec.Source,
		}
	}
	gates, err := e.featureGateFallback(run.FeatureID, run.ID)
	if err != nil {
		return in, err
	}
	in.FeatureGates = gates
	if lc, err := e.store.Features().GetLifecycle(run.FeatureID); err == nil {
		in.LifecycleStage = string(lc.Stage)
		in.LifecycleStatus = string(lc.Status)
		if lc.Stage == core.StagePrototype && lc.Status == core.StatusCompleted {
			in.DesignEvidence = true
		}
	}
	// Frozenness is the ONE shared determination (frozen_fact.go) —
	// the lifecycle snapshot no longer feeds it.
	if ff, err := e.frozenFactByID(run.FeatureID); err == nil {
		in.TruthFrozen = ff.Frozen
		in.TruthFrozenFormal = ff.Source == FrozenSourceFormalGate
		in.TruthChangePending = ff.ChangePending
		in.FrozenSource = ff.Source
		in.TruthBaselineUncommitted = ff.BaselineStatus == BaselineUncommitted
	}
	// Design evidence is run-scoped: another run's design gate pass is
	// not this run's evidence (review B3).
	if g, ok := in.LatestGates["design"]; ok && g.Verdict == "pass" {
		in.DesignEvidence = true
		in.DesignEvidenceFormal = true
	}
	// Artifact presence is this run's evidence, not the feature's
	// pooled pointer count (review B3).
	arts, err := e.store.Artifacts().ListByRun(run.ID)
	if err != nil {
		return in, err
	}
	in.ArtifactCount = len(arts)
	return in, nil
}

// featureGateFallback reads the feature-level formal verdicts a stage may
// legitimately consult (plan vetoes, formal QA/release verdicts). Rows
// bound to ANY run are excluded: `gate_results` has no run column, but
// every run-scoped judgement (a stage exit or a formal `gate evaluate
// --run`) lands a run-bound trail event naming its gate_result_id. Such a
// row is that run's own evidence (read from LatestGates) and must never
// fall back into a sibling run's exit input — otherwise run A's qa pass
// lets a zero-evidence run B skip qa (AC-003). Only rows no run ever
// claimed remain feature-level fallback.
func (e *Engine) featureGateFallback(featureID, runID string) (map[string]GateFact, error) {
	out := map[string]GateFact{}
	events, err := e.store.ListEvents(featureID)
	if err != nil {
		return out, err
	}
	runBound := map[string]bool{}
	for _, ev := range events {
		if ev.EventType != "stage_exit_evaluated" && ev.EventType != EventGateEvaluated {
			continue
		}
		// A feature-level evaluation carries no run and stays eligible.
		if !ev.RunID.Valid {
			continue
		}
		var p struct {
			GateResultID string `json:"gate_result_id"`
		}
		if len(ev.Payload) > 0 && json.Unmarshal(ev.Payload, &p) == nil && p.GateResultID != "" {
			runBound[p.GateResultID] = true
		}
	}
	all, err := e.store.Gates().ListByFeature(featureID)
	if err != nil {
		return out, err
	}
	seen := map[string]bool{}
	for _, g := range all {
		if seen[g.GateType] || runBound[g.ID] {
			continue
		}
		seen[g.GateType] = true
		out[g.GateType] = GateFact{
			Verdict: g.Verdict, ReasonCodes: g.ReasonCodes,
			EvaluatedAt: g.EvaluatedAt, Source: "feature_gate",
		}
	}
	return out, nil
}

// ExitGate evaluates the exit gate of the run's current stage from
// store state (read-only judgement; the verdict is not persisted —
// formal persisted evaluations live in the P3a gate service,
// Engine.EvaluateGate, and this judgement consumes them formal-first
// for the qa/release exits).
func (e *Engine) ExitGate(runID string) (ExitVerdict, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return ExitVerdict{}, err
	}
	in, err := e.buildExitInput(run, m)
	if err != nil {
		return ExitVerdict{}, err
	}
	return EvaluateStageExit(in), nil
}
