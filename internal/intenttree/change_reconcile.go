package intenttree

import (
	"encoding/json"
	"errors"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

func projectionEntry(n Node) IndexEntry {
	return IndexEntry{ID: n.ID, ParentID: n.ParentID, Kind: n.Kind, Status: n.Status, Title: n.Title, Path: n.Path, TruthPath: n.EffectiveTruthPath(), Constraint: n.Constraint, Fingerprint: n.Fingerprint, FingerprintAlg: n.FingerprintAlg}
}

// Legacy-only links may be omitted by the index contract. Preserve that
// representation without relaxing explicit truth links or any other field.
func compatibleProjectionEntry(n Node, existing IndexEntry) IndexEntry {
	e := projectionEntry(n)
	if existing.TruthPath == "" && n.TruthPath == "" && n.LegacyTruthPath != "" {
		e.TruthPath = ""
	}
	return e
}

func controlledChangeError(message string) error {
	return core.NewError(ReasonChangeNotAllowed, "intenttree.controlledChange", message)
}

// readControlledTree tolerates projection drift only for the explicitly named
// repair targets. ID/kind and canonical hashes are never repaired here.
func readControlledTree(st *store.Store, dir, versionID string, repairs map[string]string, pathRepairs ...map[string]ReconcileTarget) (Index, map[string]Node, []store.TreeNode, error) {
	idx, err := ReadIndex(dir)
	if err != nil {
		return idx, nil, nil, err
	}
	fail := func(s string) (Index, map[string]Node, []store.TreeNode, error) {
		return idx, nil, nil, controlledChangeError(s)
	}
	cache, err := st.Tree().ListTreeNodes(versionID)
	if err != nil {
		return idx, nil, nil, err
	}
	if repairs == nil && len(cache) != len(idx.Nodes) {
		return fail("cache version scope or membership mismatch")
	}
	cached := map[string]store.TreeNode{}
	for _, c := range cache {
		cached[c.ID] = c
	}
	nodes := map[string]Node{}
	for i, e := range idx.Nodes {
		if strings.TrimSpace(e.ID) == "" || nodes[e.ID].ID != "" {
			return fail("empty or duplicate identity")
		}
		// Reject path aliases before using IDs as filesystem components.
		if filepath.Base(e.ID) != e.ID || e.ID == "." || e.ID == ".." {
			return fail("invalid node identity")
		}
		n, err := ReadNode(dir, e.ID)
		if err != nil {
			return idx, nil, nil, err
		}
		valid, err := VerifyFingerprint(n)
		if err != nil {
			return idx, nil, nil, err
		}
		if !valid {
			return fail("invalid canonical fingerprint: " + e.ID)
		}
		base, repair := repairs[e.ID]
		if repair && (strings.TrimSpace(base) == "" || base != n.Fingerprint) {
			return idx, nil, nil, core.NewError(ReasonChangeBaseMismatch, "intenttree.ReconcileIndex", "current body fingerprint required: "+e.ID)
		}
		if n.ID != e.ID || n.Kind != e.Kind {
			return fail("canonical identity differs: " + e.ID)
		}
		c, ok := cached[e.ID]
		if repairs == nil && (!ok || c.ProjectVersionID != versionID || c.Kind != string(n.Kind)) {
			return fail("cache identity or scope differs: " + e.ID)
		}
		if !repair && (e != compatibleProjectionEntry(n, e) || (repairs == nil && (c.ParentID != n.ParentID || c.Path != n.Path || c.Title != n.Title || c.Status != string(n.Status) || c.Fingerprint != n.Fingerprint || c.FingerprintAlg != n.FingerprintAlg || store.ResolveRootPath(c.DocPath) != store.ResolveRootPath(NodePath(dir, n.ID))))) {
			return fail("other projection drift: " + e.ID)
		}
		nodes[n.ID] = n
		if repair {
			idx.Nodes[i] = compatibleProjectionEntry(n, e)
		}
	}
	for id := range repairs {
		if _, ok := nodes[id]; !ok {
			return fail("repair node not found: " + id)
		}
	}
	if findings := ValidateIndex(idx); len(findings) > 0 {
		return fail(strings.Join(findings, "; "))
	}
	for id, n := range nodes {
		seen := map[string]bool{}
		for p := id; p != ""; p = nodes[p].ParentID {
			if seen[p] {
				return fail("ancestor cycle")
			}
			seen[p] = true
		}
		if n.ParentID == "" && id != idx.RootID {
			return fail("disconnected root")
		}
	}
	if len(pathRepairs) > 0 {
		if _, err := applyRepairPaths(nodes, pathRepairs[0]); err != nil {
			return idx, nil, nil, err
		}
		for i, e := range idx.Nodes {
			idx.Nodes[i] = compatibleProjectionEntry(nodes[e.ID], e)
		}
	}
	if err := validateRepairPaths(nodes); err != nil {
		return idx, nil, nil, err
	}
	return idx, nodes, cache, nil
}

func validRepairPath(p string) bool {
	return strings.TrimSpace(p) != "" && p == strings.TrimSpace(p) && !strings.Contains(p, "\\") && !path.IsAbs(p) && path.Clean(p) == p && p != "." && p != ".." && !strings.HasPrefix(p, "../")
}

func validateRepairPaths(nodes map[string]Node) error {
	seen := map[string]bool{}
	for id, n := range nodes {
		if !validRepairPath(n.Path) || seen[n.Path] {
			return controlledChangeError("invalid or duplicate path: " + id)
		}
		seen[n.Path] = true
	}
	return nil
}

// Explicit module roots authorize only their draft parent-defined subtrees.
// Existing path prefixes are not used to determine subtree membership.
func applyRepairPaths(nodes map[string]Node, desired map[string]ReconcileTarget) ([]Node, error) {
	owned := map[string]bool{}
	bodies := []Node{}
	roots := []string{}
	for id := range desired {
		roots = append(roots, id)
	}
	sort.Strings(roots)
	var walk func(string, string) error
	walk = func(id, p string) error {
		if owned[id] {
			return controlledChangeError("overlapping path repair subtrees")
		}
		owned[id] = true
		n := nodes[id]
		if n.Status != StatusDraft {
			return controlledChangeError("path repair subtree must be draft: " + id)
		}
		if !validRepairPath(p) {
			return controlledChangeError("invalid desired path")
		}
		n.Path = p
		children := []string{}
		for cid, c := range nodes {
			if c.ParentID == id {
				children = append(children, cid)
			}
		}
		sort.Strings(children)
		n.ChildFingerprints = nil
		for _, cid := range children {
			seg := path.Base(nodes[cid].Path)
			if !validRepairPath(seg) {
				return controlledChangeError("invalid child path segment")
			}
			if err := walk(cid, p+"/"+seg); err != nil {
				return err
			}
			n.ChildFingerprints = append(n.ChildFingerprints, nodes[cid].Fingerprint)
		}
		var err error
		n, err = Stamp(n)
		if err != nil {
			return err
		}
		nodes[id] = n
		bodies = append(bodies, n)
		return nil
	}
	for _, id := range roots {
		n, ok := nodes[id]
		if !ok || n.Kind != KindModule {
			return nil, controlledChangeError("path repair requires module")
		}
		target := desired[id]
		if target.Subtree != nil && !*target.Subtree {
			if owned[id] {
				return nil, controlledChangeError("overlapping path repair subtrees")
			}
			if n.Status != StatusDraft {
				return nil, controlledChangeError("path repair target must be draft: " + id)
			}
			if !validRepairPath(*target.Path) {
				return nil, controlledChangeError("invalid desired path")
			}
			owned[id] = true
			n.Path = *target.Path
			var err error
			n, err = Stamp(n)
			if err != nil {
				return nil, err
			}
			nodes[id] = n
			bodies = append(bodies, n)
			continue
		}
		if err := walk(id, *target.Path); err != nil {
			return nil, err
		}
	}
	return bodies, nil
}

// Projection-only reconcile passes no bodies. Explicit repair-path batches
// journal each authorized draft body alongside the complete old scoped cache.
func persistControlledChange(st *store.Store, dir, versionID string, idx Index, cache []store.TreeNode, rec any, bodies []Node, projections []Node) (resultErr error) {
	var record ChangeRecord
	switch r := rec.(type) {
	case ChangeRecord:
		record = r
	case ReconcileRecord:
		record = r.ChangeRecord
	default:
		return controlledChangeError("invalid change record")
	}
	recordPath := filepath.Join(dir, "changes", record.ID+".json")
	paths := []string{IndexPath(dir), recordPath}
	for _, n := range bodies {
		paths = append(paths, NodePath(dir, n.ID))
	}
	if err := writeMutationJournal(dir, paths, cache, nil); err != nil {
		return err
	}
	// Replace the scoped cache in one transaction, including path swaps between
	// authorized targets. Recovery must use the same replacement operation.
	journalRaw, err := os.ReadFile(changeJournalPath(dir))
	if err != nil {
		return err
	}
	var journal changeJournal
	if err = json.Unmarshal(journalRaw, &journal); err != nil {
		return err
	}
	journal.ReplaceVersion = versionID
	journalRaw, err = json.Marshal(journal)
	if err != nil {
		return err
	}
	if err = atomicWrite(changeJournalPath(dir), journalRaw); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, recoverTreeChange(dir, st))
		}
	}()
	for _, n := range bodies {
		if err := WriteNode(dir, n); err != nil {
			return err
		}
	}
	for _, n := range projections {
		for i := range idx.Nodes {
			if idx.Nodes[i].ID == n.ID {
				idx.Nodes[i] = compatibleProjectionEntry(n, idx.Nodes[i])
			}
		}
		found := false
		for i, c := range cache {
			if c.ID == n.ID {
				found = true
				c.ParentID = n.ParentID
				c.Kind = string(n.Kind)
				c.Path = n.Path
				c.Status = string(n.Status)
				c.Title = n.Title
				c.Fingerprint = n.Fingerprint
				c.FingerprintAlg = n.FingerprintAlg
				c.DocPath = NodePath(dir, n.ID)
				c.UpdatedAt = record.CreatedAt
				cache[i] = c
			}
		}
		if !found {
			cache = append(cache, store.TreeNode{ID: n.ID, ProjectVersionID: versionID, ParentID: n.ParentID, Kind: string(n.Kind), Path: n.Path, Status: string(n.Status), Title: n.Title, Fingerprint: n.Fingerprint, FingerprintAlg: n.FingerprintAlg, DocPath: NodePath(dir, n.ID), CreatedAt: record.CreatedAt, UpdatedAt: record.CreatedAt})
		}
	}
	// Remove stale derived members; canonical index membership is authoritative.
	keep := map[string]bool{}
	for _, e := range idx.Nodes {
		keep[e.ID] = true
	}
	rebuilt := []store.TreeNode{}
	for _, c := range cache {
		if keep[c.ID] {
			rebuilt = append(rebuilt, c)
		}
	}
	cache = rebuilt
	if err := WriteIndex(dir, idx); err != nil {
		return err
	}
	raw, err := prettyJSON(rec)
	if err != nil {
		return err
	}
	if err = atomicWrite(recordPath, raw); err != nil {
		return err
	}
	if err = st.ReplaceTreeNodes(journal.ReplaceVersion, cache); err != nil {
		return err
	}
	if err = os.Remove(changeJournalPath(dir)); err != nil {
		return err
	}
	committed = true
	return nil
}

