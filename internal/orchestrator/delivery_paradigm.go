// Delivery paradigm (K1 / K2 / K3 / K6): the kernel-side pieces that
// make one delivery judgeable and recorded.
//
//   - Manual / external executors are first-class selectors: the run
//     is created knowingly (the CLI says it is not driven) and each
//     stage is recorded complete with `goal tick --by <identity>`,
//     instead of every tick reporting stage_executor_not_registered.
//   - Every stage that is actually left lands a persisted exit
//     judgement in gate_results (gate_type = the stage's gate axis),
//     so `gate status` and conformance read a real verdict.
//   - An external room can write a finished lake back onto its run
//     with artifact pointers (goal receive-result --external).
//   - Conformance judges one delivery as a whole: truth frozen, every
//     stage gate evaluated, a verifier attached, state exported.
package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// Executor selectors a goal run may record.
const (
	ExecutorModeManual     = "manual"
	ExecutorModeExternal   = "external"
	ExecutorModeRecording  = "recording"
	ExecutorModeSubprocess = "subprocess"
	ExecutorModeRalph      = "ralph"
)

// NormalizeRunExecutor validates the goal-run --executor selector.
// Unknown selectors are refused with stage_executor_unsupported BEFORE
// any run row is written (K1/K2 expectation: no silent creation that
// only fails on the first tick).
func NormalizeRunExecutor(selector string) (string, error) {
	s := strings.TrimSpace(selector)
	switch {
	case s == "", s == ExecutorModeManual, s == ExecutorModeExternal,
		s == ExecutorModeRecording, s == ExecutorModeSubprocess, s == ExecutorModeRalph:
		return s, nil
	case strings.HasPrefix(s, ExecutorModeSubprocess+":") && strings.TrimSpace(strings.TrimPrefix(s, ExecutorModeSubprocess+":")) != "":
		return s, nil
	}
	return "", core.NewError(core.ReasonExecutorUnsupported, "goal.CreateRun",
		"executor "+s+" is not supported; choose manual | external | recording | subprocess | subprocess:<profile> | ralph "+
			"(manual/external do not auto-drive: record each stage with `goal tick --by <identity>`)")
}

// isManualExecutor reports whether the run is advanced by an explicit
// tick identity rather than a registered executor.
func (m *runMeta) isManualExecutor() bool {
	return m != nil && (m.ExecutorMode == ExecutorModeManual || m.ExecutorMode == ExecutorModeExternal)
}

// stageGateType maps a product stage to the gate axis its exit
// judgement lands on ("" for a stage no gate covers).
func stageGateType(stage string) string {
	switch stage {
	case StagePlan:
		return string(gate.GatePlan)
	case StageTruth:
		return string(gate.GateTruthFreeze)
	case StageDesign:
		return string(gate.GateDesign)
	case StageImplement:
		return string(gate.GateImplement)
	case StageQA:
		return string(gate.GateQA)
	case StageRelease:
		return string(gate.GateRelease)
	}
	return ""
}

// StageGateView is the read-side fact `goal status` reports for the
// run's current stage: which gate governs it and whether that gate has
// a recorded verdict.
type StageGateView struct {
	Stage       string `json:"stage"`
	GateType    string `json:"gate_type"`
	Evaluated   bool   `json:"evaluated"`
	Verdict     string `json:"verdict,omitempty"`
	EvaluatedAt string `json:"evaluated_at,omitempty"`
}

// stageGateView reads the latest recorded verdict of one stage's gate.
func (e *Engine) stageGateView(featureID, stage string) *StageGateView {
	gt := stageGateType(stage)
	if gt == "" {
		return nil
	}
	v := &StageGateView{Stage: stage, GateType: gt}
	g, err := e.store.Gates().Latest(featureID, gt)
	if err != nil {
		return v
	}
	v.Evaluated = true
	v.Verdict = g.Verdict
	v.EvaluatedAt = g.EvaluatedAt
	return v
}

