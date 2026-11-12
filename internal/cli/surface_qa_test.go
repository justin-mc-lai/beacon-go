// qa-surface-e2e: the legacy notice in \`gate status\`.
package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGateStatusWarnsSurfaceUndeclared(t *testing.T) {
	root, _ := seedGateProject(t, true)
	out, err := runCLI(t, "gate", "status", "--project", root, "--feature", "feat-gate", "--json")
	if err != nil {
		t.Fatalf("status: %v\n%s", err, out)
	}
	doc := parseDoc(t, out)
	warnings, _ := doc["warnings"].([]any)
	if len(warnings) != 1 || warnings[0] != "surface_undeclared" {
		t.Fatalf("legacy package warnings = %v, want [surface_undeclared]", warnings)
	}
	gates, _ := doc["gates"].([]any)
	for _, raw := range gates {
		row, _ := raw.(map[string]any)
		if row == nil || row["gate_type"] != "qa" {
			continue
		}
		rowWarnings, _ := row["warnings"].([]any)
		if len(rowWarnings) != 1 || rowWarnings[0] != "surface_undeclared" {
			t.Fatalf("qa row warnings = %v", rowWarnings)
		}
	}
}

func TestGateStatusQuietWhenSurfacesDeclared(t *testing.T) {
	root, _ := seedGateProject(t, true)
	truthPath := filepath.Join(root, "truth.md")
	raw, err := os.ReadFile(truthPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := "---\nslug: feat-gate\nstatus: frozen\nsurfaces: [api]\n---\n# Truth\n\n- **AC-001**: the one requirement\n"
	if err := os.WriteFile(truthPath, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, "gate", "status", "--project", root, "--feature", "feat-gate", "--json")
	if err != nil {
		t.Fatalf("status: %v\n%s", err, out)
	}
	doc := parseDoc(t, out)
	if warnings, ok := doc["warnings"]; ok && warnings != nil {
		t.Fatalf("declared package must not warn, got %v (was %q)", warnings, string(raw))
	}
}
