package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"beacon-go/internal/intenttree"
)

func TestChangeGrowTwoBaselines(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	for _, id := range []string{"t1", "t2", "f", "m", "root"} {
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree")
	initial, err := intenttree.ListSnapshots(dir)
	if err != nil {
		t.Fatal(err)
	}
	history, err := os.ReadFile(filepath.Join(dir, "snapshots", initial[0], "snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	sibling, _ := intenttree.ReadNode(dir, "t2")
	for _, title := range []string{"third", "fourth"} {
		parent, err := intenttree.ReadNode(dir, "f")
		if err != nil {
			t.Fatal(err)
		}
		proposal := filepath.Join(proj, "grow.json")
		raw := []byte(`{"children":[{"kind":"touchpoint","title":"` + title + `","acs":[{"id":"AC-1","text":"works"}],"verification":{"type":"test","spec":"go test ./..."}}]}`)
		if err := os.WriteFile(proposal, raw, 0600); err != nil {
			t.Fatal(err)
		}
		args := []string{"change", "grow", "--project", proj, "--version", "v0.1.0", "--node", "f", "--proposal", proposal, "--summary", "add leaf", "--by", "maker", "--base-fingerprint", parent.Fingerprint, "--json"}
		out := mustRun(t, args...)
		var rec struct {
			Added          []string `json:"added"`
			Modified       []string `json:"modified"`
			BeforeSnapshot string   `json:"before_snapshot_id"`
		}
		if err := json.Unmarshal([]byte(out), &rec); err != nil {
			t.Fatal(err)
		}
		if len(rec.Added) != 1 || len(rec.Modified) != 3 || rec.BeforeSnapshot == "" {
			t.Fatalf("incomplete scope: %s", out)
		}
		if _, err := runCLI(t, args...); err == nil {
			t.Fatal("old base accepted")
		}
		for _, id := range append(rec.Added, "f", "m", "root") {
			if _, err := freeze(t, proj, id); err != nil {
				t.Fatal(err)
			}
		}
		current, _ := intenttree.ReadNode(dir, "t2")
		if current.Fingerprint != sibling.Fingerprint {
			t.Fatal("unrelated leaf changed")
		}
		got, _ := os.ReadFile(filepath.Join(dir, "snapshots", initial[0], "snapshot.json"))
		if string(got) != string(history) {
			t.Fatal("history rewritten")
		}
	}
	snaps, _ := intenttree.ListSnapshots(dir)
	if len(snaps) < 3 {
		t.Fatal("missing next baselines")
	}
}

func TestChangeGrowDraftConcurrentBase(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	dir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree")
	for round := 0; round < 2; round++ {
		parent, err := intenttree.ReadNode(dir, "f")
		if err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i := 0; i < 2; i++ {
			p := filepath.Join(proj, fmt.Sprintf("p%d-%d.json", round, i))
			raw := fmt.Sprintf(`{"children":[{"kind":"touchpoint","title":"leaf-%d-%d","acs":[{"id":"a","text":"works"}],"verification":{"spec":"test"}}]}`, round, i)
			if err := os.WriteFile(p, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			wg.Add(1)
			go func(i int, p string) {
				defer wg.Done()
				<-start
				_, errs[i] = runCLI(t, "change", "grow", "--project", proj, "--version", "v0.1.0", "--node", "f", "--proposal", p, "--summary", "draft growth", "--by", "maker", "--base-fingerprint", parent.Fingerprint)
			}(i, p)
		}
		close(start)
		wg.Wait()
		accepted := 0
		for _, err := range errs {
			if err == nil {
				accepted++
			} else if !strings.Contains(err.Error(), intenttree.ReasonChangeBaseMismatch) {
				t.Fatal(err)
			}
		}
		if accepted != 1 {
			t.Fatalf("round %d: want one winner, got %d", round, accepted)
		}
	}
	changes, err := intenttree.ListChanges(dir)
	if err != nil || len(changes) != 2 {
		t.Fatalf("draft changes not recorded: %v %v", changes, err)
	}
}

func TestChangeGrowRejectsInvalid(t *testing.T) {
	for name, raw := range map[string]string{"empty": `{"children":[]}`, "blank": `{"children":[{"kind":"touchpoint","title":" "}]}`, "kind": `{"children":[{"kind":"module","title":"bad"}]}`, "duplicate": `{"children":[{"kind":"touchpoint","title":"same","acs":[{"id":"a","text":"a"}],"verification":{"spec":"test"}},{"kind":"touchpoint","title":"same","acs":[{"id":"a","text":"a"}],"verification":{"spec":"test"}}]}`} {
		t.Run(name, func(t *testing.T) {
			proj := treeTestEnv(t)
			seedFreezeTree(t, proj, false)
			dir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree")
			n, _ := intenttree.ReadNode(dir, "f")
			before, _ := os.ReadFile(intenttree.IndexPath(dir))
			p := filepath.Join(proj, "proposal.json")
			if err := os.WriteFile(p, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := runCLI(t, "change", "grow", "--project", proj, "--version", "v0.1.0", "--node", "f", "--proposal", p, "--summary", "add", "--by", "maker", "--base-fingerprint", n.Fingerprint)
			if err == nil {
				t.Fatal("invalid proposal accepted")
			}
			after, _ := os.ReadFile(intenttree.IndexPath(dir))
			if string(before) != string(after) {
				t.Fatal("refusal changed tree")
			}
		})
	}
}

func TestChangeGrowModuleAndLimit(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	for _, id := range []string{"t1", "t2", "f", "m", "root"} {
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree")
	root, _ := intenttree.ReadNode(dir, "root")
	proposal := filepath.Join(proj, "modules.json")
	children := []map[string]string{}
	for i := 0; i < 6; i++ {
		children = append(children, map[string]string{"kind": "module", "title": fmt.Sprintf("new-module-%d", i)})
	}
	raw, _ := json.Marshal(map[string]any{"children": children})
	if err := os.WriteFile(proposal, raw, 0600); err != nil {
		t.Fatal(err)
	}
	out := mustRun(t, "change", "grow", "--project", proj, "--version", "v0.1.0", "--node", "root", "--proposal", proposal, "--summary", "modules", "--by", "maker", "--base-fingerprint", root.Fingerprint)
	rec := parseDoc(t, out)
	if len(rec["added"].([]any)) != 6 {
		t.Fatal(out)
	}
	for _, id := range rec["added"].([]any) {
		n, err := intenttree.ReadNode(dir, id.(string))
		if err != nil || n.Status != intenttree.StatusDraft || n.Kind != intenttree.KindModule {
			t.Fatalf("bad new module: %+v %v", n, err)
		}
	}
	root, _ = intenttree.ReadNode(dir, "root")
	if err := os.WriteFile(proposal, []byte(`{"children":[{"kind":"module","title":"overflow"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := runCLI(t, "change", "extend", "--project", proj, "--version", "v0.1.0", "--node", "root", "--proposal", proposal, "--summary", "overflow", "--by", "maker", "--base-fingerprint", root.Fingerprint)
	if err == nil || !strings.Contains(err.Error(), intenttree.ReasonChildLimit) {
		t.Fatalf("limit not enforced: %v", err)
	}
}
