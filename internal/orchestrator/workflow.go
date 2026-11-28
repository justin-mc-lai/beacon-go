package orchestrator

import (
	"encoding/json"
	"strings"

	"beacon-go/internal/intenttree"
)

func validateWorkflow(c WorkflowContract, idx intenttree.Index, bodies map[string]intenttree.Node) error {
	_, err := resolveWorkflowDelivery(c, idx, bodies)
	return err
}

func validateWorkflowContract(c WorkflowContract, idx intenttree.Index, bodies map[string]intenttree.Node) error {
	bad := func(s string) error { return qaError("workflow_contract_invalid", s) }
	if c.Schema != "beacon-workflow.v1" || strings.TrimSpace(c.Goal) == "" || strings.TrimSpace(c.Scope) == "" || len(c.Modules) == 0 || len(c.Coverage) == 0 {
		return bad("goal/scope/modules/coverage required")
	}
	if findings := intenttree.ValidateIndex(idx); len(findings) > 0 {
		return bad(findings[0])
	}
	if err := validateWorkflowDraft(c); err != nil {
		return err
	}
	scoped, err := workflowScope(c, idx, bodies)
	if err != nil {
		return err
	}
	for _, entry := range idx.Nodes {
		n, ok := bodies[entry.ID]
		valid, err := intenttree.VerifyFingerprint(n)
		if !ok || err != nil || !valid || n.Fingerprint != entry.Fingerprint || n.ParentID != entry.ParentID || n.Kind != entry.Kind || n.Status != entry.Status || n.Path != entry.Path {
			return bad("canonical body/index differs")
		}
	}
	modules := map[string]WorkflowModule{}
	moduleNodes := map[string]bool{}
	features := map[string]bool{}
	required := map[string]bool{}
	for _, m := range c.Modules {
		if strings.TrimSpace(m.ID) == "" || modules[m.ID].ID != "" || moduleNodes[m.NodeID] || bodies[m.NodeID].Kind != intenttree.KindModule || len(m.Features) == 0 {
			return bad("invalid/duplicate module")
		}
		modules[m.ID] = m
		moduleNodes[m.NodeID] = true
		for _, id := range m.Features {
			n, ok := bodies[id]
			if workflowDeprecated(id, bodies) {
				continue
			}
			if !ok || n.Kind != intenttree.KindFeature || features[id] {
				return bad("unknown/duplicate feature ownership")
			}
			features[id] = true
			if !scoped[id] {
				return bad("feature outside delivery scope")
			}
			// Every scheduled feature must be inside the declared canonical module.
			parent := n.ParentID
			seen := map[string]bool{}
			inside := false
			for parent != "" {
				if seen[parent] {
					return bad("ancestor cycle")
				}
				seen[parent] = true
				if parent == m.NodeID {
					inside = true
					break
				}
				p, ok := bodies[parent]
				if !ok {
					return bad("missing ancestor")
				}
				parent = p.ParentID
			}
			if !inside {
				return bad("feature outside module")
			}
			leaves := 0
			for _, leaf := range bodies {
				if workflowDeprecated(leaf.ID, bodies) {
					continue
				}
				if leaf.ParentID != id {
					continue
				}
				if leaf.Kind != intenttree.KindTouchpoint {
					return bad("feature child must be touchpoint")
				}
				leaves++
				if len(leaf.ACs) == 0 || leaf.Verification == nil || strings.TrimSpace(leaf.Verification.Spec) == "" {
					return bad("leaf lacks AC/verification")
				}
				for _, ac := range leaf.ACs {
					key := leaf.ID + "\x00" + ac.ID
					if strings.TrimSpace(ac.ID) == "" || strings.TrimSpace(ac.Text) == "" || required[key] {
						return bad("blank/duplicate AC")
					}
					required[key] = true
				}
			}
			if leaves == 0 {
				return bad("feature has no implementation touchpoints")
			}
		}
	}
	// Acceptance nodes (including the run anchor) are integration contracts:
	// they are not scheduled as delivery tasks, but their ACs are required.
	acceptance := map[string]bool{}
	if c.RunAnchor != "" {
		acceptance[c.RunAnchor] = true
	}
	for _, id := range c.Acceptance {
		if strings.TrimSpace(id) == "" || id != strings.TrimSpace(id) {
			return bad("invalid acceptance node")
		}
		acceptance[id] = true
	}
	for id := range acceptance {
		anchor, ok := bodies[id]
		if !ok || anchor.Kind != intenttree.KindFeature || !scoped[id] {
			return bad("invalid/out-of-scope acceptance node")
		}
		for _, leaf := range bodies {
			if workflowDeprecated(leaf.ID, bodies) || leaf.ParentID != id || leaf.Kind != intenttree.KindTouchpoint {
				continue
			}
			for _, ac := range leaf.ACs {
				key := leaf.ID + "\x00" + ac.ID
				if strings.TrimSpace(ac.ID) == "" || strings.TrimSpace(ac.Text) == "" {
					return bad("blank acceptance AC")
				}
				if !required[key] {
					required[key] = true
				}
			}
		}
	}
	if len(required) == 0 {
		return bad("zero active target ACs")
	}
	for id := range scoped {
		if acceptance[id] {
			continue
		}
		if bodies[id].Kind == intenttree.KindFeature && !features[id] {
			return bad("scope feature has no owner")
		}
	}
	visiting, done := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if visiting[id] {
			return bad("dependency cycle")
		}
		if done[id] {
			return nil
		}
		m, ok := modules[id]
		if !ok {
			return bad("unknown dependency")
		}
		visiting[id] = true
		deps := map[string]bool{}
		for _, dep := range m.Depends {
			if deps[dep] {
				return bad("duplicate dependency")
			}
			deps[dep] = true
			if err := visit(dep); err != nil {
				return err
			}
		}
		visiting[id] = false
		done[id] = true
		return nil
	}
	for id := range modules {
		if err := visit(id); err != nil {
			return err
		}
	}
	covered, sourceKeys := map[string]bool{}, map[string]bool{}
	sourceNodes := map[string]bool{}
	for _, id := range c.SourceNodes {
		if workflowDeprecated(id, bodies) {
			continue
		}
		if strings.TrimSpace(id) == "" || sourceNodes[id] || bodies[id].ID == "" || len(bodies[id].ACs) == 0 {
			return bad("invalid/duplicate source node")
		}
		sourceNodes[id] = true
	}
	if len(sourceNodes) == 0 {
		return bad("source_nodes required independently of coverage")
	}
	for id := range scoped {
		if bodies[id].Kind != intenttree.KindTouchpoint && len(bodies[id].ACs) > 0 && !sourceNodes[id] {
			return bad("canonical scope AC source omitted")
		}
	}
	for _, item := range c.Coverage {
		if workflowDeprecated(item.Source, bodies) {
			continue
		}
		if !sourceNodes[item.Source] {
			return bad("undeclared source node")
		}
		source, ok := bodies[item.Source]
		if !ok {
			return bad("unknown source node")
		}
		sourceKey := item.Source + "\x00" + item.AC
		key := item.Target + "\x00" + item.AC
		found := false
		for _, ac := range source.ACs {
			if ac.ID == item.AC {
				found = true
			}
		}
		if !found || !required[key] || sourceKeys[sourceKey] {
			return bad("missing/duplicate source or target AC")
		}
		if item.Source != item.Target {
			if target := bodies[item.Target]; len(target.ACs) == 0 {
				return bad("missing target")
			}
			for _, ac := range bodies[item.Target].ACs {
				if ac.ID == item.AC {
					for _, sourceAC := range source.ACs {
						if sourceAC.ID == item.AC && sourceAC.Text != ac.Text {
							return bad("source/target AC differs; resolve through change")
						}
					}
				}
			}
		}
		covered[key] = true
		sourceKeys[sourceKey] = true
	}
	for id := range sourceNodes {
		for _, ac := range bodies[id].ACs {
			if !sourceKeys[id+"\x00"+ac.ID] {
				return bad("uncovered source AC")
			}
		}
	}
	if len(required) != len(covered) {
		return bad("uncovered canonical AC")
	}
	return nil
}

