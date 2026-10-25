package cli

import (
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/orchestrator"
)

// A never-frozen package is not drift: nothing to compare against, so the
// command stays successful and only notes it. Real drift must fail.
func TestDriftExitSemantics(t *testing.T) {
	cases := []struct {
		name string
		view orchestrator.DriftView
		fail bool
	}{
		{"clean", orchestrator.DriftView{Clean: true}, false},
		{"never frozen", orchestrator.DriftView{Clean: false, Note: "never_frozen"}, false},
		{"body drift", orchestrator.DriftView{Clean: false, BodyDrift: true}, true},
		{"requirement added", orchestrator.DriftView{Clean: false, RequirementDrift: map[string]string{"AC-002": "added"}}, true},
		{"draft with open question", orchestrator.DriftView{Clean: false, OpenQuestions: 1}, false},
		{"base diverged", orchestrator.DriftView{Clean: false, BaseDiverged: true}, true},
	}
	for _, c := range cases {
		err := driftExit(c.view)
		if c.fail {
			if err == nil {
				t.Fatalf("%s: want error, got nil", c.name)
			}
			if ce, ok := err.(*core.Error); ok && ce.ReasonCode != core.ReasonTruthDriftDetected {
				t.Fatalf("%s: reason = %s, want %s", c.name, ce.ReasonCode, core.ReasonTruthDriftDetected)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: want nil, got %v", c.name, err)
		}
	}
}
