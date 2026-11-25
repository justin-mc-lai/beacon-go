package intenttree

import (
	"fmt"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// EvidenceRef is the registered proof that verified a leaf.
type EvidenceRef struct {
	ArtifactID  string `json:"artifact_id"`
	ContentHash string `json:"content_hash"`
	By          string `json:"by"`
	At          string `json:"at"`
}

// VerifyReport is the outcome of a leaf verification or a rollup.
type VerifyReport struct {
	NodeID   string `json:"node_id"`
	Status   string `json:"status"`
	Verified bool   `json:"verified"`
	Children int    `json:"children_verified,omitempty"`
}

// MarkVerified turns an implemented leaf into verified after its
// evidence artifact was registered under the node scope.
func MarkVerified(st *store.Store, dir, nodeID, artifactID, contentHash, by string) (VerifyReport, error) {
	idx, err := ReadIndex(dir)
	if err != nil {
		return VerifyReport{}, err
	}
	entry, ok := findEntry(idx, nodeID)
	if !ok {
		return VerifyReport{}, core.NewError(ReasonNodeNotFound, "intenttree.MarkVerified", "node not found: "+nodeID)
	}
	if entry.Kind != KindTouchpoint {
		return VerifyReport{}, core.NewError(ReasonInvalidChildKind, "intenttree.MarkVerified",
			"only touchpoints carry verification evidence: "+nodeID)
	}
	if entry.Status != StatusImplemented {
		return VerifyReport{}, core.NewError("tree_node_not_implemented", "intenttree.MarkVerified",
			"leaf must be implemented before verification: "+nodeID+" is "+string(entry.Status))
	}
	n, err := ReadNode(dir, nodeID)
	if err != nil {
		return VerifyReport{}, err
	}
	now := time.Now().UTC()
	n.Status = StatusVerified
	n.VerificationEvidence = &EvidenceRef{ArtifactID: artifactID, ContentHash: contentHash, By: by, At: now.Format(time.RFC3339Nano)}
	n, err = Stamp(n)
	if err != nil {
		return VerifyReport{}, err
	}
	if err := WriteNode(dir, n); err != nil {
		return VerifyReport{}, err
	}
	entry.Status = StatusVerified
	entry.Fingerprint = n.Fingerprint
	entry.FingerprintAlg = n.FingerprintAlg
	for i := range idx.Nodes {
		if idx.Nodes[i].ID == nodeID {
			idx.Nodes[i] = entry
		}
	}
	if err := WriteIndex(dir, idx); err != nil {
		return VerifyReport{}, err
	}
	if err := st.UpdateTreeNodeStatus(nodeID, string(StatusVerified), n.Fingerprint, now.Format(time.RFC3339Nano)); err != nil {
		return VerifyReport{}, err
	}
	return VerifyReport{NodeID: nodeID, Status: string(StatusVerified), Verified: true}, nil
}

// RollupVerified marks a parent verified only when every child already
// is; it walks post-order so one call can close a whole branch.
func RollupVerified(st *store.Store, dir, nodeID string) (VerifyReport, error) {
	idx, err := ReadIndex(dir)
	if err != nil {
		return VerifyReport{}, err
	}
	children := map[string][]IndexEntry{}
	for _, n := range idx.Nodes {
		children[n.ParentID] = append(children[n.ParentID], n)
	}
	var walk func(string) error
	walk = func(id string) error {
		for _, ch := range children[id] {
			if err := walk(ch.ID); err != nil {
				return err
			}
		}
		entry, ok := findEntry(idx, id)
		if !ok {
			return core.NewError(ReasonNodeNotFound, "intenttree.RollupVerified", "node not found: "+id)
		}
		if entry.Kind == KindTouchpoint || len(children[id]) == 0 {
			return nil
		}
		all := true
		for _, ch := range children[id] {
			e, _ := findEntry(idx, ch.ID)
			if e.Status != StatusVerified {
				all = false
				break
			}
		}
		if !all {
			return core.NewError("tree_parent_not_verified", "intenttree.RollupVerified",
				fmt.Sprintf("child of %s is not verified", id))
		}
		if entry.Status == StatusVerified {
			return nil
		}
		n, err := ReadNode(dir, id)
		if err != nil {
			return err
		}
		n.Status = StatusVerified
		var bound []string
		for _, ch := range children[id] {
			bound = append(bound, ch.Fingerprint)
		}
		n.ChildFingerprints = bound
		n, err = Stamp(n)
		if err != nil {
			return err
		}
		if err := WriteNode(dir, n); err != nil {
			return err
		}
		entry.Status = StatusVerified
		entry.Fingerprint = n.Fingerprint
		entry.FingerprintAlg = n.FingerprintAlg
		for i := range idx.Nodes {
			if idx.Nodes[i].ID == id {
				idx.Nodes[i] = entry
			}
		}
		if err := st.UpdateTreeNodeStatus(id, string(StatusVerified), n.Fingerprint, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
		return nil
	}
	if err := walk(nodeID); err != nil {
		return VerifyReport{}, err
	}
	if err := WriteIndex(dir, idx); err != nil {
		return VerifyReport{}, err
	}
	entry, _ := findEntry(idx, nodeID)
	return VerifyReport{NodeID: nodeID, Status: string(entry.Status), Verified: entry.Status == StatusVerified}, nil
}
