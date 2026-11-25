package cli

import (
	"os"
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