// recordStageExit persists the exit judgement of the stage the run is
// leaving: one gate_results row (the stage's gate axis) plus a
// stage_exit_evaluated trail event carrying who judged it and whether
// the judgement was derived or decided by a formal verdict. It runs
// inside the caller's write set, so the record and the advance commit
// as one unit — a stage can never be "left" without its record.
func (e *Engine) recordStageExit(w writeSet, run store.Run, m *runMeta, stage string, verdict ExitVerdict, by string) error {
	gt := stageGateType(stage)
	if gt == "" {
		return core.NewError(core.ReasonStageGateNotEvaluated, "goal.Tick",
			"stage "+stage+" has no gate axis; it cannot be evaluated or left")
	}
	reasons := []string{core.ReasonStageExitOK}
	if verdict.FormalVerdict != "" && verdict.FormalVerdict != "pass" {
		reasons = append(reasons, verdict.ReasonCode)
	}
	rec, err := w.Gates().Record(store.GateResult{
		FeatureID: run.FeatureID, GateType: gt, Verdict: "pass", ReasonCodes: reasons,
	})
	if err != nil {
		return err
	}
	judgeBy := strings.TrimSpace(by)
	if judgeBy == "" {
		judgeBy = "stage_exit"
	}
	payload := map[string]any{
		"stage": stage, "gate_type": gt, "verdict": "pass",
		"gate_result_id": rec.ID, "evaluator": judgeBy,
		"derived": verdict.Derived, "formal_verdict": verdict.FormalVerdict,
		"frozen_source": verdict.FrozenSource,
	}
	if m.isManualExecutor() {
		payload["executor"] = m.ExecutorMode
		payload["completed_by"] = judgeBy
	}
	return e.appendEventOn(w, run, "stage_exit_evaluated", payload)
}

// ---- External result (K3) ----

// ExternalResult is the recorded return of an external room: who
// reported it, what it says, and the artifact pointers it brought.
type ExternalResult struct {
	By          string   `json:"by"`
	Summary     string   `json:"summary"`
	ArtifactIDs []string `json:"artifact_ids,omitempty"`
	At          string   `json:"at"`
}

// ExternalResultInput is one `goal receive-result --external` call.
type ExternalResultInput struct {
	By        string
	Summary   string
	Artifacts []ArtifactInput
}

// ReceiveExternalResult writes an externally finished lake back onto
// its run: the artifact pointers are registered (absolute paths), the
// completion event names the external source, and the run closes to
// done so `goal program route` reports the lake completed. It does not
// invent a human release: the trail says external, and conformance
// keeps reporting verifier/export separately.
func (e *Engine) ReceiveExternalResult(runID string, in ExternalResultInput) (RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	if strings.TrimSpace(in.By) == "" {
		return RunView{}, core.NewError(core.ReasonExternalActorRequired, "goal.ReceiveExternalResult",
			"an external result needs the reporting identity (--by)")
	}
	if strings.TrimSpace(in.Summary) == "" {
		return RunView{}, core.NewError(core.ReasonExternalSummRequired, "goal.ReceiveExternalResult",
			"an external result needs a summary (--summary)")
	}
	if len(in.Artifacts) == 0 {
		return RunView{}, core.NewError(core.ReasonResultArtifactsMissing, "goal.ReceiveExternalResult",
			"an external result must bring at least one --artifact absolute path")
	}
	ids, err := e.registerArtifacts(runID, in.Artifacts)
	if err != nil {
		return RunView{}, err
	}
	at := time.Now().UTC().Format(time.RFC3339Nano)
	ls := m.loopState()
	if ls.CurrentStage != core.LoopReleaseGate {
		if err := ls.Transition(core.LoopReleaseGate, false, e.stopWhen); err != nil {
			return RunView{}, err
		}
	}
	m.applyLoopState(ls)
	// The external room's completion closes the run WITHOUT claiming a
	// human release: ReleaseHumanConfirmed and ReleaseConfirmed stay
	// untouched, the trail says source=external, and the recorded
	// ExternalResult carries who reported it. This is the one completion
	// path that does not go through Engine.Complete, and it says so.
	m.LoopStage = string(core.LoopDone)
	m.StageHistory = append(m.StageHistory, string(core.LoopDone))
	for i := range m.StagePlan {
		m.StagePlan[i].Status = "done"
	}
	m.CurrentStageIdx = len(m.StagePlan)
	m.ExternalResult = &ExternalResult{By: in.By, Summary: in.Summary, ArtifactIDs: ids, At: at}
	m.Blocked = false
	m.BlockedStreak = 0
	m.LastBlockReason = ""
	if err := e.store.WithTx(func(tx *store.Tx) error {
		if err := e.appendEventOn(tx, run, "external_result_received", map[string]any{
			"by": in.By, "summary": in.Summary, "artifact_ids": ids,
		}); err != nil {
			return err
		}
		if err := e.appendEventOn(tx, run, "goal_completed", map[string]any{
			"source": "external", "external_by": in.By, "summary": in.Summary,
			"human_confirmed": false,
		}); err != nil {
			return err
		}
		if err := e.saveRunOn(tx, run, m); err != nil {
			return err
		}
		// The lifecycle snapshot deliberately stays where the run was:
		// an external completion is not a human release (Engine.Complete
		// owns that transition), and the run's succeeded status is what
		// program route reads as completed.
		return nil
	}); err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, slug), nil
}

