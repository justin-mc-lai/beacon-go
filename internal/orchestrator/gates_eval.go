// Formal gate evaluation service (P3a): the write-back half of the
// beacon-gate surface. The pure verdicts live in internal/gate; this
// service gathers the facts from the store + the feature package on
// disk, runs the evaluator, and persists EVERY evaluation as one
// gate_results row + one lifecycle_events trail entry (AC-013).
//
// Re-evaluation appends a new row (gate_results is a ledger;
// LatestByType reads the newest) and is deterministic: the same
// facts yield the same verdict and reason codes.
//
// An evaluator identity is mandatory — an unattributed verdict is
// worth nothing, so an empty evaluator is refused before any fact
// is read (gate_evaluator_required).
package orchestrator

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// Event types written by the gate service.
const (
	EventGateEvaluated           = "gate_evaluated"
	EventTruthIntegrityEvaluated = "truth_integrity_evaluated"
)

// GateTypes are the five stage gates `gate evaluate` accepts (the
// integrity gate has its own entry, EvaluateIntegrity).
var evaluableGateTypes = map[string]bool{
	string(gate.GateTruthFreeze): true,
	string(gate.GateDesign):      true,
	string(gate.GateImplement):   true,
	string(gate.GateQA):          true,
	string(gate.GateRelease):     true,
}

// resolvePkgPath resolves a recorded package path against the
// project root (absolute paths and ~/ are honoured as-is).
func resolvePkgPath(root, p string) string {
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return filepath.Join(root, p)
}

func readPkgFile(root, recorded string) gate.PackageFile {
	pf := gate.PackageFile{Path: recorded}
	if recorded == "" {
		return pf
	}
	raw, err := os.ReadFile(resolvePkgPath(root, recorded))
	if err != nil {
		return pf
	}
	pf.Present = true
	pf.Text = string(raw)
	return pf
}

// siblingPkgPath returns the recorded path of a package sibling
// (tests.md / tasks.md / evidence.md next to the truth file).
func siblingPkgPath(recordedTruth, name string) string {
	if recordedTruth == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(recordedTruth), name)
}

// latestGoalRun resolves the run a gate evaluation binds to: the
// explicit run when given (it must belong to the feature), else the
// newest goal run of the feature. hasRun=false when none exists.
func (e *Engine) latestGoalRun(ctx store.FeatureContext, runID string) (store.Run, *runMeta, bool, error) {
	if runID != "" {
		run, m, _, err := e.loadRun(runID)
		if err != nil {
			return store.Run{}, nil, false, err
		}
		if run.FeatureID != ctx.Feature.ID {
			return store.Run{}, nil, false, core.NewError(core.ReasonGateRunMismatch, "gate.resolveRun",
				"run "+runID+" belongs to feature "+run.FeatureID+", not "+ctx.Feature.ID)
		}
		return run, m, true, nil
	}
	runs, err := e.store.Runs().ListByFeature(ctx.Feature.ID)
	if err != nil {
		return store.Run{}, nil, false, err
	}
	for i := len(runs) - 1; i >= 0; i-- {
		if runs[i].RunKind != "goal" {
			continue
		}
		_, m, _, err := e.loadRun(runs[i].ID)
		if err != nil {
			return store.Run{}, nil, false, err
		}
		return runs[i], m, true, nil
	}
	return store.Run{}, nil, false, nil
}

