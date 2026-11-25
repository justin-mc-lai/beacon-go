package intenttree

import (
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// LinkTruthPath records a feature node's requirement-package path. It is
// a pointer, not a second truth body: the package remains authoritative.
// Only editable nodes can be linked; frozen nodes need a change first.
func LinkTruthPath(st *store.Store, dir, nodeID, truthPath string) (Node, error) {
	truthPath = strings.TrimSpace(truthPath)
	if truthPath == "" {
		return Node{}, core.NewError(ReasonTruthPathMissing, "intenttree.LinkTruthPath", "truth path is required")
	}
	idx, err := ReadIndex(dir)
	if err != nil {
		return Node{}, err
	}
	entry, ok := findEntry(idx, nodeID)
	if !ok {
		return Node{}, core.NewError(ReasonNodeNotFound, "intenttree.LinkTruthPath", "node not found: "+nodeID)
	}
	if entry.Kind != KindFeature {
		return Node{}, core.NewError(ReasonTruthPathInvalid, "intenttree.LinkTruthPath",
			"truth_path belongs to feature nodes: "+nodeID+" is "+string(entry.Kind))
	}
	if entry.Status != StatusDraft && entry.Status != StatusChangePending {
		return Node{}, core.NewError(ReasonTruthLinkNotDraft, "intenttree.LinkTruthPath",
			"link requires draft/change_pending; "+nodeID+" is "+string(entry.Status))
	}
	n, err := ReadNode(dir, nodeID)
	if err != nil {
		return Node{}, err
	}
	n.TruthPath = truthPath
	n, err = Stamp(n)
	if err != nil {
		return Node{}, err
	}
	if err := WriteNode(dir, n); err != nil {
		return Node{}, err
	}
	entry.TruthPath = n.TruthPath
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
