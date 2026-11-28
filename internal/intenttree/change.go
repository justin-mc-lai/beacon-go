package intenttree

import (
	"encoding/base64"
	"encoding/json"
	"errors"
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
	Action          string   `json:"action"`
	ID              string   `json:"id"`
	NodeID          string   `json:"node_id"`
	Summary         string   `json:"summary"`
	By              string   `json:"by"`
	BaseFingerprint string   `json:"base_fingerprint"`
	BeforeSnapshot  string   `json:"before_snapshot_id,omitempty"`
	Affected        []string `json:"affected"`
	CreatedAt       string   `json:"created_at"`
	Added           []string `json:"added"`
	Modified        []string `json:"modified"`
	Deprecated      []string `json:"deprecated"`
	DependencyScope []string `json:"dependency_scope"`
	SourceHash      string   `json:"source_hash,omitempty"`
	SourceCommit    string   `json:"source_commit,omitempty"`
	BinarySHA       string   `json:"binary_sha256,omitempty"`
}

type changeJournal struct {
	TreeDir        string           `json:"tree_dir"`
	Files          []changeFileCopy `json:"files"`
	Cache          []store.TreeNode `json:"cache,omitempty"`
	RemoveDirs     []string         `json:"remove_dirs,omitempty"`
	ReplaceVersion string           `json:"replace_version,omitempty"`
}

type changeFileCopy struct {
	Path string `json:"path"`
	Old  string `json:"old_base64"`
}

