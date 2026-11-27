package orchestrator

import (
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

const (
	ModuleRunStateRunning = "running"
	ModuleRunStatePending = "pending"
)

// ModuleRunOptions are the inputs of one module expansion.
type ModuleRunOptions struct {
	ProjectRoot string
	Version     string
	NodeID      string
	By          string
}

// ModuleRunView is one feature run produced by a module expansion.
type ModuleRunView struct {
	RunID            string `json:"run_id"`
	FeatureID        string `json:"feature_id"`
	FeatureSlug      string `json:"feature_slug"`
	FeatureNodeID    string `json:"feature_node_id"`
	ModuleNodeID     string `json:"module_node_id"`
	Position         int    `json:"position"`
	State            string `json:"state"`
	PredecessorRunID string `json:"predecessor_run_id,omitempty"`
}

// ModuleRun expands one intent-tree module into one goal run per direct
// feature child. Runs keep declaration order; later runs stay pending at
// plan until their predecessor leaves that stage. Every run carries the
// same pool plan: one satellite per module in the tree.
func (e *Engine) ModuleRun(opts ModuleRunOptions) ([]ModuleRunView, error) {
	root := store.ResolveRootPath(opts.ProjectRoot)
	proj, err := e.store.Projects().GetByRootPath(root)
	if err != nil {
		return nil, err
	}
	vers, err := e.store.Projects().ListVersions(proj.ID)
	if err != nil {
		return nil, err
	}
	ver, ok := pickVersion(vers, opts.Version)
	if !ok {
		return nil, core.NewError(intenttree.ReasonVersionNotFound, "goal.ModuleRun", "version not found: "+opts.Version)
	}
	dir := intenttree.TreeDir(proj.RootPath, ver.DocsRoot)
	idx, err := intenttree.ReadIndex(dir)
	if err != nil {
		return nil, err
	}
	byID := map[string]intenttree.IndexEntry{}
	for _, n := range idx.Nodes {
		byID[n.ID] = n
	}
	module, ok := byID[opts.NodeID]
	if !ok {
		return nil, core.NewError(intenttree.ReasonNodeNotFound, "goal.ModuleRun", "module node not found: "+opts.NodeID)
	}
	if module.Kind != intenttree.KindModule {
		return nil, core.NewError(intenttree.ReasonInvalidChildKind, "goal.ModuleRun", "node is not a module: "+opts.NodeID)
	}
	children, err := moduleChildrenInDeclarationOrder(dir, module.ID, idx)
	if err != nil {
		return nil, err
	}
	if len(children) == 0 {
		return nil, core.NewError(intenttree.ReasonModuleEmpty, "goal.ModuleRun", "module has no feature children: "+opts.NodeID)
	}

	worktreeSlugs := moduleSlugs(idx)
	lakes := make([]LakeInput, 0, len(worktreeSlugs))
	for _, n := range idx.Nodes {
		if n.Kind != intenttree.KindModule {
			continue
		}
		lakes = append(lakes, LakeInput{Slug: worktreeSlugs[n.ID]})
	}
	plan := BuildParallelPlan(lakes, true)
	plan.Mode = ModulePoolMode
	plan.IntentSource = "module_tree"

	bodies := make([]intenttree.Node, len(children))
	slugs := make([]string, len(children))
	for i, child := range children {
		body, err := intenttree.ReadNode(dir, child.ID)
		if err != nil {
			return nil, err
		}
		slug := moduleFeatureSlug(idx, child, body)
		if err := e.admitModuleChild(proj, ver, child, body, slug); err != nil {
			return nil, err
		}
		bodies[i], slugs[i] = body, slug
	}

	var out []ModuleRunView
	predecessor := ""
	for i, child := range children {
		ctx, err := e.ensureDerivedFeature(proj, ver, slugs[i], child.ID, bodies[i])
		if err != nil {
			return nil, err
		}
		v, err := e.CreateRun(CreateRunOptions{
			ProjectRoot: proj.RootPath, Version: ver.Version, FeatureSlug: ctx.Feature.Slug,
			Utterance:  "module run " + module.ID + " -> " + child.ID,
			ExecutorID: strings.TrimSpace(opts.By), ExecutorMode: "manual",
		})
		if err != nil {
			return nil, err
		}
		state := ModuleRunStateRunning
		if i > 0 {
			state = ModuleRunStatePending
		}
		moduleSlug := worktreeSlugs[module.ID]
		info := &ModuleRunInfo{
			ModuleNodeID: module.ID, FeatureNodeID: child.ID,
			Position: i, State: state, PredecessorRunID: predecessor,
			ModuleSlug: moduleSlug,
		}
		run, m, _, err := e.loadRun(v.RunID)
		if err != nil {
			return nil, err
		}
		m.ModuleRun = info
		m.ParallelPlan = &plan
		if state == ModuleRunStatePending {
			if err := e.setStageStatus(run, m, 0, "pending"); err != nil {
				return nil, err
			}
		}
		if m.Implement == nil {
			m.Implement = &ImplementState{}
		}
		m.Implement.WorktreeRef = ".beacon/worktrees/satellite/" + moduleSlug
		if err := e.appendEvent(run, "module_run_dispatched", map[string]any{
			"module_node_id": module.ID, "feature_node_id": child.ID,
			"position": i, "state": state, "plan_mode": plan.Mode,
		}); err != nil {
			return nil, err
		}
		if err := e.saveRun(run, m); err != nil {
			return nil, err
		}
		out = append(out, ModuleRunView{
			RunID: v.RunID, FeatureID: run.FeatureID, FeatureSlug: ctx.Feature.Slug,
			FeatureNodeID: child.ID, ModuleNodeID: module.ID,
			Position: i, State: state, PredecessorRunID: predecessor,
		})
		predecessor = v.RunID
	}
	return out, nil
}

// admitModuleChild runs the shared truth admission judgement for one
// direct feature child before any write happens. Module-level refusals
// always report the AC's freeze_ack_required reason; the child's own
// reason stays in the detail for diagnosis.
func (e *Engine) admitModuleChild(proj store.Project, ver store.ProjectVersion, child intenttree.IndexEntry, body intenttree.Node, slug string) error {
	in, err := e.moduleChildExitInput(proj, ver, child, body, slug)
	if err != nil {
		return err
	}
	item := truthFrozenAdmissionItem(in)
	if item.OK {
		return nil
	}
	return core.NewError(core.ReasonFreezeAckRequired, "goal.ModuleRun",
		"child feature admission blocked: "+child.ID+" ("+item.ReasonCode+": "+item.Detail+")")
}

// moduleChildExitInput feeds one child into the shared admission item.
// A materialized feature package is judged by the shared FrozenFact
// through buildExitInput; a tree-only child is judged by its frozen tree
// body, which is exactly what ensureDerivedFeature would write as
// truth.md frontmatter.
func (e *Engine) moduleChildExitInput(proj store.Project, ver store.ProjectVersion, child intenttree.IndexEntry, body intenttree.Node, slug string) (ExitInput, error) {
	ctx, err := e.store.ResolveFeature(proj.RootPath, slug, ver.Version)
	if err == nil {
		run := store.Run{ID: "module-admission:" + child.ID, FeatureID: ctx.Feature.ID}
		m := &runMeta{CurrentStageIdx: 0, StagePlan: []StagePlanEntry{{Stage: StageImplement}}}
		return e.buildExitInput(run, m)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ExitInput{}, err
	}
	return ExitInput{
		HasFeature:          true,
		TruthPackagePresent: true,
		TruthFrozen:         moduleTreeStatusFrozen(body.Status),
		TruthDraftPresent:   body.Status == intenttree.StatusDraft,
		TruthChangePending:  body.Status == intenttree.StatusChangePending,
		FrozenSource:        FrozenSourceFrontmatter,
	}, nil
}

func moduleTreeStatusFrozen(status intenttree.Status) bool {
	switch status {
	case intenttree.StatusFrozen, intenttree.StatusImplemented, intenttree.StatusVerified, intenttree.StatusDelivered:
		return true
	default:
		return false
	}
}

// moduleChildrenInDeclarationOrder reads direct feature children and orders
// them by the parent's frozen ChildFingerprints. The index is stable but
// sorted by path; the parent body is the authority on declaration order.
func moduleChildrenInDeclarationOrder(dir, moduleID string, idx intenttree.Index) ([]intenttree.IndexEntry, error) {
	parent, err := intenttree.ReadNode(dir, moduleID)
	if err != nil {
		return nil, err
	}
	children := make([]intenttree.IndexEntry, 0)
	for _, n := range idx.Nodes {
		if n.ParentID == moduleID && n.Kind == intenttree.KindFeature {
			children = append(children, n)
		}
	}
	order := make(map[string]int, len(parent.ChildFingerprints))
	for i, fp := range parent.ChildFingerprints {
		order[fp] = i
	}
	sort.SliceStable(children, func(i, j int) bool {
		oi, oki := order[children[i].Fingerprint]
		oj, okj := order[children[j].Fingerprint]
		if oki && okj {
			return oi < oj
		}
		if oki != okj {
			return oki
		}
		return children[i].Path < children[j].Path
	})
	return children, nil
}

// moduleSlugs delegates to the tree-owned slug derivation so scheduler
// worktrees and tree gate lookups use the same canonical slugs.
func moduleSlugs(idx intenttree.Index) map[string]string {
	return intenttree.ModuleWorktreeSlugs(idx)
}

type moduleRunEvent struct {
	kind    string
	payload map[string]any
}

// writeModuleRun is the single module-scheduler write entry: stage
// status, module-run state and accompanying events commit together.
func (e *Engine) writeModuleRun(run store.Run, m *runMeta, idx int, stageStatus, moduleState string, extra ...moduleRunEvent) error {
	if idx < 0 || idx >= len(m.StagePlan) {
		return core.NewError(core.ReasonModuleStageNotFound, "goal.ModuleRun", "stage index out of range: "+strconv.Itoa(idx))
	}
	fromStage := m.StagePlan[idx].Status
	fromState := ""
	if m.ModuleRun != nil {
		fromState = m.ModuleRun.State
	}
	if fromStage == stageStatus && (moduleState == "" || fromState == moduleState) && len(extra) == 0 {
		return nil
	}
	if fromStage != stageStatus {
		m.StagePlan[idx].Status = stageStatus
	}
	if moduleState != "" && m.ModuleRun != nil {
		m.ModuleRun.State = moduleState
	}
	err := e.store.WithTx(func(tx *store.Tx) error {
		if fromStage != stageStatus {
			if err := e.appendEventOn(tx, run, "module_run_stage_transition", map[string]any{
				"stage_index": idx, "from": fromStage, "to": stageStatus,
				"module_node_id": moduleNodeID(m), "feature_node_id": moduleFeatureNodeID(m),
			}); err != nil {
				return err
			}
		}
		for _, ev := range extra {
			if err := e.appendEventOn(tx, run, ev.kind, ev.payload); err != nil {
				return err
			}
		}
		return e.saveRunOn(tx, run, m)
	})
	if err != nil {
		if fromStage != stageStatus {
			m.StagePlan[idx].Status = fromStage
		}
		if moduleState != "" && m.ModuleRun != nil {
			m.ModuleRun.State = fromState
		}
	}
	return err
}

// setStageStatus forwards stage-only transitions to the one write entry.
func (e *Engine) setStageStatus(run store.Run, m *runMeta, idx int, status string) error {
	return e.writeModuleRun(run, m, idx, status, "")
}

func moduleNodeID(m *runMeta) string {
	if m.ModuleRun == nil {
		return ""
	}
	return m.ModuleRun.ModuleNodeID
}

func moduleFeatureNodeID(m *runMeta) string {
	if m.ModuleRun == nil {
		return ""
	}
	return m.ModuleRun.FeatureNodeID
}

// moduleFeatureSlug delegates to the tree-owned canonical derivation so
// persistence, stage gates and HTML use one body-aware slug.
func moduleFeatureSlug(idx intenttree.Index, n intenttree.IndexEntry, body intenttree.Node) string {
	return intenttree.FeaturePackageSlug(idx, n.ID, body.EffectiveTruthPath())
}

// releaseModuleSuccessor moves the next pending run of the same module
// out of plan once its predecessor has left that stage.
func (e *Engine) releaseModuleSuccessor(runID string) error {
	_, m, _, err := e.loadRun(runID)
	if err != nil || m.ModuleRun == nil || m.ModuleRun.State != ModuleRunStateRunning {
		return err
	}
	runs, err := e.store.Runs().ListAll()
	if err != nil {
		return err
	}
	for _, candidate := range runs {
		if candidate.ID == runID {
			continue
		}
		raw := candidate.Metadata
		if len(raw) == 0 {
			continue
		}
		cm := &runMeta{}
		if err := json.Unmarshal(raw, cm); err != nil || cm.ModuleRun == nil {
			continue
		}
		if cm.ModuleRun.PredecessorRunID != runID || cm.ModuleRun.State != ModuleRunStatePending {
			continue
		}
		return e.writeModuleRun(candidate, cm, 0, "active", ModuleRunStateRunning, moduleRunEvent{
			kind: "module_run_released",
			payload: map[string]any{
				"module_node_id":     cm.ModuleRun.ModuleNodeID,
				"feature_node_id":    cm.ModuleRun.FeatureNodeID,
				"predecessor_run_id": runID,
			},
		})
	}
	return nil
}