func (e *Engine) WorkflowCheck(runID string) (WorkflowView, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return WorkflowView{}, err
	}
	v, err := e.workflowCheckStage(runID, m.currentStage())
	if err != nil || !v.Ready {
		return v, err
	}
	ready, reason, err := e.treeStageFacts(run, v.Stage, v.Mode == "stable")
	if err != nil {
		return v, err
	}
	if !ready {
		v.Ready = false
		v.Missing = append(v.Missing, reason)
		v.Next = "satisfy current canonical stage gate"
	}
	if v.Ready && v.Stage == StageImplement {
		admission, err := e.checkAdmission(run, m)
		if err != nil {
			return v, err
		}
		if admission.Status != "pass" {
			v.Ready = false
			v.Missing = append(v.Missing, admission.ReasonCodes...)
			v.Next = "satisfy existing implement admission"
		}
	}
	if block := e.effectiveBlockView(run, m); v.Ready && block.Blocked {
		v.Ready = false
		v.Missing = append(v.Missing, block.Reason)
		v.Next = "resolve current run blocker before advancing"
	}
	if v.Ready {
		in, err := e.buildExitInput(run, m)
		if err != nil {
			return v, err
		}
		verdict := EvaluateStageExit(in)
		if !verdict.Allowed {
			v.Ready = false
			v.Missing = append(v.Missing, verdict.ReasonCode)
			v.Next = "satisfy existing current-stage exit gate"
		}
	}
	if v.Stage == StageRelease {
		v.Ready = false
		v.Missing = append(v.Missing, "release_confirmation_required")
		v.Owner = "human"
		v.Next = "complete existing review/release gates and confirm release"
	}
	return v, nil
}