// recoverTreeChange restores a tree left half-written by an interrupted
// change. Callers hold the tree lock. The journal lives outside the tree so
// the canonical artifact never gains runtime files.
func recoverTreeChange(dir string, stores ...*store.Store) error {
	path := changeJournalPath(dir)
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var j changeJournal
	if err := json.Unmarshal(raw, &j); err != nil {
		return err
	}
	if j.TreeDir != dir {
		want, err1 := filepath.EvalSymlinks(dir)
		got, err2 := filepath.EvalSymlinks(j.TreeDir)
		if err1 != nil || err2 != nil || want != got {
			return core.NewError(ReasonStateConflict, "intenttree.recoverTreeChange", "journal belongs to another tree")
		}
	}
	for _, f := range j.Files {
		if f.Old == "" {
			if err := os.Remove(f.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			continue
		}
		old, err := base64.StdEncoding.DecodeString(f.Old)
		if err != nil {
			return err
		}
		if err := atomicWrite(f.Path, old); err != nil {
			return err
		}
	}
	for _, path := range j.RemoveDirs {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	if j.ReplaceVersion != "" {
		if len(stores) != 1 || stores[0] == nil {
			return core.NewError(ReasonStateConflict, "intenttree.recoverTreeChange", "cache recovery requires scoped store")
		}
		if err := stores[0].ReplaceTreeNodes(j.ReplaceVersion, j.Cache); err != nil {
			return err
		}
	}
	for _, cached := range j.Cache {
		if len(stores) != 1 || stores[0] == nil {
			return core.NewError(ReasonStateConflict, "intenttree.recoverTreeChange", "cache recovery requires the scoped store")
		}
		current, err := stores[0].Tree().GetTreeNode(cached.ID)
		if err != nil {
			return err
		}
		if current != cached {
			if err := stores[0].UpdateTreeNodeProjection(cached); err != nil {
				return err
			}
		}
	}
	return os.Remove(path)
}

func changeJournalPath(dir string) string {
	return filepath.Join(lockDirForTree(dir), "tree-change-journal.json")
}

func writeChangeJournal(dir string, paths []string) error {
	return writeMutationJournal(dir, paths, nil, nil)
}

func writeMutationJournal(dir string, paths []string, cache []store.TreeNode, removeDirs []string) error {
	j := changeJournal{TreeDir: dir, Cache: cache, RemoveDirs: removeDirs}
	for _, p := range paths {
		old, err := os.ReadFile(p)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		item := changeFileCopy{Path: p}
		if err == nil {
			item.Old = base64.StdEncoding.EncodeToString(old)
		}
		j.Files = append(j.Files, item)
	}
	raw, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(changeJournalPath(dir), append(raw, '\n'))
}

// RequestChange invalidates one frozen subtree: the target and every
// descendant become draft, every ancestor becomes change_pending, and
// one immutable change record is written. The caller must pass the target's
// current fingerprint; a stale base is refused before anything is written.
func RequestChange(st *store.Store, dir, versionID, nodeID, summary, by, base string) (ChangeRecord, error) {
	if strings.TrimSpace(summary) == "" {
		return ChangeRecord{}, core.NewError(ReasonChangeSummaryMissing, "intenttree.RequestChange", "summary is required")
	}
	unlock, err := lockTreeChange(dir)
	if err != nil {
		return ChangeRecord{}, err
	}
	defer unlock()
	if err := recoverTreeChange(dir, st); err != nil {
		return ChangeRecord{}, err
	}
	idx, err := ReadIndex(dir)
	if err != nil {
		return ChangeRecord{}, err
	}
	target, ok := findEntry(idx, nodeID)
	if !ok {
		return ChangeRecord{}, core.NewError(ReasonNodeNotFound, "intenttree.RequestChange", "node not found: "+nodeID)
	}
	// Re-read the node body after taking the lock. The index is a cache;
	// the canonical fingerprint is the one stamped into the node on disk.
	fresh, err := ReadNode(dir, nodeID)
	if err != nil {
		return ChangeRecord{}, err
	}
	// A stale base wins over the state check: after the first writer commits,
	// the second must see base mismatch, not a bare draft-state error.
	if strings.TrimSpace(base) != "" && base != fresh.Fingerprint {
		return ChangeRecord{}, core.NewError(ReasonChangeBaseMismatch, "intenttree.RequestChange",
			"base fingerprint does not match current node; rebase first")
	}
	switch fresh.Status {
	case StatusFrozen, StatusImplemented, StatusVerified, StatusDelivered:
	default:
		return ChangeRecord{}, core.NewError(ReasonChangeNotAllowed, "intenttree.RequestChange",
			"change requires frozen truth; "+nodeID+" is "+string(fresh.Status))
	}
	if strings.TrimSpace(base) == "" {
		return ChangeRecord{}, core.NewError(ReasonChangeBaseMismatch, "intenttree.RequestChange",
			"base fingerprint is required")
	}
	target.Fingerprint = fresh.Fingerprint
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

	var ancestors []string
	for parent := target.ParentID; parent != ""; {
		ancestors = append(ancestors, parent)
		p, ok := findEntry(idx, parent)
		if !ok {
			break
		}
		parent = p.ParentID
	}

	now := time.Now().UTC()
	beforeSnapshot := ""
	if _, sid, serr := CreateSnapshot(dir, idx.Version); serr == nil {
		beforeSnapshot = sid
	}
	id := shortID("chg_", versionID, nodeID, summary, now.Format(time.RFC3339Nano))
	rec := ChangeRecord{
		ID:     id,
		NodeID: nodeID, Summary: strings.TrimSpace(summary), By: strings.TrimSpace(by),
		BaseFingerprint: fresh.Fingerprint, BeforeSnapshot: beforeSnapshot, Affected: affected,
		CreatedAt: now.Format(time.RFC3339Nano),
	}
	bindChangeDelta(&rec, idx, "request", nil, append(append([]string{}, affected...), ancestors...), nil, ancestors)
	raw, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return ChangeRecord{}, err
	}

	paths := []string{IndexPath(dir), filepath.Join(dir, "changes", id+".json")}
	for _, node := range append(append([]string{}, affected...), ancestors...) {
		paths = append(paths, NodePath(dir, node))
	}
	if err := writeChangeJournal(dir, paths); err != nil {
		return ChangeRecord{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = recoverTreeChange(dir, st)
		}
	}()

	if err := atomicWrite(filepath.Join(dir, "changes", id+".json"), append(raw, '\n')); err != nil {
		return ChangeRecord{}, err
	}
	// Target subtree becomes editable draft.
	for _, id := range affected {
		if err := setNodeStatus(st, dir, &idx, id, StatusDraft); err != nil {
			return ChangeRecord{}, err
		}
	}
	// Ancestors stop claiming frozen truth.
	for _, parent := range ancestors {
		if err := setNodeStatus(st, dir, &idx, parent, StatusChangePending); err != nil {
			return ChangeRecord{}, err
		}
	}
	if err := WriteIndex(dir, idx); err != nil {
		return ChangeRecord{}, err
	}
	if err := os.Remove(changeJournalPath(dir)); err != nil {
		return ChangeRecord{}, err
	}
	committed = true
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
	if status == StatusDeprecated {
		return core.NewError("tree_deprecate_invalid", "intenttree.SetNodeStatus", "use DeprecateDraft to create the required change record")
	}
	idx, err := ReadIndex(dir)
	if err != nil {
		return err
	}
	current, err := ReadNode(dir, id)
	if err != nil {
		return err
	}
	if current.Status == StatusDeprecated {
		return core.NewError("tree_deprecate_invalid", "intenttree.SetNodeStatus", "deprecated node cannot be reactivated by status synchronization")
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