// ---- Conformance (K6) ----

// ConformanceItem is one delivery-paradigm check.
type ConformanceItem struct {
	Check  string `json:"check"`
	Status string `json:"status"` // pass | fail
	Detail string `json:"detail,omitempty"`
}

// ConformanceView answers one question: is this run a delivery the
// kernel can recognise? Four facts, each judged from stored evidence.
type ConformanceView struct {
	RunID       string            `json:"run_id"`
	FeatureSlug string            `json:"feature_slug,omitempty"`
	Pass        bool              `json:"pass"`
	Failed      []string          `json:"failed,omitempty"`
	Items       []ConformanceItem `json:"items"`
}

// Conformance checks truth_frozen / gates_evaluated /
// verifier_attached / state_exported for one run.
func (e *Engine) Conformance(runID string) (ConformanceView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return ConformanceView{}, err
	}
	v := ConformanceView{RunID: runID, FeatureSlug: slug}
	add := func(check string, ok bool, detail string) {
		status := "fail"
		if ok {
			status = "pass"
		} else {
			v.Failed = append(v.Failed, check)
		}
		v.Items = append(v.Items, ConformanceItem{Check: check, Status: status, Detail: detail})
	}

	ff, ferr := e.frozenFactByID(run.FeatureID)
	if ferr != nil {
		add("truth_frozen", false, ferr.Error())
	} else {
		add("truth_frozen", ff.Frozen, "frozen source: "+ff.Source)
	}

	// gates_evaluated: every stage in the run's plan must have a
	// recorded pass verdict on its gate axis. A missing row is an
	// unevaluated gate; a blocked/fail row is an evaluated gate that
	// still forbids calling the delivery done.
	var gateProblems []string
	for _, entry := range m.StagePlan {
		gt := stageGateType(entry.Stage)
		if gt == "" {
			gateProblems = append(gateProblems, entry.Stage+": no gate axis")
			continue
		}
		// The run itself must have LEFT the stage: a feature-scoped
		// verdict recorded by another run of the same feature is not
		// this run's delivery.
		if entry.Status != "done" {
			gateProblems = append(gateProblems, entry.Stage+": stage not completed ("+entry.Status+")")
			continue
		}
		g, gerr := e.store.Gates().Latest(run.FeatureID, gt)
		if gerr != nil {
			gateProblems = append(gateProblems, entry.Stage+": gate "+gt+" not evaluated")
			continue
		}
		if g.Verdict != "pass" {
			gateProblems = append(gateProblems, entry.Stage+": gate "+gt+" verdict "+g.Verdict)
		}
	}
	if len(gateProblems) == 0 {
		add("gates_evaluated", true, "every stage gate carries a recorded pass")
	} else {
		add("gates_evaluated", false, strings.Join(gateProblems, "; "))
	}

	verifierOK := m.Verifier != nil && m.Verifier.Present && len(m.Verifier.Refs) > 0
	detail := "no independent verifier attached"
	if verifierOK {
		detail = "verifier " + m.Verifier.Source + " with " + strings.Join(m.Verifier.Refs, ", ")
	}
	add("verifier_attached", verifierOK, detail)

	exported, exportPath := e.stateExported(run.FeatureID)
	add("state_exported", exported, exportPath)

	v.Pass = len(v.Failed) == 0
	return v, nil
}

// stateExported reports whether the run's project carries the
// portable state snapshot produced by `projects export-state`.
func (e *Engine) stateExported(featureID string) (bool, string) {
	feat, err := e.store.Features().GetByID(featureID)
	if err != nil {
		return false, "feature not found"
	}
	ver, err := e.store.Projects().GetVersionByID(feat.ProjectVersionID)
	if err != nil {
		return false, "project version not found"
	}
	proj, err := e.store.Projects().GetByID(ver.ProjectID)
	if err != nil {
		return false, "project not found"
	}
	abs := filepath.Join(proj.RootPath, StateSnapshotRelPath)
	if info, serr := os.Stat(abs); serr == nil && info.Mode().IsRegular() {
		return true, "snapshot " + abs
	}
	return false, "no state snapshot at " + abs + " (run `beacon-go projects export-state --project <root>`)"
}
