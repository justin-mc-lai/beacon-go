package intenttree

import (
	"beacon-go/internal/core"
	"beacon-go/internal/store"
	"beacon-go/internal/versionmanifest"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type BaselineMapping struct {
	PredecessorID string `json:"predecessor_id"`
	// SuccessorID is a qualified baseline reference, not a materialized node ID.
	SuccessorID string `json:"successor_id"`
	Fingerprint string `json:"fingerprint"`
}

type LineagePreview struct {
	Preview              bool                      `json:"preview"`
	FromVersion          string                    `json:"from_version"`
	ToVersion            string                    `json:"to_version"`
	BaselineSnapshotID   string                    `json:"baseline_snapshot_id"`
	BaselineSourceHash   string                    `json:"baseline_source_hash"`
	SourceCommit         string                    `json:"source_commit"`
	BinarySHA            string                    `json:"binary_sha256"`
	IDPolicy             string                    `json:"id_policy"`
	Mapping              []BaselineMapping         `json:"mapping"`
	Schema               string                    `json:"schema,omitempty"`
	ProjectID            string                    `json:"project_id,omitempty"`
	TargetVersionID      string                    `json:"target_version_id,omitempty"`
	PreviousDelivery     string                    `json:"previous_delivery,omitempty"`
	PreviousManifestHash string                    `json:"previous_manifest_hash,omitempty"`
	By                   string                    `json:"by,omitempty"`
	Summary              string                    `json:"summary,omitempty"`
	CreatedAt            string                    `json:"created_at,omitempty"`
	Delta                versionmanifest.TreeDelta `json:"delta"`
}

// ApplyContinuation commits a delivery baseline reference under the one logical
// tree. Target registration is explicit; no version or successor tree is created.
func ApplyContinuation(st *store.Store, root, dir, from, to, snapshot, base, previous, by, summary string) (LineagePreview, error) {
	bad := func(s string) (LineagePreview, error) {
		return LineagePreview{}, core.NewError("tree_lineage_invalid", "intenttree.ApplyContinuation", s)
	}
	if strings.TrimSpace(by) == "" || strings.TrimSpace(summary) == "" || strings.TrimSpace(base) == "" {
		return bad("by, summary and base source hash required")
	}
	unlock, err := lockTreeChange(dir)
	if err != nil {
		return LineagePreview{}, err
	}
	defer unlock()
	if err = recoverTreeChange(dir, st); err != nil {
		return LineagePreview{}, err
	}
	proj, err := st.Projects().GetByRootPath(store.ResolveRootPath(root))
	if err != nil {
		return LineagePreview{}, err
	}
	versions, err := st.Projects().ListVersions(proj.ID)
	if err != nil {
		return LineagePreview{}, err
	}
	targetID := ""
	sourceOK := false
	for _, v := range versions {
		if v.Version == to {
			targetID = v.ID
			if HasIndex(filepath.Join(root, v.DocsRoot, "tree")) {
				return bad("target already has a separate canonical tree")
			}
		}
		if v.Version == from {
			sourceOK = store.ResolveRootPath(TreeDir(root, v.DocsRoot)) == store.ResolveRootPath(dir)
		}
	}
	if targetID == "" || !sourceOK {
		return bad("source and target must be explicitly registered in the same project")
	}
	out, err := PreviewContinuation(dir, from, to, snapshot)
	if err != nil {
		return out, err
	}
	idx, err := ReadIndex(dir)
	if err != nil {
		return out, err
	}
	if idx.Version != from || base != out.BaselineSourceHash || SourceHash(idx) != base {
		return bad("stale baseline: snapshot and current tree must match base source hash")
	}
	for _, e := range idx.Nodes {
		if _, _, err := readMutationNode(dir, e.ID); err != nil {
			return out, err
		}
	}
	path := filepath.Join(dir, "lineage", to+".json")
	if _, err := os.Lstat(path); err == nil {
		return bad("delivery lineage already exists")
	} else if !os.IsNotExist(err) {
		return out, err
	}
	var old Snapshot
	if previous != "" {
		if !regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`).MatchString(previous) {
			return bad("invalid previous delivery version")
		}
		raw, err := os.ReadFile(filepath.Join(dir, "lineage", previous+".json"))
		if err != nil {
			return out, err
		}
		var prior LineagePreview
		if err = json.Unmarshal(raw, &prior); err != nil {
			return out, err
		}
		if prior.Preview || prior.Schema != "beacon-tree-delivery-lineage.v1" || prior.ProjectID != proj.ID || prior.FromVersion != from || prior.ToVersion != previous {
			return bad("previous delivery scope mismatch")
		}
		if !lineageVersionNewer(previous, to) {
			return bad("previous delivery must precede target")
		}
		old, err = ReadSnapshot(dir, prior.BaselineSnapshotID)
		if err != nil {
			return out, err
		}
		if old.SourceHash != prior.BaselineSourceHash {
			return bad("previous baseline hash mismatch")
		}
		if _, err := PreviewContinuation(dir, from, to, prior.BaselineSnapshotID); err != nil {
			return out, err
		}
		hash := sha256.Sum256(raw)
		out.PreviousManifestHash = hex.EncodeToString(hash[:])
	}
	oldNodes := map[string]SnapshotEntry{}
	for _, e := range old.Nodes {
		oldNodes[e.ID] = e
	}
	out.Delta = versionmanifest.TreeDelta{Added: []string{}, Modified: []string{}, Deprecated: []string{}, DependencyScope: []string{}}
	deps := map[string]bool{}
	snap, err := ReadSnapshot(dir, snapshot)
	if err != nil {
		return out, err
	}
	for _, e := range snap.Nodes {
		prior, exists := oldNodes[e.ID]
		delete(oldNodes, e.ID)
		changed := false
		if previous == "" {
			out.Delta.Added = append(out.Delta.Added, e.ID)
			changed = true
		} else {
			if !exists {
				out.Delta.Added = append(out.Delta.Added, e.ID)
				changed = true
			} else if e.Fingerprint != prior.Fingerprint {
				out.Delta.Modified = append(out.Delta.Modified, e.ID)
				changed = true
			}
			if e.Status == StatusDeprecated && prior.Status != StatusDeprecated {
				out.Delta.Deprecated = append(out.Delta.Deprecated, e.ID)
			}
		}
		if changed {
			for parent := e.ParentID; parent != ""; {
				if deps[parent] {
					break
				}
				deps[parent] = true
				p, ok := findEntry(idx, parent)
				if !ok {
					return bad("missing dependency ancestor")
				}
				parent = p.ParentID
			}
		}
	}
	if len(oldNodes) > 0 {
		return bad("baseline nodes removed without explicit disposition")
	}
	for id := range deps {
		out.Delta.DependencyScope = append(out.Delta.DependencyScope, id)
	}
	sort.Strings(out.Delta.Added)
	sort.Strings(out.Delta.Modified)
	sort.Strings(out.Delta.Deprecated)
	sort.Strings(out.Delta.DependencyScope)
	// Once a delivery exists, a subsequent baseline must name its predecessor;
	// branches and multiple consumers of the same predecessor are refused.
	entries, err := os.ReadDir(filepath.Join(dir, "lineage"))
	if err != nil && !os.IsNotExist(err) {
		return out, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, "lineage", entry.Name()))
		if err != nil {
			return out, err
		}
		var other LineagePreview
		if err = json.Unmarshal(raw, &other); err != nil {
			return out, err
		}
		if previous == "" || other.PreviousDelivery == previous {
			return bad("explicit unconsumed previous delivery required")
		}
	}
	out.Preview = false
	out.Schema = "beacon-tree-delivery-lineage.v1"
	out.ProjectID = proj.ID
	out.TargetVersionID = targetID
	out.PreviousDelivery = previous
	out.By = strings.TrimSpace(by)
	out.Summary = strings.TrimSpace(summary)
	out.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	raw, err := prettyJSON(out)
	if err != nil {
		return out, err
	}
	// Single-file atomic publication under the tree lock; existing records are
	// never overwritten. No node/index/cache state changes in this operation.
	if err = atomicWrite(path, raw); err != nil {
		return out, err
	}
	return out, nil
}

// PreviewContinuation validates an explicit immutable baseline, returning only
// references. It neither creates another authoritative tree nor changes current
// project version. Materialized cross-version IDs remain version-namespaced.
func PreviewContinuation(dir, from, to, snapshot string) (LineagePreview, error) {
	bad := func(s string) (LineagePreview, error) {
		return LineagePreview{}, core.NewError("tree_lineage_invalid", "intenttree.PreviewContinuation", s)
	}
	versionRE := regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	if !versionRE.MatchString(from) || !versionRE.MatchString(to) {
		return bad("explicit x.y.z versions required")
	}
	if !lineageVersionNewer(from, to) {
		return bad("successor version must be newer")
	}
	if strings.TrimSpace(snapshot) == "" || snapshot == "latest" || filepath.Base(snapshot) != snapshot || strings.ContainsAny(snapshot, `/\`) {
		return bad("explicit snapshot ID required")
	}
	snap, bodies, err := ReadSnapshotWithBodies(dir, snapshot)
	if err != nil {
		return LineagePreview{}, err
	}
	if snap.Version != from || snap.ID != snapshot || len(snap.Nodes) == 0 {
		return bad("baseline scope mismatch or empty baseline")
	}
	idx := Index{Schema: Schema, Version: from, RootID: snap.RootID}
	out := LineagePreview{Preview: true, FromVersion: from, ToVersion: to, BaselineSnapshotID: snapshot, BaselineSourceHash: snap.SourceHash, SourceCommit: snap.RuntimeCommit, BinarySHA: snap.BinarySHA, IDPolicy: "qualified_baseline_reference; not materialized successor node IDs"}
	seen := map[string]bool{}
	for _, e := range snap.Nodes {
		n, ok := bodies[e.ID]
		if !ok || seen[e.ID] || n.ID != e.ID || n.ParentID != e.ParentID || n.Kind != e.Kind || n.Path != e.Path || n.Status != e.Status || n.Title != e.Title || n.Constraint != e.Constraint || n.EffectiveTruthPath() != e.TruthPath {
			return bad("duplicate or inconsistent baseline node")
		}
		seen[e.ID] = true
		switch e.Status {
		case StatusFrozen, StatusImplemented, StatusVerified, StatusDelivered, StatusDeprecated:
		default:
			return bad("baseline contains non-frozen node")
		}
		idx.Nodes = append(idx.Nodes, IndexEntry{ID: e.ID, ParentID: e.ParentID, Kind: e.Kind, Status: e.Status, Title: e.Title, Path: e.Path, TruthPath: e.TruthPath, Constraint: e.Constraint, Fingerprint: e.Fingerprint, FingerprintAlg: e.FingerprintAlg})
		out.Mapping = append(out.Mapping, BaselineMapping{PredecessorID: e.ID, SuccessorID: to + "@" + snapshot + "/" + e.ID, Fingerprint: e.Fingerprint})
	}
	if findings := ValidateIndex(idx); len(findings) > 0 {
		return bad(strings.Join(findings, "; "))
	}
	if SourceHash(idx) != snap.SourceHash {
		return bad("baseline source hash mismatch")
	}
	return out, nil
}

func lineageVersionNewer(from, to string) bool {
	a := strings.Split(strings.TrimPrefix(from, "v"), ".")
	b := strings.Split(strings.TrimPrefix(to, "v"), ".")
	if len(a) != 3 || len(b) != 3 {
		return false
	}
	for i := range a {
		x, e := strconv.ParseUint(a[i], 10, 64)
		y, f := strconv.ParseUint(b[i], 10, 64)
		if e != nil || f != nil {
			return false
		}
		if x != y {
			return y > x
		}
	}
	return false
}
