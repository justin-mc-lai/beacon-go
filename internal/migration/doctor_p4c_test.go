package migration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// Frozen-body fingerprint drift (P4c): when a freeze event recorded
// a package fingerprint baseline, doctor rebuild recomputes it
// from the on-disk package. An untouched body reports nothing; a
// body edited outside change/refreeze reports truth_body_drift —
// and --apply never "corrects" it, because rebuild does not
// rewrite truth.
func TestDoctorTruthBodyDrift(t *testing.T) {
	root := buildV1Sample(t)
	st := openTestStore(t)
	if _, err := ImportV1(st, root, Options{}); err != nil {
		t.Fatal(err)
	}
	ctx, err := st.ResolveFeature(root, "feat-old", "v1.1.0")
	if err != nil {
		t.Fatal(err)
	}
	truthRaw, err := os.ReadFile(filepath.Join(root, ctx.Feature.TruthPath))
	if err != nil {
		t.Fatal(err)
	}
	testsRaw, err := os.ReadFile(filepath.Join(root, ctx.Feature.TestsPath))
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{
		"slug": "feat-old", "by": "test",
		"package_fingerprint": core.PackageFingerprint(truthRaw, testsRaw),
		"fingerprint_alg":     core.FingerprintAlg,
	})
	if _, err := st.AppendEvent(store.LifecycleEvent{
		FeatureID: ctx.Feature.ID, EventType: "truth_frozen", Payload: payload,
	}); err != nil {
		t.Fatal(err)
	}

	rep, err := Rebuild(st, root, false)
	if err != nil {
		t.Fatal(err)
	}
	if d := driftOf(rep, DriftTruthBodyDrift, "feat-old"); d != nil {
		t.Fatalf("untouched frozen body reported drift: %+v", d)
	}

	// Edit the body, frontmatter status untouched: this is exactly
	// the post-freeze edit the fingerprint exists to catch.
	writeFile(t, filepath.Join(root, ctx.Feature.TruthPath),
		"---\nslug: feat-old\nstatus: frozen\n---\n# Truth: feat-old\n\nEDITED OUTSIDE CHANGE/REFREEZE\n")
	rep, err = Rebuild(st, root, true)
	if err != nil {
		t.Fatal(err)
	}
	d := driftOf(rep, DriftTruthBodyDrift, "feat-old")
	if d == nil {
		t.Fatalf("edited frozen body produced no truth_body_drift; drift = %+v", rep.Drift)
	}
	for _, c := range rep.Corrected {
		if strings.Contains(c, "fingerprint") || strings.Contains(c, "feat-old") && strings.Contains(c, "truth") {
			t.Errorf("apply must never correct a body drift, corrected = %v", rep.Corrected)
		}
	}
	// The document on disk is exactly what the editor left — the
	// rebuild did not rewrite truth.
	raw, err := os.ReadFile(filepath.Join(root, ctx.Feature.TruthPath))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "EDITED OUTSIDE CHANGE/REFREEZE") {
		t.Error("doctor rebuild rewrote the truth document; it must never do that")
	}
}