// persistGateResult writes the gate_results row + the trail event
// and stamps the evaluation with the persisted identity. The two
// writes commit as ONE transaction (P6 write-path fix): a verdict
// can never land without its trail entry or vice versa.
func (e *Engine) persistGateResult(ctx store.FeatureContext, ev gate.Evaluation, runID, eventType string, extra map[string]any) (gate.Evaluation, error) {
	err := e.store.WithTx(func(tx *store.Tx) error {
		rec, err := tx.Gates().Record(store.GateResult{
			FeatureID: ctx.Feature.ID, GateType: string(ev.GateType),
			Verdict: string(ev.Verdict), ReasonCodes: ev.ReasonCodes,
		})
		if err != nil {
			return err
		}
		ev.GateResultID = rec.ID
		ev.EvaluatedAt = rec.EvaluatedAt
		ev.FeatureID = ctx.Feature.ID
		ev.FeatureSlug = ctx.Feature.Slug
		ev.RunID = runID
		payload := map[string]any{
			"gate_type": string(ev.GateType), "verdict": string(ev.Verdict),
			"reason_codes": ev.ReasonCodes, "evaluator": ev.Evaluator,
			"gate_result_id": rec.ID,
		}
		for k, v := range extra {
			payload[k] = v
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		runRef := sql.NullString{}
		if runID != "" {
			runRef = sql.NullString{String: runID, Valid: true}
		}
		_, err = tx.AppendEvent(store.LifecycleEvent{
			FeatureID: ctx.Feature.ID, RunID: runRef, EventType: eventType, Payload: raw,
		})
		return err
	})
	if err != nil {
		return gate.Evaluation{}, err
	}
	return ev, nil
}

// EvaluateGate formally evaluates one of the five stage gates for a
// feature and persists the verdict. runID binds run-scoped gates
// (design / implement / release) to a specific run; empty means the
// feature's newest goal run.
func (e *Engine) EvaluateGate(ctx store.FeatureContext, gateType, runID, evaluator string) (gate.Evaluation, error) {
	if strings.TrimSpace(evaluator) == "" {
		return gate.Evaluation{}, core.NewError(core.ReasonGateEvaluatorRequired, "gate.EvaluateGate",
			"an evaluator identity is required (--evaluator); unattributed verdicts are not recorded")
	}
	if !evaluableGateTypes[gateType] {
		return gate.Evaluation{}, core.NewError(core.ReasonGateTypeUnknown, "gate.EvaluateGate",
			"gate type "+gateType+" is not evaluable here (five stage gates; truth_integrity runs via `gate integrity`)")
	}
	var ev gate.Evaluation
	var boundRun string
	var err error
	switch gate.GateType(gateType) {
	case gate.GateTruthFreeze:
		ev, err = e.evalTruthFreeze(ctx, evaluator)
	case gate.GateDesign:
		run, _, has, rerr := e.latestGoalRun(ctx, runID)
		if rerr != nil {
			return gate.Evaluation{}, rerr
		}
		if has {
			boundRun = run.ID
			ev, err = e.evalDesign(ctx, run, evaluator)
		} else {
			ev = gate.EvaluateDesign(gate.DesignFacts{HasRun: false}, evaluator)
		}
	case gate.GateImplement:
		run, _, has, rerr := e.latestGoalRun(ctx, runID)
		if rerr != nil {
			return gate.Evaluation{}, rerr
		}
		if has {
			boundRun = run.ID
			ev, err = e.evalImplement(ctx, run, evaluator)
		} else {
			ev = gate.EvaluateImplement(gate.ImplementFacts{HasRun: false}, evaluator)
		}
	case gate.GateQA:
		// QA facts are feature-level; an explicit --run is validated
		// for membership and recorded as the evaluation's binding.
		if runID != "" {
			if _, _, _, verr := e.latestGoalRun(ctx, runID); verr != nil {
				return gate.Evaluation{}, verr
			}
			boundRun = runID
		}
		ev, err = e.evalQA(ctx, evaluator)
	case gate.GateRelease:
		if runID != "" {
			if _, _, _, verr := e.latestGoalRun(ctx, runID); verr != nil {
				return gate.Evaluation{}, verr
			}
			boundRun = runID
		}
		ev, err = e.evalRelease(ctx, evaluator)
	}
	if err != nil {
		return gate.Evaluation{}, err
	}
	return e.persistGateResult(ctx, ev, boundRun, EventGateEvaluated, nil)
}

// GateStatus returns the latest recorded verdict per gate type
// (read-only; the `gate status` / spine read side).
func (e *Engine) GateStatus(ctx store.FeatureContext) ([]store.GateResult, error) {
	return e.store.Gates().LatestByType(ctx.Feature.ID)
}
