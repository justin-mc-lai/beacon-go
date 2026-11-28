package orchestrator

import (
	"sort"
	"strings"

	"beacon-go/internal/intenttree"
)

const (
	reasonScopeNotFrozen     = "scope_node_not_frozen"
	reasonScopeChangePending = "scope_change_pending"
)

// scopeFrozenAdmissionItem is the range-level freeze gate. A single wrapper
// feature being frozen cannot stand in for an unfrozen child in the scope.
// Legacy runs without an explicit workflow binding stay not-applicable.
func (e *Engine) scopeFrozenAdmissionItem(runID string) (AdmissionItem, error) {
	bound, _, err := e.workflowBinding(runID)
	if err != nil || bound == nil || len(bound.Contract.ScopeNodes) == 0 {
		return AdmissionItem{Check: "scope_frozen", Applicable: false, OK: true, Detail: "no stable delivery scope"}, nil
	}
	// Tree/contract availability and validity are owned by the workflow gate.
	// This item only adds the range-level freeze judgement when the tree is
	// readable; it must not turn a missing tree into a second error path.
	_, _, idx, bodies, err := e.workflowInput(runID)
	if err != nil {
		return AdmissionItem{Check: "scope_frozen", Applicable: false, OK: true, Detail: "tree unavailable; workflow gate owns scope checks"}, nil
	}
	scoped, err := workflowScope(bound.Contract, idx, bodies)
	if err != nil {
		return AdmissionItem{Check: "scope_frozen", Applicable: false, OK: true, Detail: "scope not resolved; workflow gate owns contract checks"}, nil
	}
	var blocked []string
	var changePending []string
	for id := range scoped {
		n := bodies[id]
		if workflowDeprecated(id, bodies) || n.Kind == intenttree.KindIntent || n.Kind == intenttree.KindModule {
			continue
		}
		switch n.Status {
		case intenttree.StatusFrozen, intenttree.StatusImplemented, intenttree.StatusVerified, intenttree.StatusDelivered:
		case intenttree.StatusChangePending:
			changePending = append(changePending, id)
		default:
			blocked = append(blocked, id+"("+string(n.Status)+")")
		}
	}
	sort.Strings(blocked)
	sort.Strings(changePending)
	if len(changePending) > 0 {
		return AdmissionItem{Check: "scope_frozen", Applicable: true,
			ReasonCode: reasonScopeChangePending,
			Detail:     "delivery scope has unrefrozen changes: " + strings.Join(changePending, ", ")}, nil
	}
	if len(blocked) > 0 {
		return AdmissionItem{Check: "scope_frozen", Applicable: true,
			ReasonCode: reasonScopeNotFrozen,
			Detail:     "delivery scope not frozen: " + strings.Join(blocked, ", ")}, nil
	}
	return AdmissionItem{Check: "scope_frozen", Applicable: true, OK: true,
		Detail: "all active delivery-scope nodes frozen or later"}, nil
}