func (e *Engine) workflowCheckStage(runID, stage string) (WorkflowView, error) {
	_, _, _, err := e.loadRun(runID)
	if err != nil {
		return WorkflowView{}, err
	}
	v := WorkflowView{RunID: runID, Mode: "legacy", Stage: stage, Ready: true, Next: "existing stage gates", Owner: "host", Missing: []string{}}
	bound, art, err := e.workflowBinding(runID)
	if err != nil {
		return v, err
	}
	if bound == nil {
		return v, nil
	}
	v.Mode = "stable"
	v.ModuleCount = len(bound.Contract.Modules)
	v.Ready = false
	fileHash, fileErr := qaHash(bound.Path)
	if fileErr != nil || fileHash != bound.FileHash {
		v.Missing = append(v.Missing, "workflow_contract_changed")
		v.Next = "restore contract or explicitly rebind at plan/truth"
		return v, nil
	}
	if reason, ok := e.workflowScopeReady(runID, bound); !ok {
		v.Missing = append(v.Missing, reason)
		v.Next = "repair scope and explicitly rebind at plan/truth"
		return v, nil
	}
	if stage == StagePlan {
		v.Ready = true
		v.Next = "goal tick --by <host>; continue investigation/decision at plan"
		return v, nil
	}
	_, _, idx, bodies, err := e.workflowInput(runID)
	if err != nil {
		v.Missing = append(v.Missing, "workflow_tree_missing")
		v.Next = "create this delivery's canonical tree through truth"
		return v, nil
	}
	if err := validateWorkflow(bound.Contract, idx, bodies); err != nil {
		v.Missing = append(v.Missing, "workflow_contract_invalid")
		v.Next = "repair contract and explicitly rebind at plan/truth"
		return v, nil
	}
	if err := e.workflowRunFeature(runID, bound.Contract, idx, bodies); err != nil {
		v.Missing = append(v.Missing, "workflow_feature_ambiguous")
		v.Next = err.Error()
		return v, nil
	}
	if art == nil {
		v.Missing = append(v.Missing, "workflow_artifacts_missing")
		v.Next = "goal workflow prepare"
		return v, nil
	}
	v.HTML, v.Manifest = art.HTML, art.Manifest
	v.SourceHash, v.PreparedHash = art.Source, workflowPreparedHash(*art)
	viewIndex, viewBodies, source, err := workflowRenderScope(bound.Contract, idx, bodies)
	if err != nil {
		return v, err
	}
	htmlHash, he := qaHash(art.HTML)
	manifestHash, me := qaHash(art.Manifest)
	expectedManifest, _ := json.MarshalIndent(bound.Contract, "", "  ")
	if art.BindingHash != bound.Hash || art.Source != source || he != nil || me != nil || htmlHash != art.HTMLHash || manifestHash != art.ManifestHash || htmlHash != digestOf(string(intenttree.RenderHTMLTree(viewIndex, viewBodies))) || manifestHash != digestOf(string(expectedManifest)) {
		v.Missing = append(v.Missing, "workflow_artifacts_stale")
		v.Next = "goal workflow prepare"
		return v, nil
	}
	confirmed, err := e.workflowConfirmed(runID, *art)
	if err != nil {
		return v, err
	}
	if !confirmed {
		v.Missing = append(v.Missing, "workflow_confirmation_required")
		v.Owner = "human"
		v.Next = "review scoped HTML/coverage, then goal workflow confirm --by <identity> --prepared-hash " + v.PreparedHash
		return v, nil
	}
	v.Ready = true
	v.Next = "goal tick; existing stage gates still required"
	return v, nil
}
