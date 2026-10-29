package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// shell returns the embedded desktop shell. The app embeds the
// frontend directory, so the file lives at frontend/index.html on disk
// and at index.html once fs.Sub is applied.
func shell(t *testing.T) string {
	t.Helper()
	b, err := shellFS.ReadFile("frontend/index.html")
	if err != nil {
		t.Fatalf("embedded shell: %v", err)
	}
	return string(b)
}

func TestShellIsRealNotFabricated(t *testing.T) {
	body := shell(t)
	if strings.Contains(body, "sha256-verified-") {
		t.Error("the shell must not print a hard-coded evidence fingerprint")
	}
	if strings.Contains(body, "机器自动化验收通过") {
		t.Error("the shell must not claim QA passed before a gate ran")
	}
	for _, want := range []string{"inspector-pane", "workspace-main", "nav-pane"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
	if !strings.Contains(body, `invoke("StartGoal"`) {
		t.Error("goal creation must go through the real backend")
	}
}

// TestShellScriptParses catches the class of defect that shipped on
// 2026-10-28: a stray block left the whole script unparseable, so the
// window rendered but nothing worked.
func TestShellScriptParses(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not on PATH; skipping JS syntax check")
	}
	body := shell(t)
	start := strings.Index(body, "<script>")
	end := strings.LastIndex(body, "</script>")
	if start < 0 || end < 0 || end <= start {
		t.Fatal("no inline script block found")
	}
	js := body[start+len("<script>") : end]
	path := filepath.Join(t.TempDir(), "shell.js")
	if err := os.WriteFile(path, []byte(js), 0o644); err != nil {
		t.Fatalf("write js: %v", err)
	}
	if out, err := exec.Command(node, "--check", path).CombinedOutput(); err != nil {
		t.Fatalf("inline script does not parse:\n%s", out)
	}
}

// TestSingleSurface enforces the desktop-only decision: no second
// shell, no web console package, and no browser fallback path left in
// the frontend.
func TestSingleSurface(t *testing.T) {
	body := shell(t)
	for _, banned := range []string{"/api/invoke", "EventSource(", "/api/events"} {
		if strings.Contains(body, banned) {
			t.Errorf("browser fallback %q must not exist in a desktop-only shell", banned)
		}
	}
	for _, stale := range []string{
		filepath.Join("..", "internal", "ui"),
		filepath.Join("..", "internal", "webui"),
	} {
		if _, err := os.Stat(stale); err == nil {
			t.Errorf("web surface still exists: %s", stale)
		}
	}
}
