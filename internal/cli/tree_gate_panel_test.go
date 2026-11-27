package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/store"
)

func TestTreeHTMLRendersGateVerdicts(t *testing.T) {
	proj := treeTestEnv(t)
	mustRun(t, "truth", "create", "--project", proj, "--slug", "demo", "--json")
	mustRun(t, "migrate", "tree", "--project", proj, "--json")

	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := st.ResolveFeature(proj, "demo", "")
	if err != nil {
		st.Close()
		t.Fatal(err)
	}
	if _, err := st.Gates().Record(store.GateResult{
		FeatureID: ctx.Feature.ID, GateType: "qa", Verdict: "blocked",
		ReasonCodes: []string{"tree_gate_panel_test"}, EvaluatedAt: "2026-11-27T00:00:00Z",
	}); err != nil {
		st.Close()
		t.Fatal(err)
	}
	st.Close()

	mustRun(t, "plan", "tree", "--project", proj, "--format", "html")
	htmlPath := filepath.Join(proj, ".beacon", "artifacts", "tree", "v0.1.0", "index.html")
	raw, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"gate-panel", "qa", "blocked", "tree_gate_panel_test"} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("rendered html missing %q", want)
		}
	}
}
