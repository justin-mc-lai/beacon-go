package intenttree

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// ReasonReopenNotAllowed is the one reason a demotion may refuse.
const ReasonReopenNotAllowed = "tree_reopen_not_allowed"

// ReopenReport is the outcome of one lifecycle demotion.
type ReopenReport struct {
	NodeID  string `json:"node_id"`
	From    string `json:"from"`
	To      string `json:"to"`
	Status  string `json:"status"`
	Already bool   `json:"already,omitempty"`
}

// lifecycleRank orders the forward statuses a demotion may walk back.
// draft/change_pending are edit states and are not reachable through
// reopen: a requirement change owns that transition.
var lifecycleRank = map[Status]int{
	StatusFrozen:      1,
	StatusImplemented: 2,
	StatusVerified:    3,
	StatusDelivered:   4,
}

// ReopenNode is the ONE demotion entry. It moves a node back along
// verified -> implemented -> frozen and never edits the requirement
// body. Verification evidence is cleared whenever the node leaves the
// verified state: evidence for a later state cannot support an earlier
// one. A frozen target also clears the implementation record, because
// "frozen" means the leaf has not been executed yet.
func ReopenNode(st *store.Store, dir, nodeID, target, by, reason string) (ReopenReport, error) {
	targetStatus := Status(strings.TrimSpace(target))
	if targetStatus != StatusImplemented && targetStatus != StatusFrozen {
		return ReopenReport{}, core.NewError(ReasonReopenNotAllowed, "intenttree.ReopenNode",
			"target must be implemented or frozen, got "+string(targetStatus))
	}
	if strings.TrimSpace(by) == "" {
		return ReopenReport{}, core.NewError(ReasonReopenNotAllowed, "intenttree.ReopenNode", "--by is required")
	}
	idx, err := ReadIndex(dir)
	if err != nil {
		return ReopenReport{}, err
	}
	entry, ok := findEntry(idx, nodeID)
	if !ok {
		return ReopenReport{}, core.NewError(ReasonNodeNotFound, "intenttree.ReopenNode", "node not found: "+nodeID)
	}
	n, err := ReadNode(dir, nodeID)
	if err != nil {
		return ReopenReport{}, err
	}
	curRank, known := lifecycleRank[n.Status]
	if !known {
		return ReopenReport{}, core.NewError(ReasonReopenNotAllowed, "intenttree.ReopenNode",
			"node is "+string(n.Status)+"; reopen only walks frozen/implemented/verified back")
	}
	if lifecycleRank[targetStatus] >= curRank {
		return ReopenReport{NodeID: nodeID, From: string(n.Status), To: string(targetStatus),
			Status: string(n.Status), Already: true}, nil
	}
	from := n.Status
	n.Status = targetStatus
	n.VerificationEvidence = nil
	if targetStatus == StatusFrozen {
		n.Implementation = nil
	}
	n, err = Stamp(n)
	if err != nil {
		return ReopenReport{}, err
	}
	if err := WriteNode(dir, n); err != nil {
		return ReopenReport{}, err
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
		return ReopenReport{}, err
	}
	now := time.Now().UTC()
	if err := st.UpdateTreeNodeStatus(n.ID, string(n.Status), n.Fingerprint, now.Format(time.RFC3339Nano)); err != nil {
		return ReopenReport{}, err
	}
	event := map[string]any{
		"type": "node_reopened", "node_id": nodeID, "by": strings.TrimSpace(by),
		"from": string(from), "to": string(targetStatus), "reason": strings.TrimSpace(reason),
		"at": now.Format(time.RFC3339Nano),
	}
	raw, _ := json.MarshalIndent(event, "", "  ")
	_ = atomicWrite(filepath.Join(dir, "events", nodeID+"."+now.Format("20060102T150405.000000000Z")+".json"), append(raw, '\n'))
	return ReopenReport{NodeID: nodeID, From: string(from), To: string(targetStatus), Status: string(targetStatus)}, nil
}
