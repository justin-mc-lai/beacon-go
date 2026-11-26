package intenttree

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// ChangeRecord is one subtree change request, stored under tree/changes/.
type ChangeRecord struct {
	ID              string   `json:"id"`
	NodeID          string   `json:"node_id"`
	Summary         string   `json:"summary"`
	By              string   `json:"by"`
	BaseFingerprint string   `json:"base_fingerprint"`
	Affected        []string `json:"affected"`
	CreatedAt       string   `json:"created_at"`
}

// RequestChange invalidates one frozen subtree: the target and every
// descendant become draft, every ancestor becomes change_pending, and
// one immutable change record is written.
func RequestChange(st *store.Store, dir, versionID, nodeID, summary, by, base string) (ChangeRecord, error) {
	if strings.TrimSpace(summary) == "" {
		return ChangeRecord{}, core.NewError(ReasonChangeSummaryMissing, "intenttree.RequestChange", "summary is required")
	}
	idx, err := ReadIndex(dir)
	if err != nil {
		return ChangeRecord{}, err
	}
	target, ok := findEntry(idx, nodeID)
	if !ok {
		return ChangeRecord{}, core.NewError(ReasonNodeNotFound, "intenttree.RequestChange", "node not found: "+nodeID)
	}
	switch target.Status {
	case StatusFrozen, StatusImplemented, StatusVerified:
	default:
		return ChangeRecord{}, core.NewError(ReasonChangeNotAllowed, "intenttree.RequestChange",
			"change requires frozen truth; "+nodeID+" is "+string(target.Status))
	}
	if base != "" && base != target.Fingerprint {
		return ChangeRecord{}, core.NewError(ReasonChangeBaseMismatch, "intenttree.RequestChange",
			"base fingerprint does not match current node; rebase first")
	}
	children := map[string][]IndexEntry{}
	for _, n := range idx.Nodes {
		children[n.ParentID] = append(children[n.ParentID], n)
	}
	var affected []string
	var walk func(string)
	walk = func(id string) {
		affected = append(affected, id)
		for _, ch := range children[id] {
			walk(ch.ID)
		}
	}
	walk(nodeID)
	sort.Strings(affected)

	now := time.Now().UTC()
	rec := ChangeRecord{
		ID:     shortID("chg_", versionID, nodeID, summary, now.Format(time.RFC3339Nano)),
		NodeID: nodeID, Summary: strings.TrimSpace(summary), By: strings.TrimSpace(by),
		BaseFingerprint: target.Fingerprint, Affected: affected,
		CreatedAt: now.Format(time.RFC3339Nano),
	}
	raw, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return ChangeRecord{}, err
	}
	if err := atomicWrite(filepath.Join(dir, "changes", rec.ID+".json"), append(raw, '\n')); err != nil {
		return ChangeRecord{}, err
	}
	// Target subtree becomes editable draft.
	for _, id := range affected {
		if err := setNodeStatus(st, dir, &idx, id, StatusDraft); err != nil {
			return ChangeRecord{}, err
		}
	}
	// Ancestors stop claiming frozen truth.
	for parent := target.ParentID; parent != ""; {
		if err := setNodeStatus(st, dir, &idx, parent, StatusChangePending); err != nil {
			return ChangeRecord{}, err
		}
		p, ok := findEntry(idx, parent)
		if !ok {
			break
		}
		parent = p.ParentID
	}
	if err := WriteIndex(dir, idx); err != nil {
		return ChangeRecord{}, err
	}
	return rec, nil
}

// ListChanges reads all change records, newest first.
func ListChanges(dir string) ([]ChangeRecord, error) {
	changeDir := filepath.Join(dir, "changes")
	entries, err := os.ReadDir(changeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []ChangeRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(changeDir, e.Name()))
		if err != nil {
			return nil, err
		}
		var rec ChangeRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out, nil
}

// SetNodeStatus applies one explicit status transition to a node and its
// cached index rows. The caller owns the transition's policy checks.
func SetNodeStatus(st *store.Store, dir, id string, status Status) error {
	idx, err := ReadIndex(dir)
	if err != nil {
		return err
	}
	if err := setNodeStatus(st, dir, &idx, id, status); err != nil {
		return err
	}
	return WriteIndex(dir, idx)
}

func setNodeStatus(st *store.Store, dir string, idx *Index, id string, status Status) error {
	entry, ok := findEntry(*idx, id)
	if !ok {
		return core.NewError(ReasonNodeNotFound, "intenttree.setNodeStatus", "node not found: "+id)
	}
	n, err := ReadNode(dir, id)
	if err != nil {
		return err
	}
	n.Status = status
	n, err = Stamp(n)
	if err != nil {
		return err
	}
	if err := WriteNode(dir, n); err != nil {
		return err
	}
	entry.Status = status
	entry.Fingerprint = n.Fingerprint
	entry.FingerprintAlg = n.FingerprintAlg
	for i := range idx.Nodes {
		if idx.Nodes[i].ID == id {
			idx.Nodes[i] = entry
		}
	}
	return st.UpdateTreeNodeStatus(id, string(status), n.Fingerprint, time.Now().UTC().Format(time.RFC3339Nano))
}
