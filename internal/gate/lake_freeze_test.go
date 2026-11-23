// kernel-lake-freeze-closure AC-001: the contract face (truth, tests,
// tasks, restatement) is what a freeze governs; evidence.md is the
// byproduct of implementation and appending to it must not drift the
// frozen surface. The same test file proves the other direction: any
// contract-face edit still drifts (AC-004 keeps the old strictness).
package gate

import (
	"testing"

	"beacon-go/internal/core"
)

func lakeSurfaceFixture() (truth, tests, tasks, restatement, evidence []byte) {
	truth = []byte("---\nslug: feat-a\nstatus: frozen\n---\n- **AC-001**：the one requirement\n")
	tests = []byte("| AC ID | TC ID |\n|---|---|\n| AC-001 | TC-001 |\n")
	tasks = []byte("---\nslug: feat-a\n---\n- [ ] the work · ac=AC-001\n")
	restatement = []byte("---\nslug: feat-a\nconfirmed_by: lai\nhash: abc\n---\n## 1 交付形态\n")
	evidence = []byte("# Evidence\n\n| Surface | Artifact |\n|---|---|\n| qa | artifacts/qa.md |\n")
	return
}

func TestEvidenceAppendDoesNotDriftPackageSurface(t *testing.T) {
	truth, tests, tasks, restatement, evidence := lakeSurfaceFixture()
	frozen := core.PackageContractFingerprint(truth, tests, tasks, restatement)
	// The freeze-time surface is fresh against the same contract face.
	if core.PackageSurfaceDrifted(frozen, truth, tests, tasks, evidence, restatement) {
		t.Fatal("a fresh freeze must not report drift")
	}
	// Appending implementation evidence is the byproduct path (issue #10
	// R2) — the frozen surface must survive it.
	appended := append(append([]byte{}, evidence...), []byte("\n- test log: all green\n")...)
	if core.PackageSurfaceDrifted(frozen, truth, tests, tasks, appended, restatement) {
		t.Fatal("evidence-only append drifted the frozen package surface")
	}
	// Rewriting the evidence body is still evidence-only.
	rewritten := []byte("# Evidence\n\n| Surface | Artifact |\n|---|---|\n| qa | artifacts/qa-v2.md |\n")
	if core.PackageSurfaceDrifted(frozen, truth, tests, tasks, rewritten, restatement) {
		t.Fatal("evidence rewrite drifted the frozen package surface")
	}
	if core.ContractSurfaceFingerprintAlg != "pkgcontract-v1" {
		t.Fatalf("contract alg changed: %s", core.ContractSurfaceFingerprintAlg)
	}
}

func TestContractEditStillDriftsPackageSurface(t *testing.T) {
	truth, tests, tasks, restatement, evidence := lakeSurfaceFixture()
	frozen := core.PackageContractFingerprint(truth, tests, tasks, restatement)
	cases := []struct {
		name                      string
		truth, tests, tasks, rest []byte
	}{
		{"truth", append(append([]byte{}, truth...), []byte("- **AC-002**：added\n")...), tests, tasks, restatement},
		{"tests", truth, append(append([]byte{}, tests...), []byte("| AC-002 | TC-002 |\n")...), tasks, restatement},
		{"tasks", truth, tests, append(append([]byte{}, tasks...), []byte("- [ ] more · ac=AC-002\n")...), restatement},
		{"restatement", truth, tests, tasks, append(append([]byte{}, restatement...), []byte("## 2 界面与交互\n")...)},
	}
	for _, c := range cases {
		if !core.PackageSurfaceDrifted(frozen, c.truth, c.tests, c.tasks, evidence, c.rest) {
			t.Fatalf("contract-face edit to %s must still drift", c.name)
		}
	}
}
