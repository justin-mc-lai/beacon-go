// --strict on `gate integrity` (AC-001/AC-003): a frozen package
// keeps AC-101's audit-only default; an explicit --strict turns the
// same package into a blocking check so a release candidate cannot
// pass with unbound task rows.
package cli

import (
	"testing"

	"beacon-go/internal/core"
)

func TestGateIntegrityStrictBlocksFrozenPackage(t *testing.T) {
	root, _ := seedGateProject(t, true) // frozen, tasks.md row has no ac=
	out, err := runCLI(t, "gate", "integrity", "--project", root, "--feature", "feat-gate",
		"--evaluator", "probe", "--strict", "--json")
	if err == nil {
		t.Fatalf("--strict must block a frozen package with an unbound task row\n%s", out)
	}
	doc := parseDoc(t, out)
	if strField(t, doc, "verdict") != "blocked" || strField(t, doc, "mode") != "block" {
		t.Fatalf("want blocked/block, got %v", doc)
	}
	if rc, ok := core.ReasonCodeOf(err); !ok || rc != core.ReasonIntegrityPackageValid {
		t.Fatalf("headline code must stay %s, got %v", core.ReasonIntegrityPackageValid, err)
	}
}

func TestGateIntegrityFrozenDefaultStaysAudit(t *testing.T) {
	root, _ := seedGateProject(t, true)
	out, err := runCLI(t, "gate", "integrity", "--project", root, "--feature", "feat-gate",
		"--evaluator", "probe", "--json")
	if err != nil {
		t.Fatalf("default must audit, not block (AC-101): %v\n%s", err, out)
	}
	doc := parseDoc(t, out)
	if strField(t, doc, "verdict") != "audit" || strField(t, doc, "mode") != "audit_only" {
		t.Fatalf("want audit/audit_only, got %v", doc)
	}
	findings, _ := doc["findings"].([]any)
	found := false
	for _, x := range findings {
		if m, ok := x.(map[string]any); ok && m["kind"] == "feature_package_validation" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the audit verdict must still itemise feature_package_validation, got %v", findings)
	}
}
