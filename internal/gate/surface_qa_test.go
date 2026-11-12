// qa-surface-e2e: unit tests for the surface declaration, the
// desktop-app project type and the two fail-closed qa doors.
package gate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
)

func TestParseSurfaceDeclarationInlineAndACInheritance(t *testing.T) {
	doc := "---\nslug: feat-x\nsurfaces: [api, desktop]\n---\n# Truth\n\n" +
		"| AC ID | Requirement | Surface |\n|---|---|---|\n" +
		"| AC-001 | api answer | |\n" +
		"| AC-002 | window opens | surface=desktop |\n"
	d := ParseSurfaceDeclaration(doc)
	if !d.Declared || len(d.Surfaces) != 2 || d.Surfaces[0] != "api" || d.Surfaces[1] != "desktop" {
		t.Fatalf("surfaces = %+v", d)
	}
	if !d.EffectiveStrict() {
		t.Fatal("declared surfaces without an explicit flag must be strict by default")
	}
	if got := d.ACSurfaces["AC-001"]; len(got) != 2 || got[0] != "api" {
		t.Fatalf("AC-001 must inherit the package list, got %v", got)
	}
	if got := d.ACSurfaces["AC-002"]; len(got) != 1 || got[0] != "desktop" {
		t.Fatalf("AC-002 explicit surface = %v", got)
	}
}

func TestParseSurfaceDeclarationBlockListAndExplicitFalse(t *testing.T) {
	doc := "---\nslug: feat-x\nsurfaces:\n  - web\n  - cli\nsurface_strict: false\n---\n# Truth\n"
	d := ParseSurfaceDeclaration(doc)
	if len(d.Surfaces) != 2 || d.Surfaces[0] != "web" || d.Surfaces[1] != "cli" {
		t.Fatalf("block list = %+v", d.Surfaces)
	}
	if !d.StrictDeclared || d.Strict || d.EffectiveStrict() {
		t.Fatalf("explicit false must stay non-strict: %+v", d)
	}
}

func TestParseSurfaceDeclarationLegacy(t *testing.T) {
	d := ParseSurfaceDeclaration("---\nslug: legacy\nstatus: frozen\n---\n# Truth\n")
	if d.Declared || len(d.Surfaces) != 0 || d.EffectiveStrict() {
		t.Fatalf("legacy package = %+v", d)
	}
	if codes := ValidateSurfaceDeclaration(d); len(codes) != 0 {
		t.Fatalf("legacy package must validate cleanly, got %v", codes)
	}
}

func TestValidateSurfaceDeclaration(t *testing.T) {
	base := SurfaceDeclaration{Declared: true, Surfaces: []string{"api", "desktop"}, Strict: true}
	if codes := ValidateSurfaceDeclaration(base); len(codes) != 0 {
		t.Fatalf("valid declaration codes = %v", codes)
	}
	invalid := SurfaceDeclaration{Declared: true, Surfaces: []string{"foo"}}
	if codes := ValidateSurfaceDeclaration(invalid); len(codes) != 1 ||
		codes[0] != core.ReasonTruthSurfaceInvalid {
		t.Fatalf("invalid surface codes = %v", codes)
	}
	empty := SurfaceDeclaration{Declared: true}
	if codes := ValidateSurfaceDeclaration(empty); len(codes) != 1 ||
		codes[0] != core.ReasonTruthSurfaceRequired {
		t.Fatalf("empty surface codes = %v", codes)
	}
	undeclaredAC := SurfaceDeclaration{
		Declared: true, Surfaces: []string{"api"}, Strict: true,
		ACSurfaces: map[string][]string{"AC-002": {"desktop"}},
	}
	if codes := ValidateSurfaceDeclaration(undeclaredAC); len(codes) != 1 ||
		codes[0] != core.ReasonTruthACSurfaceUndeclared {
		t.Fatalf("undeclared AC surface codes = %v", codes)
	}
}

func writeSurfaceFixture(t *testing.T, root, name, text string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInferDesktopProjectType(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, root string)
	}{
		{"electron dependency", func(t *testing.T, root string) {
			writeSurfaceFixture(t, root, "package.json", "{\"name\":\"app\",\"devDependencies\":{\"electron\":\"^30.0.0\"}}")
		}},
		{"electron builder appId", func(t *testing.T, root string) {
			writeSurfaceFixture(t, root, "package.json", "{\"name\":\"app\",\"build\":{\"appId\":\"com.acme.app\"}}")
		}},
		{"tauri config", func(t *testing.T, root string) {
			writeSurfaceFixture(t, root, "src-tauri/tauri.conf.json", "{\"productName\":\"app\"}")
		}},
		{"entitlements file", func(t *testing.T, root string) {
			writeSurfaceFixture(t, root, "build/Release.entitlements", "<plist/>")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)
			if got := InferProjectType(root, ProjectScan{Frontend: "web"}); got != "desktop-app" {
				t.Fatalf("InferProjectType = %q, want desktop-app (web frontend must not win)", got)
			}
		})
	}
	// A plain web project keeps its old type: the new signal must not
	// capture everything.
	webRoot := t.TempDir()
	writeSurfaceFixture(t, webRoot, "package.json", "{\"name\":\"site\",\"dependencies\":{\"react\":\"^18.0.0\"}}")
	if got := InferProjectType(webRoot, ProjectScan{Frontend: "web"}); got != "web-app" {
		t.Fatalf("plain web project inferred %q, want web-app", got)
	}
}

