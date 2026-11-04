package cli

import (
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/skills"
)

func TestSkillsVerifyReportOnlyAndStrict(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(filepath.Join(dir, "beacon-go"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Report-only: a partial install exits 0.
	out, err := runCLI(t, "skills", "verify", "--home", home, "--json")
	if err != nil {
		t.Fatalf("report-only must exit 0: %v\n%s", err, out)
	}
	doc := parseDoc(t, out)
	hosts, _ := doc["hosts"].([]any)
	if len(hosts) == 0 {
		t.Fatalf("no hosts in report: %v", doc)
	}
	// Strict: the same partial install fails.
	if _, err := runCLI(t, "skills", "verify", "--home", home, "--strict"); err == nil {
		t.Fatal("strict must fail on drift")
	} else if rc, _ := core.ReasonCodeOf(err); rc != "skill_install_drift" {
		t.Fatalf("reason = %q", rc)
	}
	// A complete install passes strict.
	for _, name := range skills.ExpectedInstallNames() {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := runCLI(t, "skills", "verify", "--home", home, "--strict"); err != nil {
		t.Fatalf("complete install must pass strict: %v", err)
	}
}
