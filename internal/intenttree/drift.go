package intenttree

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"beacon-go/internal/core"
)

// DriftEntry is one requirement-level difference. Identity and location
// are reported, while lifecycle fingerprints are never treated as truth.
type DriftEntry struct {
	ID    string `json:"id"`
	Path  string `json:"path"`
	Title string `json:"title"`
	Kind  Kind   `json:"kind"`
}

// DriftReport compares the current tree with the latest immutable snapshot.
type DriftReport struct {
	Schema     string       `json:"schema"`
	SnapshotID string       `json:"snapshot_id"`
	NodeID     string       `json:"node_id"`
	Added      []DriftEntry `json:"added"`
	Changed    []DriftEntry `json:"changed"`
	Removed    []DriftEntry `json:"removed"`
}

// RequirementBody is the semantic surface of a node. Status, grouping,
// fingerprints, truth-package links and execution evidence are outside it:
// regrouping or recomputing a fingerprint is not requirement drift.
type RequirementBody struct {
	Title        string        `json:"title"`
	Intent       string        `json:"intent"`
	Doc          string        `json:"doc"`
	Constraint   bool          `json:"constraint"`
	ACs          []AC          `json:"acs"`
	TestCases    []TestCase    `json:"test_cases"`
	DomainFSM    *DomainFSM    `json:"domain_fsm"`
	Verification *Verification `json:"verification"`
}

func requirementBody(n Node) RequirementBody {
	b := RequirementBody{
		Title: n.Title, Intent: n.Intent, Doc: n.Doc, Constraint: n.Constraint,
		ACs: n.ACs, TestCases: n.TestCases, DomainFSM: n.DomainFSM,
		Verification: n.Verification,
	}
	if b.ACs == nil {
		b.ACs = []AC{}
	}
	if b.TestCases == nil {
		b.TestCases = []TestCase{}
	}
	return b
}

func requirementHash(n Node) (string, error) {
	raw, err := json.Marshal(requirementBody(n))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// Drift compares the subtree rooted at nodeID against one explicit snapshot
// key ("latest" is an accepted key, not an implicit lookup). A missing node
// in either side is reported as added or removed; bodies present on both
// sides are compared by RequirementBody only.
func Drift(dir, nodeID, snapshotID string) (DriftReport, error) {
	if strings.TrimSpace(snapshotID) == "" {
		return DriftReport{}, core.NewError(ReasonSnapshotKeyRequired, "intenttree.Drift", "snapshot key is required")
	}
	if strings.TrimSpace(snapshotID) == "latest" {
		id, _, err := latestSnapshot(dir)
		if err != nil {
			return DriftReport{}, err
		}
		if id == "" {
			return DriftReport{}, core.NewError(ReasonNodeNotFound, "intenttree.Drift", "no snapshot baseline; create one with truth snapshot create")
		}
		snapshotID = id
	}
	snap, baseBodies, err := ReadSnapshotWithBodies(dir, snapshotID)
	if err != nil {
		return DriftReport{}, err
	}
	idx, err := ReadIndex(dir)
	if err != nil {
		return DriftReport{}, err
	}
	driftRootID := strings.TrimSpace(nodeID)
	if driftRootID == "" {
		driftRootID = idx.RootID
	}

	curChildren := childrenByIndex(idx.Nodes)
	baseChildren := childrenBySnapshot(snap.Nodes)
	curScope := subtreeIDs(driftRootID, curChildren)
	baseScope := subtreeIDs(driftRootID, baseChildren)
	if len(curScope) == 0 && len(baseScope) == 0 {
		return DriftReport{}, core.NewError(ReasonNodeNotFound, "intenttree.Drift", "node not found: "+driftRootID)
	}

	curBodies := map[string]Node{}
	for id := range curScope {
		n, err := ReadNode(dir, id)
		if err != nil {
			return DriftReport{}, err
		}
		curBodies[id] = n
	}
	for id := range baseBodies {
		if !baseScope[id] {
			delete(baseBodies, id)
		}
	}

	rep := DriftReport{
		Schema: "beacon-intent-tree-drift-v1", SnapshotID: snap.ID, NodeID: driftRootID,
		Added: []DriftEntry{}, Changed: []DriftEntry{}, Removed: []DriftEntry{},
	}
	for id, n := range curBodies {
		old, ok := baseBodies[id]
		if !ok {
			rep.Added = append(rep.Added, driftEntry(n))
			continue
		}
		before, err := requirementHash(old)
		if err != nil {
			return DriftReport{}, err
		}
		after, err := requirementHash(n)
		if err != nil {
			return DriftReport{}, err
		}
		if before != after {
			rep.Changed = append(rep.Changed, driftEntry(n))
		}
	}
	for id, n := range baseBodies {
		if _, ok := curBodies[id]; !ok {
			rep.Removed = append(rep.Removed, driftEntry(n))
		}
	}
	sortDrift(rep.Added)
	sortDrift(rep.Changed)
	sortDrift(rep.Removed)
	return rep, nil
}


func childrenByIndex(nodes []IndexEntry) map[string][]string {
	out := map[string][]string{}
	for _, e := range nodes {
		out[e.ParentID] = append(out[e.ParentID], e.ID)
	}
	return out
}

func childrenBySnapshot(nodes []SnapshotEntry) map[string][]string {
	out := map[string][]string{}
	for _, e := range nodes {
		out[e.ParentID] = append(out[e.ParentID], e.ID)
	}
	return out
}

func subtreeIDs(root string, children map[string][]string) map[string]bool {
	seen := map[string]bool{}
	var walk func(string)
	walk = func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true
		for _, ch := range children[id] {
			walk(ch)
		}
	}
	walk(root)
	return seen
}

func driftEntry(n Node) DriftEntry {
	return DriftEntry{ID: n.ID, Path: n.Path, Title: n.Title, Kind: n.Kind}
}

func sortDrift(entries []DriftEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Path != entries[j].Path {
			return entries[i].Path < entries[j].Path
		}
		return entries[i].ID < entries[j].ID
	})
}
