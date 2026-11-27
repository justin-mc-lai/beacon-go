package intenttree

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

func loadMutationNode(st *store.Store, dir, nodeID string) (Index, Node, error) {
	unlock, err := lockTreeChange(dir)
	if err != nil {
		return Index{}, Node{}, err
	}
	defer unlock()
	if err := recoverTreeChange(dir, st); err != nil {
		return Index{}, Node{}, err
	}
	return readMutationNode(dir, nodeID)
}

func readMutationNode(dir, nodeID string) (Index, Node, error) {
	idx, err := ReadIndex(dir)
	if err != nil {
		return Index{}, Node{}, err
	}
	entry, found := findEntry(idx, nodeID)
	if !found {
		return Index{}, Node{}, core.NewError(ReasonNodeNotFound, "intenttree.readMutationNode", "node not found: "+nodeID)
	}
	node, err := ReadNode(dir, nodeID)
	if err != nil {
		return Index{}, Node{}, err
	}
	valid, err := VerifyFingerprint(node)
	if err != nil {
		return Index{}, Node{}, err
	}
	if !valid || node.ID != entry.ID || node.ParentID != entry.ParentID || node.Kind != entry.Kind ||
		node.Path != entry.Path || node.Title != entry.Title || node.Status != entry.Status ||
		node.Fingerprint != entry.Fingerprint || node.FingerprintAlg != entry.FingerprintAlg {
		return Index{}, Node{}, core.NewError(ReasonStateConflict, "intenttree.readMutationNode", "canonical node and index differ: "+nodeID)
	}
	return idx, node, nil
}

func persistMutation(st *store.Store, dir string, idx Index, node Node, files map[string][]byte, snapshotID string) (resultErr error) {
	cached, err := st.Tree().GetTreeNode(node.ID)
	if err != nil {
		return err
	}
	paths := []string{NodePath(dir, node.ID), IndexPath(dir)}
	for path := range files {
		paths = append(paths, path)
	}
	var removeDirs []string
	if snapshotID != "" {
		removeDirs = []string{filepath.Join(snapshotsDir(dir), snapshotID)}
		if _, err := os.Lstat(removeDirs[0]); !os.IsNotExist(err) {
			if err != nil {
				return err
			}
			return core.NewError(ReasonStateConflict, "intenttree.persistMutation", "snapshot already exists")
		}
	}
	if err := writeMutationJournal(dir, paths, []store.TreeNode{cached}, removeDirs); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, recoverTreeChange(dir, st))
		}
	}()
	if err := WriteNode(dir, node); err != nil {
		return err
	}
	for index := range idx.Nodes {
		if idx.Nodes[index].ID == node.ID {
			idx.Nodes[index].Status = node.Status
			idx.Nodes[index].Title = node.Title
			idx.Nodes[index].Constraint = node.Constraint
			idx.Nodes[index].TruthPath = node.EffectiveTruthPath()
			idx.Nodes[index].Fingerprint = node.Fingerprint
			idx.Nodes[index].FingerprintAlg = node.FingerprintAlg
		}
	}
	if err := WriteIndex(dir, idx); err != nil {
		return err
	}
	for path, raw := range files {
		if err := atomicWrite(path, raw); err != nil {
			return err
		}
	}
	if snapshotID != "" {
		if _, _, err := createSnapshot(dir, idx.Version, snapshotID); err != nil {
			return err
		}
	}
	if _, _, err := readMutationNode(dir, node.ID); err != nil {
		return err
	}
	projection := cached
	projection.ParentID, projection.Kind, projection.Path = node.ParentID, string(node.Kind), node.Path
	projection.Status, projection.Title = string(node.Status), node.Title
	projection.Fingerprint, projection.FingerprintAlg = node.Fingerprint, node.FingerprintAlg
	projection.DocPath = NodePath(dir, node.ID)
	projection.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := st.UpdateTreeNodeProjection(projection); err != nil {
		return err
	}
	if err := os.Remove(changeJournalPath(dir)); err != nil {
		return err
	}
	committed = true
	return nil
}
