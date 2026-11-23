package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/gate"
)

func TestSDDReviewRejectsSelfAndStaleContent(t *testing.T) {
	env := newTruthEnv(t)
	_, err := env.eng.CreateTruthPackageSDD(env.root, "", "reviewed")
	if err != nil {
		t.Fatal(err)
	}
	fc := env.ctx(t, "reviewed")
	text := `---
slug: reviewed
version: v2.0.0
package_contract: truth-tasks
package_author: writer
package_maturity: filled
intent_confirmed: false
---
## Grounding
| Source | Reference | Supports |
|---|---|---|
| user | conversation:request | rounding behavior |
## Assumptions
None.
## Acceptance Criteria
| AC ID | Criterion |
|---|---|
| AC-001 | Prices round to two decimals. |
`
	if err = writeTruthText(fc, text); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(env.root, "receipt.json")
	record := func(reviewer string) error {
		receipt := SDDReviewReceipt{Fingerprint: env.eng.SDDReviewFingerprint(fc), Reviewer: reviewer, Verdict: "pass", Summary: "Rounding behavior, boundaries and task scope agree with the grounded request."}
		raw, _ := json.Marshal(receipt)
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := env.eng.RecordSDDReview(fc, path)
		return err
	}
	if err = record("writer"); err == nil {
		t.Fatal("self review accepted")
	}
	if err = record("checker"); err != nil {
		t.Fatal(err)
	}
	pkg := env.eng.packageFiles(fc)
	if fs := env.eng.sddRecordedReviewFindings(fc, pkg, false); len(fs) != 0 {
		t.Fatalf("current independent review blocked: %+v", fs)
	}
	old := gate.SDDReviewFingerprint(pkg)
	signed, _ := setFrontmatterScalar(pkg.Truth.Text, "intent_confirmation_fingerprint", old)
	signed, _ = setFrontmatterScalar(signed, "intent_confirmation_by", "owner")
	signed, _ = setFrontmatterScalar(signed, "intent_confirmed", "true")
	pkg.Truth.Text = signed
	if gate.SDDReviewFingerprint(pkg) != old {
		t.Fatal("confirmation metadata changed review fingerprint")
	}
	pkg.Truth.Text = strings.Replace(signed, "two decimals", "three decimals", 1)
	if fs := gate.SDDReviewFindings(pkg, true); len(fs) == 0 {
		t.Fatal("requirement edit preserved stale signoff")
	}
}
