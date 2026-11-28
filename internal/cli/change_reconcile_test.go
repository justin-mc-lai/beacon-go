package cli

import (
	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestChangeReconcileIndex(t *testing.T) {
	for _, mode := range []string{"success", "stale", "invalid", "identity", "other", "empty", "blank", "duplicate", "empty-index", "alias", "unknown-alg"} {
		t.Run(mode, func(t *testing.T) {
			proj := treeTestEnv(t)
			seedFreezeTree(t, proj, false)
			dir := filepath.Join(proj, "docs/beacon/v0.1.0/tree")
			n, _ := intenttree.ReadNode(dir, "f")
			idx, _ := intenttree.ReadIndex(dir)
			for i := range idx.Nodes {
				if idx.Nodes[i].ID == "f" {
					idx.Nodes[i].Title = "stale index"
				}
				if mode == "other" && idx.Nodes[i].ID == "t1" {
					idx.Nodes[i].Title = "other drift"
				}
			}
			if mode == "duplicate" {
				idx.Nodes = append(idx.Nodes, idx.Nodes[0])
			}
			if mode == "empty-index" {
				idx.Nodes = nil
			}
			if mode == "alias" {
				idx.Nodes[0].ID = "../alias"
			}
			if err := intenttree.WriteIndex(dir, idx); err != nil {
				t.Fatal(err)
			}
			base := n.Fingerprint
			if mode == "invalid" {
				n.Title = "tampered"
				if err := intenttree.WriteNode(dir, n); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "unknown-alg" {
				n.FingerprintAlg = "unknown"
				if err := intenttree.WriteNode(dir, n); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "identity" {
				n.ID = "different"
				n, _ = intenttree.Stamp(n)
				base = n.Fingerprint
				if err := intenttree.WriteNode(dir, n); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "stale" {
				base = "wrong"
			}
			if mode == "empty" {
				base = ""
			}
			if mode == "blank" {
				base = " "
			}
			before, _ := os.ReadFile(intenttree.IndexPath(dir))
			body, _ := os.ReadFile(intenttree.NodePath(dir, "f"))
			out, err := runCLI(t, "change", "reconcile-index", "--project", proj, "--version", "v0.1.0", "--node", "f", "--base-fingerprint", base, "--summary", "repair projection", "--by", "maker")
			after, _ := os.ReadFile(intenttree.IndexPath(dir))
			gotBody, _ := os.ReadFile(intenttree.NodePath(dir, "f"))
			if string(body) != string(gotBody) {
				t.Fatal("reconcile rewrote body")
			}
			if mode != "success" {
				if err == nil {
					t.Fatalf("accepted %s: %s", mode, out)
				}
				if string(before) != string(after) {
					t.Fatal("refusal wrote index")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			idx, _ = intenttree.ReadIndex(dir)
			for _, e := range idx.Nodes {
				if e.ID == "f" && e.Title != n.Title {
					t.Fatal("index not projected")
				}
			}
			st, err := store.Open(storePathFromEnv(t))
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			c, err := st.Tree().GetTreeNode("f")
			if err != nil || c.Title != n.Title || c.Fingerprint != n.Fingerprint {
				t.Fatalf("cache: %+v %v", c, err)
			}
			recs, _ := intenttree.ListChanges(dir)
			if len(recs) != 1 || recs[0].Action != "reconcile-index" {
				t.Fatal(out)
			}
		})
	}
}

func TestChangeReconcileNonrecursiveFrozenChildren(t *testing.T) {
	for _, mode := range []string{"explicit-false", "default-recursive", "explicit-true"} {
		t.Run(mode, func(t *testing.T) {
			proj := treeTestEnv(t)
			seedFreezeTree(t, proj, false)
			dir := filepath.Join(proj, "docs/beacon/v0.1.0/tree")
			for _, id := range []string{"t1", "t2", "f"} {
				if _, err := freeze(t, proj, id); err != nil {
					t.Fatal(err)
				}
			}
			// Keep a draft module with its historical child fingerprint list intact.
			idx, _ := intenttree.ReadIndex(dir)
			m, _ := intenttree.ReadNode(dir, "m")
			f, _ := intenttree.ReadNode(dir, "f")
			m.ChildFingerprints = []string{f.Fingerprint}
			m, _ = intenttree.Stamp(m)
			if err := intenttree.WriteNode(dir, m); err != nil {
				t.Fatal(err)
			}
			other := m
			other.ID = "other"
			other.ChildFingerprints = nil
			other, _ = intenttree.Stamp(other)
			if err := intenttree.WriteNode(dir, other); err != nil {
				t.Fatal(err)
			}
			for i := range idx.Nodes {
				if idx.Nodes[i].ID == m.ID {
					idx.Nodes[i].Fingerprint = m.Fingerprint
				}
			}
			idx.Nodes = append(idx.Nodes, intenttree.IndexEntry{ID: other.ID, Kind: other.Kind, ParentID: other.ParentID, Path: other.Path, Title: other.Title, Status: other.Status, Fingerprint: other.Fingerprint, FingerprintAlg: other.FingerprintAlg})
			if err := intenttree.WriteIndex(dir, idx); err != nil {
				t.Fatal(err)
			}
			target := map[string]any{"node_id": "m", "base_fingerprint": m.Fingerprint, "path": "root/module-unique"}
			if mode == "explicit-false" {
				target["subtree"] = false
			}
			if mode == "explicit-true" {
				target["subtree"] = true
			}
			file := filepath.Join(proj, "nonrecursive.json")
			raw, _ := json.Marshal(map[string]any{"nodes": []any{target}})
			if err := os.WriteFile(file, raw, 0600); err != nil {
				t.Fatal(err)
			}
			before := map[string]string{}
			for _, id := range []string{"root", "m", "f", "t1", "t2", "other"} {
				raw, _ := os.ReadFile(intenttree.NodePath(dir, id))
				before[id] = string(raw)
			}
			out, err := runCLI(t, "change", "reconcile-index", "--project", proj, "--version", "v0.1.0", "--file", file, "--summary", "module path dedup only", "--by", "maker")
			if mode != "explicit-false" {
				if err == nil {
					t.Fatal("recursive repair crossed frozen boundary")
				}
				for id, want := range before {
					raw, _ := os.ReadFile(intenttree.NodePath(dir, id))
					if string(raw) != want {
						t.Fatal("refusal changed body")
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for id, want := range before {
				if id == "m" {
					continue
				}
				raw, _ := os.ReadFile(intenttree.NodePath(dir, id))
				if string(raw) != want {
					t.Fatalf("changed protected body %s", id)
				}
			}
			got, _ := intenttree.ReadNode(dir, "m")
			valid, _ := intenttree.VerifyFingerprint(got)
			if !valid || got.Path != "root/module-unique" || got.ParentID != m.ParentID || !reflect.DeepEqual(got.ChildFingerprints, m.ChildFingerprints) {
				t.Fatalf("bad narrow repair %+v", got)
			}
			var rec map[string]any
			if err := json.Unmarshal([]byte(out), &rec); err != nil {
				t.Fatal(err)
			}
			nodes := rec["nodes"].([]any)
			if rec["action"] != "repair-path" || len(rec["modified"].([]any)) != 1 || nodes[0].(map[string]any)["subtree"] != false {
				t.Fatal(out)
			}
		})
	}
}

func TestChangeReconcileLegacyTruthOmission(t *testing.T) {
	for _, mode := range []string{"success", "title-drift", "wrong-link", "explicit-link-missing"} {
		t.Run(mode, func(t *testing.T) {
			proj := treeTestEnv(t)
			seedFreezeTree(t, proj, false)
			dir := filepath.Join(proj, "docs/beacon/v0.1.0/tree")
			n, _ := intenttree.ReadNode(dir, "f")
			n.LegacyTruthPath = "docs/requirements/legacy"
			if mode == "explicit-link-missing" {
				n.TruthPath = "docs/requirements/current"
			}
			n, _ = intenttree.Stamp(n)
			if err := intenttree.WriteNode(dir, n); err != nil {
				t.Fatal(err)
			}
			idx, _ := intenttree.ReadIndex(dir)
			for i := range idx.Nodes {
				e := &idx.Nodes[i]
				if e.ID == "f" {
					e.Fingerprint = n.Fingerprint
					if mode == "title-drift" {
						e.Title = "other"
					}
					if mode == "wrong-link" {
						e.TruthPath = "wrong"
					}
				}
				if e.ID == "t1" || e.ID == "t2" {
					e.Path = "stale-" + e.ID
				}
			}
			if err := intenttree.WriteIndex(dir, idx); err != nil {
				t.Fatal(err)
			}
			bodyBefore, _ := os.ReadFile(intenttree.NodePath(dir, "f"))
			indexBefore, _ := os.ReadFile(intenttree.IndexPath(dir))
			targets := []map[string]string{}
			for _, id := range []string{"t1", "t2"} {
				n, _ := intenttree.ReadNode(dir, id)
				targets = append(targets, map[string]string{"node_id": id, "base_fingerprint": n.Fingerprint})
			}
			raw, _ := json.Marshal(map[string]any{"nodes": targets})
			p := filepath.Join(proj, "legacy-repair.json")
			if err := os.WriteFile(p, raw, 0600); err != nil {
				t.Fatal(err)
			}
			out, err := runCLI(t, "change", "reconcile-index", "--project", proj, "--version", "v0.1.0", "--file", p, "--summary", "repair leaves only", "--by", "maker")
			after, _ := os.ReadFile(intenttree.NodePath(dir, "f"))
			if string(after) != string(bodyBefore) {
				t.Fatal("legacy body changed")
			}
			if mode != "success" {
				if err == nil {
					t.Fatal("unlisted drift accepted")
				}
				after, _ = os.ReadFile(intenttree.IndexPath(dir))
				if string(after) != string(indexBefore) {
					t.Fatal("refusal wrote index")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			idx, _ = intenttree.ReadIndex(dir)
			for _, e := range idx.Nodes {
				if e.ID == "f" && e.TruthPath != "" {
					t.Fatal("implicitly rewrote unlisted legacy projection")
				}
			}
			var rec intenttree.ReconcileRecord
			if err := json.Unmarshal([]byte(out), &rec); err != nil {
				t.Fatal(err)
			}
			if len(rec.Modified) != 2 {
				t.Fatal(out)
			}
		})
	}
}

func TestChangeReconcilePathRepair(t *testing.T) {
	for _, mode := range []string{"success", "empty-cache", "stale-cache", "unlisted", "invalid-body", "frozen-child", "frozen-target", "collision", "blank-path", "absolute", "alias", "wrong-kind", "stale-base", "overlap"} {
		t.Run(mode, func(t *testing.T) {
			proj := treeTestEnv(t)
			seedFreezeTree(t, proj, false)
			dir := filepath.Join(proj, "docs/beacon/v0.1.0/tree")
			idx, _ := intenttree.ReadIndex(dir)
			// Two canonical modules collide; descendants have historical non-prefix paths.
			root, _ := intenttree.ReadNode(dir, "root")
			root.Status = intenttree.StatusFrozen
			root, _ = intenttree.Stamp(root)
			if err := intenttree.WriteNode(dir, root); err != nil {
				t.Fatal(err)
			}
			for i := range idx.Nodes {
				if idx.Nodes[i].ID == root.ID {
					idx.Nodes[i].Status = root.Status
					idx.Nodes[i].Fingerprint = root.Fingerprint
				}
			}
			m, _ := intenttree.ReadNode(dir, "m")
			m.Path = "root/node-2"
			m, _ = intenttree.Stamp(m)
			if err := intenttree.WriteNode(dir, m); err != nil {
				t.Fatal(err)
			}
			for i := range idx.Nodes {
				if idx.Nodes[i].ID == m.ID {
					idx.Nodes[i].Path = m.Path
					idx.Nodes[i].Fingerprint = m.Fingerprint
				}
			}
			other := m
			other.ID = "other"
			if mode == "overlap" {
				other.ParentID = "m"
			}
			other, _ = intenttree.Stamp(other)
			if err := intenttree.WriteNode(dir, other); err != nil {
				t.Fatal(err)
			}
			idx.Nodes = append(idx.Nodes, intenttree.IndexEntry{ID: other.ID, Kind: other.Kind, ParentID: other.ParentID, Path: other.Path, Title: other.Title, Status: other.Status, Fingerprint: other.Fingerprint, FingerprintAlg: other.FingerprintAlg})
			if mode == "frozen-child" || mode == "frozen-target" {
				id := "t1"
				if mode == "frozen-target" {
					id = "m"
				}
				n, _ := intenttree.ReadNode(dir, id)
				n.Status = intenttree.StatusFrozen
				n, _ = intenttree.Stamp(n)
				if err := intenttree.WriteNode(dir, n); err != nil {
					t.Fatal(err)
				}
				for i := range idx.Nodes {
					if idx.Nodes[i].ID == id {
						idx.Nodes[i].Status = n.Status
						idx.Nodes[i].Fingerprint = n.Fingerprint
					}
				}
			}
			for i := range idx.Nodes {
				if idx.Nodes[i].ID == "f" || idx.Nodes[i].ID == "t1" {
					idx.Nodes[i].Path = "stale-" + idx.Nodes[i].ID
					idx.Nodes[i].Fingerprint = "stale"
				}
				if mode == "unlisted" && idx.Nodes[i].ID == "t2" {
					idx.Nodes[i].Title = "unauthorized"
				}
			}
			if err := intenttree.WriteIndex(dir, idx); err != nil {
				t.Fatal(err)
			}
			st, err := store.Open(storePathFromEnv(t))
			if err != nil {
				t.Fatal(err)
			}
			c, _ := st.Tree().GetTreeNode("root")
			cache := []store.TreeNode{c}
			if mode == "stale-cache" {
				stale := c
				stale.ID = "cache-only"
				stale.Path = "obsolete"
				cache = append(cache, stale)
			}
			if mode == "empty-cache" {
				cache = nil
			}
			if err := st.ReplaceTreeNodes(c.ProjectVersionID, cache); err != nil {
				t.Fatal(err)
			}
			st.Close()
			desired := "root/repaired"
			if mode == "collision" {
				desired = "root/node-2"
			}
			if mode == "blank-path" {
				desired = " "
			}
			if mode == "absolute" {
				desired = "/root/repaired"
			}
			if mode == "alias" {
				desired = "root/../repaired"
			}
			targets := []map[string]string{}
			for _, id := range []string{"f", "t1", "m"} {
				n, _ := intenttree.ReadNode(dir, id)
				target := map[string]string{"node_id": id, "base_fingerprint": n.Fingerprint}
				if id == "m" {
					target["path"] = desired
				}
				if mode == "wrong-kind" && id == "m" {
					delete(target, "path")
				}
				if mode == "wrong-kind" && id == "f" {
					target["path"] = desired
				}
				targets = append(targets, target)
			}
			if mode == "stale-base" {
				targets[2]["base_fingerprint"] = "stale"
			}
			if mode == "overlap" {
				targets = append(targets, map[string]string{"node_id": "other", "base_fingerprint": other.Fingerprint, "path": "root/repaired"})
			}
			if mode == "invalid-body" {
				n, _ := intenttree.ReadNode(dir, "t2")
				n.Title = "tampered"
				if err := intenttree.WriteNode(dir, n); err != nil {
					t.Fatal(err)
				}
			}
			before := map[string]string{}
			for _, id := range []string{"root", "m", "f", "t1", "t2", "other"} {
				raw, _ := os.ReadFile(intenttree.NodePath(dir, id))
				before[id] = string(raw)
			}
			indexBefore, _ := os.ReadFile(intenttree.IndexPath(dir))
			file := filepath.Join(proj, "repair-path.json")
			raw, _ := json.Marshal(map[string]any{"nodes": targets})
			if err := os.WriteFile(file, raw, 0600); err != nil {
				t.Fatal(err)
			}
			out, err := runCLI(t, "change", "reconcile-index", "--project", proj, "--version", "v0.1.0", "--file", file, "--summary", "explicit path repair", "--by", "maker")
			success := mode == "success" || mode == "empty-cache" || mode == "stale-cache"
			if !success {
				if err == nil {
					t.Fatal("accepted invalid repair")
				}
				for id, want := range before {
					raw, _ := os.ReadFile(intenttree.NodePath(dir, id))
					if string(raw) != want {
						t.Fatal("refusal changed body")
					}
				}
				raw, _ := os.ReadFile(intenttree.IndexPath(dir))
				if string(raw) != string(indexBefore) {
					t.Fatal("refusal changed index")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var rec intenttree.ReconcileRecord
			if err := json.Unmarshal([]byte(out), &rec); err != nil {
				t.Fatal(err)
			}
			if rec.Action != "repair-path" || len(rec.Modified) != 4 || len(rec.Nodes) != 3 {
				t.Fatal(out)
			}
			for id, want := range map[string]string{"m": "root/repaired", "f": "root/repaired/f", "t1": "root/repaired/f/t1", "t2": "root/repaired/f/t2"} {
				n, _ := intenttree.ReadNode(dir, id)
				ok, _ := intenttree.VerifyFingerprint(n)
				if n.Path != want || !ok || n.ID != id {
					t.Fatalf("bad repair %+v", n)
				}
			}
			for _, id := range []string{"root", "other"} {
				raw, _ := os.ReadFile(intenttree.NodePath(dir, id))
				if string(raw) != before[id] {
					t.Fatal("unrelated body changed")
				}
			}
			st, err = store.Open(storePathFromEnv(t))
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			cache, err = st.Tree().ListTreeNodes(c.ProjectVersionID)
			if err != nil || len(cache) != 6 {
				t.Fatalf("cache rebuild %d %v", len(cache), err)
			}
		})
	}
}

func TestChangeReconcileBatchLegacy(t *testing.T) {
	for _, mode := range []string{"success", "unlisted", "stale", "duplicate", "empty", "mutual", "invalid-structure"} {
		t.Run(mode, func(t *testing.T) {
			proj := treeTestEnv(t)
			seedFreezeTree(t, proj, false)
			dir := filepath.Join(proj, "docs/beacon/v0.1.0/tree")
			// Historical over-capacity tree: fixture creation is deliberately not a mutation API.
			idx, _ := intenttree.ReadIndex(dir)
			st, err := store.Open(storePathFromEnv(t))
			if err != nil {
				t.Fatal(err)
			}
			c, _ := st.Tree().GetTreeNode("t1")
			for i := 0; i < 20; i++ {
				n := intenttree.Node{ID: fmt.Sprintf("legacy-%d", i), Kind: intenttree.KindModule, ParentID: "m", Path: fmt.Sprintf("m/legacy-%d", i), Title: "legacy", Status: intenttree.StatusDraft}
				n, _ = intenttree.Stamp(n)
				if err := intenttree.WriteNode(dir, n); err != nil {
					t.Fatal(err)
				}
				idx.Nodes = append(idx.Nodes, intenttree.IndexEntry{ID: n.ID, Kind: n.Kind, ParentID: n.ParentID, Path: n.Path, Title: n.Title, Status: n.Status, Fingerprint: n.Fingerprint, FingerprintAlg: n.FingerprintAlg})
				nc := c
				nc.ID = n.ID
				nc.Kind = string(n.Kind)
				nc.ParentID = n.ParentID
				nc.Path = n.Path
				nc.Title = n.Title
				nc.Status = string(n.Status)
				nc.Fingerprint = n.Fingerprint
				nc.FingerprintAlg = n.FingerprintAlg
				nc.DocPath = intenttree.NodePath(dir, n.ID)
				if err := st.AppendTreeNodes(c.ProjectVersionID, []store.TreeNode{nc}); err != nil {
					t.Fatal(err)
				}
			}
			for _, id := range []string{"f", "t1"} {
				cached, err := st.Tree().GetTreeNode(id)
				if err != nil {
					t.Fatal(err)
				}
				cached.ParentID = "missing"
				cached.Path = "wrong-" + id
				if err := st.UpdateTreeNodeProjection(cached); err != nil {
					t.Fatal(err)
				}
			}
			// Cross-target path swap must be repaired transactionally, rather than
			// failing SQLite's unique version/path constraint half way through.
			for id, p := range map[string]string{"f": "m/f/t1", "t1": "m/f"} {
				c, err := st.Tree().GetTreeNode(id)
				if err != nil {
					t.Fatal(err)
				}
				c.Path = p
				if err := st.UpdateTreeNodeProjection(c); err != nil {
					t.Fatal(err)
				}
			}
			st.Close()
			bodies := map[string]string{}
			targets := []map[string]string{}
			for _, id := range []string{"f", "t1"} {
				n, _ := intenttree.ReadNode(dir, id)
				raw, _ := os.ReadFile(intenttree.NodePath(dir, id))
				bodies[id] = string(raw)
				targets = append(targets, map[string]string{"node_id": id, "base_fingerprint": n.Fingerprint})
			}
			for i := range idx.Nodes {
				if idx.Nodes[i].ID == "f" || idx.Nodes[i].ID == "t1" {
					idx.Nodes[i].ParentID = "missing"
					idx.Nodes[i].Path = "wrong"
				}
				if mode == "unlisted" && idx.Nodes[i].ID == "t2" {
					idx.Nodes[i].Title = "drift"
				}
			}
			if mode == "invalid-structure" {
				n, _ := intenttree.ReadNode(dir, "f")
				n.ParentID = "t1"
				n, _ = intenttree.Stamp(n)
				if err := intenttree.WriteNode(dir, n); err != nil {
					t.Fatal(err)
				}
				targets[0]["base_fingerprint"] = n.Fingerprint
				raw, _ := os.ReadFile(intenttree.NodePath(dir, "f"))
				bodies["f"] = string(raw)
			}
			if err := intenttree.WriteIndex(dir, idx); err != nil {
				t.Fatal(err)
			}
			if mode == "stale" {
				targets[1]["base_fingerprint"] = "wrong"
			}
			if mode == "duplicate" {
				targets = append(targets, targets[0])
			}
			if mode == "empty" {
				targets = nil
			}
			file := filepath.Join(proj, "repair.json")
			raw, _ := json.Marshal(map[string]any{"nodes": targets})
			if err := os.WriteFile(file, raw, 0600); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(intenttree.IndexPath(dir))
			args := []string{"change", "reconcile-index", "--project", proj, "--version", "v0.1.0", "--file", file, "--summary", "repair explicit set", "--by", "maker"}
			if mode == "mutual" {
				args = append(args, "--node", "f", "--base-fingerprint", targets[0]["base_fingerprint"])
			}
			out, err := runCLI(t, args...)
			for id, want := range bodies {
				got, _ := os.ReadFile(intenttree.NodePath(dir, id))
				if string(got) != want {
					t.Fatal("body changed")
				}
			}
			if mode != "success" {
				if err == nil {
					t.Fatalf("accepted %s", mode)
				}
				after, _ := os.ReadFile(intenttree.IndexPath(dir))
				if string(before) != string(after) {
					t.Fatal("refusal wrote")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var rec struct {
				Affected []string            `json:"affected"`
				Nodes    []map[string]string `json:"nodes"`
			}
			if err := json.Unmarshal([]byte(out), &rec); err != nil {
				t.Fatal(err)
			}
			if len(rec.Affected) != 2 || len(rec.Nodes) != 2 {
				t.Fatal(out)
			}
			s, err := store.Open(storePathFromEnv(t))
			if err != nil {
				t.Fatal(err)
			}
			fixed, _ := intenttree.ReadIndex(dir)
			for _, id := range []string{"f", "t1"} {
				n, _ := intenttree.ReadNode(dir, id)
				c, err := s.Tree().GetTreeNode(id)
				if err != nil || c.ParentID != n.ParentID || c.Path != n.Path || c.Fingerprint != n.Fingerprint {
					t.Fatalf("cache not repaired: %+v %v", c, err)
				}
				for _, e := range fixed.Nodes {
					if e.ID == id && (e.ParentID != n.ParentID || e.Path != n.Path) {
						t.Fatal("index not repaired")
					}
				}
			}
			s.Close()
			mustRun(t, "change", "move", "--project", proj, "--version", "v0.1.0", "--node", "f", "--parent", "legacy-0", "--base-fingerprint", targets[0]["base_fingerprint"], "--summary", "move despite unrelated legacy capacity", "--by", "maker")
		})
	}
}
