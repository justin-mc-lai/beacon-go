package intenttree

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// LegacyFeature is one flat feature package to import as a feature node.
type LegacyFeature struct {
	Slug      string
	TruthPath string
	TestsPath string
}

// ImportInput binds one registered project version to its flat packages.
type ImportInput struct {
	ProjectRoot string
	DocsRoot    string
	Version     string
	VersionID   string
	ProjectName string
	Features    []LegacyFeature
}

// ImportReport is the CLI/JSON outcome of `migrate tree`.
type ImportReport struct {
	Version      string `json:"version"`
	VersionID    string `json:"version_id"`
	RootID       string `json:"root_id"`
	NodeCount    int    `json:"node_count"`
	Already      bool   `json:"already"`
	TreeDir      string `json:"tree_dir"`
	IndexPath    string `json:"index_path"`
	SnapshotPath string `json:"snapshot_path,omitempty"`
}

func shortID(prefix string, parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return prefix + hex.EncodeToString(sum[:])[:12]
}

// Import performs the one-time, snapshot-first import of flat feature
// packages into the canonical tree. It is idempotent: a second call
// returns the existing tree without writing anything.
func Import(st *store.Store, in ImportInput) (ImportReport, error) {
	dir := TreeDir(in.ProjectRoot, in.DocsRoot)
	rep := ImportReport{Version: in.Version, VersionID: in.VersionID, TreeDir: dir, IndexPath: IndexPath(dir)}

	if _, err := st.Tree().GetTreeMigration(in.VersionID); err == nil {
		idx, rerr := ReadIndex(dir)
		if rerr != nil {
			return rep, rerr
		}
		rep.Already = true
		rep.RootID = idx.RootID
		rep.NodeCount = len(idx.Nodes)
		return rep, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return rep, err
	}
	if HasIndex(dir) {
		idx, err := ReadIndex(dir)
		if err != nil {
			return rep, err
		}
		if findings := ValidateIndex(idx); len(findings) > 0 {
			return rep, core.NewError(ReasonInvalidIndex, "intenttree.Import", strings.Join(findings, "; "))
		}
		return rep.rebuildCache(st, in, dir, idx)
	}

	now := time.Now().UTC()
	snapshot := filepath.Join(in.ProjectRoot, ".beacon", "backups",
		"pre-tree-"+now.Format("20060102T150405.000000000Z")+".db")
	if err := st.BackupTo(snapshot); err != nil {
		return rep, err
	}
	rep.SnapshotPath = snapshot

	rootID := shortID("nroot_", in.Version)
	legacyID := shortID("nmod_", in.Version, "legacy")
	root := Node{
		ID: rootID, Kind: KindIntent, Title: in.ProjectName,
		Intent: "Legacy sea/lake packages imported under one intent root.",
		Path:   "root", Status: StatusDraft,
	}
	legacy := Node{
		ID: legacyID, Kind: KindModule, ParentID: rootID, Title: "Legacy packages",
		Intent: "One module holding the imported feature packages.",
		Path:   "legacy", Status: StatusDraft,
	}
	nodes := []Node{root, legacy}
	for _, f := range in.Features {
		n := Node{
			ID: shortID("nfeat_", in.Version, f.Slug), Kind: KindFeature,
			ParentID: legacyID, Title: f.Slug, Path: "legacy/" + f.Slug,
			Status: StatusDraft, LegacyTruthPath: f.TruthPath, LegacyTestsPath: f.TestsPath,
		}
		nodes = append(nodes, n)
	}

	idx := Index{Schema: Schema, Version: in.Version, RootID: rootID}
	cache := make([]store.TreeNode, 0, len(nodes))
	stamped := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		n, err := Stamp(n)
		if err != nil {
			return rep, err
		}
		if err := WriteNode(dir, n); err != nil {
			return rep, err
		}
		idx.Nodes = append(idx.Nodes, IndexEntry{
			ID: n.ID, ParentID: n.ParentID, Kind: n.Kind, Status: n.Status,
			Title: n.Title, Path: n.Path, Fingerprint: n.Fingerprint, FingerprintAlg: n.FingerprintAlg,
		})
		cache = append(cache, store.TreeNode{
			ID: n.ID, ProjectVersionID: in.VersionID, ParentID: n.ParentID,
			Kind: string(n.Kind), Path: n.Path, Status: string(n.Status), Title: n.Title,
			Fingerprint: n.Fingerprint, FingerprintAlg: n.FingerprintAlg,
			DocPath: NodePath(dir, n.ID), CreatedAt: now.Format(time.RFC3339Nano), UpdatedAt: now.Format(time.RFC3339Nano),
		})
		stamped = append(stamped, n)
	}
	if findings := ValidateIndex(idx); len(findings) > 0 {
		return rep, core.NewError(ReasonInvalidIndex, "intenttree.Import", strings.Join(findings, "; "))
	}
	if err := WriteIndex(dir, idx); err != nil {
		return rep, err
	}
	mig := store.TreeMigration{
		ID: shortID("tmig_", in.Version), ProjectVersionID: in.VersionID,
		Source: "legacy_feature_packages", SnapshotPath: snapshot,
		NodeCount: len(stamped), CreatedAt: now.Format(time.RFC3339Nano),
	}
	if err := st.SaveTree(in.VersionID, cache, mig); err != nil {
		return rep, err
	}
	rep.RootID = rootID
	rep.NodeCount = len(stamped)
	_ = cache
	return rep, nil
}

// rebuildCache is the "delete the new tables and rerun" path: the repo
// JSON is authoritative, so a missing migration record can be restored
// from it after a snapshot-first backup.
func (rep ImportReport) rebuildCache(st *store.Store, in ImportInput, dir string, idx Index) (ImportReport, error) {
	now := time.Now().UTC()
	snapshot := filepath.Join(in.ProjectRoot, ".beacon", "backups",
		"pre-rebuild-"+now.Format("20060102T150405.000000000Z")+".db")
	if err := st.BackupTo(snapshot); err != nil {
		return rep, err
	}
	cache, err := cacheFromIndex(dir, idx, in.VersionID, now)
	if err != nil {
		return rep, err
	}
	mig := store.TreeMigration{
		ID: shortID("tmig_", in.Version), ProjectVersionID: in.VersionID,
		Source: "rebuilt_from_repo_json", SnapshotPath: snapshot,
		NodeCount: len(cache), CreatedAt: now.Format(time.RFC3339Nano),
	}
	if err := st.SaveTree(in.VersionID, cache, mig); err != nil {
		return rep, err
	}
	rep.RootID = idx.RootID
	rep.NodeCount = len(cache)
	rep.SnapshotPath = snapshot
	return rep, nil
}

func cacheFromIndex(dir string, idx Index, versionID string, now time.Time) ([]store.TreeNode, error) {
	var cache []store.TreeNode
	for _, e := range idx.Nodes {
		n, err := ReadNode(dir, e.ID)
		if err != nil {
			return nil, err
		}
		cache = append(cache, store.TreeNode{
			ID: n.ID, ProjectVersionID: versionID, ParentID: n.ParentID,
			Kind: string(n.Kind), Path: n.Path, Status: string(n.Status), Title: n.Title,
			Fingerprint: n.Fingerprint, FingerprintAlg: n.FingerprintAlg,
			DocPath: NodePath(dir, n.ID), CreatedAt: now.Format(time.RFC3339Nano),
			UpdatedAt: now.Format(time.RFC3339Nano),
		})
	}
	return cache, nil
}