func newControlledRecord(idx Index, versionID, node, summary, by, base, action string, modified, deps []string) ChangeRecord {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rec := ChangeRecord{ID: shortID("chg_", versionID, node, now), NodeID: node, Summary: strings.TrimSpace(summary), By: strings.TrimSpace(by), BaseFingerprint: base, CreatedAt: now, SourceHash: SourceHash(idx), Affected: append([]string{}, modified...)}
	bindChangeDelta(&rec, idx, action, nil, modified, nil, deps)
	return rec
}

// ReconcileIndex projects one valid canonical body without altering its identity.
func ReconcileIndex(st *store.Store, dir, versionID, node, summary, by, base string) (ChangeRecord, error) {
	rec, err := ReconcileIndexes(st, dir, versionID, []ReconcileTarget{{NodeID: node, BaseFingerprint: base}}, summary, by)
	return rec.ChangeRecord, err
}

type ReconcileTarget struct {
	NodeID          string  `json:"node_id"`
	BaseFingerprint string  `json:"base_fingerprint"`
	Path            *string `json:"path,omitempty"`
	Subtree         *bool   `json:"subtree,omitempty"`
}

// ReconcileRecord persists the exact authorized targets and their body bases.
type ReconcileRecord struct {
	ChangeRecord
	Nodes []ReconcileTarget `json:"nodes"`
}

