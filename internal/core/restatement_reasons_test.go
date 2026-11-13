package core

import "testing"

// TC-011: the five restatement reason codes are stable contract
// strings and every one has a catalog description.
func TestRestatementReasonCodes(t *testing.T) {
	cases := []struct{ name, got, want string }{
		{"required", ReasonRestatementRequired, "restatement_required"},
		{"unconfirmed", ReasonRestatementUnconfirmed, "restatement_unconfirmed"},
		{"stale", ReasonRestatementStale, "restatement_stale"},
		{"sections", ReasonRestatementSectionsMissing, "restatement_sections_missing"},
		{"traceability", ReasonACNotTraceableToRestatement, "ac_not_traceable_to_restatement"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
		if _, ok := ReasonCatalog[c.got]; !ok {
			t.Errorf("reason %q has no catalog description", c.got)
		}
	}
}
