package orchestrator

import (
	"strings"
	"testing"

	"beacon-go/internal/intenttree"
)

func TestDerivedFrontmatterNeverFreezesAScaffold(t *testing.T) {
	real := intenttree.Node{ID: "f1", Kind: intenttree.KindFeature, Title: "real",
		Status: intenttree.StatusFrozen, Doc: "## 问题\n\n真实正文。\n"}
	front := derivedFrontmatter("m-f1", "v0.1.0", "f1", real)
	for _, want := range []string{"status: frozen", "package_maturity: complete", "tree_node_id: f1"} {
		if !strings.Contains(front, want) {
			t.Fatalf("real doc frontmatter missing %q:\n%s", want, front)
		}
	}

	scaffold := intenttree.Node{ID: "f2", Kind: intenttree.KindFeature, Title: "empty",
		Status: intenttree.StatusFrozen}
	front = derivedFrontmatter("m-f2", "v0.1.0", "f2", scaffold)
	for _, want := range []string{"status: draft", "package_maturity: scaffold"} {
		if !strings.Contains(front, want) {
			t.Fatalf("scaffold frontmatter missing %q:\n%s", want, front)
		}
	}
	if strings.Contains(front, "status: frozen") {
		t.Fatalf("a scaffold must never be marked frozen:\n%s", front)
	}

	draftDoc := intenttree.Node{ID: "f3", Kind: intenttree.KindFeature, Title: "draft",
		Status: intenttree.StatusDraft, Doc: "## 问题\n\n正文。\n"}
	front = derivedFrontmatter("m-f3", "v0.1.0", "f3", draftDoc)
	if !strings.Contains(front, "status: draft") || !strings.Contains(front, "package_maturity: complete") {
		t.Fatalf("draft node with real prose = draft/complete:\n%s", front)
	}
}
