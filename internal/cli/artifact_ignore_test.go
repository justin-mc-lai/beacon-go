package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifactsAreIgnoredOnInitAndRender(t *testing.T) {
	proj := treeTestEnv(t)
	mustRun(t, "plan", "init", "--project", proj, "--title", "demo", "--intent", "demo", "--json")
	rootIgnore, err := os.ReadFile(filepath.Join(proj, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rootIgnore), ".beacon/") {
		t.Fatalf("root .gitignore missing artifacts rule: %s", rootIgnore)
	}
	if _, err := os.Stat(filepath.Join(proj, ".beacon", "artifacts", ".gitignore")); err != nil {
		t.Fatalf("nested artifacts .gitignore missing: %v", err)
	}
	out := mustRun(t, "truth", "tree", "--project", proj, "--format", "html")
	if !strings.Contains(out, "派生产物") {
		t.Fatalf("render hint missing: %s", out)
	}
	outside := filepath.Join(t.TempDir(), "outside.html")
	out = mustRun(t, "truth", "tree", "--project", proj, "--format", "html", "--out", outside)
	if !strings.Contains(out, "项目外") {
		t.Fatalf("outside --out hint missing: %s", out)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("explicit output not written: %v", err)
	}
}

func TestPlanInitLocalOnly(t *testing.T) {
	proj := treeTestEnv(t)
	// A pre-existing tracked SDD doc must be reported, not silently untracked.
	if err := os.MkdirAll(filepath.Join(proj, "docs", "beacon", "existing"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "docs", "beacon", "existing", "truth.md"), []byte("# existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", proj, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(proj, ".gitignore"), []byte("node_modules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", proj, "add", ".gitignore", "docs/beacon/existing/truth.md").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v %s", err, out)
	}
	out := mustRun(t, "plan", "init", "--project", proj, "--title", "demo", "--intent", "demo", "--local-only", "--json")
	doc := parseDoc(t, out)
	if doc["local_only"] != true {
		t.Fatalf("local_only: %v", doc)
	}
	if got := doc["tracked_docs"].([]any); len(got) == 0 {
		t.Fatalf("tracked docs not reported: %v", doc)
	}
	if !strings.Contains(doc["untrack_command"].(string), "git rm -r --cached docs/beacon") {
		t.Fatalf("untrack command: %v", doc["untrack_command"])
	}
	excludePath := filepath.Join(proj, ".git", "info", "exclude")
	exclude, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(exclude), "docs/beacon/") || !strings.Contains(string(exclude), ".beacon/") {
		t.Fatalf("local exclude missing rules: %s", exclude)
	}
	if ignore, _ := os.ReadFile(filepath.Join(proj, ".gitignore")); string(ignore) != "node_modules\n" {
		t.Fatalf("local-only must not modify tracked .gitignore: %s", ignore)
	}
	cfg, err := os.ReadFile(filepath.Join(proj, ".beacon", "config"))
	if err != nil || !strings.Contains(string(cfg), "local_only = true") {
		t.Fatalf("local_only config missing: %v %s", err, cfg)
	}
	mustRun(t, "truth", "tree", "--project", proj, "--format", "html")
	exclude, _ = os.ReadFile(excludePath)
	if !strings.Contains(string(exclude), "docs/beacon/") {
		t.Fatalf("docs exclude removed after render: %s", exclude)
	}
}
