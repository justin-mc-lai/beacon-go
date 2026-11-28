package cli

import (
	"beacon-go/internal/intenttree"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestChangeContinueApplyTwoDeliveries(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	for _, id := range []string{"t1", "t2", "f", "m", "root"} {
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree")
	original, _ := intenttree.ReadNode(dir, "t2")
	previous := ""
	for round, to := range []string{"v0.2.0", "v0.3.0"} {
		mustRun(t, "projects", "register", "--project", proj, "--version", to, "--json")
		ids, _ := intenttree.ListSnapshots(dir)
		snap, err := intenttree.ReadSnapshot(dir, ids[0])
		if err != nil {
			t.Fatal(err)
		}
		history, _ := os.ReadFile(filepath.Join(dir, "snapshots", ids[0], "snapshot.json"))
		source := "v0.1.0"
		if previous != "" {
			source = previous
		}
		args := []string{"change", "continue", "--apply", "--project", proj, "--version", source, "--to-version", to, "--snapshot", ids[0], "--base-source-hash", snap.SourceHash, "--by", "maker", "--summary", "delivery baseline", "--json"}
		if previous != "" {
			args = append(args, "--previous-delivery", previous)
		}
		for _, pair := range [][2]string{{snap.SourceHash, "stale"}, {to, "v9.0.0"}, {ids[0], "latest"}, {"maker", " "}} {
			bad := append([]string{}, args...)
			for i := range bad {
				if bad[i] == pair[0] {
					bad[i] = pair[1]
				}
			}
			if _, err := runCLI(t, bad...); err == nil {
				t.Fatalf("invalid apply accepted: %v", pair)
			}
		}
		out := mustRun(t, args...)
		mustRun(t, "plan", "tree", "--project", proj, "--version", to, "--json")
		mustRun(t, "truth", "node", "freeze", "--project", proj, "--version", to, "--node", "root", "--json")
		doc := parseDoc(t, out)
		if doc["preview"] != false {
			t.Fatal(out)
		}
		raw, err := os.ReadFile(filepath.Join(dir, "lineage", to+".json"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := runCLI(t, args...); err == nil {
			t.Fatal("duplicate delivery accepted")
		}
		bad := append([]string{}, args...)
		for i := range bad {
			if bad[i] == snap.SourceHash {
				bad[i] = "stale"
			}
		}
		if _, err := runCLI(t, bad...); err == nil {
			t.Fatal("stale base accepted")
		}
		got, _ := os.ReadFile(filepath.Join(dir, "lineage", to+".json"))
		if string(got) != string(raw) {
			t.Fatal("immutable lineage changed")
		}
		got, _ = os.ReadFile(filepath.Join(dir, "snapshots", ids[0], "snapshot.json"))
		if string(got) != string(history) {
			t.Fatal("history changed")
		}
		if _, err := os.Stat(filepath.Join(proj, "docs", "beacon", to, "tree")); !os.IsNotExist(err) {
			t.Fatal("second authority created")
		}
		if round == 0 {
			parent, _ := intenttree.ReadNode(dir, "f")
			p := filepath.Join(proj, "next.json")
			if err := os.WriteFile(p, []byte(`{"children":[{"kind":"touchpoint","title":"next","acs":[{"id":"a","text":"ok"}],"verification":{"spec":"test"}}]}`), 0600); err != nil {
				t.Fatal(err)
			}
			grown := parseDoc(t, mustRun(t, "change", "grow", "--project", proj, "--version", to, "--node", "f", "--proposal", p, "--summary", "next leaf", "--by", "maker", "--base-fingerprint", parent.Fingerprint))
			for _, id := range []string{grown["added"].([]any)[0].(string), "f", "m", "root"} {
				if _, err := freeze(t, proj, id); err != nil {
					t.Fatal(err)
				}
			}
		} else {
			delta := doc["delta"].(map[string]any)
			if len(delta["added"].([]any)) != 1 {
				t.Fatal(fmt.Sprint(delta))
			}
		}
		sibling, _ := intenttree.ReadNode(dir, "t2")
		if sibling.Fingerprint != original.Fingerprint {
			t.Fatal("unrelated branch invalidated")
		}
		previous = to
	}
}

func TestChangeDeltaCommonContract(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	for _, id := range []string{"t1", "t2", "f", "m", "root"} {
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree")
	n, _ := intenttree.ReadNode(dir, "t1")
	requested := parseDoc(t, mustRun(t, "change", "request", "--project", proj, "--version", "v0.1.0", "--node", "t1", "--summary", "retire", "--by", "maker", "--base-fingerprint", n.Fingerprint, "--json"))
	if requested["action"] != "request" || len(requested["modified"].([]any)) != 4 || requested["source_hash"] == "" {
		t.Fatal(requested)
	}
	n, _ = intenttree.ReadNode(dir, "t1")
	retired := parseDoc(t, mustRun(t, "change", "deprecate", "--project", proj, "--version", "v0.1.0", "--node", "t1", "--summary", "retire", "--by", "maker", "--base-fingerprint", n.Fingerprint))
	if retired["action"] != "deprecate" || len(retired["deprecated"].([]any)) != 1 || len(retired["dependency_scope"].([]any)) != 3 {
		t.Fatal(retired)
	}
	for _, rec := range []map[string]any{requested, retired} {
		for _, field := range []string{"added", "modified", "deprecated", "dependency_scope"} {
			if rec[field] == nil {
				t.Fatalf("missing %s: %v", field, rec)
			}
		}
	}
}

func TestChangeLineageExplicitBaseline(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	for _, id := range []string{"t1", "t2", "f", "m", "root"} {
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree")
	ids, _ := intenttree.ListSnapshots(dir)
	before, _ := os.ReadFile(intenttree.IndexPath(dir))
	out := mustRun(t, "change", "continue", "--project", proj, "--version", "v0.1.0", "--snapshot", ids[0], "--to-version", "v0.2.0", "--json")
	doc := parseDoc(t, out)
	if doc["preview"] != true || doc["baseline_snapshot_id"] != ids[0] {
		t.Fatalf("bad lineage: %s", out)
	}
	mappings := doc["mapping"].([]any)
	if len(mappings) != 5 {
		t.Fatalf("incomplete mapping: %s", out)
	}
	for _, item := range mappings {
		m := item.(map[string]any)
		if m["predecessor_id"] == m["successor_id"] {
			t.Fatal("cross-version ID falsely preserved")
		}
	}
	after, _ := os.ReadFile(intenttree.IndexPath(dir))
	if string(before) != string(after) {
		t.Fatal("preview changed authority")
	}
	if _, err := os.Stat(filepath.Join(proj, "docs", "beacon", "v0.2.0", "tree")); !os.IsNotExist(err) {
		t.Fatal("preview materialized second tree")
	}
	for _, snapshot := range []string{"latest", "", "../escape"} {
		if _, err := runCLI(t, "change", "continue", "--project", proj, "--version", "v0.1.0", "--snapshot", snapshot, "--to-version", "v0.2.0"); err == nil {
			t.Fatalf("accepted snapshot %q", snapshot)
		}
	}
}
