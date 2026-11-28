package cli

import (
	"beacon-go/internal/intenttree"
	"os"
	"path/filepath"
	"testing"
)

func TestChangeGrowDeprecateRefreeze(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	for _, id := range []string{"t1", "t2", "f", "m", "root"} {
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree")
	p := filepath.Join(proj, "grow.json")
	if err := os.WriteFile(p, []byte(`{"children":[{"kind":"touchpoint","title":"retired","acs":[{"id":"a","text":"a"}],"verification":{"spec":"test"}}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	parent, _ := intenttree.ReadNode(dir, "f")
	rec := parseDoc(t, mustRun(t, "change", "grow", "--project", proj, "--version", "v0.1.0", "--node", "f", "--proposal", p, "--summary", "append", "--by", "maker", "--base-fingerprint", parent.Fingerprint))
	id := rec["added"].([]any)[0].(string)
	leaf, _ := intenttree.ReadNode(dir, id)
	mustRun(t, "change", "deprecate", "--project", proj, "--version", "v0.1.0", "--node", id, "--summary", "retire", "--by", "maker", "--base-fingerprint", leaf.Fingerprint)
	tomb, _ := intenttree.ReadNode(dir, id)
	for _, id := range []string{"f", "m", "root"} {
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatal(err)
		}
	}
	parent, _ = intenttree.ReadNode(dir, "f")
	found := false
	for _, fp := range parent.ChildFingerprints {
		if fp == tomb.Fingerprint {
			found = true
		}
	}
	if !found {
		t.Fatal("tombstone not bound")
	}
	ids, _ := intenttree.ListSnapshots(dir)
	snap, _ := intenttree.ReadSnapshot(dir, ids[0])
	mustRun(t, "projects", "register", "--project", proj, "--version", "v0.2.0", "--json")
	mustRun(t, "change", "continue", "--apply", "--project", proj, "--version", "v0.1.0", "--to-version", "v0.2.0", "--snapshot", ids[0], "--base-source-hash", snap.SourceHash, "--summary", "delivery", "--by", "maker")
	mustRun(t, "truth", "node", "freeze", "--project", proj, "--version", "v0.2.0", "--node", "root", "--json")
	current, _ := intenttree.ReadNode(dir, id)
	if current.Status != intenttree.StatusDeprecated || current.Fingerprint != tomb.Fingerprint {
		t.Fatal("tombstone changed")
	}
}
