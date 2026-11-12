// Gate fact gatherers (P3a): store + package-file reads that feed
// the pure evaluators of internal/gate. Each gatherer is the ONLY
// place its gate's facts are collected — in particular evalImplement
// wraps the shared admission checker (G-04) and implements no
// judgement of its own.
package orchestrator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/harness"
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
// mapped 1:1 — no second judgement is implemented here. The ONE item
// the gate adds is the exit's work evidence (implement-work-gate
// AC-001): admission asks "may this stage start", the work item asks
// "did this stage do anything" — the beacon-go#4 hole.
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
	work, err := e.implementWorkFacts(run, m)
	if err != nil {
		return gate.Evaluation{}, err
	}
	facts.Work = work
	return gate.EvaluateImplement(facts, evaluator), nil
}

// implementWorkFacts gathers the implement exit's work evidence
// (implement-work-gate AC-001..006) with READ-ONLY reads only: the
// run's declaration event, the bound worktree's HEAD + dirty state
// and the feature's recorded baseline commit. Nothing here writes the
// worktree, creates a commit or touches the run snapshot, so the same
// run evaluated twice yields the same facts (AC-006).
//
// Unreadable work trees are reported as Unverified, never as a
// failure AND never as a pass (AC-004): a missing directory, a non-git
// project, a git failure and a run with no consumable baseline commit
// all land there.
func (e *Engine) implementWorkFacts(run store.Run, m *runMeta) (gate.ImplementWorkFacts, error) {
	f := gate.ImplementWorkFacts{GateEnabled: e.implementWorkGateEnabled()}
	recorded, err := e.implementEmptyAllowedRecorded(run.ID)
	if err != nil {
		return f, err
	}
	f.EmptyAllowed = recorded
	// Both not_applicable legs settle the item already: reading git
	// would buy nothing.
	if !f.GateEnabled || f.EmptyAllowed {
		return f, nil
	}
	slug := ""
	if feat, ferr := e.store.Features().GetByID(run.FeatureID); ferr == nil {
		slug = feat.Slug
	}
	b, err := e.bindingOf(run, m, slug)
	if err != nil {
		return f, err
	}
	if !b.Applicable {
		f.Unverified, f.UnverifiedDetail = true, b.Note
		return f, nil
	}
	st, err := harness.SnapshotWorkTree(context.Background(), b.Dir)
	if err != nil {
		f.Unverified = true
		f.UnverifiedDetail = "git read failed in " + b.Dir + ": " + strings.TrimSpace(err.Error())
		return f, nil
	}
	commit, status, frozen, err := e.baselineCommitOf(run.FeatureID)
	if err != nil {
		return f, err
	}
	f.HeadCommit, f.Dirty, f.DirtyCount = st.Head, st.Dirty(), len(st.DirtyPaths)
	if !frozen || status != BaselineCommitted || commit == "" {
		f.Unverified = true
		f.UnverifiedDetail = "no consumable truth_baseline_commit to compare HEAD against (baseline_status=" + status + ")"
		if !frozen {
			f.UnverifiedDetail = "truth is not frozen, so there is no baseline commit to compare HEAD against"
		}
		return f, nil
	}
	f.BaselineCommit = commit
	// HEAD "moved" = it no longer equals the baseline commit. A HEAD
	// that moved backwards also counts: the stage did SOMETHING, and
	// judging direction is qa's job, not this check's (non-goals).
	f.HeadMoved = st.Head != commit
	return f, nil
}

// qaKind reports whether an artifact kind marks QA evidence.
func qaKind(kind string) bool {
	switch {
	case kind == "qa", kind == "scorecard", kind == "test_result", kind == "test_results":
		return true
	case kind == "a11y_tree", kind == "browser_trace", kind == "e2e_report":
		return true
	case strings.HasPrefix(kind, "qa_"):
		return true
	}
	return false
}

