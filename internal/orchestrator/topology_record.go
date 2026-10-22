// Topology S3: recording and consumption of BranchContext
// projections.
//
// The store is the only state (charter rule 7) and the schema is
// frozen, so a projection is recorded as a branch_context_projected
// event on an anchor feature's trail — the same append-only trail
// every other kernel fact uses — carrying the full context in its
// payload. Resolution is project-wide: the latest projection on
// ANY feature trail of the project is the project's current
// BranchContext. A generated draft (source=generated_draft) is
// visible to `goal topology show` immediately, but admission only
// consumes it after a branch_context_confirmed event lands for its
// ref — a draft nobody confirmed is treated as missing, exactly
// like having no projection at all.
package orchestrator

import (
	"encoding/json"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/harness"
	"beacon-go/internal/store"
	"context"
)

// BranchContextRecord is a recorded projection plus its
// consumption state.
type BranchContextRecord struct {
	Context         BranchContext `json:"branch_context"`
	AnchorFeatureID string        `json:"anchor_feature_id"`
	Confirmed       bool          `json:"confirmed"`
}

// ProjectGitScopeGraph validates, projects and records a graph.
// The recorded gate status is a LIVE evaluation at projection
// time (a snapshot; admission re-evaluates live regardless).
func (e *Engine) ProjectGitScopeGraph(anchorFeatureID string, g GitScopeGraph, source, by string) (BranchContext, error) {
	if err := ValidateGitScopeGraph(g); err != nil {
		return BranchContext{}, err
	}
	fc, err := e.featureContextByID(anchorFeatureID)
	if err != nil {
		return BranchContext{}, err
	}
	bc := ProjectBranchContext(g)
	bc.Source = source
	bc.Confirmed = source == TopologySourceFile
	bc.ProjectedAt = time.Now().UTC().Format(time.RFC3339Nano)
	pkgRel := path.Dir(filepath.ToSlash(fc.Feature.TruthPath))
	verdict := e.evaluateTopologyLive(bc, fc.Project.RootPath, pkgRel)
	bc.TopologyGateStatus = verdict.Status
	if err := e.appendFeatureEvent(anchorFeatureID, EventBranchContextProjected, map[string]any{
		"branch_context": bc, "graph_hash": bc.GraphHash,
		"topology_gate_ref": bc.TopologyGateRef, "topology_gate_status": bc.TopologyGateStatus,
		"source": source, "confirmed": bc.Confirmed, "by": by,
	}); err != nil {
		return BranchContext{}, err
	}
	return bc, nil
}

// latestBranchContext resolves the project's current recorded
// projection: the newest branch_context_projected event across
// the project's feature trails, with its confirmation state (a
// draft is confirmed by a later branch_context_confirmed event
// naming its ref; a file projection is confirmed at record time).
func (e *Engine) latestBranchContext(projectID string) (*BranchContextRecord, error) {
	features, err := e.store.Projects().ListFeatures(projectID)
	if err != nil {
		return nil, err
	}
	var best *BranchContextRecord
	var bestAt string
	for _, f := range features {
		events, err := e.store.ListEvents(f.ID)
		if err != nil {
			return nil, err
		}
		var rec *BranchContextRecord
		var recAt string
		for _, ev := range events {
			var p map[string]json.RawMessage
			if len(ev.Payload) > 0 {
				_ = json.Unmarshal(ev.Payload, &p)
			}
			switch ev.EventType {
			case EventBranchContextProjected:
				var bc BranchContext
				if raw, ok := p["branch_context"]; ok {
					if err := json.Unmarshal(raw, &bc); err == nil {
						rec = &BranchContextRecord{Context: bc, AnchorFeatureID: f.ID, Confirmed: bc.Confirmed}
						recAt = ev.CreatedAt
					}
				}
			case EventBranchContextConfirmed:
				if rec != nil {
					var ref string
					if raw, ok := p["topology_gate_ref"]; ok {
						_ = json.Unmarshal(raw, &ref)
					}
					if ref == rec.Context.TopologyGateRef {
						rec.Confirmed = true
					}
				}
			}
		}
		if rec != nil && (best == nil || recAt >= bestAt) {
			best, bestAt = rec, recAt
		}
	}
	return best, nil
}

// ConfirmBranchContext confirms the project's latest draft
// projection (idempotent). The confirmation is its own trail
// event naming the ref — never an edit of the projection event.
func (e *Engine) ConfirmBranchContext(projectID, by string) (BranchContext, error) {
	rec, err := e.latestBranchContext(projectID)
	if err != nil {
		return BranchContext{}, err
	}
	if rec == nil {
		return BranchContext{}, core.NewError(core.ReasonTopologyGateMissing, "topology.ConfirmBranchContext",
			"no BranchContext projection is recorded for this project; project or detect one first")
	}
	if rec.Confirmed {
		return rec.Context, nil
	}
	if err := e.appendFeatureEvent(rec.AnchorFeatureID, EventBranchContextConfirmed, map[string]any{
		"topology_gate_ref": rec.Context.TopologyGateRef, "by": by,
	}); err != nil {
		return BranchContext{}, err
	}
	out := rec.Context
	out.Confirmed = true
	return out, nil
}

// ---- exported surfaces for the CLI layer ----

// ResolveProjectAnchor resolves a project root (+ optional feature
// slug) to the anchor feature a projection is recorded on and the
// project id resolution keys off.
func (e *Engine) ResolveProjectAnchor(projectRoot, slug string) (anchorFeatureID, projectID string, err error) {
	root := store.ResolveRootPath(projectRoot)
	fc, err := e.store.ResolveFeature(root, slug, "")
	if err != nil {
		return "", "", err
	}
	return fc.Feature.ID, fc.Project.ID, nil
}

