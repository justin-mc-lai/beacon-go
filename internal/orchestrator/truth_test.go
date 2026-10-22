package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// truthEnv seeds a registered project (real temp root, one current
// version) with NO packages — truth tests create their own.
type truthEnv struct {
	st   *store.Store
	eng  *Engine
	root string
	ver  store.ProjectVersion
}

func newTruthEnv(t *testing.T) truthEnv {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	root := filepath.Join(t.TempDir(), "proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := st.Projects().Register(store.Project{RootPath: root, Name: "proj"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := st.Projects().AddVersion(store.ProjectVersion{
		ProjectID: p.ID, Version: "v2.0.0", DocsRoot: filepath.Join("docs", "beacon", "v2.0.0"), IsCurrent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return truthEnv{st: st, eng: NewEngine(st), root: root, ver: v}
}

func (env truthEnv) ctx(t *testing.T, slug string) store.FeatureContext {
	t.Helper()
	ctx, err := env.st.ResolveFeature(env.root, slug, "")
	if err != nil {
		t.Fatalf("ResolveFeature(%s): %v", slug, err)
	}
	return ctx
}

func (env truthEnv) create(t *testing.T, slug string) TruthPackageView {
	t.Helper()
	v, err := env.eng.CreateTruthPackage(env.root, "", slug)
	if err != nil {
		t.Fatalf("CreateTruthPackage(%s): %v", slug, err)
	}
	return v
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestTruthCreateSkeletonShape(t *testing.T) {
	env := newTruthEnv(t)
	v := env.create(t, "login-2fa")
	if v.Status != "draft" || v.Frozen || v.ChangePending {
		t.Fatalf("new package view = %+v, want draft", v)
	}
	if v.LifecycleStage != string(core.StageThink) || v.LifecycleStatus != string(core.StatusNotStarted) {
		t.Fatalf("lifecycle = %s/%s, want Think/NotStarted", v.LifecycleStage, v.LifecycleStatus)
	}
	dir := filepath.Join(env.root, "docs", "beacon", "v2.0.0", "features", "login-2fa")
	truth := readFile(t, filepath.Join(dir, "truth.md"))
	scalars, refs, ok := gate.ParseFrontmatter(truth)
	if !ok || scalars["slug"] != "login-2fa" || scalars["status"] != "draft" || scalars["version"] != "v2.0.0" {
		t.Fatalf("truth frontmatter = %v (ok=%v)", scalars, ok)
	}
	for _, k := range []string{"truth", "tests", "tasks", "evidence"} {
		if refs[k] == "" {
			t.Fatalf("canonical_refs missing %s: %v", k, refs)
		}
	}
	tests := readFile(t, filepath.Join(dir, "tests.md"))
	if !strings.Contains(tests, "| AC ID | TC ID | Layer | Command | Assertion |") ||
		!strings.Contains(tests, "| AC-001 | TC-001 |") {
		t.Fatalf("tests.md must carry the frozen AC-first coverage matrix:\n%s", tests)
	}
	tasks := readFile(t, filepath.Join(dir, "tasks.md"))
	ts, _, ok := gate.ParseFrontmatter(tasks)
	if !ok || ts["slug"] != "login-2fa" {
		t.Fatalf("tasks frontmatter slug = %v (ok=%v)", ts, ok)
	}
	if ev := readFile(t, filepath.Join(dir, "evidence.md")); !strings.Contains(ev, "# Evidence Index: login-2fa") {
		t.Fatalf("evidence.md shape wrong:\n%s", ev)
	}
	feat, err := env.st.Features().GetBySlug(env.ver.ID, "login-2fa")
	if err != nil || feat.Status != "draft" {
		t.Fatalf("store row = %+v err=%v, want draft", feat, err)
	}
	// Duplicate slug + bad slug + unregistered project all refused.
	if _, err := env.eng.CreateTruthPackage(env.root, "", "login-2fa"); err == nil ||
		!strings.Contains(err.Error(), ReasonTruthPackageExists) {
		t.Fatalf("duplicate create err = %v", err)
	}
	if _, err := env.eng.CreateTruthPackage(env.root, "", "Bad Slug!"); err == nil ||
		!strings.Contains(err.Error(), ReasonTruthSlugInvalid) {
		t.Fatalf("bad slug err = %v", err)
	}
	if _, err := env.eng.CreateTruthPackage(filepath.Join(t.TempDir(), "nope"), "", "x-1"); err == nil ||
		!strings.Contains(err.Error(), ReasonProjectNotRegistered) {
		t.Fatalf("unregistered project err = %v", err)
	}
}

func TestTruthFreezeHappyAndIdempotent(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "feat-x")
	ctx := env.ctx(t, "feat-x")
	truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
	draftBytes := readFile(t, truthPath)

	v, err := env.eng.FreezeTruthPackage(ctx, "qa-bot")
	if err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	if !v.Frozen || v.Status != "frozen" {
		t.Fatalf("frozen view = %+v", v)
	}
	if v.TruthSHA256 != sha256Hex([]byte(draftBytes)) {
		t.Fatalf("view hash = %s, want sha256 of the draft bytes %s", v.TruthSHA256, sha256Hex([]byte(draftBytes)))
	}
	frozenText := readFile(t, truthPath)
	scalars, _, ok := gate.ParseFrontmatter(frozenText)
	if !ok || scalars["status"] != "frozen" {
		t.Fatalf("frontmatter after freeze = %v", scalars)
	}
	// Only the status line may differ from the draft bytes.
	if want := strings.Replace(draftBytes, "status: draft", "status: frozen", 1); frozenText != want {
		t.Fatalf("freeze changed more than the status line:\n--- draft ---\n%s\n--- frozen ---\n%s", draftBytes, frozenText)
	}
	if v.LifecycleStage != string(core.StageFreeze) || v.LifecycleStatus != string(core.StatusCompleted) {
		t.Fatalf("lifecycle = %s/%s, want Freeze/Completed", v.LifecycleStage, v.LifecycleStatus)
	}
	feat, err := env.st.Features().GetByID(ctx.Feature.ID)
	if err != nil || feat.Status != "frozen" {
		t.Fatalf("package status = %q err=%v, want frozen", feat.Status, err)
	}
	// The truth_frozen event must carry the ORIGINAL (pre-flip) hash.
	events, err := env.st.ListEvents(ctx.Feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	frozenEvents := 0
	for _, ev := range events {
		if ev.EventType != EventTruthFrozen {
			continue
		}
		frozenEvents++
		var p map[string]any
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			t.Fatal(err)
		}
		if p["truth_sha256_before"] != sha256Hex([]byte(draftBytes)) {
			t.Fatalf("event hash = %v, want draft hash %s", p["truth_sha256_before"], sha256Hex([]byte(draftBytes)))
		}
	}
	if frozenEvents != 1 {
		t.Fatalf("truth_frozen events = %d, want 1", frozenEvents)
	}
	// Idempotent: a second freeze succeeds, changes nothing, and
	// stacks no second freeze event.
	v2, err := env.eng.FreezeTruthPackage(ctx, "qa-bot")
	if err != nil {
		t.Fatalf("second Freeze: %v", err)
	}
	if !v2.AlreadyFrozen {
		t.Fatalf("second freeze view = %+v, want already_frozen", v2)
	}
	if got := readFile(t, truthPath); got != frozenText {
		t.Fatal("idempotent freeze rewrote the document")
	}
	events, _ = env.st.ListEvents(ctx.Feature.ID)
	n := 0
	for _, ev := range events {
		if ev.EventType == EventTruthFrozen {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("truth_frozen events after idempotent re-freeze = %d, want 1", n)
	}
}

func TestTruthFreezeBlockedOnEmptyMember(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "feat-empty")
	ctx := env.ctx(t, "feat-empty")
	if err := os.WriteFile(filepath.Join(env.root, ctx.Feature.TestsPath), []byte("  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := env.eng.FreezeTruthPackage(ctx, "qa-bot")
	if err == nil {
		t.Fatal("freeze of an incomplete package must fail")
	}
	if !v.Blocked || len(v.Findings) == 0 || v.Findings[0].Kind != "missing_or_empty_tests" {
		t.Fatalf("blocked view = %+v", v)
	}
	// Nothing was frozen as a side effect.
	scalars, _, _ := gate.ParseFrontmatter(readFile(t, filepath.Join(env.root, ctx.Feature.TruthPath)))
	if scalars["status"] != "draft" {
		t.Fatalf("status after blocked freeze = %q, want draft", scalars["status"])
	}
}

func TestTruthFreezeBlockedOnIntegrityFinding(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "feat-deferred")
	ctx := env.ctx(t, "feat-deferred")
	truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
	text := readFile(t, truthPath)
	if err := os.WriteFile(truthPath, []byte(text+"\n| pending |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := env.eng.FreezeTruthPackage(ctx, "qa-bot")
	if err == nil {
		t.Fatal("freeze with an undecided deferral must fail")
	}
	if !v.Blocked {
		t.Fatalf("view = %+v, want blocked", v)
	}
	found := false
	for _, code := range v.BlockReasons {
		if code == core.ReasonIntegrityDeferralPending {
			found = true
		}
	}
	if !found {
		t.Fatalf("block reasons = %v, want %s among them", v.BlockReasons, core.ReasonIntegrityDeferralPending)
	}
}

func TestTruthChangeRequiresFrozen(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "feat-draft")
	ctx := env.ctx(t, "feat-draft")
	if _, err := env.eng.RequestTruthChange(ctx, "tweak the scope", "alice"); err == nil ||
		!strings.Contains(err.Error(), ReasonTruthNotFrozen) {
		t.Fatalf("change on draft err = %v, want %s", err, ReasonTruthNotFrozen)
	}
}

func TestTruthChangeAndRefreeze(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "feat-chg")
	ctx := env.ctx(t, "feat-chg")
	if _, err := env.eng.FreezeTruthPackage(ctx, "qa-bot"); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
	frozenHash := sha256Hex([]byte(readFile(t, truthPath)))

	cv, err := env.eng.RequestTruthChange(ctx, "widen AC-001 to cover SSO", "alice")
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	if !cv.ChangePending || cv.ChangeID == "" || cv.Status != "change_pending" {
		t.Fatalf("change view = %+v", cv)
	}
	if got := sha256Hex([]byte(readFile(t, truthPath))); got != frozenHash {
		t.Fatalf("change touched the truth body: hash %s -> %s", frozenHash, got)
	}
	docPath := filepath.Join(env.root, filepath.FromSlash(cv.ChangeDoc))
	if doc := readFile(t, docPath); !strings.Contains(doc, cv.ChangeID) || !strings.Contains(doc, "widen AC-001") {
		t.Fatalf("change doc wrong:\n%s", doc)
	}
	feat, err := env.st.Features().GetByID(ctx.Feature.ID)
	if err != nil || feat.Status != "change_pending" {
		t.Fatalf("package status = %q err=%v, want change_pending", feat.Status, err)
	}
	// A second change while one is pending is refused; freeze too.
	if _, err := env.eng.RequestTruthChange(ctx, "another", "alice"); err == nil ||
		!strings.Contains(err.Error(), ReasonTruthChangePending) {
		t.Fatalf("second change err = %v", err)
	}
	if _, err := env.eng.FreezeTruthPackage(ctx, "qa-bot"); err == nil ||
		!strings.Contains(err.Error(), core.ReasonTruthChangePending) {
		t.Fatalf("freeze during pending err = %v", err)
	}
	// The author edits the body, then refreezes.
	edited := strings.Replace(readFile(t, truthPath), "第一条可验证的完成标准", "第一条可验证的完成标准（含 SSO）", 1)
	if err := os.WriteFile(truthPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	rv, err := env.eng.RefreezeTruthPackage(ctx, "qa-bot")
	if err != nil {
		t.Fatalf("Refreeze: %v", err)
	}
	if rv.ChangePending || !rv.Frozen || rv.Status != "frozen" {
		t.Fatalf("refrozen view = %+v", rv)
	}
	feat, err = env.st.Features().GetByID(ctx.Feature.ID)
	if err != nil || feat.Status != "frozen" {
		t.Fatalf("package status after refreeze = %q err=%v", feat.Status, err)
	}
	// The refreeze event names the closed change id.
	events, err := env.st.ListEvents(ctx.Feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	for _, ev := range events {
		if ev.EventType != EventTruthRefrozen {
			continue
		}
		var p struct {
			ChangeIDs []string `json:"change_ids"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			t.Fatal(err)
		}
		for _, id := range p.ChangeIDs {
			if id == cv.ChangeID {
				closed = true
			}
		}
	}
	if !closed {
		t.Fatal("truth_refrozen event does not name the closed change id")
	}
	// Refreeze with nothing pending is refused.
	if _, err := env.eng.RefreezeTruthPackage(ctx, "qa-bot"); err == nil ||
		!strings.Contains(err.Error(), ReasonTruthNoChangePending) {
		t.Fatalf("second refreeze err = %v", err)
	}
}

func TestTruthRefreezeBlockedByIntegrity(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "feat-bad-edit")
	ctx := env.ctx(t, "feat-bad-edit")
	if _, err := env.eng.FreezeTruthPackage(ctx, "qa-bot"); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	if _, err := env.eng.RequestTruthChange(ctx, "edit", "alice"); err != nil {
		t.Fatalf("Change: %v", err)
	}
	truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
	if err := os.WriteFile(truthPath, []byte(readFile(t, truthPath)+"\n| pending |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := env.eng.RefreezeTruthPackage(ctx, "qa-bot")
	if err == nil {
		t.Fatal("refreeze over a fractured edit must fail")
	}
	if !v.Blocked || !v.ChangePending {
		t.Fatalf("view = %+v, want blocked + still change_pending", v)
	}
}

func TestSetFrontmatterStatus(t *testing.T) {
	in := "---\nslug: x\nstatus: draft\nversion: v1\n---\n\nbody status: draft stays\n"
	out, changed := setFrontmatterStatus(in, "frozen")
	if !changed || !strings.Contains(out, "status: frozen") || !strings.Contains(out, "body status: draft stays") {
		t.Fatalf("replace: changed=%v out=%q", changed, out)
	}
	in2 := "---\nslug: x\nversion: v1\n---\nbody\n"
	out2, changed2 := setFrontmatterStatus(in2, "frozen")
	if !changed2 || !strings.Contains(out2, "slug: x\nstatus: frozen\nversion: v1") {
		t.Fatalf("insert: changed=%v out=%q", changed2, out2)
	}
	if _, changed3 := setFrontmatterStatus("no frontmatter\n", "frozen"); changed3 {
		t.Fatal("no frontmatter must report unchanged")
	}
	if _, changed4 := setFrontmatterStatus(out, "frozen"); changed4 {
		t.Fatal("already-frozen must report unchanged")
	}
}
