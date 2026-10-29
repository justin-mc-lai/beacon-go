package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestService builds a service over a temp project root and a temp
// ledger. The docs version directory exists so registration can detect
// it; nothing else is pre-seeded.
func newTestService(t *testing.T) *Service {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "beacon", "v2.0.0"), 0o755); err != nil {
		t.Fatalf("seed docs dir: %v", err)
	}
	svc, err := Open(Options{Root: root, StorePath: filepath.Join(t.TempDir(), "ledger.db")})
	if err != nil {
		t.Fatalf("open workspace: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func TestUnregisteredProjectIsEmptyNotInvented(t *testing.T) {
	svc := newTestService(t)
	if svc.Registered() {
		t.Fatal("fresh temp root must not be registered")
	}
	list, err := svc.Features()
	if err != nil {
		t.Fatalf("features: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("unregistered project must list zero features, got %d", len(list))
	}
}

func TestEnsureRegisteredDetectsVersion(t *testing.T) {
	svc := newTestService(t)
	p, err := svc.EnsureRegistered()
	if err != nil {
		t.Fatalf("ensure registered: %v", err)
	}
	if p.RootPath != svc.Root {
		t.Fatalf("registered root = %q, want %q", p.RootPath, svc.Root)
	}
	if svc.Version != "v2.0.0" {
		t.Fatalf("detected version = %q, want v2.0.0", svc.Version)
	}
	// Idempotent: a second call must not create a second project row.
	if _, err := svc.EnsureRegistered(); err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	all, err := svc.Store().Projects().List()
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 project row, got %d", len(all))
	}
}

func TestStartGoalCreatesPackageAndRealRun(t *testing.T) {
	svc := newTestService(t)
	view, err := svc.StartGoal("demo-feature", "add a demo feature", "tester")
	if err != nil {
		t.Fatalf("start goal: %v", err)
	}
	if view.RunID == "" {
		t.Fatal("run id must not be empty")
	}
	if view.FeatureSlug != "demo-feature" {
		t.Fatalf("feature slug = %q", view.FeatureSlug)
	}
	if len(view.StagePlan) == 0 {
		t.Fatal("run must carry a real stage plan")
	}
	// No executor is registered, so nothing may claim progress: the
	// plan starts at plan/active and every later stage is pending.
	if view.StagePlan[0].Status != "active" {
		t.Fatalf("first stage status = %q, want active", view.StagePlan[0].Status)
	}
	for _, s := range view.StagePlan[1:] {
		if s.Status != "pending" {
			t.Fatalf("stage %s status = %q, want pending", s.Stage, s.Status)
		}
	}

	// The package must exist on disk with the four pieces.
	pkgDir := filepath.Join(svc.Root, "docs", "beacon", "v2.0.0", "features", "demo-feature")
	for _, name := range []string{"truth.md", "tests.md", "tasks.md", "evidence.md"} {
		if _, err := os.Stat(filepath.Join(pkgDir, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}

	// Status reads back through the ledger.
	again, err := svc.Goal(view.RunID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if again.RunID != view.RunID {
		t.Fatalf("status run id = %q", again.RunID)
	}
}

func TestStartGoalRejectsInvalidSlug(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.StartGoal("Not A Slug", "x", "tester"); err == nil {
		t.Fatal("invalid slug must be rejected")
	}
}

func TestReleaseReportsRealVerdictWithoutCompleting(t *testing.T) {
	svc := newTestService(t)
	view, err := svc.StartGoal("demo-feature", "add a demo feature", "tester")
	if err != nil {
		t.Fatalf("start goal: %v", err)
	}
	res, err := svc.Release(view.RunID, "operator")
	if err != nil {
		t.Fatalf("release: %v", err)
	}
	if res.Verdict == "pass" {
		t.Fatalf("a fresh run cannot pass the release gate: %+v", res)
	}
	if res.Completed {
		t.Fatal("a blocked release must never report completed")
	}
	if len(res.Reasons) == 0 {
		t.Fatal("a blocked release must name its reason codes")
	}
}

// A package that exists on disk but not in the ledger must still be
// visible (marked unregistered) and must be adopted, not overwritten,
// when a goal starts.
func TestOnDiskPackageIsListedThenAdopted(t *testing.T) {
	svc := newTestService(t)
	pkg := filepath.Join(svc.Root, "docs", "beacon", "v2.0.0", "features", "legacy-feature")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatalf("seed package: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkg, "truth.md"), []byte("# Truth\n\n> **Status**: `FROZEN`\n\nlegacy body"), 0o644); err != nil {
		t.Fatalf("write truth: %v", err)
	}

	list, err := svc.Features()
	if err != nil {
		t.Fatalf("features: %v", err)
	}
	if len(list) != 1 || list[0].Slug != "legacy-feature" {
		t.Fatalf("expected the on-disk package to be listed, got %+v", list)
	}
	if list[0].Registered {
		t.Fatal("an unledgered package must be marked unregistered")
	}
	if !list[0].Frozen {
		t.Fatal("the FROZEN marker in the header should be surfaced")
	}

	d, err := svc.Detail("legacy-feature")
	if err != nil {
		t.Fatalf("detail from disk: %v", err)
	}
	if !strings.Contains(d.TruthMD, "legacy body") {
		t.Fatalf("disk detail lost the body: %q", d.TruthMD)
	}

	run, err := svc.StartGoal("legacy-feature", "continue the legacy feature", "tester")
	if err != nil {
		t.Fatalf("start goal on existing package: %v", err)
	}
	if run.RunID == "" {
		t.Fatal("adoption must still create a run")
	}
	after, err := svc.Features()
	if err != nil {
		t.Fatalf("features after adopt: %v", err)
	}
	if len(after) != 1 || !after[0].Registered {
		t.Fatalf("package should be adopted into the ledger, got %+v", after)
	}
	// The body must not have been rewritten by adoption.
	body, _ := os.ReadFile(filepath.Join(pkg, "truth.md"))
	if !strings.Contains(string(body), "legacy body") {
		t.Fatal("adoption must not rewrite the truth body")
	}
}

func TestDetailCarriesMarkdownAndGates(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.StartGoal("demo-feature", "add a demo feature", "tester"); err != nil {
		t.Fatalf("start goal: %v", err)
	}
	d, err := svc.Detail("demo-feature")
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if d.TruthMD == "" || d.TestsMD == "" {
		t.Fatal("detail must return the real package Markdown")
	}
	if d.Frozen {
		t.Fatal("a freshly created draft package is not frozen")
	}
}
