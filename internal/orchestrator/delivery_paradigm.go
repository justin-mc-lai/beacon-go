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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	// Source names where the judgement came from. Only this run's own
	// stage_exit_evaluated record is a run-scoped fact; another run's
	// verdict on the same feature must never appear as this run's gate.
	Source string `json:"source,omitempty"`
}

// stageGateView reads the verdict THIS RUN recorded for the stage. A
// run that has not left a stage has no exit record, so the view says
// evaluated=false even when another run of the same feature already
// passed the same gate axis (review B3: no cross-run green).
func (e *Engine) stageGateView(runID, stage string) *StageGateView {
	gt := stageGateType(stage)
	if gt == "" {
		return nil
	}
	v := &StageGateView{Stage: stage, GateType: gt}
	byGate, byStage, err := e.runGateFacts(runID)
	if err != nil {
		return v
	}
	if rec, ok := byStage[stage]; ok {
		v.Evaluated = true
		v.Verdict = rec.Verdict
		v.EvaluatedAt = rec.EvaluatedAt
		v.Source = rec.Source
		return v
	}
	// A formal gate evaluation bound to this run is also run-scoped
	// evidence (e.g. `gate evaluate design --run <id>`), never another
	// run's row.
	if rec, ok := byGate[gt]; ok {
		v.Evaluated = true
		v.Verdict = rec.Verdict
		v.EvaluatedAt = rec.EvaluatedAt
		v.Source = rec.Source
	}
	return v
}

// runGateRecord is one gate fact attributed to a run by its own trail.
type runGateRecord struct {
	GateType     string
	Stage        string
	Verdict      string
	ReasonCodes  []string
	Evaluator    string
	EvaluatedAt  string
	GateResultID string
	Source       string // stage_exit | formal_gate | admission_bundle
}

