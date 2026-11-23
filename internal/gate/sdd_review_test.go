package gate

import (
	"strings"
	"testing"
)

func TestSDDReviewFingerprintBindsTasksAndIgnoresSigning(t *testing.T) {
	pkg := PackageFiles{Contract: "truth-tasks", Truth: PackageFile{Text: "---\nstatus: draft\nintent_confirmed: false\n---\nPrice rounding\n"}, Tasks: PackageFile{Text: "Run rounding test\n"}}
	before := SDDReviewFingerprint(pkg)
	pkg.Truth.Text = strings.Replace(pkg.Truth.Text, "status: draft", "status: frozen\nsdd_review_by: checker\nintent_confirmation_by: owner", 1)
	pkg.Truth.Text = strings.Replace(pkg.Truth.Text, "intent_confirmed: false", "intent_confirmed: true", 1)
	if SDDReviewFingerprint(pkg) != before {
		t.Fatal("kernel signing metadata invalidated review")
	}
	pkg.Tasks.Text += "Change rounding mode\n"
	if SDDReviewFingerprint(pkg) == before {
		t.Fatal("task changes escaped reviewed fingerprint")
	}
}

func TestSDDReviewFingerprintSurvivesEffectiveStrictMaterialization(t *testing.T) {
	pkg := PackageFiles{Truth: PackageFile{Text: "---\nstatus: draft\nsurfaces: [cli]\n---\nRequirement\n"}}
	before := SDDReviewFingerprint(pkg)
	pkg.Truth.Text = strings.Replace(pkg.Truth.Text, "status: draft", "status: frozen\nsurface_strict: true", 1)
	if SDDReviewFingerprint(pkg) != before {
		t.Fatal("freeze invalidated reviewed effective strict policy")
	}
	pkg.Truth.Text = strings.Replace(pkg.Truth.Text, "surface_strict: true", "surface_strict: false", 1)
	if SDDReviewFingerprint(pkg) == before {
		t.Fatal("explicit policy weakening escaped review")
	}
}