func ReconcileIndexes(st *store.Store, dir, versionID string, targets []ReconcileTarget, summary, by string) (ReconcileRecord, error) {
	var rec ReconcileRecord
	if len(targets) == 0 || strings.TrimSpace(summary) == "" || strings.TrimSpace(by) == "" {
		return rec, controlledChangeError("nonempty targets, summary and by required")
	}
	repairs := map[string]string{}
	desired := map[string]ReconcileTarget{}
	ids := []string{}
	for _, t := range targets {
		if strings.TrimSpace(t.NodeID) == "" {
			return rec, controlledChangeError("node required")
		}
		if _, ok := repairs[t.NodeID]; ok {
			return rec, controlledChangeError("duplicate repair target")
		}
		if strings.TrimSpace(t.BaseFingerprint) == "" {
			return rec, core.NewError(ReasonChangeBaseMismatch, "intenttree.ReconcileIndexes", "base fingerprint required")
		}
		repairs[t.NodeID] = t.BaseFingerprint
		if t.Subtree != nil && t.Path == nil {
			return rec, controlledChangeError("subtree requires explicit path")
		}
		if t.Path != nil {
			if !validRepairPath(*t.Path) {
				return rec, controlledChangeError("invalid desired path")
			}
			desired[t.NodeID] = t
		}
		ids = append(ids, t.NodeID)
	}
	unlock, err := lockTreeChange(dir)
	if err != nil {
		return rec, err
	}
	defer unlock()
	if err = recoverTreeChange(dir, st); err != nil {
		return rec, err
	}
	// Keep the original source hash in the change record, while validation and
	// persistence consume the complete corrected projection.
	before, err := ReadIndex(dir)
	if err != nil {
		return rec, err
	}
	idx, nodes, cache, err := readControlledTree(st, dir, versionID, repairs, desired)
	if err != nil {
		return rec, err
	}
	projections := []Node{}
	bodies := []Node{}
	modified := map[string]bool{}
	for _, id := range ids {
		modified[id] = true
	}
	for _, e := range idx.Nodes {
		n := nodes[e.ID]
		projections = append(projections, n)
		old, err := ReadNode(dir, n.ID)
		if err != nil {
			return rec, err
		}
		if old.Fingerprint != n.Fingerprint {
			bodies = append(bodies, n)
			modified[n.ID] = true
		}
	}
	changed := []string{}
	for id := range modified {
		changed = append(changed, id)
	}
	sort.Strings(changed)
	action := "reconcile-index"
	if len(desired) > 0 {
		action = "repair-path"
	}
	rec = ReconcileRecord{ChangeRecord: newControlledRecord(before, versionID, ids[0], summary, by, repairs[ids[0]], action, changed, nil), Nodes: append([]ReconcileTarget{}, targets...)}
	return rec, persistControlledChange(st, dir, versionID, idx, cache, rec, bodies, projections)
}
