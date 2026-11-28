package intenttree

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// GrowChange appends draft children without invalidating existing siblings.
// The complete ancestor closure is recorded and made change_pending under the
// same lock and rollback journal as other governed mutations.
func GrowChange(st *store.Store, dir, versionID, parentID, summary, by, base string, p Proposal) (rec ChangeRecord, resultErr error) {
	bad := func(s string) error { return core.NewError(ReasonChangeNotAllowed, "intenttree.GrowChange", s) }
	if strings.TrimSpace(summary) == "" || strings.TrimSpace(by) == "" {
		return rec, bad("summary and by required")
	}
	unlock, err := lockTreeChange(dir)
	if err != nil {
		return rec, err
	}
	defer unlock()
	if err = recoverTreeChange(dir, st); err != nil {
		return rec, err
	}
	idx, parent, err := readMutationNode(dir, parentID)
	if err != nil {
		return rec, err
	}
	if strings.TrimSpace(base) == "" || base != parent.Fingerprint {
		return rec, core.NewError(ReasonChangeBaseMismatch, "intenttree.GrowChange", "current base fingerprint required")
	}
	if findings := ValidateIndex(idx); len(findings) > 0 {
		return rec, bad(strings.Join(findings, "; "))
	}
	if err = ValidateProposal(parent.Kind, p); err != nil {
		return rec, err
	}
	oldCache, err := st.Tree().ListTreeNodes(versionID)
	if err != nil {
		return rec, err
	}
	scoped := false
	for _, c := range oldCache {
		if c.ID == parentID {
			scoped = true
		}
	}
	if !scoped {
		return rec, bad("version scope mismatch")
	}
	used := map[string]bool{}
	count := 0
	for _, e := range idx.Nodes {
		used[e.Path] = true
		if e.ParentID == parentID {
			count++
		}
	}
	if count+len(p.Children) > MaxChildren {
		return rec, core.NewError(ReasonChildLimit, "intenttree.GrowChange", "parent child limit exceeded")
	}
	var added []Node
	for _, ch := range p.Children {
		path := parent.Path + "/" + proposalSegment(ch.Title, ch.Slug)
		if used[path] {
			return rec, core.NewError(ReasonChildSlugCollision, "intenttree.GrowChange", "duplicate path: "+path)
		}
		used[path] = true
		n := Node{ID: shortID("n_", versionID, parentID, path), ParentID: parentID, Path: path, Kind: ch.Kind, Title: ch.Title, Intent: ch.Intent, Doc: ch.Doc, Constraint: ch.Constraint, Status: StatusDraft, ACs: ch.ACs, TestCases: ch.TestCases, DomainFSM: ch.DomainFSM, Verification: ch.Verification}
		if _, exists := findEntry(idx, n.ID); exists {
			return rec, bad("duplicate ID")
		}
		n, err = Stamp(n)
		if err != nil {
			return rec, err
		}
		added = append(added, n)
	}
	var modified []Node
	seen := map[string]bool{}
	for id := parentID; id != ""; {
		if seen[id] {
			return rec, bad("ancestor cycle")
		}
		seen[id] = true
		_, n, e := readMutationNode(dir, id)
		if e != nil {
			return rec, e
		}
		switch n.Status {
		case StatusDraft, StatusChangePending, StatusFrozen, StatusImplemented, StatusVerified, StatusDelivered:
		default:
			return rec, bad("ancestor cannot grow in " + string(n.Status))
		}
		n.Status = StatusChangePending
		n.ChildFingerprints = nil
		// Bind the new membership even while draft: a second append using the
		// same draft base must lose optimistic concurrency too.
		for _, e := range SortedIndex(idx).Nodes {
			if e.ParentID == id {
				fp := e.Fingerprint
				for _, changed := range modified {
					if changed.ID == e.ID {
						fp = changed.Fingerprint
					}
				}
				n.ChildFingerprints = append(n.ChildFingerprints, fp)
			}
		}
		if id == parentID {
			for _, child := range added {
				n.ChildFingerprints = append(n.ChildFingerprints, child.Fingerprint)
			}
		}
		n, err = Stamp(n)
		if err != nil {
			return rec, err
		}
		modified = append(modified, n)
		id = n.ParentID
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rec = ChangeRecord{ID: shortID("chg_", versionID, parentID, now), NodeID: parentID, Summary: strings.TrimSpace(summary), By: strings.TrimSpace(by), BaseFingerprint: base, CreatedAt: now, SourceHash: SourceHash(idx)}
	// Only a frozen whole-tree baseline can be snapshotted. Draft changes still
	// carry the exact pre-mutation source hash, never an implicit latest baseline.
	root, _ := findEntry(idx, idx.RootID)
	if root.Status == StatusFrozen || root.Status == StatusImplemented || root.Status == StatusVerified || root.Status == StatusDelivered {
		_, rec.BeforeSnapshot, err = CreateSnapshot(dir, idx.Version)
		if err != nil {
			return rec, err
		}
	}
	for _, n := range added {
		rec.Added = append(rec.Added, n.ID)
	}
	for _, n := range modified {
		rec.Modified = append(rec.Modified, n.ID)
	}
	rec.Affected = append(append([]string{}, rec.Modified...), rec.Added...)
	rec.DependencyScope = append([]string{}, rec.Modified...)
	bindChangeDelta(&rec, idx, "grow", rec.Added, rec.Modified, nil, rec.DependencyScope)
	recordPath := filepath.Join(dir, "changes", rec.ID+".json")
	paths := []string{IndexPath(dir), recordPath}
	for _, n := range append(added, modified...) {
		paths = append(paths, NodePath(dir, n.ID))
	}
	if err = writeMutationJournal(dir, paths, oldCache, nil); err != nil {
		return rec, err
	}
	// Enrich the existing journal with full cache replacement, including removal
	// of newly inserted rows if a write is interrupted.
	journalRaw, err := os.ReadFile(changeJournalPath(dir))
	if err != nil {
		return rec, err
	}
	var j changeJournal
	if err = json.Unmarshal(journalRaw, &j); err != nil {
		return rec, err
	}
	j.ReplaceVersion = versionID
	journalRaw, err = json.Marshal(j)
	if err != nil {
		return rec, err
	}
	if err = atomicWrite(changeJournalPath(dir), journalRaw); err != nil {
		return rec, err
	}
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, recoverTreeChange(dir, st))
		}
	}()
	cache := append([]store.TreeNode{}, oldCache...)
	for _, n := range append(added, modified...) {
		if err = WriteNode(dir, n); err != nil {
			return rec, err
		}
		e := IndexEntry{ID: n.ID, ParentID: n.ParentID, Kind: n.Kind, Status: n.Status, Title: n.Title, Path: n.Path, TruthPath: n.EffectiveTruthPath(), Constraint: n.Constraint, Fingerprint: n.Fingerprint, FingerprintAlg: n.FingerprintAlg}
		found := false
		for i := range idx.Nodes {
			if idx.Nodes[i].ID == n.ID {
				idx.Nodes[i] = e
				found = true
			}
		}
		if !found {
			idx.Nodes = append(idx.Nodes, e)
		}
		c := store.TreeNode{ID: n.ID, ProjectVersionID: versionID, ParentID: n.ParentID, Kind: string(n.Kind), Status: string(n.Status), Title: n.Title, Path: n.Path, Fingerprint: n.Fingerprint, FingerprintAlg: n.FingerprintAlg, DocPath: NodePath(dir, n.ID), CreatedAt: now, UpdatedAt: now}
		found = false
		for i := range cache {
			if cache[i].ID == n.ID {
				c.CreatedAt = cache[i].CreatedAt
				cache[i] = c
				found = true
			}
		}
		if !found {
			cache = append(cache, c)
		}
	}
	if err = WriteIndex(dir, idx); err != nil {
		return rec, err
	}
	raw, err := prettyJSON(rec)
	if err != nil {
		return rec, err
	}
	if err = atomicWrite(recordPath, raw); err != nil {
		return rec, err
	}
	if err = st.ReplaceTreeNodes(versionID, cache); err != nil {
		return rec, err
	}
	if err = os.Remove(changeJournalPath(dir)); err != nil {
		return rec, err
	}
	committed = true
	return rec, nil
}