// LatestBranchContext is the exported form of latestBranchContext.
func (e *Engine) LatestBranchContext(projectID string) (*BranchContextRecord, error) {
	return e.latestBranchContext(projectID)
}

// TopologyFact is the run fact layer's topology summary
// (AC-173's loop-start consumption): written at CreateRun, never
// blocking creation — the implement gate is where missing/blocked
// bites.
type TopologyFact struct {
	Status       string `json:"status"` // pass | blocked | missing | not_applicable
	Ref          string `json:"topology_gate_ref,omitempty"`
	TopologyKind string `json:"topology_kind,omitempty"`
	Detail       string `json:"detail,omitempty"`
}

// topologyFactFor computes the topology fact for one feature
// context (shared by CreateRun and the topology surfaces).
func (e *Engine) topologyFactFor(fc store.FeatureContext) *TopologyFact {
	pkgRel := path.Dir(filepath.ToSlash(fc.Feature.TruthPath))
	return e.topologyFactForProject(fc.Project.ID, fc.Project.RootPath, pkgRel)
}

func (e *Engine) topologyFactForProject(projectID, root, pkgRel string) *TopologyFact {
	rec, err := e.latestBranchContext(projectID)
	if err != nil || rec == nil {
		if harness.IsGitRepo(context.Background(), root) {
			return &TopologyFact{Status: TopologyGateMissing,
				Detail: "no BranchContext projection recorded for this git project (AC-173)"}
		}
		return &TopologyFact{Status: TopologyNotApplicable,
			Detail: "not a git repository and no projection recorded: topology gate does not apply"}
	}
	if !rec.Confirmed {
		return &TopologyFact{Status: TopologyGateMissing, Ref: rec.Context.TopologyGateRef,
			TopologyKind: rec.Context.TopologyKind,
			Detail:       "a generated draft projection exists but is not confirmed; run `beacon goal topology confirm` (AC-173 treats an unconfirmed draft as missing)"}
	}
	verdict := e.evaluateTopologyLive(rec.Context, root, pkgRel)
	return &TopologyFact{Status: verdict.Status, Ref: rec.Context.TopologyGateRef,
		TopologyKind: rec.Context.TopologyKind,
		Detail:       "live topology gate evaluation over the recorded BranchContext " + rec.Context.TopologyGateRef}
}

// AdmissionCheckTopologyGate is admission item 8 (AC-173).
const AdmissionCheckTopologyGate = "topology_gate"

// evalTopologyGateItem is the ONE topology judgement shared by
// admission item 8 and the bundle's branch-governance sub-result:
// no recorded projection on a git project -> topology_gate_missing;
// an unconfirmed draft -> topology_gate_missing; a live gate that
// is not pass -> topology_gate_blocked; non-git with no projection
// -> not applicable (the S1/S2 caliber: git-only checks never paint
// a non-git project red).
func (e *Engine) evalTopologyGateItem(run store.Run) (AdmissionItem, *TopologyGateVerdict, *BranchContextRecord, error) {
	item := AdmissionItem{Check: AdmissionCheckTopologyGate}
	fc, ok, err := e.featureContextOf(run.FeatureID)
	if err != nil {
		return item, nil, nil, err
	}
	if !ok {
		item.Applicable, item.ReasonCode = true, core.ReasonTopologyGateMissing
		item.Detail = "feature is not registered; no topology projection can be resolved"
		return item, nil, nil, nil
	}
	pkgRel := path.Dir(filepath.ToSlash(fc.Feature.TruthPath))
	rec, err := e.latestBranchContext(fc.Project.ID)
	if err != nil {
		return item, nil, nil, err
	}
	if rec == nil {
		if harness.IsGitRepo(context.Background(), fc.Project.RootPath) {
			item.Applicable, item.ReasonCode = true, core.ReasonTopologyGateMissing
			item.Detail = "no BranchContext projection recorded for this git project; " +
				"project one with `beacon goal topology project` or detect + confirm (AC-173)"
			return item, nil, nil, nil
		}
		item.OK, item.Applicable = true, false
		item.Detail = "not a git repository and no projection recorded: topology gate does not apply"
		return item, nil, nil, nil
	}
	if !rec.Confirmed {
		item.Applicable, item.ReasonCode = true, core.ReasonTopologyGateMissing
		item.Detail = "the recorded projection " + rec.Context.TopologyGateRef +
			" is an unconfirmed generated draft; run `beacon goal topology confirm` first (AC-173)"
		return item, nil, rec, nil
	}
	verdict := e.evaluateTopologyLive(rec.Context, fc.Project.RootPath, pkgRel)
	if verdict.Status != TopologyGatePass {
		item.Applicable, item.ReasonCode = true, core.ReasonTopologyGateBlocked
		item.Detail = "topology gate blocked over " + rec.Context.TopologyGateRef + ": " +
			joinScopeBlocks(verdict)
		return item, &verdict, rec, nil
	}
	item.OK, item.Applicable = true, true
	item.Detail = "topology gate pass over " + rec.Context.TopologyGateRef +
		" (" + rec.Context.TopologyKind + ", " + strconv.Itoa(len(rec.Context.Scopes)) + " scope(s))"
	return item, &verdict, rec, nil
}

func joinScopeBlocks(v TopologyGateVerdict) string {
	out := ""
	for _, sv := range v.Scopes {
		if sv.Status == TopologyGateBlocked {
			if out != "" {
				out += "; "
			}
			out += sv.ScopeID + ": " + sv.Detail
		}
	}
	if out == "" {
		out = "aggregate blocked"
	}
	return out
}
