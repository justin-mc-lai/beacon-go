package intenttree

import (
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// InitTree creates the root intent node for a project version. It is the
// greenfield entry; migrated projects use Import instead.
func InitTree(st *store.Store, projectRoot, docsRoot, version, versionID, title, intent string) (Node, string, error) {
	dir, resolveErr := ResolveTreeLocation(projectRoot, docsRoot)
	if resolveErr != nil {
		return Node{}, "", resolveErr
	}
	if HasIndex(dir) {
		return Node{}, "", core.NewError(ReasonStateConflict, "intenttree.InitTree", "tree already exists")
	}
	root := Node{
		ID: shortID("nroot_", versionID), Kind: KindIntent, Title: title,
		Intent: intent, Path: "root", Status: StatusDraft,
	}
	var err error
	root, err = Stamp(root)
	if err != nil {
		return Node{}, "", err
	}
	if err := WriteNode(dir, root); err != nil {
		return Node{}, "", err
	}
	idx := Index{Schema: Schema, Version: version, RootID: root.ID}
	idx.Nodes = append(idx.Nodes, IndexEntry{
		ID: root.ID, Kind: root.Kind, Status: root.Status, Title: root.Title,
		Path: root.Path, Fingerprint: root.Fingerprint, FingerprintAlg: root.FingerprintAlg,
	})
	if err := WriteIndex(dir, idx); err != nil {
		return Node{}, "", err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	mig := store.TreeMigration{
		ID: shortID("tmig_", versionID), ProjectVersionID: versionID,
		Source: "plan_init", NodeCount: 1, CreatedAt: now,
	}
	if err := st.SaveTree(versionID, []store.TreeNode{{
		ID: root.ID, ProjectVersionID: versionID, Kind: string(root.Kind),
		Path: root.Path, Status: string(root.Status), Title: root.Title,
		Fingerprint: root.Fingerprint, FingerprintAlg: root.FingerprintAlg,
		DocPath: NodePath(dir, root.ID), CreatedAt: now, UpdatedAt: now,
	}}, mig); err != nil {
		return Node{}, "", err
	}
	return root, dir, nil
}
