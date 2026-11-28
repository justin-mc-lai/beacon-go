package cli

import (
	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestChangeMove(t *testing.T) {
	for _, mode := range []string{"success", "module", "kind", "cycle", "descendant-cycle", "frozen", "frozen-child", "ancestor", "stale", "blank", "limit", "collision", "touchpoint", "same-parent", "missing-parent"} {
		t.Run(mode, func(t *testing.T) {
			proj := treeTestEnv(t)
			seedFreezeTree(t, proj, false)
			dir := filepath.Join(proj, "docs/beacon/v0.1.0/tree")
			root, _ := intenttree.ReadNode(dir, "root")
			p := filepath.Join(proj, "move-proposal.json")
			if err := os.WriteFile(p, []byte(`{"children":[{"kind":"module","title":"destination"}]}`), 0600); err != nil {
				t.Fatal(err)
			}
			out := mustRun(t, "change", "grow", "--project", proj, "--version", "v0.1.0", "--node", "root", "--proposal", p, "--base-fingerprint", root.Fingerprint, "--summary", "destination", "--by", "maker")
			dest := parseDoc(t, out)["added"].([]any)[0].(string)
			node := "f"
			parent := dest
			if mode == "module" {
				node = "m"
			}
			if mode == "kind" {
				parent = "root"
			}
			if mode == "cycle" {
				node = "m"
				parent = "m"
			}
			if mode == "descendant-cycle" {
				d, _ := intenttree.ReadNode(dir, "m")
				mustWriteMoveProposal(t, p, `{"children":[{"kind":"module","title":"nested"}]}`)
				o := mustRun(t, "change", "grow", "--project", proj, "--version", "v0.1.0", "--node", "m", "--proposal", p, "--base-fingerprint", d.Fingerprint, "--summary", "nested", "--by", "maker")
				node = "m"
				parent = parseDoc(t, o)["added"].([]any)[0].(string)
			}
			if mode == "touchpoint" {
				node = "t1"
			}
			if mode == "same-parent" {
				parent = "m"
			}
			if mode == "missing-parent" {
				parent = "missing"
			}
			if mode == "frozen-child" {
				if _, err := freeze(t, proj, "t1"); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "frozen" {
				for _, id := range []string{"t1", "t2", "f"} {
					if _, err := freeze(t, proj, id); err != nil {
						t.Fatal(err)
					}
				}
			}
			if mode == "ancestor" {
				for _, id := range []string{"t1", "t2", "f", "m"} {
					if _, err := freeze(t, proj, id); err != nil {
						t.Fatal(err)
					}
				}
			}
			if mode == "limit" || mode == "collision" {
				d, _ := intenttree.ReadNode(dir, dest)
				raw := `{"children":[`
				count := 7
				if mode == "collision" {
					count = 1
				}
				for i := 0; i < count; i++ {
					if i > 0 {
						raw += ","
					}
					title := string(rune('a' + i))
					if mode == "collision" {
						title = "f"
					}
					raw += `{"kind":"feature","title":"` + title + `"}`
				}
				raw += `]}`
				if err := os.WriteFile(p, []byte(raw), 0600); err != nil {
					t.Fatal(err)
				}
				mustRun(t, "change", "grow", "--project", proj, "--version", "v0.1.0", "--node", dest, "--proposal", p, "--base-fingerprint", d.Fingerprint, "--summary", "fill", "--by", "maker")
			}
			n, _ := intenttree.ReadNode(dir, node)
			leaf, _ := intenttree.ReadNode(dir, "t1")
			base := n.Fingerprint
			if mode == "stale" {
				base = "wrong"
			}
			if mode == "blank" {
				base = " "
			}
			before, _ := os.ReadFile(intenttree.IndexPath(dir))
			out, err := runCLI(t, "change", "move", "--project", proj, "--version", "v0.1.0", "--node", node, "--parent", parent, "--base-fingerprint", base, "--summary", "reorganize", "--by", "maker")
			if mode != "success" && mode != "module" {
				if err == nil {
					t.Fatalf("accepted %s: %s", mode, out)
				}
				after, _ := os.ReadFile(intenttree.IndexPath(dir))
				if string(before) != string(after) {
					t.Fatal("refusal changed index")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			moved, _ := intenttree.ReadNode(dir, node)
			gotLeaf, _ := intenttree.ReadNode(dir, "t1")
			if moved.ID != n.ID || moved.ParentID != dest || moved.Status != intenttree.StatusDraft || moved.Fingerprint == n.Fingerprint || !reflect.DeepEqual(leaf.ACs, gotLeaf.ACs) || leaf.ID != gotLeaf.ID || leaf.Path == gotLeaf.Path {
				t.Fatalf("bad move: %+v %+v", moved, gotLeaf)
			}
			old, _ := intenttree.ReadNode(dir, "m")
			if old.Status != intenttree.StatusDraft {
				t.Fatal("draft ancestor no longer editable")
			}
			// A real update proves draft ancestors remain editable after the move.
			mustWriteMoveProposal(t, p, `{"intent":"still editable"}`)
			mustRun(t, "truth", "node", "update", "--project", proj, "--version", "v0.1.0", "--node", "m", "--file", p)
			st, err := store.Open(storePathFromEnv(t))
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			c, err := st.Tree().GetTreeNode("t1")
			if err != nil || c.Path != gotLeaf.Path || c.Fingerprint != gotLeaf.Fingerprint {
				t.Fatalf("cache not moved: %+v %v", c, err)
			}
			recs, _ := intenttree.ListChanges(dir)
			if len(recs) != 2 || recs[0].Action != "move" {
				t.Fatal(out)
			}
		})
	}
}

func mustWriteMoveProposal(t *testing.T, p, raw string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
}