func TestDesktopApplicabilityMatrix(t *testing.T) {
	got := QAApplicability("desktop-app", true)
	want := map[string]string{
		"unit": "required", "interface": "required", "contract": "na",
		"integration": "conditional", "smoke": "required", "functional": "required",
		"regression": "required", "exception": "required", "compatibility": "conditional",
	}
	for layer, state := range want {
		if got[layer] != state {
			t.Fatalf("layer %s = %q, want %q", layer, got[layer], state)
		}
	}
}

func TestE2ERunnerPresetsFunctionalAndCompatibility(t *testing.T) {
	const wantWeb = "qae2e run --surface web --spec .beacon/e2e/<feature>.json"
	const wantDesktop = "qae2e run --surface desktop --spec .beacon/e2e/<feature>.json"
	for _, qaType := range []string{"functional", "compatibility"} {
		if got := DefaultCommandForQAType(qaType, "ego-browser"); got != wantWeb {
			t.Fatalf("%s ego-browser = %q", qaType, got)
		}
		if got := DefaultCommandForQAType(qaType, "agent-desktop"); got != wantDesktop {
			t.Fatalf("%s agent-desktop = %q", qaType, got)
		}
	}
}

func qaBaseFacts() QAFacts {
	return QAFacts{
		QAArtifactCount: 1,
		QAStageExecuted: true,
		Surfaces:        []string{"api", "desktop"},
		SurfaceStrict:   true,
		SurfaceEvidence: map[string]map[string]bool{
			"api":     {"e2e_report": true},
			"desktop": {},
		},
	}
}

func hasReason(ev Evaluation, code string) bool {
	for _, rc := range ev.ReasonCodes {
		if rc == code {
			return true
		}
	}
	return false
}

func findCheck(ev Evaluation, name string) CheckItem {
	for _, c := range ev.Checks {
		if c.Check == name {
			return c
		}
	}
	return CheckItem{Check: name, Detail: "missing"}
}

func TestEvaluateQASurfaceEvidenceGate(t *testing.T) {
	f := qaBaseFacts()
	f.AutotestRecorded = true
	ev := EvaluateQA(f, "qa-bot")
	if ev.Verdict != VerdictBlocked || !hasReason(ev, "qa_surface_evidence_miss:desktop") {
		t.Fatalf("missing desktop evidence must block with the surface name, got %+v", ev)
	}
	f.SurfaceEvidence["desktop"] = map[string]bool{"a11y_tree": true}
	ev = EvaluateQA(f, "qa-bot")
	if ev.Verdict != VerdictPass {
		t.Fatalf("complete evidence must pass, got %+v", ev)
	}
	// A declared web surface with only an a11y_tree is still missing.
	f.Surfaces = []string{"web"}
	f.SurfaceEvidence = map[string]map[string]bool{"web": {"a11y_tree": true}}
	ev = EvaluateQA(f, "qa-bot")
	if ev.Verdict != VerdictBlocked || !hasReason(ev, "qa_surface_evidence_miss:web") {
		t.Fatalf("missing browser_trace must block with qa_surface_evidence_miss:web, got %+v", ev)
	}
}

func TestEvaluateQAAutotestGate(t *testing.T) {
	f := qaBaseFacts()
	f.Surfaces = []string{"api"}
	f.SurfaceEvidence = map[string]map[string]bool{"api": {"e2e_report": true}}
	ev := EvaluateQA(f, "qa-bot")
	if ev.Verdict != VerdictBlocked || !hasReason(ev, core.ReasonQAAutotestRecordedMissing) {
		t.Fatalf("no autotest record must block, got %+v", ev)
	}
	f.AutotestRecorded = true
	f.AutotestDetail = "autotest evidence recorded for run r1"
	ev = EvaluateQA(f, "qa-bot")
	if ev.Verdict != VerdictPass {
		t.Fatalf("autotest record must unblock, got %+v", ev)
	}
}

func TestEvaluateQALegacySurfaceWaiver(t *testing.T) {
	f := qaBaseFacts()
	f.SurfaceStrict = false
	f.Surfaces = nil
	f.SurfaceEvidence = nil
	ev := EvaluateQA(f, "qa-bot")
	if ev.Verdict != VerdictPass {
		t.Fatalf("legacy package must pass with warnings, got %+v", ev)
	}
	if len(ev.ReasonCodes) != 0 {
		t.Fatalf("legacy pass must carry no fail reason, got %v", ev.ReasonCodes)
	}
	for _, name := range []string{"qa_autotest_recorded", "qa_surface_evidence"} {
		c := findCheck(ev, name)
		if c.Severity != "warn" || c.ReasonCode != core.ReasonLegacyWaiverSurface || !c.OK {
			t.Fatalf("%s = %+v, want a legacy_waiver_surface warning", name, c)
		}
	}
}

func TestSurfaceEvidenceReasonNamesSurface(t *testing.T) {
	if got := SurfaceEvidenceReason("desktop"); !strings.HasSuffix(got, ":desktop") ||
		!strings.HasPrefix(got, core.ReasonQASurfaceEvidenceMiss) {
		t.Fatalf("reason = %q", got)
	}
}
