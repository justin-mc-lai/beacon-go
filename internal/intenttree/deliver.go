package intenttree

import (
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// MarkDelivered promotes a verified node to the terminal delivered state.
// Delivery is an explicit human/host act; it is never inferred from a
// passing test or from an implementation report.
func MarkDelivered(st *store.Store, dir, nodeID, by string) (Node, error) {
	if strings.TrimSpace(by) == "" {
		return Node{}, core.NewError(ReasonInvalidIndex, "intenttree.MarkDelivered", "--by is required")
	}
	idx, err := ReadIndex(dir)
	if err != nil {
		return Node{}, err
	}
	entry, ok := findEntry(idx, nodeID)
	if !ok {
		return Node{}, core.NewError(ReasonNodeNotFound, "intenttree.MarkDelivered", "node not found: "+nodeID)
	}
	if entry.Status != StatusVerified {
		return Node{}, core.NewError("tree_node_not_verified", "intenttree.MarkDelivered",
			"only verified nodes can be delivered: "+nodeID+" is "+string(entry.Status))
	}
	children := 0
	for _, e := range idx.Nodes {
		if e.ParentID != nodeID {
			continue
		}
		children++
		if e.Status != StatusVerified && e.Status != StatusDelivered {
			return Node{}, core.NewError("tree_child_not_verified", "intenttree.MarkDelivered",
				"child "+e.ID+" is "+string(e.Status)+"; verify or deliver children first")
		}
	}
	n, err := ReadNode(dir, nodeID)
	if err != nil {
		return Node{}, err
	}
	n.Status = StatusDelivered
	n, err = Stamp(n)
	if err != nil {
		return Node{}, err
	}
	if err := WriteNode(dir, n); err != nil {
		return Node{}, err
	}
	entry.Status = n.Status
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
	if err := st.UpdateTreeNodeStatus(nodeID, string(n.Status), n.Fingerprint, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return Node{}, err
	}
	return n, nil
}
