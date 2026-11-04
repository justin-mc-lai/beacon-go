// Charter guard (AC-005): the per-feature release gate's four
// prerequisites are truth_freeze, design (when applicable),
// implement and qa. The integrity gate stays outside it (AC-101);
// this test fails if a future change silently adds it.
package gate

import (
	"strings"
	"testing"
)

func TestEvaluateReleaseHasNoIntegrityPrereq(t *testing.T) {
	res := EvaluateRelease(ReleaseFacts{
		TruthFreeze:    GateFact{Present: true, Verdict: string(VerdictPass)},
		Implement:      GateFact{Present: true, Verdict: string(VerdictPass)},
		QA:             GateFact{Present: true, Verdict: string(VerdictPass)},
		HumanConfirmed: true,
	}, "probe")
	for _, c := range res.Checks {
		if strings.Contains(strings.ToLower(c.Check), "integrity") {
			t.Fatalf("release must not gain an integrity prerequisite (AC-101), got check %+v", c)
		}
	}
}
