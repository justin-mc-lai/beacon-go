package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
)

// init must not demand --version: it inherits the newest docs version on
// disk, then falls back to the default for a fresh project.
func TestDetectDocsVersion(t *testing.T) {
	root := t.TempDir()
	if got := detectDocsVersion(root); got != DefaultDocsVersion {
		t.Fatalf("empty project = %s, want %s", got, DefaultDocsVersion)
	}
	for _, v := range []string{"v1.6.19", "v2.0.0", "v1.7.0", "archive", "vX"} {
		if err := os.MkdirAll(filepath.Join(root, "docs", "beacon", v), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got := detectDocsVersion(root); got != "v2.0.0" {
		t.Fatalf("newest = %s, want v2.0.0 (numeric, not string order)", got)
	}
}

func TestVersionLessNumeric(t *testing.T) {
	if !versionLess("v1.6.19", "v2.0.0") {
		t.Fatal("v1.6.19 < v2.0.0")
	}
	if versionLess("v2.0.0", "v1.9.9") {
		t.Fatal("v2.0.0 is not less than v1.9.9")
	}
	if !versionLess("v1.9.9", "v1.10.0") {
		t.Fatal("v1.9.9 < v1.10.0 (numeric, not lexicographic)")
	}
	if versionLess("v0.1", "v0.1") {
		t.Fatal("equal versions are not less")
	}
}
