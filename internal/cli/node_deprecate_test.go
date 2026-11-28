package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

func TestNodeDeprecateRecordsChangeAndRetainsBody(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	dir := filepath.Join(proj, "docs/beacon/v0.1.0/tree")
	before, err := intenttree.ReadNode(dir, "t1")
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"change", "deprecate", "--project", proj, "--node", "t1", "--base-fingerprint", before.Fingerprint, "--summary", "superseded draft", "--by", "owner", "--json"}
	out, err := runCLI(t, args...)
	if err != nil {
		t.Fatalf("deprecate: %v %s", err, out)
	}
	n, err := intenttree.ReadNode(dir, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if n.Status != intenttree.StatusDeprecated || n.Intent != before.Intent || len(n.ACs) != len(before.ACs) {
		t.Fatalf("body lost: %+v", n)
	}
	changes, err := intenttree.ListChanges(dir)
	if err != nil || len(changes) != 1 || changes[0].NodeID != "t1" {
		t.Fatalf("changes: %+v %v", changes, err)
	}
	if _, err := runCLI(t, args...); err == nil {
		t.Fatal("stale/repeated request accepted")
	}
	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := intenttree.SetNodeStatus(st, dir, "t2", intenttree.StatusDeprecated); err == nil {
		t.Fatal("generic API bypass accepted")
	}
	if err := intenttree.SetNodeStatus(st, dir, "t1", intenttree.StatusFrozen); err == nil {
		t.Fatal("deprecated reactivated")
	}
	changes, err = intenttree.ListChanges(dir)
	if err != nil || len(changes) != 1 {
		t.Fatalf("repeat wrote record: %v %v", changes, err)
	}
}

func TestNodeDeprecateRejectsCanonicalAncestorDrift(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	dir := filepath.Join(proj, "docs/beacon/v0.1.0/tree")
	parent, err := intenttree.ReadNode(dir, "f")
	if err != nil {
		t.Fatal(err)
	}
	parent.Status = intenttree.StatusFrozen
	parent, err = intenttree.Stamp(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := intenttree.WriteNode(dir, parent); err != nil {
		t.Fatal(err)
	}
	n, err := intenttree.ReadNode(dir, "t1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = runCLI(t, "change", "deprecate", "--project", proj, "--node", "t1", "--base-fingerprint", n.Fingerprint, "--summary", "obsolete", "--by", "owner")
	if err == nil || !strings.Contains(err.Error(), "tree_state_conflict") {
		t.Fatalf("ancestor drift accepted: %v", err)
	}
}

func TestNodeDeprecateRefusals(t *testing.T) {
	for _, mode := range []string{"frozen", "children", "blank-summary", "blank-by", "missing-base", "stale-base"} {
		t.Run(mode, func(t *testing.T) {
			proj := treeTestEnv(t)
			seedFreezeTree(t, proj, false)
			dir := filepath.Join(proj, "docs/beacon/v0.1.0/tree")
			id := "t1"
			if mode == "children" {
				id = "f"
			}
			if mode == "frozen" {
				if _, err := freeze(t, proj, id); err != nil {
					t.Fatal(err)
				}
			}
			n, err := intenttree.ReadNode(dir, id)
			if err != nil {
				t.Fatal(err)
			}
			base, summary, by := n.Fingerprint, "obsolete", "owner"
			switch mode {
			case "blank-summary":
				summary = " "
			case "blank-by":
				by = " "
			case "missing-base":
				base = ""
			case "stale-base":
				base = strings.Repeat("0", 64)
			}
			_, err = runCLI(t, "change", "deprecate", "--project", proj, "--node", id, "--base-fingerprint", base, "--summary", summary, "--by", by)
			if err == nil {
				t.Fatal("invalid deprecation accepted")
			}
			if !strings.Contains(err.Error(), "tree_deprecate_invalid") && !strings.Contains(err.Error(), "tree_change_base_mismatch") {
				t.Fatalf("wrong rejection: %v", err)
			}
			after, err := intenttree.ReadNode(dir, id)
			if err != nil {
				t.Fatal(err)
			}
			if after.Fingerprint != n.Fingerprint || after.Status != n.Status {
				t.Fatal("refusal changed node")
			}
			changes, err := intenttree.ListChanges(dir)
			if err != nil || len(changes) != 0 {
				t.Fatalf("refusal wrote record: %v %v", changes, err)
			}
		})
	}
}
