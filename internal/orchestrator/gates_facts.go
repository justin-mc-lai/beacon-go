// Gate fact gatherers (P3a): store + package-file reads that feed
// the pure evaluators of internal/gate. Each gatherer is the ONLY
// place its gate's facts are collected — in particular evalImplement
// wraps the shared admission checker (G-04) and implements no
// judgement of its own.
package orchestrator

import (
	"encoding/json"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// evalTruthFreeze gathers the truth_freeze facts — the WRITE side
// of the frozen question. The gate itemises the package's own
// declarations and mints a verdict; the shared read-side
// determination (Engine.FrozenFact, frozen_fact.go) consumes the
// document first and this gate's verdict only when the document
// is silent, so sharing primitives here creates no loop: the
// frontmatter read is frontmatterStatusOf and the pending-change
// scan is pendingChangeIDs, the same helpers FrozenFact uses.
func (e *Engine) evalTruthFreeze(ctx store.FeatureContext, evaluator string) (gate.Evaluation, error) {
	root := ctx.Project.RootPath
	feat := ctx.Feature
	truth := readPkgFile(root, feat.TruthPath)
	testsPath := feat.TestsPath
	if testsPath == "" {
		testsPath = siblingPkgPath(feat.TruthPath, "tests.md")
	}
	tests := readPkgFile(root, testsPath)
	f := gate.TruthFreezeFacts{
		TruthPresent:  truth.Present,
		TruthNonEmpty: truth.Present && strings.TrimSpace(truth.Text) != "",
		TestsPresent:  tests.Present,
		TestsNonEmpty: tests.Present && strings.TrimSpace(tests.Text) != "",
	}
	if status, parsed := frontmatterStatusOf(truth); parsed {
		f.FrozenMarker = status == "frozen"
	}
	if lc, err := e.store.Features().GetLifecycle(feat.ID); err == nil {
		f.LifecycleFrozen = lc.Stage == core.StageFreeze && lc.Status == core.StatusCompleted
	}
	pending, err := e.pendingChangeIDs(feat.ID)
	if err != nil {
		return gate.Evaluation{}, err
	}
	f.PendingChange = len(pending) > 0
	return gate.EvaluateTruthFreeze(f, evaluator), nil
}

// evalDesign gathers the design facts from the run's binding record.
func (e *Engine) evalDesign(ctx store.FeatureContext, run store.Run, evaluator string) (gate.Evaluation, error) {
	_, m, _, err := e.loadRun(run.ID)
	if err != nil {
		return gate.Evaluation{}, err
	}
	f := gate.DesignFacts{HasRun: true, Required: designRequired(m), Bound: designBound(m)}
	if m.DesignBinding != nil {
		f.BindingSet = m.DesignBinding.PrimitiveSet
		f.BindingVersion = m.DesignBinding.Version
	}
	events, err := e.store.ListEventsByRun(run.ID)
	if err != nil {
		return gate.Evaluation{}, err
	}
	for _, ev := range events {
		if ev.EventType != "design_primitive_bound" {
			continue
		}
		var p struct {
			Version string `json:"version"`
		}
		if len(ev.Payload) > 0 {
			_ = json.Unmarshal(ev.Payload, &p)
		}
		f.HasBoundEvent = true
		f.EventVersion = p.Version
	}
	return gate.EvaluateDesign(f, evaluator), nil
}

// evalImplement wraps the ONE shared admission checker (G-04): the
// gate's verdict is the admission verdict, with the admission items
// mapped 1:1 — no second judgement is implemented here.
func (e *Engine) evalImplement(ctx store.FeatureContext, run store.Run, evaluator string) (gate.Evaluation, error) {
	_, m, _, err := e.loadRun(run.ID)
	if err != nil {
		return gate.Evaluation{}, err
	}
	adm, err := e.checkAdmission(run, m)
	if err != nil {
		return gate.Evaluation{}, err
	}
	facts := gate.ImplementFacts{
		HasRun: true, AllowImplement: adm.AllowImplement, HeadlineCode: adm.ReasonCode,
	}
	for _, it := range adm.Items {
		facts.Items = append(facts.Items, gate.CheckItem{
			Check: it.Check, OK: it.OK, Applicable: it.Applicable,
			ReasonCode: it.ReasonCode, Detail: it.Detail,
		})
	}
	return gate.EvaluateImplement(facts, evaluator), nil
}

// qaKind reports whether an artifact kind marks QA evidence.
func qaKind(kind string) bool {
	switch {
	case kind == "qa", kind == "scorecard", kind == "test_result", kind == "test_results":
		return true
	case strings.HasPrefix(kind, "qa_"):
		return true
	}
	return false
}

// evalQA gathers the QA-evidence facts across the feature's runs.
func (e *Engine) evalQA(ctx store.FeatureContext, evaluator string) (gate.Evaluation, error) {
	f := gate.QAFacts{}
	runs, err := e.store.Runs().ListByFeature(ctx.Feature.ID)
	if err != nil {
		return gate.Evaluation{}, err
	}
	for _, run := range runs {
		arts, err := e.store.Artifacts().ListByRun(run.ID)
		if err != nil {
			return gate.Evaluation{}, err
		}
		for _, a := range arts {
			if !qaKind(a.Kind) {
				continue
			}
			f.QAArtifactCount++
			if stale, reason := store.ArtifactStaleness(a); stale {
				f.StaleArtifacts = append(f.StaleArtifacts, gate.StaleRef{Kind: a.Kind, Path: a.AbsPath, Reason: reason})
			}
		}
		events, err := e.store.ListEventsByRun(run.ID)
		if err != nil {
			return gate.Evaluation{}, err
		}
		for _, ev := range events {
			if ev.EventType != "stage_executed" {
				continue
			}
			var p struct {
				Stage   string `json:"stage"`
				Outcome string `json:"outcome"`
			}
			if len(ev.Payload) > 0 {
				_ = json.Unmarshal(ev.Payload, &p)
			}
			if p.Stage == StageQA && p.Outcome != "failed" {
				f.QAStageExecuted = true
			}
		}
	}
	return gate.EvaluateQA(f, evaluator), nil
}

// evalRelease composes the release facts: the other four gates'
// latest verdicts + the human confirmation on record.
func (e *Engine) evalRelease(ctx store.FeatureContext, evaluator string) (gate.Evaluation, error) {
	f := gate.ReleaseFacts{}
	latest, err := e.store.Gates().LatestByType(ctx.Feature.ID)
	if err != nil {
		return gate.Evaluation{}, err
	}
	byType := map[string]store.GateResult{}
	for _, g := range latest {
		byType[g.GateType] = g
	}
	fact := func(t gate.GateType) gate.GateFact {
		g, ok := byType[string(t)]
		if !ok {
			return gate.GateFact{}
		}
		return gate.GateFact{Present: true, Verdict: g.Verdict, ReasonCodes: g.ReasonCodes}
	}
	f.TruthFreeze = fact(gate.GateTruthFreeze)
	f.Design = fact(gate.GateDesign)
	f.Implement = fact(gate.GateImplement)
	f.QA = fact(gate.GateQA)
	// M7 (R2a): materials freshness — a surface still marked
	// stale after a truth change blocks the release verdict.
	f.MaterialsStale = e.staleMaterialSurfaces(ctx)
	// Design applicability + human confirmation both live on the
	// newest goal run; the release_confirmed event is the trail-side
	// record of the same confirmation (P2a口径).
	_, m, has, err := e.latestGoalRun(ctx, "")
	if err != nil {
		return gate.Evaluation{}, err
	}
	if has {
		f.DesignApplicable = designRequired(m)
		f.HumanConfirmed = m.ReleaseHumanConfirmed
		f.HumanConfirmedBy = m.HumanConfirmedBy
	}
	if !f.HumanConfirmed {
		events, err := e.store.ListEvents(ctx.Feature.ID)
		if err != nil {
			return gate.Evaluation{}, err
		}
		for _, ev := range events {
			if ev.EventType != "release_confirmed" {
				continue
			}
			f.HumanConfirmed = true
			var p struct {
				By string `json:"by"`
			}
			if len(ev.Payload) > 0 {
				_ = json.Unmarshal(ev.Payload, &p)
			}
			if f.HumanConfirmedBy == "" {
				f.HumanConfirmedBy = p.By
			}
		}
	}
	return gate.EvaluateRelease(f, evaluator), nil
}
