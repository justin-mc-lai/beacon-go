package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/skills"
)

// The install face must land the Scheme A set in the host directory the
// operator actually runs. workbuddy was missing from the host list, so
// `beacon skills install` wrote to ~/.agents/skills and the local skill
// picker stayed empty.
func TestSkillsInstallIntoWorkBuddyDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skills")
	out, err := runCLI(t, "skills", "install", "--host", "workbuddy", "--dir", dir)
	if err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	if !strings.Contains(out, "installed 7 skills") {
		t.Fatalf("unexpected output:\n%s", out)
	}
	for _, id := range skills.IDs() {
		for _, rel := range []string{"SKILL.md", "references/core.md", "evals/evals.json"} {
			p := filepath.Join(dir, "beacon-"+id, filepath.FromSlash(rel))
			if _, err := os.Stat(p); err != nil {
				t.Fatalf("missing %s: %v", p, err)
			}
		}
		if n := len(skills.Examples(id)); n < 3 {
			t.Fatalf("skill %s ships %d examples, want >= 3", id, n)
		}
	}
}

func TestSkillsHostsListsWorkBuddy(t *testing.T) {
	out, err := runCLI(t, "skills", "hosts")
	if err != nil {
		t.Fatalf("hosts: %v", err)
	}
	if !strings.Contains(out, "workbuddy") {
		t.Fatalf("workbuddy host not listed:\n%s", out)
	}
	if !strings.Contains(out, filepath.Join(".workbuddy-ai", "skills")) {
		t.Fatalf("workbuddy dir not shown:\n%s", out)
	}
}

func TestSkillsInstallRejectsUnknownHost(t *testing.T) {
	out, err := runCLI(t, "skills", "install", "--host", "not-a-host")
	if err == nil {
		t.Fatalf("unknown host must fail, got:\n%s", out)
	}
	if !strings.Contains(err.Error(), "unknown host") {
		t.Fatalf("unexpected error: %v", err)
	}
}
