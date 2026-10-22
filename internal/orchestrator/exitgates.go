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
	// LatestGates maps gate_type -> latest fact.
	LatestGates map[string]GateFact
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
	Allowed       bool   `json:"allowed"`
	ReasonCode    string `json:"reason_code"`
	ResumeAction  string `json:"resume_action"`
	FillSkill     string `json:"fill_skill,omitempty"`
	Derived       bool   `json:"derived"`
	FormalVerdict string `json:"formal_verdict,omitempty"`
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
	return ExitVerdict{Allowed: false, ReasonCode: reason, ResumeAction: resume, FillSkill: fill, Derived: true}
}

func passVerdict() ExitVerdict {
	return ExitVerdict{Allowed: true, ReasonCode: ReasonStageExitOK, ResumeAction: core.ResumeNone, Derived: true}
}

// formalVerdictOf returns the latest formal verdict recorded for a
// gate type ("" when never evaluated).
func formalVerdictOf(in ExitInput, gateType string) string {
	if g, ok := in.LatestGates[gateType]; ok {
		return g.Verdict
	}
	return ""
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
			if g, ok := in.LatestGates[gt]; ok && g.Verdict == "blocked" {
				v := blockedVerdict(firstCode(g.ReasonCodes, ReasonStageExitPlanIncompl),
					core.ResumeLoadNextSkill, "beacon-gen-truth")
				v.Derived, v.FormalVerdict = false, g.Verdict
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
			v.FormalVerdict = formalVerdictOf(in, "truth_freeze")
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
			v.FormalVerdict = formalVerdictOf(in, "truth_freeze")
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
			v.FormalVerdict = formalVerdictOf(in, "truth_freeze")
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
			v.FormalVerdict = formalVerdictOf(in, "design")
			return v
		}
		v := blockedVerdict(ReasonStageExitDesignIncompl, core.ResumeLoadNextSkill, "beacon-design")
		v.FormalVerdict = formalVerdictOf(in, "design")
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
		if g, ok := in.LatestGates["qa"]; ok {
			if g.Verdict == "pass" {
				v := passVerdict()
				v.Derived, v.FormalVerdict = false, g.Verdict
				return v
			}
			v := blockedVerdict(firstCode(g.ReasonCodes, ReasonStageExitQAEvidence),
				core.ResumeLoadNextSkill, "beacon-qa")
			v.Derived, v.FormalVerdict = false, g.Verdict
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
		if g, ok := in.LatestGates["release"]; ok {
			if g.Verdict == "pass" {
				v := blockedVerdict(core.ReasonReleaseAwaitingComplete, core.ResumeAwaitHumanRelease, "beacon-release")
				v.Derived, v.FormalVerdict = false, g.Verdict
				return v
			}
			v := blockedVerdict(firstCode(g.ReasonCodes, core.ReasonReleaseRequiresHuman),
				core.ResumeAwaitHumanRelease, "beacon-release")
			v.Derived, v.FormalVerdict = false, g.Verdict
			return v
		}
		// Source-exact: release never auto-exits on tick.
		return blockedVerdict(core.ReasonReleaseRequiresHuman, core.ResumeAwaitHumanRelease, "beacon-release")
	default:
		// Unknown stage: fail closed, never default-pass.
		return blockedVerdict(ReasonStageExitPlanIncompl, core.ResumeNone, "")
	}
}

// buildExitInput reads the store facts for one run's current stage.
func (e *Engine) buildExitInput(run store.Run, m *runMeta) (ExitInput, error) {
	in := ExitInput{Stage: m.currentStage(), HasFeature: run.FeatureID != "", LatestGates: map[string]GateFact{}}
	feat, err := e.store.Features().GetByID(run.FeatureID)
	if err == nil {
		in.TruthPackagePresent = feat.TruthPath != ""
		in.TruthDraftPresent = feat.Status == "draft"
	}
	gates, err := e.store.Gates().LatestByType(run.FeatureID)
	if err != nil {
		return in, err
	}
	for _, g := range gates {
		in.LatestGates[g.GateType] = GateFact{Verdict: g.Verdict, ReasonCodes: g.ReasonCodes}
	}
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
	if g, ok := in.LatestGates["design"]; ok && g.Verdict == "pass" {
		in.DesignEvidence = true
		in.DesignEvidenceFormal = true
	}
	runs, err := e.store.Runs().ListByFeature(run.FeatureID)
	if err != nil {
		return in, err
	}
	for _, r := range runs {
		arts, err := e.store.Artifacts().ListByRun(r.ID)
		if err != nil {
			return in, err
		}
		in.ArtifactCount += len(arts)
	}
	return in, nil
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
