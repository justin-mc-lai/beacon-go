package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/store"
)

func TestArtifactCLIExplicitNodeScope(t *testing.T) {
	base := t.TempDir()
	t.Setenv(store.EnvStorePath, filepath.Join(base, "store.db"))
	proof := filepath.Join(base, "proof.txt")
	if err := os.WriteFile(proof, []byte("evidence"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := mustRun(t, "goal", "artifacts", "register", "--scope-kind", "node", "--scope-id", "n1",
		"--kind", "evidence", "--path", proof, "--by", "tester", "--json")
	if parseDoc(t, out)["already_registered"] != false {
		t.Fatalf("first register: %s", out)
	}
	out = mustRun(t, "goal", "artifacts", "list", "--scope-kind", "node", "--scope-id", "n1", "--json")
	if got := len(parseDoc(t, out)["artifact_refs"].([]any)); got != 1 {
		t.Fatalf("node scope artifacts = %d, want 1", got)
	}
	out = mustRun(t, "goal", "artifacts", "register", "--scope-kind", "node", "--scope-id", "n1",
		"--kind", "evidence", "--path", proof, "--by", "tester", "--json")
	if parseDoc(t, out)["already_registered"] != true {
		t.Fatalf("second register must be idempotent: %s", out)
	}
	if _, err := runCLI(t, "goal", "artifacts", "register", "--scope-kind", "team", "--scope-id", "n1",
		"--kind", "evidence", "--path", proof, "--by", "tester", "--json"); err == nil || !strings.Contains(err.Error(), "artifact_scope_kind_unknown") {
		t.Fatalf("unknown scope kind: %v", err)
	}
	if _, err := runCLI(t, "goal", "artifacts", "list", "--scope-kind", "node", "--scope-id", "n1",
		"--run-id", "run_1", "--json"); err == nil || !strings.Contains(err.Error(), "artifact_scope_required") {
		t.Fatalf("two scopes must fail: %v", err)
	}
}
