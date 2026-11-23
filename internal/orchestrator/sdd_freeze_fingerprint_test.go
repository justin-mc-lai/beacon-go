package orchestrator

import (
	"strings"
	"testing"
)

func TestSDDDecomposeFreezeNormalizesAliasedMatrixFingerprint(t *testing.T) {
	env := newTruthEnv(t)
	if _, err := env.eng.CreateTruthPackageSDD(env.root, "", "normalized"); err != nil {
		t.Fatal(err)
	}
	fc := env.ctx(t, "normalized")
	// Exercise the actual freeze write with a prevalidated input; this regression
	// targets its stored before/after fingerprint, not semantic review sufficiency.
	if _, err := env.eng.applyFreeze(fc, EventTruthFrozen, map[string]any{"by": "test"}); err != nil {
		t.Fatal(err)
	}
	text, err := readTruthText(fc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "surface_strict: true") {
		t.Fatal("strict normalization not exercised")
	}
	if _, err := env.eng.RequestTruthChange(fc, "change boundary", "owner"); err != nil {
		t.Fatal(err)
	}
	if err := writeTruthText(fc, text+"\n## Boundary correction\nNo currency conversion.\n"); err != nil {
		t.Fatal(err)
	}
	if err := env.eng.refreezeTxGuard(fc); err != nil {
		t.Fatalf("legitimate governed change falsely drifted: %v", err)
	}
}
