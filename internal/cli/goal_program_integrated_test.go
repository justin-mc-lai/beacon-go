// The integrated-delivery surface must have a real CLI entry under
// `goal program` (truth.md surfaces: [cli], surface_strict: true).
package cli

import (
	"strings"
	"testing"
)

func TestGoalProgramIntegratedCLIWiring(t *testing.T) {
	out, err := runCLI(t, "goal", "program", "integrated", "--help")
	if err != nil {
		t.Fatalf("goal program integrated --help: %v", err)
	}
	for _, sub := range []string{"enable", "handoff", "dispatch", "merge", "route", "qa", "approve", "finalize"} {
		if !strings.Contains(out, sub) {
			t.Fatalf("integrated help missing %q:\n%s", sub, out)
		}
	}

	// A missing run is a real engine call, not an unknown command:
	// this proves the subcommand reaches ProgramIntegratedRoute.
	_, err = runCLI(t, "goal", "program", "integrated", "route", "--run-id", "no-such-run")
	if err == nil || strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("integrated route not wired: %v", err)
	}

	// Legacy Program is unchanged by the new entry: the old route
	// command still owns the same missing-run error.
	_, legacyErr := runCLI(t, "goal", "program", "route", "--run-id", "no-such-run")
	if legacyErr == nil {
		t.Fatal("legacy program route unexpectedly succeeded")
	}
}
