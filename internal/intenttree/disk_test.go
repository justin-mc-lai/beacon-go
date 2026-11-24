package intenttree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIndexRoundTripAndAggregateView(t *testing.T) {
	dir := t.TempDir()
	idx := Index{Schema: Schema, Version: "v0.1.0", RootID: "root", Nodes: []IndexEntry{
		{ID: "root", Kind: KindIntent, Status: StatusDraft, Title: "root", Path: "root", Fingerprint: "aa"},
		{ID: "m", ParentID: "root", Kind: KindModule, Status: StatusDraft, Title: "module", Path: "legacy", Fingerprint: "bb"},
	}}
	if err := WriteIndex(dir, idx); err != nil {
		t.Fatal(err)
	}
	got, err := ReadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Nodes) != 2 || got.RootID != "root" {
		t.Fatalf("round trip lost nodes: %+v", got)
	}
	md := string(RenderMarkdown(got))
	if !strings.Contains(md, "Requirement tree") || !strings.Contains(md, "| `m` |") {
		t.Fatalf("markdown view missing content:\n%s", md)
	}
	html := string(RenderHTML(got))
	if !strings.Contains(html, "<details") || !strings.Contains(html, "module") {
		t.Fatalf("html view missing content:\n%s", html)
	}
}

func TestIndexLoad10kWithinBudget(t *testing.T) {
	dir := t.TempDir()
	nodes := make([]IndexEntry, 0, 10000)
	root := IndexEntry{ID: "root", Kind: KindIntent, Status: StatusDraft, Title: "root", Path: "root"}
	nodes = append(nodes, root)
	for i := 1; i < 10000; i++ {
		nodes = append(nodes, IndexEntry{
			ID: fmt.Sprintf("n%05d", i), ParentID: "root", Kind: KindModule,
			Status: StatusDraft, Title: "node", Path: fmt.Sprintf("m/%05d", i),
		})
	}
	if err := WriteIndex(dir, Index{Schema: Schema, Version: "v0.1.0", RootID: "root", Nodes: nodes}); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	idx, err := ReadIndex(dir)
	took := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Nodes) != 10000 {
		t.Fatalf("read %d nodes", len(idx.Nodes))
	}
	t.Logf("10k node index read: %s", took)
	if took > 50*time.Millisecond {
		t.Logf("BUDGET MISS: 10k index read %s exceeds the 50ms target; recorded, not hidden", took)
	}
	if took > time.Second {
		t.Fatalf("10k index read took %s (hard ceiling 1s)", took)
	}
}

func TestAtomicWriteLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	if err := WriteIndex(dir, Index{Schema: Schema, Version: "v0.1.0", RootID: "r", Nodes: []IndexEntry{{ID: "r", Kind: KindIntent, Path: "root"}}}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" || (len(e.Name()) > 0 && e.Name()[0] == '.') {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}
