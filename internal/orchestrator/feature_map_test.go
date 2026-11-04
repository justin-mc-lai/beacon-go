// feature map (tweet 1's "Feature Map + CLI"): every AC gets an
// entry point from the task ledger and a runnable command from the
// tests table. The map is a one-way projection; truth is never
// rewritten, and a stale map is detected by `truth map verify`.
package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedFeatureMapPackage(t *testing.T, env testEnv) {
	t.Helper()
	if err := os.MkdirAll(env.root, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(env.root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("truth.md", "---\nslug: feat-a\nstatus: frozen\n---\n# Truth\n\n- **AC-001**：first requirement\n- **AC-002**：second requirement\n- **AC-003**：third requirement\n")
	write("tests.md", "---\nslug: feat-a\n---\n| AC ID | TC ID | Layer | Command | Assertion |\n|-------|-------|-------|---------|-----------|\n| AC-001 | TC-001 | behavior | `go test ./internal/gate -run TestA` | passes |\n")
	write("tasks.md", "---\nslug: feat-a\n---\n- [x] TASK-001 first work · ac=AC-001 · evidence=internal/gate/pkgvalidate.go\n- [x] TASK-002 second work · ac=AC-002 · evidence=internal/cli/gate.go\n")
	write("evidence.md", "# Evidence\n\n## Evidence Index\n\n| Surface | Authority | Canonical Artifact | Status | Route |\n|---------|-----------|--------------------|--------|-------|\n| truth | requirement_truth | truth.md | frozen | truth |\n| tests | test_truth | tests.md | frozen | qa |\n")
}

func TestBuildFeatureMapBindsACs(t *testing.T) {
	env := newTestEnv(t, "frozen")
	seedFeatureMapPackage(t, env)
	truthBefore, err := os.ReadFile(filepath.Join(env.root, "truth.md"))
	if err != nil {
		t.Fatal(err)
	}

	v, err := env.eng.BuildFeatureMap(env.root, "feat-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Rows) != 3 {
		t.Fatalf("rows = %+v", v.Rows)
	}
	if v.Rows[0].AC != "AC-001" || v.Rows[0].Status != "mapped" ||
		v.Rows[0].EntryPoint != "internal/gate/pkgvalidate.go" ||
		!strings.Contains(v.Rows[0].Verification, "go test") {
		t.Fatalf("AC-001 row = %+v", v.Rows[0])
	}
	if v.Rows[1].Status != "missing_verification" {
		t.Fatalf("AC-002 row = %+v", v.Rows[1])
	}
	if v.Rows[2].Status != "missing_entry" {
		t.Fatalf("AC-003 row = %+v", v.Rows[2])
	}
	if v.Mapped != 1 || v.Status != "incomplete" {
		t.Fatalf("summary = %+v", v)
	}
	if _, err := os.Stat(v.Path); err != nil {
		t.Fatalf("feature-map.md not written: %v", err)
	}
	// One-way projection: truth is byte-for-byte unchanged.
	truthAfter, err := os.ReadFile(filepath.Join(env.root, "truth.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(truthBefore) != string(truthAfter) {
		t.Fatal("BuildFeatureMap rewrote truth.md")
	}

	// Fresh map passes the verify leg; a later package edit makes it stale.
	verified, err := env.eng.TruthMapVerify(env.root, "feat-a")
	if err != nil {
		t.Fatal(err)
	}
	if verified.FeatureMapStatus != "fresh" {
		t.Fatalf("fresh map = %+v", verified.FeatureMapStatus)
	}
	appendFile := func(name, text string) {
		f, err := os.OpenFile(filepath.Join(env.root, name), os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if _, err := f.WriteString(text); err != nil {
			t.Fatal(err)
		}
	}
	appendFile("tests.md", "| AC-002 | TC-002 | behavior | `go test ./internal/cli -run TestB` | passes |\n")
	stale, err := env.eng.TruthMapVerify(env.root, "feat-a")
	if err != nil {
		t.Fatal(err)
	}
	if stale.FeatureMapStatus != "stale" || stale.Status != "blocked" {
		t.Fatalf("stale map = %+v / %s", stale.FeatureMapStatus, stale.Status)
	}
}
