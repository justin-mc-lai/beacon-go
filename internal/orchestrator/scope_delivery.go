package orchestrator

import (
	"encoding/json"
	"sort"
	"strings"

	"beacon-go/internal/intenttree"
)

type DeliveryTask struct {
	ID            string   `json:"id"`
	FeatureNodeID string   `json:"feature_node_id"`
	ModuleNodeID  string   `json:"module_node_id"`
	DependsOn     []string `json:"depends_on"`
	Status        string   `json:"status"`
}

// scopeTaskStatus derives a task's progress from canonical leaf states, so
// the task view never becomes a second ledger. verified/delivered outrank
// implemented; any pending leaf keeps the task pending.
func scopeTaskStatus(featureID string, bodies map[string]intenttree.Node) string {
	leaves, implemented, verified := 0, 0, 0
	for _, n := range bodies {
		if n.ParentID != featureID || n.Kind != intenttree.KindTouchpoint || workflowDeprecated(n.ID, bodies) {
			continue
		}
		leaves++
		switch n.Status {
		case intenttree.StatusVerified, intenttree.StatusDelivered:
			implemented++
			verified++
		case intenttree.StatusImplemented:
			implemented++
		}
	}
	if leaves == 0 {
		return "pending"
	}
	if verified == leaves {
		return "verified"
	}
	if implemented == leaves {
		return "implemented"
	}
	return "pending"
}

type WorkflowDelivery struct {
	ProjectID   string         `json:"project_id,omitempty"`
	VersionID   string         `json:"version_id,omitempty"`
	Fingerprint string         `json:"fingerprint"`
	Tasks       []DeliveryTask `json:"tasks"`
}

func resolveWorkflowDelivery(c WorkflowContract, idx intenttree.Index, bodies map[string]intenttree.Node) (WorkflowDelivery, error) {
	if err := validateWorkflowContract(c, idx, bodies); err != nil {
		return WorkflowDelivery{}, err
	}
	modules := map[string]WorkflowModule{}
	var ids []string
	for _, m := range c.Modules {
		if m.ID != strings.TrimSpace(m.ID) || m.NodeID != strings.TrimSpace(m.NodeID) {
			return WorkflowDelivery{}, qaError("workflow_contract_invalid", "noncanonical module identity")
		}
		modules[m.ID] = m
		ids = append(ids, m.ID)
	}
	sort.Strings(ids)
	_, _, source, err := workflowRenderScope(c, idx, bodies)
	if err != nil {
		return WorkflowDelivery{}, err
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return WorkflowDelivery{}, err
	}
	out := WorkflowDelivery{Fingerprint: digestOf(string(raw) + "\x00" + source), Tasks: []DeliveryTask{}}
	done := map[string]bool{}
	tasks := map[string][]string{}
	var visit func(string)
	visit = func(id string) {
		if done[id] {
			return
		}
		done[id] = true
		m := modules[id]
		deps := append([]string{}, m.Depends...)
		sort.Strings(deps)
		var prerequisite []string
		for _, dep := range deps {
			visit(dep)
			prerequisite = append(prerequisite, tasks[dep]...)
		}
		sort.Strings(prerequisite)
		features := append([]string{}, m.Features...)
		sort.Strings(features)
		for _, feature := range features {
			if workflowDeprecated(feature, bodies) {
				continue
			}
			task := DeliveryTask{ID: "task_" + digestOf(idx.Version+"\x00"+feature), FeatureNodeID: feature, ModuleNodeID: m.NodeID, DependsOn: append([]string{}, prerequisite...), Status: scopeTaskStatus(feature, bodies)}
			out.Tasks = append(out.Tasks, task)
			tasks[id] = append(tasks[id], task.ID)
		}
		if len(tasks[id]) == 0 {
			tasks[id] = prerequisite
		}
	}
	for _, id := range ids {
		visit(id)
	}
	if len(out.Tasks) == 0 {
		return WorkflowDelivery{}, qaError("workflow_contract_invalid", "zero delivery tasks")
	}
	return out, nil
}

func (e *Engine) workflowScopeReady(runID string, bound *workflowBinding) (string, bool) {
	if len(bound.Contract.Modules) == 0 {
		return "", true
	}
	if bound.ScopeFingerprint == "" {
		return "workflow_scope_unbound", false
	}
	if _, err := e.WorkflowTasks(runID); err != nil {
		return "workflow_scope_changed", false
	}
	return "", true
}

// WorkflowTasks is a read-only projection, not an execution receipt or gate.
// It deliberately does not resolve run slugs: the bound node IDs own the plan.
func (e *Engine) WorkflowTasks(runID string) (WorkflowDelivery, error) {
	bound, _, err := e.workflowBinding(runID)
	if err != nil {
		return WorkflowDelivery{}, err
	}
	if bound == nil {
		return WorkflowDelivery{}, qaError("workflow_contract_invalid", "explicit workflow binding required")
	}
	if len(bound.Contract.Modules) > 0 && bound.ScopeFingerprint == "" {
		return WorkflowDelivery{}, qaError("workflow_scope_unbound", "binding predates scope fingerprint; explicitly rebind at plan/truth")
	}
	h, err := qaHash(bound.Path)
	if err != nil || h != bound.FileHash {
		return WorkflowDelivery{}, qaError("workflow_contract_changed", "explicit rebind required")
	}
	_, _, idx, bodies, err := e.workflowInput(runID)
	if err != nil {
		return WorkflowDelivery{}, err
	}
	d, err := resolveWorkflowDelivery(bound.Contract, idx, bodies)
	if err != nil {
		return WorkflowDelivery{}, err
	}
	run, _, _, err := e.loadRun(runID)
	if err != nil {
		return WorkflowDelivery{}, err
	}
	fc, err := e.featureContext(run.FeatureID)
	if err != nil {
		return WorkflowDelivery{}, err
	}
	d = namespaceWorkflowDelivery(d, fc.Project.ID, fc.Version.ID)
	if bound.ScopeFingerprint != "" && bound.ScopeFingerprint != d.Fingerprint {
		return WorkflowDelivery{}, qaError("workflow_scope_changed", "canonical scope changed; explicitly rebind at plan/truth")
	}
	return d, nil
}

func namespaceWorkflowDelivery(d WorkflowDelivery, projectID, versionID string) WorkflowDelivery {
	d.ProjectID, d.VersionID = projectID, versionID
	d.Fingerprint = digestOf(projectID + "\x00" + versionID + "\x00" + d.Fingerprint)
	ids := map[string]string{}
	for _, task := range d.Tasks {
		ids[task.ID] = "task_" + digestOf(projectID+"\x00"+versionID+"\x00"+task.FeatureNodeID)
	}
	for i := range d.Tasks {
		d.Tasks[i].ID = ids[d.Tasks[i].ID]
		for j, id := range d.Tasks[i].DependsOn {
			d.Tasks[i].DependsOn[j] = ids[id]
		}
	}
	return d
}
