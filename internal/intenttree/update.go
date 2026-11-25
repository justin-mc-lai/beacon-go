package intenttree

import (
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// NodePatch is the editable surface of a draft node. Identity, path,
// parent, kind, status, fingerprints and evidence are never patched here.
type NodePatch struct {
	Title        string        `json:"title"`
	Intent       string        `json:"intent"`
	Doc          string        `json:"doc,omitempty"`
	ACs          []AC          `json:"acs"`
	TestCases    []TestCase    `json:"test_cases,omitempty"`
	Verification *Verification `json:"verification,omitempty"`
	DomainFSM    *DomainFSM    `json:"domain_fsm,omitempty"`
}

// UpdateNode rewrites a draft or change_pending node body. It is the
// host's safe edit path after `change request`; frozen bodies stay
// immutable until changed.
func UpdateNode(st *store.Store, dir, nodeID string, patch NodePatch) (Node, error) {
	idx, err := ReadIndex(dir)
	if err != nil {
		return Node{}, err
	}
	entry, ok := findEntry(idx, nodeID)
	if !ok {
		return Node{}, core.NewError(ReasonNodeNotFound, "intenttree.UpdateNode", "node not found: "+nodeID)
	}
	if entry.Status != StatusDraft && entry.Status != StatusChangePending {
		return Node{}, core.NewError(ReasonNodeNotDraft, "intenttree.UpdateNode",
			"only draft/change_pending nodes can be updated: "+nodeID+" is "+string(entry.Status))
	}
	n, err := ReadNode(dir, nodeID)
	if err != nil {
		return Node{}, err
	}
	if strings.TrimSpace(patch.Title) != "" {
		n.Title = strings.TrimSpace(patch.Title)
	}
	if strings.TrimSpace(patch.Intent) != "" {
		n.Intent = strings.TrimSpace(patch.Intent)
	}
	if patch.Doc != "" {
		n.Doc = patch.Doc
	}
	if patch.ACs != nil {
		n.ACs = patch.ACs
	}
	if patch.TestCases != nil {
		if n.Kind != KindTouchpoint && n.Kind != KindFeature {
			return Node{}, core.NewError(ReasonTestCaseInvalid, "intenttree.UpdateNode",
				"test cases belong to touchpoints or features: "+nodeID+" is "+string(n.Kind))
		}
		if findings := ValidateTestCases(n.ACs, patch.TestCases); len(findings) > 0 {
			return Node{}, core.NewError(ReasonTestCaseInvalid, "intenttree.UpdateNode", strings.Join(findings, "; "))
		}
		n.TestCases = patch.TestCases
	}
	if patch.Verification != nil {
		n.Verification = patch.Verification
	}
	if patch.DomainFSM != nil {
		if n.Kind != KindFeature {
			return Node{}, core.NewError(ReasonFSMScope, "intenttree.UpdateNode",
				"domain FSM belongs to feature nodes: "+nodeID+" is "+string(n.Kind))
		}
		if findings := ValidateDomainFSM(patch.DomainFSM); len(findings) > 0 {
			return Node{}, core.NewError(ReasonFSMInvalid, "intenttree.UpdateNode", strings.Join(findings, "; "))
		}
		n.DomainFSM = patch.DomainFSM
	}
	n, err = Stamp(n)
	if err != nil {
		return Node{}, err
	}
	if err := WriteNode(dir, n); err != nil {
		return Node{}, err
	}
	entry.Title = n.Title
	entry.Fingerprint = n.Fingerprint
	entry.FingerprintAlg = n.FingerprintAlg
	for i := range idx.Nodes {
		if idx.Nodes[i].ID == nodeID {
			idx.Nodes[i] = entry
		}
	}
	if err := WriteIndex(dir, idx); err != nil {
		return Node{}, err
	}
	if err := st.UpdateTreeNodeStatus(n.ID, string(n.Status), n.Fingerprint, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return Node{}, err
	}
	return n, nil
}