// runGateFacts reads the gates THIS RUN evaluated, from its own events:
// stage exits (recorded in the same transaction as the advance), formal
// gate evaluations bound to the run, and admission bundles. It never
// consults the feature-level latest row — that row is shared by every
// run of the feature and is the source of the cross-run false green
// review B3 reproduced.
func (e *Engine) runGateFacts(runID string) (map[string]runGateRecord, map[string]runGateRecord, error) {
	byGate := map[string]runGateRecord{}
	byStage := map[string]runGateRecord{}
	events, err := e.store.ListEventsByRun(runID)
	if err != nil {
		return byGate, byStage, err
	}
	type gatePayload struct {
		Stage       string   `json:"stage"`
		GateType    string   `json:"gate_type"`
		Verdict     string   `json:"verdict"`
		ReasonCodes []string `json:"reason_codes"`
		Evaluator   string   `json:"evaluator"`
		GateResult  string   `json:"gate_result_id"`
	}
	for _, ev := range events {
		source := ""
		switch ev.EventType {
		case "stage_exit_evaluated":
			source = "stage_exit"
		case EventGateEvaluated:
			source = "formal_gate"
		case EventAdmissionBundleEvaluated:
			source = "admission_bundle"
		default:
			continue
		}
		if len(ev.Payload) == 0 {
			continue
		}
		var p gatePayload
		if err := json.Unmarshal(ev.Payload, &p); err != nil || p.GateType == "" {
			continue
		}
		rec := runGateRecord{
			GateType: p.GateType, Stage: p.Stage, Verdict: p.Verdict,
			ReasonCodes: p.ReasonCodes, Evaluator: p.Evaluator,
			EvaluatedAt: ev.CreatedAt, GateResultID: p.GateResult, Source: source,
		}
		byGate[p.GateType] = rec
		if p.Stage != "" {
			byStage[p.Stage] = rec
		}
	}
	return byGate, byStage, nil
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
		"evaluated_at": rec.EvaluatedAt,
		"derived":      verdict.Derived, "formal_verdict": verdict.FormalVerdict,
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
	// Review B1: the external room reports that the LAKE finished; it
	// does not testify that this run evaluated every stage gate. The
	// stage plan therefore keeps whatever status the run actually
	// reached — marking all stages "done" here made an unevaluated run
	// look conformant. The completion itself is the loop state + the
	// external_result_received / goal_completed(source=external) trail.
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
	Status string `json:"status"` // pass | fail | not_required
	Detail string `json:"detail,omitempty"`
	// ReasonCodes / Missing are the machine-readable half of a failed
	// traceability item: the frozen reason code(s) and the AC/TC ids
	// that are not covered.
	ReasonCodes []string `json:"reason_codes,omitempty"`
	Missing     []string `json:"missing,omitempty"`
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
	} else if ff.Frozen {
		add("truth_frozen", true, "frozen source: "+ff.Source)
	} else {
		// P2: an unfrozen package must not print a "frozen source".
		add("truth_frozen", false, "truth package is not frozen (status source: "+ff.Source+")")
	}

	// gates_evaluated: every stage in the run's plan must carry THIS
	// RUN's own evaluation. The evidence is the run's stage_exit_evaluated
	// trail (resolved to its gate_results row), never StagePlan[i].Status
	// (self-reported) and never the feature-level latest row (shared by
	// sibling runs). Release is the human-gated stage: its evidence is a
	// formal release gate evaluation bound to this run.
	byGate, byStage, evErr := e.runGateFacts(runID)
	var gateProblems []string
	if evErr != nil {
		gateProblems = append(gateProblems, "run trail unreadable: "+evErr.Error())
	} else {
		for _, entry := range m.StagePlan {
			gt := stageGateType(entry.Stage)
			if gt == "" {
				gateProblems = append(gateProblems, entry.Stage+": no gate axis")
				continue
			}
			var rec runGateRecord
			var ok bool
			if entry.Stage == StageRelease {
				rec, ok = byGate[gt]
				if !ok || rec.Source != "formal_gate" {
					gateProblems = append(gateProblems, entry.Stage+": no release gate verdict bound to this run")
					continue
				}
			} else {
				rec, ok = byStage[entry.Stage]
				if !ok {
					gateProblems = append(gateProblems, entry.Stage+": stage exit not evaluated by this run")
					continue
				}
				if rec.GateType != gt {
					gateProblems = append(gateProblems, entry.Stage+": exit recorded gate "+rec.GateType+", want "+gt)
					continue
				}
			}
			if rec.Verdict != "pass" {
				gateProblems = append(gateProblems, entry.Stage+": verdict "+rec.Verdict)
				continue
			}
			if rec.GateResultID == "" {
				gateProblems = append(gateProblems, entry.Stage+": exit record carries no gate result")
				continue
			}
			g, gerr := e.store.Gates().GetByID(rec.GateResultID)
			if gerr != nil {
				gateProblems = append(gateProblems, entry.Stage+": recorded gate result missing")
				continue
			}
			if g.Verdict != rec.Verdict {
				gateProblems = append(gateProblems, entry.Stage+": trail verdict differs from the stored gate result")
			}
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

	// K7 (kernel-truth-traceability AC-001/002/003): a package that
	// declares the traceability contract must prove AC -> TC ->
	// in-package evidence. Undeclared historical packages report
	// not_required and keep the four judgements above authoritative.
	tv := e.conformanceTraceability(run)
	v.Items = append(v.Items, ConformanceItem{
		Check: conformanceTraceabilityCheck, Status: tv.Status, Detail: tv.Detail,
		ReasonCodes: tv.ReasonCodes, Missing: tv.Missing,
	})
	if tv.Status == "fail" {
		v.Failed = append(v.Failed, traceabilityFailureLabel(tv))
	}

	v.Pass = len(v.Failed) == 0
	return v, nil
}

const conformanceTraceabilityCheck = "ac_tc_evidence_coverage"

// traceabilityVerdict is the ac_tc_evidence_coverage judgement: the
// item status (pass | fail | not_required), the reason codes that
// failed, and the AC/TC ids the failure names.
type traceabilityVerdict struct {
	Status      string
	Detail      string
	ReasonCodes []string
	Missing     []string
}

// traceabilityFailureLabel is the `failed` entry for a coverage
// failure. It carries the check name, the reason code and the missing
// ids so the JSON `failed` array is actionable on its own (AC-002).
func traceabilityFailureLabel(tv traceabilityVerdict) string {
	label := conformanceTraceabilityCheck
	if len(tv.ReasonCodes) > 0 {
		label += ":reason=" + strings.Join(tv.ReasonCodes, "+")
	}
	if len(tv.Missing) > 0 {
		label += ":missing=" + strings.Join(tv.Missing, "|")
	}
	return label
}

// conformanceTraceability judges AC -> TC -> evidence for the run's
// package. The contract is opt-in: only a package whose truth
// frontmatter says `traceability_required: true` is hard-checked.
// Missing tests.md, an unparseable matrix, no AC definition, a soft or
// docs-only command, a missing Evidence column, an out-of-package
// evidence path and a non-existent evidence file all fail closed.
func (e *Engine) conformanceTraceability(run store.Run) traceabilityVerdict {
	fail := func(codes, missing []string, detail string) traceabilityVerdict {
		return traceabilityVerdict{
			Status: "fail", ReasonCodes: codes, Missing: missing,
			Detail: strings.Join(codes, "+") + ": " + detail,
		}
	}
	ctx, err := e.featureContextByID(run.FeatureID)
	if err != nil {
		return fail([]string{core.ReasonTraceabilityMissingTC}, nil,
			"feature package unreadable: "+err.Error())
	}
	pkg := e.packageFiles(ctx)
	if !pkg.Truth.Present {
		return fail([]string{core.ReasonTraceabilityMissingTC}, nil,
			"truth.md missing; coverage cannot be judged (fail-closed)")
	}
	scalars, _, ok := gate.ParseFrontmatter(pkg.Truth.Text)
	if !ok {
		return fail([]string{core.ReasonTraceabilityMissingTC}, nil,
			"truth.md frontmatter unreadable; coverage cannot be judged (fail-closed)")
	}
	if !strings.EqualFold(strings.TrimSpace(scalars["traceability_required"]), "true") {
		return traceabilityVerdict{
			Status: "not_required",
			Detail: "package does not declare traceability_required: true; the four base judgements decide conformance",
		}
	}
	defined := gate.DefinedACs(pkg.Truth.Text)
	if len(defined) == 0 {
		return fail([]string{core.ReasonTraceabilityMissingTC}, nil,
			"truth defines no AC; coverage cannot be judged (fail-closed)")
	}
	ids := make([]string, 0, len(defined))
	for id := range defined {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if !pkg.Tests.Present {
		return fail([]string{core.ReasonTraceabilityMissingTC}, ids,
			"tests.md missing; AC without TC: "+strings.Join(ids, ", "))
	}
	rows, hasEvidenceColumn, matrixOK := gate.ParseCoverageMatrix(pkg.Tests.Text)
	if !matrixOK {
		return fail([]string{core.ReasonTraceabilityMissingTC}, ids,
			"tests.md has no parseable AC ID coverage matrix; AC without TC: "+strings.Join(ids, ", "))
	}
	root := ctx.Project.RootPath
	pkgDir := filepath.Dir(resolvePkgPath(root, ctx.Feature.TruthPath))
	evidenceDir := filepath.Join(pkgDir, "evidence")

	byAC := map[string][]gate.TCRow{}
	for _, row := range rows {
		if !defined[row.AC] {
			continue // unknown AC ids are the integrity gate's finding
		}
		if row.IsSoft() {
			continue
		}
		if strings.TrimSpace(row.Command) == "" || gate.IsDocsOnlyCommand(row.Command) {
			continue
		}
		byAC[row.AC] = append(byAC[row.AC], row)
	}
	var missingTC []string
	for _, id := range ids {
		if len(byAC[id]) == 0 {
			missingTC = append(missingTC, id)
		}
	}
	var missingEv, evDetail []string
	for _, id := range ids {
		for _, row := range byAC[id] {
			tc := row.TC
			if tc == "" {
				tc = "(no TC ID)"
			}
			ref := strings.Trim(strings.TrimSpace(row.Evidence), "`")
			if !hasEvidenceColumn || ref == "" {
				missingEv = append(missingEv, id+"/"+tc)
				evDetail = append(evDetail, id+"/"+tc+" has no Evidence path")
				continue
			}
			abs, inside := traceabilityEvidencePath(root, pkgDir, evidenceDir, ref)
			if !inside {
				missingEv = append(missingEv, id+"/"+tc)
				evDetail = append(evDetail, id+"/"+tc+" evidence "+ref+" is outside the package evidence/ directory")
				continue
			}
			info, serr := os.Stat(abs)
			if serr != nil || !info.Mode().IsRegular() {
				missingEv = append(missingEv, id+"/"+tc)
				evDetail = append(evDetail, id+"/"+tc+" evidence file does not exist: "+ref)
			}
		}
	}
	var codes, details, missing []string
	if len(missingTC) > 0 {
		codes = append(codes, core.ReasonTraceabilityMissingTC)
		details = append(details, "AC without executable TC: "+strings.Join(missingTC, ", "))
		missing = append(missing, missingTC...)
	}
	if len(missingEv) > 0 {
		codes = append(codes, core.ReasonTraceabilityMissingEvidence)
		details = append(details, "TC without in-package evidence: "+strings.Join(evDetail, "; "))
		missing = append(missing, missingEv...)
	}
	if len(codes) > 0 {
		return fail(codes, missing, strings.Join(details, "; "))
	}
	tcCount := 0
	for _, trs := range byAC {
		tcCount += len(trs)
	}
	return traceabilityVerdict{
		Status: "pass",
		Detail: fmt.Sprintf("%d AC covered by %d executable TC(s) with in-package evidence", len(ids), tcCount),
	}
}

// traceabilityEvidencePath resolves one Evidence cell against the
// package's evidence directory. Both the project-relative form
// (docs/.../evidence/x.json) and the package-relative form
// (evidence/x.json) are accepted; anything that resolves outside the
// evidence directory is refused.
func traceabilityEvidencePath(root, pkgDir, evidenceDir, ref string) (string, bool) {
	clean := filepath.Clean(filepath.FromSlash(ref))
	candidates := []string{filepath.Join(root, clean), filepath.Join(pkgDir, clean)}
	if filepath.IsAbs(clean) {
		candidates = []string{clean}
	}
	base := filepath.Clean(evidenceDir)
	for _, cand := range candidates {
		rel, err := filepath.Rel(base, filepath.Clean(cand))
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		return filepath.Clean(cand), true
	}
	return "", false
}

// stateExported reports whether the run's project carries the
// portable state snapshot produced by `projects export-state`, OR the
// feature trail carries a state_snapshot_exported record (truth Q5:
// either evidence counts; only when both are missing does it fail).
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
	if events, eerr := e.store.ListEvents(featureID); eerr == nil {
		for i := len(events) - 1; i >= 0; i-- {
			if events[i].EventType != EventStateExported {
				continue
			}
			var p struct {
				ExportedAt string `json:"exported_at"`
			}
			if len(events[i].Payload) > 0 {
				_ = json.Unmarshal(events[i].Payload, &p)
			}
			return true, "export record " + p.ExportedAt + " (snapshot file absent at " + abs + ")"
		}
	}
	return false, "no state snapshot at " + abs + " (run `beacon-go projects export-state --project <root>`)"
}
