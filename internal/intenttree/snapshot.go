package intenttree

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"beacon-go/internal/core"
)

// SnapshotSchema is the immutable history format.
const SnapshotSchema = "beacon-intent-tree-snapshot-v1"

// SnapshotEntry is one index row plus the body location.
type SnapshotEntry struct {
	ID             string `json:"id"`
	ParentID       string `json:"parent_id,omitempty"`
	Kind           Kind   `json:"kind"`
	Status         Status `json:"status"`
	Title          string `json:"title"`
	Path           string `json:"path"`
	Fingerprint    string `json:"fingerprint"`
	FingerprintAlg string `json:"fingerprint_alg"`
	NodeRef        string `json:"node_ref"`
}

// Snapshot is the immutable version-line tree.
type Snapshot struct {
	Schema     string          `json:"schema"`
	Version    string          `json:"version"`
	RootID     string          `json:"root_id"`
	CreatedAt  string          `json:"created_at"`
	SourceHash string          `json:"source_hash"`
	Nodes      []SnapshotEntry `json:"nodes"`
}

func snapshotsDir(dir string) string { return filepath.Join(dir, "snapshots") }

// CreateSnapshot writes one immutable snapshot. Unchanged nodes point at
// the previous snapshot's body instead of storing a second copy.
func CreateSnapshot(dir, version string) (Snapshot, string, error) {
	idx, err := ReadIndex(dir)
	if err != nil {
		return Snapshot{}, "", err
	}
	if findings := ValidateIndex(idx); len(findings) > 0 {
		return Snapshot{}, "", core.NewError(ReasonInvalidIndex, "intenttree.CreateSnapshot", strings.Join(findings, "; "))
	}
	root, ok := findEntry(idx, idx.RootID)
	if !ok || root.Status != StatusFrozen {
		return Snapshot{}, "", core.NewError(ReasonParentNotFrozen, "intenttree.CreateSnapshot", "root must be frozen before a snapshot")
	}
	prevID, prev, _ := latestSnapshot(dir)
	prevBodies := map[string]SnapshotEntry{}
	if prev != nil {
		for _, e := range prev.Nodes {
			prevBodies[e.ID] = e
		}
	}
	stamp := time.Now().UTC()
	snapID := fmt.Sprintf("%s-%s", version, stamp.Format("20060102T150405.000000000Z"))
	snapDir := filepath.Join(snapshotsDir(dir), snapID)
	if err := os.MkdirAll(filepath.Join(snapDir, "nodes"), 0o755); err != nil {
		return Snapshot{}, "", err
	}
	snap := Snapshot{
		Schema: SnapshotSchema, Version: version, RootID: idx.RootID,
		CreatedAt: stamp.Format(time.RFC3339Nano), SourceHash: SourceHash(idx),
	}
	for _, e := range idx.Nodes {
		ref := filepath.ToSlash(filepath.Join("nodes", e.ID+".json"))
		if p, ok := prevBodies[e.ID]; ok && p.Fingerprint == e.Fingerprint {
			actual := filepath.Join(filepath.Join(snapshotsDir(dir), prevID), p.NodeRef)
			if _, err := os.Stat(actual); err == nil {
				rel, rerr := filepath.Rel(snapDir, actual)
				if rerr == nil {
					ref = filepath.ToSlash(rel)
				}
			}
		}
		if ref == filepath.ToSlash(filepath.Join("nodes", e.ID+".json")) {
			body, err := os.ReadFile(NodePath(dir, e.ID))
			if err != nil {
				return Snapshot{}, "", err
			}
			if err := atomicWrite(filepath.Join(snapDir, "nodes", e.ID+".json"), body); err != nil {
				return Snapshot{}, "", err
			}
		}
		snap.Nodes = append(snap.Nodes, SnapshotEntry{
			ID: e.ID, ParentID: e.ParentID, Kind: e.Kind, Status: e.Status,
			Title: e.Title, Path: e.Path, Fingerprint: e.Fingerprint,
			FingerprintAlg: e.FingerprintAlg, NodeRef: ref,
		})
	}
	raw, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return Snapshot{}, "", err
	}
	if err := atomicWrite(filepath.Join(snapDir, "snapshot.json"), append(raw, '\n')); err != nil {
		return Snapshot{}, "", err
	}
	return snap, snapID, nil
}

// ListSnapshots returns snapshot ids newest first.
func ListSnapshots(dir string) ([]string, error) {
	entries, err := os.ReadDir(snapshotsDir(dir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	return ids, nil
}

// ReadSnapshot loads a snapshot and verifies every referenced body.
func ReadSnapshot(dir, id string) (Snapshot, error) {
	if id == "latest" {
		ids, err := ListSnapshots(dir)
		if err != nil {
			return Snapshot{}, err
		}
		if len(ids) == 0 {
			return Snapshot{}, core.NewError(ReasonNodeNotFound, "intenttree.ReadSnapshot", "no snapshots")
		}
		id = ids[0]
	}
	snapDir := filepath.Join(snapshotsDir(dir), id)
	raw, err := os.ReadFile(filepath.Join(snapDir, "snapshot.json"))
	if err != nil {
		return Snapshot{}, err
	}
	var snap Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return Snapshot{}, err
	}
	if snap.Schema != SnapshotSchema {
		return Snapshot{}, core.NewError(ReasonSchemaUnknown, "intenttree.ReadSnapshot", "unknown snapshot schema "+snap.Schema)
	}
	for _, e := range snap.Nodes {
		body, err := os.ReadFile(filepath.Join(snapDir, e.NodeRef))
		if err != nil {
			return Snapshot{}, core.NewError(ReasonInvalidIndex, "intenttree.ReadSnapshot", "missing body for "+e.ID)
		}
		var n Node
		if err := json.Unmarshal(body, &n); err != nil {
			return Snapshot{}, err
		}
		ok, err := VerifyFingerprint(n)
		if err != nil {
			return Snapshot{}, err
		}
		if !ok || n.Fingerprint != e.Fingerprint {
			return Snapshot{}, core.NewError(ReasonInvalidIndex, "intenttree.ReadSnapshot", "fingerprint drift in snapshot for "+e.ID)
		}
	}
	return snap, nil
}

func latestSnapshot(dir string) (string, *Snapshot, error) {
	ids, err := ListSnapshots(dir)
	if err != nil || len(ids) == 0 {
		return "", nil, err
	}
	snap, err := ReadSnapshot(dir, ids[0])
	if err != nil {
		return "", nil, err
	}
	return ids[0], &snap, nil
}
