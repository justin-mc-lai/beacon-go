package intenttree

import (
	"strings"
	"testing"
)

func writeDriftTree(t *testing.T, dir string) string {
	t.Helper()
	root := Node{ID: "root", Kind: KindIntent, Title: "root", Intent: "root intent", Path: "root", Status: StatusFrozen}
	child := Node{ID: "child", ParentID: "root", Kind: KindModule, Title: "child", Intent: "child intent", Path: "root/child", Status: StatusFrozen}
	stampedRoot, err := Stamp(root)
	if err != nil {
		t.Fatal(err)
	}
	stampedChild, err := Stamp(child)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []Node{stampedRoot, stampedChild} {
		if err := WriteNode(dir, n); err != nil {
			t.Fatal(err)
		}
	}
	idx := Index{Schema: Schema, Version: "v0.1.0", RootID: "root", Nodes: []IndexEntry{
		{ID: "root", Kind: KindIntent, Status: StatusFrozen, Title: "root", Path: "root", Fingerprint: stampedRoot.Fingerprint, FingerprintAlg: FingerprintAlg},
		{ID: "child", ParentID: "root", Kind: KindModule, Status: StatusFrozen, Title: "child", Path: "root/child", Fingerprint: stampedChild.Fingerprint, FingerprintAlg: FingerprintAlg},
	}}
	if err := WriteIndex(dir, idx); err != nil {
		t.Fatal(err)
	}
	_, id, err := CreateSnapshot(dir, idx.Version)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestDrift(t *testing.T) {
	dir := t.TempDir()
	snapID := writeDriftTree(t, dir)

	rep, err := Drift(dir, "root", snapID)
	if err != nil {
		t.Fatal(err)
	}
	if rep.SnapshotID != snapID || len(rep.Added) != 0 || len(rep.Changed) != 0 || len(rep.Removed) != 0 {
		t.Fatalf("clean drift = %+v", rep)
	}

	n, err := ReadNode(dir, "child")
	if err != nil {
		t.Fatal(err)
	}
	n.Intent = "changed intent"
	n.Status = StatusChangePending
	stamped, err := Stamp(n)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteNode(dir, stamped); err != nil {
		t.Fatal(err)
	}
	rep, err = Drift(dir, "root", snapID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Changed) != 1 || rep.Changed[0].ID != "child" {
		t.Fatalf("changed = %+v", rep.Changed)
	}

	rep, err = Drift(dir, "  ", snapID)
	if err != nil || rep.NodeID != "root" {
		t.Fatalf("blank node: rep=%+v err=%v", rep, err)
	}
	if _, err := Drift(dir, "root", "  "); err == nil || !strings.Contains(err.Error(), ReasonSnapshotKeyRequired) {
		t.Fatalf("blank snapshot key must fail, got %v", err)
	}
	if _, err := Drift(dir, "root", "does-not-exist"); err == nil {
		t.Fatal("unknown snapshot key must fail")
	}
	rep, err = Drift(dir, "root", "latest")
	if err != nil || rep.SnapshotID != snapID {
		t.Fatalf("latest alias: rep=%+v err=%v", rep, err)
	}
}

func TestDriftWithoutSnapshotFailsClosed(t *testing.T) {
	dir := t.TempDir()
	root := Node{ID: "root", Kind: KindIntent, Title: "root", Path: "root", Status: StatusFrozen}
	stamped, err := Stamp(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteNode(dir, stamped); err != nil {
		t.Fatal(err)
	}
	idx := Index{Schema: Schema, Version: "v0.1.0", RootID: "root", Nodes: []IndexEntry{
		{ID: "root", Kind: KindIntent, Status: StatusFrozen, Title: "root", Path: "root", Fingerprint: stamped.Fingerprint, FingerprintAlg: FingerprintAlg},
	}}
	if err := WriteIndex(dir, idx); err != nil {
		t.Fatal(err)
	}
	if _, err := Drift(dir, "root", "latest"); err == nil || !strings.Contains(err.Error(), "no snapshot baseline") {
		t.Fatalf("empty snapshot set must fail, got %v", err)
	}
}
