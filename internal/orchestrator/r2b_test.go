package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
)

// ---- H5: review write-back consequences ----

func reviewPacket(route string, udr bool, findings ...core.ReviewFinding) core.ReviewPacket {
	n := len(findings)
	return core.ReviewPacket{
		ReviewID: "rv-1", ArtifactRef: "art-1", RecommendedRoute: route,
		UserDecisionRequired: udr, Findings: findings,
		DeclaredFindingCount: &n, SynthesisMarker: true,
	}
}

// A P0 review blocks the run: admission refuses implement with
// the review's own code, tick will not advance, continue refuses.
func TestReviewDecisionP0BlocksRun(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	run := env.createRun(t)
	rec, _, err := env.eng.RecordReviewDecision(run.RunID,
		reviewPacket("", false, core.ReviewFinding{Severity: "P0", Message: "spec broken"}), "test")
	if err != nil {
		t.Fatal(err)
	}
	if rec.NextStage != string(core.LoopErrorRecovery) || rec.ReasonCode != core.ReasonPlannerReviewBlockingFinding {
		t.Fatalf("routing: %+v", rec)
	}
	st, err := env.eng.Status(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Blocked || st.Review == nil || st.Review.ReasonCode != core.ReasonPlannerReviewBlockingFinding {
		t.Fatalf("run must be blocked by the review: %+v", st)
	}
	adm, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	var item *AdmissionItem
	for i := range adm.Items {
		if adm.Items[i].Check == "review_decision" {
			item = &adm.Items[i]
		}
	}
	if item == nil || item.OK || item.ReasonCode != core.ReasonPlannerReviewBlockingFinding {
		t.Fatalf("admission review item: %+v", item)
	}
	if adm.AllowImplement {
		t.Fatal("admission must refuse implement under an unresolved P0 review")
	}
	// Tick does not advance a review-held run.
	v, err := env.eng.Tick(context.Background(), run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if v.CurrentStage != run.CurrentStage {
		t.Fatalf("tick advanced a review-held run: %+v", v)
	}
	// Continue refuses to clear a review-held block: it returns
	// the held view carrying the review's code (the house caliber
	// for refusals — cf. the blocked-streak branch), still blocked.
	cv, err := env.eng.Continue(context.Background(), run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !cv.Blocked || cv.ReasonCode != core.ReasonPlannerReviewBlockingFinding {
		t.Fatalf("continue must hold the review block: %+v", cv)
	}
}

// A drift-route review parks the loop in drift_routing (the truth
// change pipeline is the way out), not in plain error recovery.
func TestReviewDecisionDriftRouting(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	rec, _, err := env.eng.RecordReviewDecision(run.RunID,
		reviewPacket("beacon-gen-change", false, core.ReviewFinding{Severity: "P1", Message: "drift"}), "test")
	if err != nil {
		t.Fatal(err)
	}
	if rec.NextStage != string(core.LoopDriftRouting) {
		t.Fatalf("routing: %+v", rec)
	}
	st, _ := env.eng.Status(run.RunID)
	if st.LoopStage != string(core.LoopDriftRouting) || !st.Blocked {
		t.Fatalf("run state: %+v", st)
	}
}

// A user-decision review waits; once the decision is on record,
// re-routing the same packet falls through to the advisory
// branch and the block clears.
func TestReviewDecisionUserDecisionFlow(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	p := reviewPacket("", true, core.ReviewFinding{Severity: "P2", Message: "needs a human call"})
	rec, _, err := env.eng.RecordReviewDecision(run.RunID, p, "test")
	if err != nil {
		t.Fatal(err)
	}
	if rec.ReasonCode != core.ReasonPlannerReviewUserDecisionReq || rec.Resolved {
		t.Fatalf("first routing: %+v", rec)
	}
	if _, err := env.eng.RecordUserDecision(run.RunID, core.PlannerReviewDecisionKey, "proceed"); err != nil {
		t.Fatal(err)
	}
	rec, _, err = env.eng.RecordReviewDecision(run.RunID, p, "test")
	if err != nil {
		t.Fatal(err)
	}
	if rec.NextStage != string(core.LoopStandardExecutionPrecheck) || !rec.Resolved {
		t.Fatalf("second routing: %+v", rec)
	}
	st, _ := env.eng.Status(run.RunID)
	if st.Blocked {
		t.Fatalf("block must clear after the advisory routing: %+v", st)
	}
}

// A P1 advisory continue bumps p1_continue_count (the source's
// metrics) and does not block.
func TestReviewDecisionP1AdvisoryMetrics(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	rec, view, err := env.eng.RecordReviewDecision(run.RunID,
		reviewPacket("continue_delivery", false, core.ReviewFinding{Severity: "P1", Message: "watch it"}), "test")
	if err != nil {
		t.Fatal(err)
	}
	if !rec.ContinuedWithAdvisory {
		t.Fatalf("record: %+v", rec)
	}
	if view.HumanStops == nil || view.HumanStops.P1ContinueCount != 1 {
		t.Fatalf("human stops: %+v", view.HumanStops)
	}
	if view.Blocked {
		t.Fatal("an advisory routing must not block")
	}
}

// ---- M8: records ----

func TestOrchestrationRecords(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	// Intent fork leaves a trail entry + event.
	v, err := env.eng.RecordIntentFork(run.RunID, "truth", "改走方案 B", "fork_to_b", "intent_changed")
	if err != nil {
		t.Fatal(err)
	}
	_ = v
	types := traceEventTypes(t, env, run.RunID)
	if !hasEvent(types, "intent_fork") {
		t.Fatalf("intent_fork event missing: %v", types)
	}
	// Gate card: build -> duplicate once decided -> resolve ->
	// re-resolve refused -> assert passes.
	res, err := env.eng.BuildGateCard(run.RunID, "deploy_ok", "可以上线吗?", "ctx", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Duplicate || res.Card.DecisionKey != "deploy_ok" {
		t.Fatalf("card: %+v", res)
	}
	if ok, code, err := env.eng.AssertGateResolved(run.RunID, "deploy_ok"); err != nil || ok || code != core.ReasonGateUnresolved {
		t.Fatalf("assert unresolved: %v %s %v", ok, code, err)
	}
	if _, err := env.eng.ResolveGate(run.RunID, "deploy_ok", "yes"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.ResolveGate(run.RunID, "deploy_ok", "yes again"); err == nil {
		t.Fatal("second resolve must be refused")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonGateAlreadyResolved {
		t.Fatalf("re-resolve code = %s", rc)
	}
	res, err = env.eng.BuildGateCard(run.RunID, "deploy_ok", "可以上线吗?", "ctx", "")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Duplicate || res.CachedAnswer != "yes" {
		t.Fatalf("decided key must return the cached answer: %+v", res)
	}
	if ok, _, err := env.eng.AssertGateResolved(run.RunID, "deploy_ok"); err != nil || !ok {
		t.Fatalf("assert resolved: %v %v", ok, err)
	}
	// Error recovery through the one door.
	v, err = env.eng.RouteToErrorRecovery(run.RunID, "executor_died")
	if err != nil {
		t.Fatal(err)
	}
	if v.LoopStage != string(core.LoopErrorRecovery) || !v.Blocked {
		t.Fatalf("error recovery state: %+v", v)
	}
	// Host interrupt feeds the human-stop metrics.
	if _, err := env.eng.RecordInterrupt(run.RunID, "implement"); err != nil {
		t.Fatal(err)
	}
	rv, err := env.eng.Resume(run.RunID, "")
	if err != nil {
		t.Fatal(err)
	}
	if rv.HumanStops == nil || rv.HumanStops.HumanStopsPerRun < 1 {
		t.Fatalf("human stop metrics: %+v", rv.HumanStops)
	}
	if rv.HumanStops.StopReasonCounts["host_turn_interrupted"] < 1 {
		t.Fatalf("stop reason counts: %+v", rv.HumanStops.StopReasonCounts)
	}
	if rv.HumanStops.PseudoStallFlag {
		t.Fatal("pseudo_stall_flag is never set true in the source; the port keeps it false")
	}
}

// ---- H6: driver consumes quota ----

// An operator gate (pending steering) makes the driver requeue
// without burning a turn or budget.
func TestDriverQuotaOperatorGateSkips(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	if _, err := env.eng.Steer(run.RunID, SteeringObjective, "hold on"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.DriverStart(run.RunID, DriverStartOptions{}); err != nil {
		t.Fatal(err)
	}
	dv, err := env.eng.DriverRunOnce(context.Background(), run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if dv.State != DriverRequeue || dv.ReasonCode != core.ReasonOperatorGateOpen {
		t.Fatalf("driver view: %+v", dv)
	}
	if dv.Driver == nil || dv.Driver.TurnN != 0 {
		t.Fatalf("a quota skip must not burn a turn: %+v", dv.Driver)
	}
	qv, err := env.eng.QuotaStatus(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if qv.Decision.Decision != "skip" || qv.Decision.ReasonCode != core.ReasonOperatorGateOpen {
		t.Fatalf("quota view: %+v", qv.Decision)
	}
}

// Slot exhaustion halts the driver with quota_exhausted after the
// allowed turns are spent.
func TestDriverQuotaExhaustionHalts(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	run := env.createRun(t)
	if _, err := env.eng.ConfigureQuota(run.RunID, core.QuotaConfig{AllowedSlots: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.DriverStart(run.RunID, DriverStartOptions{}); err != nil {
		t.Fatal(err)
	}
	dv, err := env.eng.DriverRunOnce(context.Background(), run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if dv.State != DriverRequeue {
		t.Fatalf("turn 1: %+v", dv)
	}
	dv, err = env.eng.DriverRunOnce(context.Background(), run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if dv.State != DriverHaltBudget || dv.ReasonCode != core.ReasonQuotaExhausted {
		t.Fatalf("turn 2 must halt on quota: %+v", dv)
	}
	qv, _ := env.eng.QuotaStatus(run.RunID)
	if qv.Decision.SpentSlots != 1 {
		t.Fatalf("quota spent = %d, want 1", qv.Decision.SpentSlots)
	}
}

// ---- M9: spend ledger through the engine ----

func TestRecordSpendGuards(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	codeOf := func(err error) string { rc, _ := core.ReasonCodeOf(err); return rc }
	if _, err := env.eng.RecordSpend(run.RunID, "poll", "d1", true, ""); codeOf(err) != core.ReasonQuietSkipNoSpend {
		t.Fatalf("quiet source: %v", err)
	}
	if _, err := env.eng.RecordSpend(run.RunID, "delivery_completion", "d1", false, ""); codeOf(err) != core.ReasonSpendWithoutWriteback {
		t.Fatalf("writeback: %v", err)
	}
	if _, err := env.eng.RecordSpend(run.RunID, "bogus", "d1", true, ""); codeOf(err) != core.ReasonSpendSourceNotAllowed {
		t.Fatalf("source: %v", err)
	}
	l, err := env.eng.RecordSpend(run.RunID, "delivery_completion", "d1", true, "wb-1")
	if err != nil {
		t.Fatal(err)
	}
	if l.Spent != 1 {
		t.Fatalf("ledger: %+v", l)
	}
	if _, err := env.eng.RecordSpend(run.RunID, "delivery_completion", "d1", true, "wb-1"); codeOf(err) != core.ReasonSpendIdempotencyConf {
		t.Fatalf("duplicate digest: %v", err)
	}
}

// ---- H7: version check over real packages ----

func TestReleaseVersionCheckEngine(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "rel-box")
	// Draft package: the version check must block (not frozen,
	// no human confirmation, no QA evidence...).
	res, err := env.eng.ReleaseVersionCheck(env.root, "", "full")
	if err != nil {
		t.Fatal(err)
	}
	if res.ReleaseReady {
		t.Fatalf("a draft package's version must not be release-ready: %+v", res)
	}
	found := false
	for _, name := range res.FailedChecks {
		if name == gate.VersionCheckFrozen {
			found = true
		}
	}
	if !found {
		t.Fatalf("failed checks = %v", res.FailedChecks)
	}
	// Unknown version: registered check fails.
	res, err = env.eng.ReleaseVersionCheck(env.root, "v9.9.9", "full")
	if err != nil {
		t.Fatal(err)
	}
	if res.ReleaseReady || res.FeatureCount != 0 {
		t.Fatalf("unknown version: %+v", res)
	}
	// Invalid profile is refused with the profile code.
	if _, err := env.eng.ReleaseVersionCheck(env.root, "", "banana"); err == nil {
		t.Fatal("invalid profile must be refused")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonReleaseProfileInvalid {
		t.Fatalf("profile code = %s", rc)
	}
}

// ---- M15: reviews over a real package + write-back ----

func TestReviewFacesAndWriteBack(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "rev-box")
	run, err := env.eng.CreateRun(CreateRunOptions{ProjectRoot: env.root, FeatureSlug: "rev-box", Utterance: "build it"})
	if err != nil {
		t.Fatal(err)
	}
	// Spec: the skeleton carries the implementation-marker warn.
	spec, err := env.eng.Review(env.root, "rev-box", ReviewSpec)
	if err != nil {
		t.Fatal(err)
	}
	marker := false
	for _, is := range spec.Issues {
		if is.Code == "implementation_review_marker_missing" {
			marker = true
		}
	}
	if !marker {
		t.Fatalf("spec issues = %+v", spec.Issues)
	}
	// Design: no design on record, spec present -> warn only.
	design, err := env.eng.Review(env.root, "rev-box", ReviewDesign)
	if err != nil {
		t.Fatal(err)
	}
	if design.Status != "warn" || !design.GateReady {
		t.Fatalf("design: %+v", design)
	}
	// Quality: nothing verified, package open -> fail, and the
	// write-back blocks the run through the H5 channel.
	quality, err := env.eng.Review(env.root, "rev-box", ReviewQuality)
	if err != nil {
		t.Fatal(err)
	}
	if quality.Status != "fail" || quality.GateReady {
		t.Fatalf("quality: %+v", quality)
	}
	if !quality.ReviewRecorded || quality.RouteReasonCode != core.ReasonPlannerReviewBlockingFinding {
		t.Fatalf("quality write-back: %+v", quality)
	}
	st, err := env.eng.Status(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Blocked {
		t.Fatalf("the quality review must block the run: %+v", st)
	}
}

// ---- M16: truth map standalone vs bundle ----

func TestTruthMapStandaloneAndBundleAgree(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "map-box")
	v, err := env.eng.TruthMapVerify(env.root, "map-box")
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != "pass" || !v.LayoutOK || v.AuthoritySource != TruthMapAuthoritySource || v.JSONAuthority {
		t.Fatalf("verify on skeleton: %+v", v)
	}
	r, err := env.eng.TruthMapResolve(env.root, "map-box")
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "resolved" || r.TruthDoc == "" {
		t.Fatalf("resolve: %+v", r)
	}
	// Break the layout: remove tests.md. Both faces must agree.
	ctx := env.ctx(t, "map-box")
	if err := os.Remove(filepath.Join(env.root, ctx.Feature.TestsPath)); err != nil {
		t.Fatal(err)
	}
	v, err = env.eng.TruthMapVerify(env.root, "map-box")
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != "blocked" || v.LayoutOK {
		t.Fatalf("verify on broken package: %+v", v)
	}
	run, err := env.eng.CreateRun(CreateRunOptions{ProjectRoot: env.root, FeatureSlug: "map-box", Utterance: "build it"})
	if err != nil {
		t.Fatal(err)
	}
	adm, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range adm.Items {
		if it.Check == BundleTruthMap && it.OK {
			t.Fatalf("bundle truth-map item must agree with the standalone verify (both fail): %+v", adm.Items)
		}
	}
}
