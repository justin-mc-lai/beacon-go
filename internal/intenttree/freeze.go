package intenttree

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// FreezeReport is the outcome of one node freeze.
type FreezeReport struct {
	NodeID      string `json:"node_id"`
	Status      string `json:"status"`
	Fingerprint string `json:"fingerprint"`
	Already     bool   `json:"already_frozen"`
	Children    int    `json:"children_bound"`
	SnapshotID  string `json:"snapshot_id,omitempty"`
}

// FreezeNode freezes one node after its exit conditions hold: leaves
// need ACs + executable verification; modules need children; parents
// need every child frozen. The parent's frozen fingerprint binds the
// ordered child fingerprints, so a later child change is detectable.
func FreezeNode(st *store.Store, dir, versionID, nodeID string) (FreezeReport, error) {
	idx, err := ReadIndex(dir)
	if err != nil {
		return FreezeReport{}, err
	}
	entry, ok := findEntry(idx, nodeID)
	if !ok {
		return FreezeReport{}, core.NewError(ReasonNodeNotFound, "intenttree.FreezeNode", "node not found: "+nodeID)
	}
	if entry.Status == StatusFrozen {
		return FreezeReport{NodeID: nodeID, Status: string(entry.Status), Fingerprint: entry.Fingerprint, Already: true}, nil
	}
	if entry.Status != StatusDraft && entry.Status != StatusChangePending {
		return FreezeReport{}, core.NewError(ReasonNodeNotDraft, "intenttree.FreezeNode",
			"only draft or change_pending nodes can freeze: "+nodeID+" is "+string(entry.Status))
	}
	n, err := ReadNode(dir, nodeID)
	if err != nil {
		return FreezeReport{}, err
	}
	if n.DomainFSM != nil {
		if n.Kind != KindFeature {
			return FreezeReport{}, core.NewError(ReasonFSMScope, "intenttree.FreezeNode",
				"domain FSM belongs to feature nodes: "+nodeID)
		}
		if findings := ValidateDomainFSM(n.DomainFSM); len(findings) > 0 {
			return FreezeReport{}, core.NewError(ReasonFSMInvalid, "intenttree.FreezeNode", strings.Join(findings, "; "))
		}
	}
	var children []IndexEntry
	for _, e := range idx.Nodes {
		if e.ParentID == nodeID {
			children = append(children, e)
		}
	}
	sort.Slice(children, func(i, j int) bool { return children[i].Path < children[j].Path })

	if n.Kind == KindTouchpoint {
		if len(children) != 0 {
			return FreezeReport{}, core.NewError(ReasonLeafHasChildren, "intenttree.FreezeNode",
				"touchpoint must not have children: "+nodeID)
		}
		if len(n.ACs) == 0 {
			return FreezeReport{}, core.NewError(ReasonACMissing, "intenttree.FreezeNode",
				"touchpoint needs at least one AC before freeze: "+nodeID)
		}
		if n.Verification == nil || strings.TrimSpace(n.Verification.Spec) == "" {
			return FreezeReport{}, core.NewError(ReasonVerificationMissing, "intenttree.FreezeNode",
				"touchpoint needs an executable verification spec before freeze: "+nodeID)
		}
	}
	if n.Kind == KindModule && len(children) == 0 {
		return FreezeReport{}, core.NewError(ReasonModuleEmpty, "intenttree.FreezeNode",
			"module needs at least one child before freeze: "+nodeID)
	}
	bound := make([]string, 0, len(children))
	for _, ch := range children {
		if ch.Status != StatusFrozen && ch.Status != StatusImplemented && ch.Status != StatusVerified {
			return FreezeReport{}, core.NewError(ReasonParentNotFrozen, "intenttree.FreezeNode",
				fmt.Sprintf("child %s is %s; freeze children first", ch.ID, ch.Status))
		}
		body, err := ReadNode(dir, ch.ID)
		if err != nil {
			return FreezeReport{}, err
		}
		if body.Fingerprint != ch.Fingerprint {
			return FreezeReport{}, core.NewError(ReasonInvalidIndex, "intenttree.FreezeNode",
				"index fingerprint for child "+ch.ID+" does not match its body")
		}
		bound = append(bound, ch.Fingerprint)
	}
	n.Status = StatusFrozen
	n.ChildFingerprints = bound
	n, err = Stamp(n)
	if err != nil {
		return FreezeReport{}, err
	}
	if err := WriteNode(dir, n); err != nil {
		return FreezeReport{}, err
	}
	entry.Status = StatusFrozen
	entry.Fingerprint = n.Fingerprint
	entry.FingerprintAlg = n.FingerprintAlg
	for i := range idx.Nodes {
		if idx.Nodes[i].ID == nodeID {
			idx.Nodes[i] = entry
		}
	}
	if err := WriteIndex(dir, idx); err != nil {
		return FreezeReport{}, err
	}
	if err := st.UpdateTreeNodeStatus(n.ID, string(n.Status), n.Fingerprint, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return FreezeReport{}, err
	}
	rep := FreezeReport{NodeID: nodeID, Status: string(n.Status), Fingerprint: n.Fingerprint, Children: len(bound)}
	if n.ParentID == "" {
		_, id, err := CreateSnapshot(dir, idx.Version)
		if err != nil {
			return rep, err
		}
		rep.SnapshotID = id
	}
	return rep, nil
}

func findEntry(idx Index, id string) (IndexEntry, bool) {
	for _, e := range idx.Nodes {
		if e.ID == id {
			return e, true
		}
	}
	return IndexEntry{}, false
}
