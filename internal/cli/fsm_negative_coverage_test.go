package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFSMNegativeCoverageIntegrityCLI(t *testing.T) {
	for _, sdd := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "sdd"}[sdd], func(t *testing.T) {
			root, _ := seedGateProject(t, false)
			path := filepath.Join(root, "truth.md")
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			truth := string(original) + "\n## Domain Model\n| Entity | Key | Precedence |\n|---|---|---|\n| Order | id | root |\n## Domain FSM - Order\n| To | From | Guard |\n|---|---|---|\n| paid | pending | confirmed |\n## Illegal Transitions\n| From | To | Aggregate |\n|---|---|---|\n| paid | pending | Order |\n## Acceptance Criteria\n| AC ID | Criterion |\n|---|---|\n| AC-001 | Order rejects rollback |\n"
			if sdd {
				truth = strings.Replace(truth, "status: draft", "status: draft\npackage_contract: truth-tasks", 1)
				truth += "\n## Coverage Matrix\n| AC ID | TC ID | Layer | Command | Assertion |\n|---|---|---|---|---|\n| AC-001 | TC-010 | behavior | go test ./x | rollback refused |\n"
			}
			truth = strings.ReplaceAll(truth, "pending", "open")
			truth = strings.Replace(truth, "status: draft", "status: draft\nversion: v2.0.0\nlanguage: zh\nparser_contract: beacon-feature-package-v2\ncanonical_refs:\n  truth: truth.md\n  tests: tests.md\n  tasks: tasks.md\n  evidence: evidence.md", 1)
			for name, text := range map[string]string{
				"tasks.md":    "---\nslug: feat-gate\n---\n- [ ] work ac=AC-001 evidence=evidence.md\n",
				"evidence.md": "# Evidence\n| Surface | Authority | Artifact |\n|---|---|---|\n| notes | support_advisory | none |\n",
			} {
				if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if sdd {
				tasks := "| TASK | Action | Depends | AC | Command | Verify | Evidence | Auto |\n|---|---|---|---|---|---|---|---|\n| TASK-001 | Implement rollback refusal | - | AC-001 | go test ./x | go test ./x | evidence.md | false |\n"
				if err := os.WriteFile(filepath.Join(root, "tasks.md"), []byte(tasks), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(path, []byte(truth), 0o644); err != nil {
				t.Fatal(err)
			}
			args := []string{"gate", "integrity", "--project", root, "--feature", "feat-gate", "--evaluator", "negative-coverage-checker", "--json"}
			out, err := runCLI(t, args...)
			if err == nil || !strings.Contains(out, `"reason_code": "uncovered"`) || !strings.Contains(out, "from=paid to=open") {
				t.Fatalf("missing slot must refuse with scoped uncovered: %v\n%s", err, out)
			}
			t.Logf("missing slot rejected: %v", err)
			if parseDoc(t, out)["reason_code"] != "uncovered" {
				t.Fatalf("top-level AC reason_code missing: %s", out)
			}
			coverage := "\n| AC ID | TC ID | Layer | Command | Assertion | Aggregate | From | To |\n|---|---|---|---|---|---|---|---|\n| AC-001 | TC-001 | negative | go test ./x | rejected | Order | paid | pending |\n"
			coverage = strings.ReplaceAll(coverage, "pending", "open")
			if sdd {
				truth += coverage
				if err := os.WriteFile(path, []byte(truth), 0o644); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(filepath.Join(root, "tests.md"), []byte("---\nslug: feat-gate\n---\n"+coverage), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			out, err = runCLI(t, args...)
			if strings.Contains(out, `"reason_code": "uncovered"`) {
				t.Fatalf("covered slot must clear uncovered: %v\n%s", err, out)
			}
			if err != nil || parseDoc(t, out)["verdict"] != "pass" {
				t.Fatalf("complete input must pass: %v\n%s", err, out)
			}
			t.Logf("covered slot integrity: %s", out)
		})
	}
}
