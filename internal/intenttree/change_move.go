package intenttree

import (
	"path"
	"sort"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// MoveChange relocates an editable module/feature while preserving node and AC IDs.
func MoveChange(st *store.Store, dir, versionID, node, parent, summary, by, base string) (ChangeRecord, error) {
	var rec ChangeRecord
	if strings.TrimSpace(node) == "" || strings.TrimSpace(parent) == "" || strings.TrimSpace(summary) == "" || strings.TrimSpace(by) == "" {
		return rec, controlledChangeError("node, parent, summary and by required")
	}
	unlock, err := lockTreeChange(dir)
	if err != nil {
		return rec, err
	}
	defer unlock()
	if err = recoverTreeChange(dir, st); err != nil {
		return rec, err
	}
	idx, nodes, cache, err := readControlledTree(st, dir, versionID, nil)
	if err != nil {
		return rec, err
	}
	n, ok := nodes[node]
	if !ok {
		return rec, controlledChangeError("node not found")
	}
	if strings.TrimSpace(base) == "" || base != n.Fingerprint {
		return rec, core.NewError(ReasonChangeBaseMismatch, "intenttree.MoveChange", "current body fingerprint required")
	}
	p, ok := nodes[parent]
	if !ok {
		return rec, controlledChangeError("parent not found")
	}
	if n.Status != StatusDraft || (n.Kind != KindModule && n.Kind != KindFeature) || n.ParentID == "" || n.ParentID == parent {
		return rec, controlledChangeError("only draft module/feature reparenting allowed")
	}
	if err = ValidateChild(p.Kind, n.Kind); err != nil {
		return rec, err
	}
	deps := map[string]bool{}
	for _, start := range []string{n.ParentID, parent} {
		for id := start; id != ""; id = nodes[id].ParentID {
			if id == node {
				return rec, controlledChangeError("move would create cycle")
			}
			a := nodes[id]
			if a.Status != StatusDraft && a.Status != StatusChangePending {
				return rec, controlledChangeError("ancestor must be draft or change_pending: " + id)
			}
			deps[id] = true
		}
	}
	count := 0
	for _, e := range idx.Nodes {
		if e.ParentID == parent {
			count++
		}
	}
	if count >= MaxChildren {
		return rec, core.NewError(ReasonChildLimit, "intenttree.MoveChange", "destination full")
	}
	newPath := p.Path + "/" + path.Base(n.Path)
	moved := map[string]bool{}
	var relocate func(string, string) error
	relocate = func(id, target string) error {
		v := nodes[id]
		if v.Status != StatusDraft {
			return controlledChangeError("subtree must be draft: " + id)
		}
		moved[id] = true
		v.Path = target
		if id == node {
			v.ParentID = parent
		}
		nodes[id] = v
		for _, e := range idx.Nodes {
			if e.ParentID == id {
				if err := relocate(e.ID, target+"/"+path.Base(e.Path)); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err = relocate(node, newPath); err != nil {
		return rec, err
	}
	paths := map[string]bool{}
	for _, v := range nodes {
		if paths[v.Path] {
			return rec, controlledChangeError("destination path collision")
		}
		paths[v.Path] = true
	}
	modified := map[string]bool{}
	for id := range moved {
		modified[id] = true
	}
	for id := range deps {
		modified[id] = true
	}
	// Stamp bottom-up so the ancestor base binds the new membership and paths.
	var bodies []Node
	done := map[string]bool{}
	var stamp func(string) error
	stamp = func(id string) error {
		if done[id] || !modified[id] {
			return nil
		}
		v := nodes[id]
		v.ChildFingerprints = nil
		children := []string{}
		for cid, c := range nodes {
			if c.ParentID == id {
				children = append(children, cid)
			}
		}
		sort.Slice(children, func(i, j int) bool {
			a, b := nodes[children[i]], nodes[children[j]]
			if a.Path == b.Path {
				return a.ID < b.ID
			}
			return a.Path < b.Path
		})
		for _, cid := range children {
			if err := stamp(cid); err != nil {
				return err
			}
			v.ChildFingerprints = append(v.ChildFingerprints, nodes[cid].Fingerprint)
		}
		v, err = Stamp(v)
		if err != nil {
			return err
		}
		nodes[id] = v
		bodies = append(bodies, v)
		done[id] = true
		return nil
	}
	ids := []string{}
	dependency := []string{}
	for id := range modified {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for id := range deps {
		dependency = append(dependency, id)
	}
	sort.Strings(dependency)
	for _, id := range ids {
		if err = stamp(id); err != nil {
			return rec, err
		}
	}
	rec = newControlledRecord(idx, versionID, node, summary, by, base, "move", ids, dependency)
	return rec, persistControlledChange(st, dir, versionID, idx, cache, rec, bodies, bodies)
}
