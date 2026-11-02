package gate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func taskBindings(t *testing.T, tasks string) []Finding {
	t.Helper()
	return ValidateFeaturePackage(PackageFiles{
		Slug:  "demo",
		Tasks: PackageFile{Path: "tasks.md", Present: true, Text: tasks},
	}, nil, nil)
}

func findKind(fs []Finding, kind, substr string) bool {
	for _, f := range fs {
		if strings.Contains(f.Detail, kind) && strings.Contains(f.Detail, substr) {
			return true
		}
	}
	return false
}

func TestValidateTaskBindingMissingAC(t *testing.T) {
	fs := taskBindings(t, "# Tasks\n\n- [ ] TASK-001 has no binding\n- [ ] TASK-002 bound · ac=AC-001\n")
	if !findKind(fs, "task_binding_missing", "TASK-001 missing ac=") {
		t.Fatalf("want TASK-001 task_binding_missing, got %+v", fs)
	}
	if findKind(fs, "task_binding_missing", "TASK-002") {
		t.Fatalf("bound row must not be flagged: %+v", fs)
	}
}

func TestValidateTaskBindingMissingEvidence(t *testing.T) {
	fs := taskBindings(t, "# Tasks\n\n- [x] TASK-001 checked · ac=AC-001\n- [ ] TASK-002 unchecked · ac=AC-002\n")
	if !findKind(fs, "task_binding_missing", "TASK-001 missing evidence=") {
		t.Fatalf("want checked row evidence finding, got %+v", fs)
	}
	if findKind(fs, "task_binding_missing", "TASK-002") {
		t.Fatalf("unchecked row must not demand evidence=: %+v", fs)
	}
}

func TestValidateTaskBindingEvidenceFile(t *testing.T) {
	const tasks = "# Tasks\n\n- [x] TASK-001 checked · ac=AC-001 · evidence=`.beacon/evidence/AC-001.json`\n" +
		"- [x] TASK-002 checked · ac=AC-002 · evidence=TestSomething\n"
	exists := map[string]bool{"/proj/.beacon/evidence/AC-001.json": false}
	fs := ValidateFeaturePackage(PackageFiles{
		Slug:  "demo",
		Tasks: PackageFile{Path: "tasks.md", Present: true, Text: tasks},
	}, func(ref string) string { return "/proj/" + ref }, func(abs string) bool { return exists[abs] })
	if !findKind(fs, "checkbox_checked_without_evidence_file", "AC-001.json") {
		t.Fatalf("want missing evidence file finding, got %+v", fs)
	}
	if findKind(fs, "checkbox_checked_without_evidence_file", "TestSomething") {
		t.Fatalf("identifier-shaped evidence is a declaration, not a file: %+v", fs)
	}
	// A non-trivial file clears the finding.
	exists["/proj/.beacon/evidence/AC-001.json"] = true
	fs = ValidateFeaturePackage(PackageFiles{
		Slug:  "demo",
		Tasks: PackageFile{Path: "tasks.md", Present: true, Text: tasks},
	}, func(ref string) string { return "/proj/" + ref }, func(abs string) bool { return exists[abs] })
	if findKind(fs, "checkbox_checked_without_evidence_file", "AC-001.json") {
		t.Fatalf("existing evidence must pass: %+v", fs)
	}
}

func TestValidateTaskBindingUncheckedNoEvidenceOK(t *testing.T) {
	fs := taskBindings(t, "# Tasks\n\n- [ ] TASK-001 pending · ac=AC-001\n")
	if len(fs) != 0 {
		t.Fatalf("unchecked row without evidence must stay clean, got %+v", fs)
	}
}

func TestValidateTaskBindingKeepsExistingVerdicts(t *testing.T) {
	fs := ValidateFeaturePackage(PackageFiles{
		Slug:  "demo",
		Tasks: PackageFile{Path: "tasks.md", Present: true, Text: "- [x] TASK-001 ok · ac=AC-001 · evidence=TestA\n"},
	}, nil, nil)
	if len(fs) != 0 {
		t.Fatalf("fully bound ledger must not add findings, got %+v", fs)
	}
}

func TestHostPackagesTaskBindingsClean(t *testing.T) {
	for _, slug := range []string{
		"agent-desktop-workspace", "agent-runtime-dual-mode",
		"agent-session-streaming", "p0-executor-desktop",
	} {
		data, err := os.ReadFile(filepath.Join("..", "..", "docs", "beacon", "v2.0.0", "features", slug, "tasks.md"))
		if err != nil {
			t.Fatalf("%s: %v", slug, err)
		}
		fs := ValidateFeaturePackage(PackageFiles{
			Slug:  slug,
			Tasks: PackageFile{Path: "tasks.md", Present: true, Text: string(data)},
		}, nil, nil)
		for _, f := range fs {
			if strings.Contains(f.Detail, "task_binding_missing") || strings.Contains(f.Detail, "checkbox_checked_without_evidence_file") {
				t.Errorf("%s: %s", slug, f.Detail)
			}
		}
	}
}
