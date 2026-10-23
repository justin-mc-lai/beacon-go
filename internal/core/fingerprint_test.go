package core

import (
	"strings"
	"testing"
)

func TestRequirementFingerprintsTrackOneAC(t *testing.T) {
	base := `---
slug: x
status: draft
---
## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | user can sign in |
| AC-002 | user can sign out |
`
	fp := RequirementFingerprints([]byte(base))
	if len(fp) != 2 || fp["AC-001"] == "" || fp["AC-002"] == "" {
		t.Fatalf("fingerprints = %v, want AC-001 and AC-002", fp)
	}
	edited := strings.Replace(base, "user can sign out", "user can sign out everywhere", 1)
	fp2 := RequirementFingerprints([]byte(edited))
	if fp2["AC-001"] != fp["AC-001"] {
		t.Fatal("editing AC-002 must not move AC-001's fingerprint")
	}
	if fp2["AC-002"] == fp["AC-002"] {
		t.Fatal("editing AC-002 must move its fingerprint")
	}
}
