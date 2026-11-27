package intenttree

import (
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// NodePatch is the editable surface of a draft node. Identity, path,
// parent, kind, status, fingerprints and evidence are never patched here.
type NodePatch struct {
	ExpectedFingerprint string           `json:"-"`
	TestCaseBinding     *TestCaseBinding `json:"-"`
	Title               string           `json:"title"`
	Intent              string           `json:"intent"`
	Doc                 string           `json:"doc,omitempty"`
	Constraint          *bool            `json:"constraint,omitempty"`
	ACs                 []AC             `json:"acs"`
	TestCases           []TestCase       `json:"test_cases,omitempty"`
	Verification        *Verification    `json:"verification,omitempty"`
	DomainFSM           *DomainFSM       `json:"domain_fsm,omitempty"`
}

type TestCaseBinding struct {
	ID   string
	ACID string
}

// UpdateNode rewrites a draft or change_pending node body. It is the
// host's safe edit path after `change request`; frozen bodies stay
// immutable until changed.
func UpdateNode(st *store.Store, dir, nodeID string, patch NodePatch) (Node, error) {
	unlock, err := lockTreeChange(dir)
	if err != nil {
		return Node{}, err
	}
	defer unlock()
	if err := recoverTreeChange(dir, st); err != nil {
		return Node{}, err
	}
	idx, n, err := readMutationNode(dir, nodeID)
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
	if patch.ExpectedFingerprint != "" && patch.ExpectedFingerprint != n.Fingerprint {
		return Node{}, core.NewError(ReasonChangeBaseMismatch, "intenttree.UpdateNode", "base fingerprint does not match current node; reload first")
	}
	if n.Status != StatusDraft && n.Status != StatusChangePending {
		return Node{}, core.NewError(ReasonNodeNotDraft, "intenttree.UpdateNode", "canonical node is not editable: "+nodeID)
	}
	if patch.TestCaseBinding != nil {
		binding := patch.TestCaseBinding
		if strings.TrimSpace(binding.ID) == "" || strings.TrimSpace(binding.ACID) == "" || patch.TestCases != nil {
			return Node{}, core.NewError(ReasonTestCaseInvalid, "intenttree.UpdateNode", "one explicit test case and AC binding required")
		}
		acCount, caseCount := 0, 0
		seenAC := map[string]bool{}
		for _, ac := range n.ACs {
			if strings.TrimSpace(ac.ID) == "" || seenAC[ac.ID] {
				return Node{}, core.NewError(ReasonTestCaseInvalid, "intenttree.UpdateNode", "ambiguous AC IDs")
			}
			seenAC[ac.ID] = true
			if ac.ID == binding.ACID {
				acCount++
			}
		}
		for index := range n.TestCases {
			if n.TestCases[index].ID == binding.ID {
				caseCount++
				n.TestCases[index].ACID = binding.ACID
			}
		}
		if acCount != 1 || caseCount != 1 {
			return Node{}, core.NewError(ReasonTestCaseInvalid, "intenttree.UpdateNode", "unknown or ambiguous test case / AC ID")
		}
		patch.TestCases = n.TestCases
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
	if patch.Constraint != nil {
		n.Constraint = *patch.Constraint
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
	if err := persistMutation(st, dir, idx, n, nil, ""); err != nil {
		return Node{}, err
	}
	return n, nil
}
