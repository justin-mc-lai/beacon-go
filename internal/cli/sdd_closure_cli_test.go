package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSDDClosureCLIDecompose(t *testing.T) {
	t.Setenv("BEACON_STORE_PATH", filepath.Join(t.TempDir(), "store.db"))
	root := t.TempDir()
	run := func(args ...string) (string, error) { return runCLI(t, args...) }
	if out, err := run("projects", "register", "--path", root, "--version", "v0.0.1", "--json"); err != nil {
		t.Fatalf("register: %s %v", out, err)
	}
	if out, err := run("truth", "create", "--project", root, "--slug", "format", "--sdd", "--json"); err != nil {
		t.Fatalf("create: %s %v", out, err)
	}
	out, err := run("truth", "decompose", "--project", root, "--feature", "format", "--utterance", "报价保留两位小数", "--json")
	if err != nil {
		t.Fatalf("decompose: %s %v", out, err)
	}
	if !strings.Contains(out, `"semantic_ready": false`) || !strings.Contains(out, "deterministic_non_llm") {
		t.Fatalf("missing honest method status: %s", out)
	}
	truth := filepath.Join(root, "docs/beacon/v0.0.1/features/format/truth.md")
	b, err := os.ReadFile(truth)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "## Acceptance Criteria") || !strings.Contains(string(b), "## Decomposition Suggestions") {
		t.Fatal("truth overwritten by test document")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(truth), "tests.md")); !os.IsNotExist(err) {
		t.Fatal("legacy tests file created")
	}
	if _, err := run("truth", "freeze", "--project", root, "--feature", "format", "--by", "owner", "--json"); err == nil {
		t.Fatal("unreviewed candidate froze")
	}
}

func TestSDDDecomposeHelpHonest(t *testing.T) {
	out, err := runCLI(t, "truth", "decompose", "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "non-LLM") || !strings.Contains(out, "candidate suggestions") {
		t.Fatalf("help misrepresents decomposition: %s", out)
	}
}
