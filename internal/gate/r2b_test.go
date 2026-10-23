package gate

import (
	"testing"

	"beacon-go/internal/core"
)

// H7: select_release_profile — normalisation, gate lists, and the
// invalid-profile block (prod_ops_v168, verbatim).
func TestSelectReleaseProfile(t *testing.T) {
	sel := SelectReleaseProfile("")
	if !sel.OK || sel.Profile != "full" || len(sel.Gates) != 1 || sel.Gates[0] != "full_suite" {
		t.Errorf("default: %+v", sel)
	}
	sel = SelectReleaseProfile(" Hotfix ")
	if !sel.OK || sel.Profile != "hotfix" || len(sel.Gates) != len(HotfixRequiredGates) {
		t.Errorf("hotfix: %+v", sel)
	}
	sel = SelectReleaseProfile("docs_only")
	if !sel.OK || len(sel.Gates) != 2 {
		t.Errorf("docs_only: %+v", sel)
	}
	sel = SelectReleaseProfile("banana")
	if !sel.Blocked || sel.ReasonCode != core.ReasonReleaseProfileInvalid {
		t.Errorf("invalid: %+v", sel)
	}
}

func goodFacts() VersionReleaseFacts {
	return VersionReleaseFacts{
		Version: "v2.0.0", VersionRegistered: true, MetadataOK: true,
		Features: []VersionFeatureFacts{{
			Slug: "feat-a", Frozen: true, IntegrityVerdict: "audit",
			GatesPass: true, QAArtifactCount: 2, ACCoverage: 0.9, HumanConfirmed: true,
		}},
	}
}

func itemByName(res VersionReleaseResult, name string) VersionCheckItem {
	for _, it := range res.Checks {
		if it.Name == name {
			return it
		}
	}
	return VersionCheckItem{}
}

// H7: a clean version passes under full; a metadata failure is a
// hard block under full, a downgraded warning under hotfix.
func TestVersionReleaseProfiles(t *testing.T) {
	res := EvaluateVersionRelease(goodFacts(), SelectReleaseProfile("full"))
	if !res.ReleaseReady || res.Status != "pass" {
		t.Fatalf("clean version must pass: %+v", res)
	}
	f := goodFacts()
	f.MetadataOK = false
	res = EvaluateVersionRelease(f, SelectReleaseProfile("full"))
	if res.ReleaseReady || itemByName(res, VersionCheckMetadata).Status != "fail" {
		t.Fatalf("full must hard-block on metadata: %+v", res)
	}
	if res.ReasonCode != core.ReasonReleaseCheckFailed {
		t.Errorf("headline code = %s", res.ReasonCode)
	}
	res = EvaluateVersionRelease(f, SelectReleaseProfile("hotfix"))
	md := itemByName(res, VersionCheckMetadata)
	if !res.ReleaseReady || md.Status != "warn" || !md.Downgraded || md.ReasonCode != core.ReasonReleaseCheckDowngraded {
		t.Fatalf("hotfix must downgrade metadata: %+v item %+v", res, md)
	}
	// But hotfix never downgrades the human confirmation.
	f2 := goodFacts()
	f2.Features[0].HumanConfirmed = false
	res = EvaluateVersionRelease(f2, SelectReleaseProfile("hotfix"))
	if res.ReleaseReady || itemByName(res, VersionCheckHuman).Status != "fail" {
		t.Fatalf("hotfix must keep human_release hard: %+v", res)
	}
	// docs_only: gates/evidence downgrade, frozen stays hard.
	f3 := goodFacts()
	f3.Features[0].GatesPass = false
	res = EvaluateVersionRelease(f3, SelectReleaseProfile("docs_only"))
	if !res.ReleaseReady || !itemByName(res, VersionCheckGates).Downgraded {
		t.Fatalf("docs_only must downgrade gates: %+v", res)
	}
	f3.Features[0].Frozen = false
	res = EvaluateVersionRelease(f3, SelectReleaseProfile("docs_only"))
	if res.ReleaseReady || itemByName(res, VersionCheckFrozen).Status != "fail" {
		t.Fatalf("docs_only must keep frozen hard: %+v", res)
	}
	// Coverage below the 0.8 bar fails; no-AC features are exempt.
	f4 := goodFacts()
	f4.Features[0].ACCoverage = 0.5
	res = EvaluateVersionRelease(f4, SelectReleaseProfile("full"))
	if res.ReleaseReady {
		t.Fatalf("coverage 0.5 must block: %+v", res)
	}
	f4.Features[0].ACCoverage = -1
	res = EvaluateVersionRelease(f4, SelectReleaseProfile("full"))
	if !res.ReleaseReady {
		t.Fatalf("no-AC feature must not fail coverage: %+v", res)
	}
}
