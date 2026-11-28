package intenttree

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// DeprecateDraft is the single draft->deprecated writer. Nonempty parents
// require explicit child disposition first; deprecation never silently cascades.
func DeprecateDraft(st *store.Store, dir, versionID, id, summary, by, base string) (ChangeRecord, error) {
	bad := func(detail string) (ChangeRecord, error) {
		return ChangeRecord{}, core.NewError("tree_deprecate_invalid", "intenttree.DeprecateDraft", detail)
	}
	if strings.TrimSpace(summary) == "" || strings.TrimSpace(by) == "" || strings.TrimSpace(base) == "" {
		return bad("summary, by and base fingerprint required")
	}
	unlock, err := lockTreeChange(dir)
	if err != nil {
		return ChangeRecord{}, err
	}
	defer unlock()
	if err := recoverTreeChange(dir, st); err != nil {
		return ChangeRecord{}, err
	}
	idx, n, err := readMutationNode(dir, id)
	if err != nil {
		return ChangeRecord{}, err
	}
	if findings := ValidateIndex(idx); len(findings) > 0 {
		return bad(findings[0])
	}
	if base != n.Fingerprint {
		return ChangeRecord{}, core.NewError(ReasonChangeBaseMismatch, "intenttree.DeprecateDraft", "stale base fingerprint")
	}
	if n.Status != StatusDraft {
		return bad("only draft nodes may be deprecated")
	}
	if id == idx.RootID {
		return bad("root cannot be deprecated")
	}
	for _, child := range idx.Nodes {
		if child.ParentID == id {
			return bad("nonempty parent requires explicit child disposition")
		}
	}
	// Never alter a frozen ancestor through a draft-only operation.
	seen := map[string]bool{id: true}
	for parent := n.ParentID; parent != ""; {
		if seen[parent] {
			return bad("ancestor cycle")
		}
		seen[parent] = true
		p, ok := findEntry(idx, parent)
		if !ok {
			return bad("missing ancestor")
		}
		if _, _, err := readMutationNode(dir, parent); err != nil {
			return ChangeRecord{}, err
		}
		if p.Status != StatusDraft && p.Status != StatusChangePending {
			return bad("ancestor must be draft/change_pending; request a governed change first")
		}
		parent = p.ParentID
	}
	cached, err := st.Tree().GetTreeNode(id)
	if err != nil {
		return ChangeRecord{}, err
	}
	if cached.ProjectVersionID != versionID {
		return bad("version scope mismatch")
	}
	now := time.Now().UTC()
	rec := ChangeRecord{ID: shortID("chg_", versionID, id, "deprecate", now.Format(time.RFC3339Nano)), NodeID: id, Summary: strings.TrimSpace(summary), By: strings.TrimSpace(by), BaseFingerprint: base, Affected: []string{id}, CreatedAt: now.Format(time.RFC3339Nano)}
	path := filepath.Join(dir, "changes", rec.ID+".json")
	n.Status = StatusDeprecated
	n, err = Stamp(n)
	if err != nil {
		return ChangeRecord{}, err
	}
	raw, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return ChangeRecord{}, err
	}
	if err := persistMutation(st, dir, idx, n, map[string][]byte{path: append(raw, '\n')}, ""); err != nil {
		return ChangeRecord{}, err
	}
	return rec, nil
}