// evalQA gathers the QA-evidence facts across the feature's runs.
func (e *Engine) evalQA(ctx store.FeatureContext, evaluator string) (gate.Evaluation, error) {
	f := gate.QAFacts{}
	// qa-surface-e2e: the requirement truth names the surfaces the run
	// must prove. A package without the key is legacy (strict=false).
	decl := gate.ParseSurfaceDeclaration(readPkgFile(ctx.Project.RootPath, ctx.Feature.TruthPath).Text)
	f.Surfaces = decl.Surfaces
	f.SurfaceStrict = decl.EffectiveStrict()
	f.SurfaceEvidence = map[string]map[string]bool{}
	runs, err := e.store.Runs().ListByFeature(ctx.Feature.ID)
	if err != nil {
		return gate.Evaluation{}, err
	}
	// AC-003/AC-004 (artifact-upsert): only the newest row per
	// (run, path) is the live pointer. Legacy DBs may still hold
	// several rows for one path — validating the older ones would
	// read every re-registration as qa_artifact_stale forever. No
	// migration is needed: ListByRun ascends by (created_at, rowid),
	// so the last row seen for a key wins and the older rows simply
	// never reach the staleness check.
	latest := map[string]store.ArtifactRef{}
	seen := map[string]bool{}
	keys := []string{}
	// externalCompleted remembers runs whose receive-result record says
	// the lake finished in an external room (AC-003).
	externalCompleted := map[string]bool{}
	// liveQA remembers runs with at least one QA-kind artifact whose
	// file still hashes to its registered content.
	liveQA := map[string]bool{}
	liveKinds := map[string]bool{}
	for _, run := range runs {
		arts, err := e.store.Artifacts().ListByRun(run.ID)
		if err != nil {
			return gate.Evaluation{}, err
		}
		// qa_autotest_recorded: a non-empty .beacon/autotest/<run-id>/
		// directory, or a registered pointer inside it, is the run's
		// autotest record. Checked per run so one run's evidence cannot
		// silently vouch for another.
		if !f.AutotestRecorded && autotestEvidence(ctx.Project.RootPath, run.ID, arts) {
			f.AutotestRecorded = true
			f.AutotestDetail = "autotest evidence recorded for run " + run.ID
		}
		for _, a := range arts {
			if !qaKind(a.Kind) {
				continue
			}
			key := run.ID + "\x00" + a.AbsPath
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
			latest[key] = a
		}
		events, err := e.store.ListEventsByRun(run.ID)
		if err != nil {
			return gate.Evaluation{}, err
		}
		for _, ev := range events {
			var p struct {
				Stage       string `json:"stage"`
				Outcome     string `json:"outcome"`
				Source      string `json:"source"`
				Placeholder *bool  `json:"placeholder"`
				Recording   *bool  `json:"recording"`
			}
			if len(ev.Payload) > 0 {
				_ = json.Unmarshal(ev.Payload, &p)
			}
			switch ev.EventType {
			case "stage_executed":
				// AC-002: only a real event counts. The recording marker
				// must be present AND false; a missing marker is treated
				// as recording state, and a true marker (placeholder or
				// recording) is refused outright.
				real := p.Placeholder != nil && !*p.Placeholder &&
					(p.Recording == nil || !*p.Recording)
				if real && p.Stage == StageQA && p.Outcome != "failed" {
					f.QAStageExecuted = true
				}
			case "goal_completed":
				// AC-003: the external completion record is the
				// whole-lake path; the source field is required.
				if p.Source == ExecutorModeExternal {
					externalCompleted[run.ID] = true
				}
			}
		}
	}
	for _, key := range keys {
		a := latest[key]
		f.QAArtifactCount++
		if stale, reason := store.ArtifactStaleness(a); stale {
			f.StaleArtifacts = append(f.StaleArtifacts, gate.StaleRef{Kind: a.Kind, Path: a.AbsPath, Reason: reason})
		} else {
			liveQA[a.RunID] = true
			liveKinds[a.Kind] = true
		}
	}
	for _, surface := range f.Surfaces {
		f.SurfaceEvidence[surface] = liveKinds
	}
	// AC-003: an external completion record plus at least one registered
	// QA artifact whose hash still matches is equivalent to a real qa
	// stage_executed event. It never substitutes for the per-stage path
	// when that path exists; it is the whole-lake fallback.
	if !f.QAStageExecuted {
		for runID := range externalCompleted {
			if liveQA[runID] {
				f.QAStageExecuted = true
				break
			}
		}
	}
	return gate.EvaluateQA(f, evaluator), nil
}

// autotestEvidence reports whether one run left auto-test evidence:
// a non-empty .beacon/autotest/<run-id>/ directory, or a registered
// artifact whose path lives under it. Directory listing is read-only
// and never creates anything.
func autotestEvidence(root, runID string, arts []store.ArtifactRef) bool {
	dir := filepath.Join(root, ".beacon", "autotest", runID)
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return true
	}
	for _, a := range arts {
		rel, err := filepath.Rel(dir, a.AbsPath)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		return true
	}
	return false
}

// releaseFacts composes the release facts: the other four gates'
// latest verdicts + the human confirmation on record. Shared by the
// formal release gate and the auto_shadow preview (the shadow sets
// HumanConfirmed=true only on its own copy).
func (e *Engine) releaseFacts(ctx store.FeatureContext) (gate.ReleaseFacts, error) {
	f := gate.ReleaseFacts{}
	latest, err := e.store.Gates().LatestByType(ctx.Feature.ID)
	if err != nil {
		return gate.ReleaseFacts{}, err
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
		return gate.ReleaseFacts{}, err
	}
	if has {
		f.DesignApplicable = designRequired(m)
		f.HumanConfirmed = m.ReleaseHumanConfirmed
		f.HumanConfirmedBy = m.HumanConfirmedBy
	}
	if !f.HumanConfirmed {
		events, err := e.store.ListEvents(ctx.Feature.ID)
		if err != nil {
			return gate.ReleaseFacts{}, err
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
	return f, nil
}

// evalRelease is the formal release gate: the facts plus the human
// confirmation exactly as recorded. The shadow path never goes
// through here.
func (e *Engine) evalRelease(ctx store.FeatureContext, evaluator string) (gate.Evaluation, error) {
	f, err := e.releaseFacts(ctx)
	if err != nil {
		return gate.Evaluation{}, err
	}
	return gate.EvaluateRelease(f, evaluator), nil
}
