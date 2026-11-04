// Release-candidate integrity (AC-002/AC-004): AC-101's audit-only
// default applies to historical frozen packages, not to a package
// that already has a run. The version release check is the reader
// that must make that distinction.
package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/gate"
)

func seedReleasePackage(t *testing.T, env testEnv, bound bool) {
	t.Helper()
	if err := os.MkdirAll(env.root, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(env.root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("truth.md", "---\nslug: feat-a\nstatus: frozen\n---\n# Truth\n\n- **AC-001**：the one requirement\n")
	write("tests.md", "---\nslug: feat-a\n---\n| AC ID | TC ID | Layer | Command | Assertion |\n|-------|-------|-------|---------|-----------|\n| AC-001 | TC-001 | unit | `go test ./...` | passes |\n")
	tasks := "---\nslug: feat-a\n---\n- [ ] the work\n"
	if bound {
		tasks = "---\nslug: feat-a\n---\n- [ ] the work · ac=AC-001 · evidence=evidence.md\n"
	}
	write("tasks.md", tasks)
	write("evidence.md", "# Evidence\n\n## Evidence Index\n\n| Surface | Authority | Canonical Artifact | Status | Route |\n|---------|-----------|--------------------|--------|-------|\n| truth | requirement_truth | truth.md | frozen | truth |\n| tests | test_truth | tests.md | frozen | qa |\n")
}

func integrityItem(res gate.VersionReleaseResult) gate.VersionCheckItem {
	for _, c := range res.Checks {
		if c.Name == gate.VersionCheckIntegrity {
			return c
		}
	}
	return gate.VersionCheckItem{}
}

func TestReleaseCandidateIntegrityBlocks(t *testing.T) {
	env := newTestEnv(t, "frozen")
	seedReleasePackage(t, env, false) // unbound task row
	env.createRun(t)                  // a run makes this a release candidate

	res, err := env.eng.ReleaseVersionCheck(env.root, "v2.0.0", "full")
	if err != nil {
		t.Fatal(err)
	}
	it := integrityItem(res)
	if it.Status != "fail" {
		t.Fatalf("release candidate integrity must fail, got %+v", it)
	}
	if len(it.Features) != 1 || it.Features[0] != "feat-a" {
		t.Fatalf("failing feature must be named, got %+v", it)
	}
}

func TestHistoricalFrozenIntegrityAudits(t *testing.T) {
	env := newTestEnv(t, "frozen")
	seedReleasePackage(t, env, false) // no run: historical package

	res, err := env.eng.ReleaseVersionCheck(env.root, "v2.0.0", "full")
	if err != nil {
		t.Fatal(err)
	}
	it := integrityItem(res)
	if it.Status == "fail" {
		t.Fatalf("historical frozen package must stay audit-only (AC-101), got %+v", it)
	}
}
