package intenttree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func goldenCases() map[string]Index {
	deep := Index{Schema: Schema, Version: "v0.1.0", RootID: "r", Nodes: []IndexEntry{
		{ID: "r", Kind: KindIntent, Status: StatusDraft, Title: "root", Path: "root", Fingerprint: "a1"},
		{ID: "i2", ParentID: "r", Kind: KindIntent, Status: StatusDraft, Title: "sub intent", Path: "root/sub", Fingerprint: "b2"},
		{ID: "m3", ParentID: "i2", Kind: KindModule, Status: StatusDraft, Title: "module", Path: "root/sub/m", Fingerprint: "c3"},
		{ID: "f4", ParentID: "m3", Kind: KindFeature, Status: StatusDraft, Title: "feature", Path: "root/sub/m/f", Fingerprint: "d4"},
		{ID: "t5", ParentID: "f4", Kind: KindTouchpoint, Status: StatusDraft, Title: "touch", Path: "root/sub/m/f/t", Fingerprint: "e5"},
	}}
	return map[string]Index{
		"empty": {Schema: Schema, Version: "v0.1.0", RootID: "root"},
		"single": {Schema: Schema, Version: "v0.1.0", RootID: "root", Nodes: []IndexEntry{
			{ID: "root", Kind: KindIntent, Status: StatusDraft, Title: "only", Path: "root", Fingerprint: "aa"},
		}},
		"deep": deep,
		"duplicate": {Schema: Schema, Version: "v0.1.0", RootID: "root", Nodes: []IndexEntry{
			{ID: "root", Kind: KindIntent, Status: StatusDraft, Title: "root", Path: "root"},
			{ID: "root", Kind: KindIntent, Status: StatusDraft, Title: "copy", Path: "root2"},
		}},
	}
}

func TestRenderGolden(t *testing.T) {
	update := os.Getenv("UPDATE_GOLDEN") == "1"
	for name, idx := range goldenCases() {
		for _, tc := range []struct {
			ext  string
			data []byte
		}{
			{"md", RenderMarkdown(idx)},
			{"html", RenderHTML(idx)},
		} {
			path := filepath.Join("testdata", "render", name+"."+tc.ext+".golden")
			if update {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, tc.data, 0o644); err != nil {
					t.Fatal(err)
				}
				continue
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%s: missing golden (run UPDATE_GOLDEN=1 go test ./internal/intenttree): %v", path, err)
			}
			if string(want) != string(tc.data) {
				t.Fatalf("%s drift:\n--- want ---\n%s\n--- got ---\n%s", path, want, tc.data)
			}
		}
	}
}

func TestRenderSourceHashTracksChange(t *testing.T) {
	idx := goldenCases()["single"]
	a := SourceHash(idx)
	idx.Nodes[0].Title = "changed"
	b := SourceHash(idx)
	if b == a {
		t.Fatal("source hash must change when the index changes")
	}
	md := string(RenderMarkdown(idx))
	if !strings.Contains(md, b) || strings.Contains(md, a) {
		t.Fatalf("render must carry its own source hash %s (old %s)", b, a)
	}
}
