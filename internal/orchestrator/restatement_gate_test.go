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
)

// restTestPath is the restatement.md beside a fixture's truth document.
func restTestPath(t *testing.T, env truthEnv, slug string) string {
	t.Helper()
	ctx := env.ctx(t, slug)
	return filepath.Join(env.root, filepath.Dir(ctx.Feature.TruthPath), "restatement.md")
}

// writeRestTestDoc writes a restatement with a hash matching body.
func writeRestTestDoc(t *testing.T, path, slug, confirmedBy, body string) {
	t.Helper()
	sum := sha256.Sum256([]byte(body))
	doc := "---\nslug: " + slug + "\nconfirmed_by: " + confirmedBy +
		"\nconfirmed_at: 2026-11-13T00:00:00Z\nhash: " + hex.EncodeToString(sum[:]) +
		"\n---\n" + body
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertRestBlocked(t *testing.T, view TruthPackageView, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("freeze must be blocked by %s; view = %+v", want, view)
	}
	if !view.Blocked {
		t.Fatalf("view must be blocked: %+v", view)
	}
	found := false
	for _, code := range view.BlockReasons {
		if code == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("block reasons = %v, want %s", view.BlockReasons, want)
	}
	if rc, _ := core.ReasonCodeOf(err); rc != want {
		t.Fatalf("error reason = %s, want %s (%v)", rc, want, err)
	}
	if view.Frozen || view.Status != "draft" {
		t.Fatalf("blocked freeze must leave the package draft: %+v", view)
	}
}

// TC-002: no restatement.md blocks a first freeze.
func TestFreezeRequiresRestatement(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "rest-missing")
	ctx := env.ctx(t, "rest-missing")
	if err := os.Remove(restTestPath(t, env, "rest-missing")); err != nil {
		t.Fatal(err)
	}
	view, err := env.eng.FreezeTruthPackage(ctx, "qa-bot")
	assertRestBlocked(t, view, err, core.ReasonRestatementRequired)
}

// TC-003: an empty confirmed_by does not count as confirmation.
func TestFreezeRejectsUnconfirmedRestatement(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "rest-unconfirmed")
	writeRestTestDoc(t, restTestPath(t, env, "rest-unconfirmed"), "rest-unconfirmed", "", restatementFixtureBody)
	view, err := env.eng.FreezeTruthPackage(env.ctx(t, "rest-unconfirmed"), "qa-bot")
	assertRestBlocked(t, view, err, core.ReasonRestatementUnconfirmed)
}

// TC-004: one character added after confirmation makes the body stale.
func TestFreezeRejectsStaleRestatement(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "rest-stale")
	path := restTestPath(t, env, "rest-stale")
	doc := readFile(t, path)
	if err := os.WriteFile(path, []byte(doc+"确认后改一个字。\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	view, err := env.eng.FreezeTruthPackage(env.ctx(t, "rest-stale"), "qa-bot")
	assertRestBlocked(t, view, err, core.ReasonRestatementStale)
}

// TC-005: dropping one of the five sections blocks the freeze.
func TestFreezeRejectsMissingSections(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "rest-sections")
	body := strings.Replace(restatementFixtureBody, "## 4 边界与不做\n\n不改既有语义。\n\n", "", 1)
	if body == restatementFixtureBody {
		t.Fatal("fixture body has no section 4 block to drop")
	}
	writeRestTestDoc(t, restTestPath(t, env, "rest-sections"), "rest-sections", "justin-mc-lai", body)
	view, err := env.eng.FreezeTruthPackage(env.ctx(t, "rest-sections"), "qa-bot")
	assertRestBlocked(t, view, err, core.ReasonRestatementSectionsMissing)
	detail := ""
	for _, f := range view.Findings {
		if f.ReasonCode == core.ReasonRestatementSectionsMissing {
			detail = f.Detail
		}
	}
	if !strings.Contains(detail, "4") {
		t.Fatalf("finding must name the missing section 4: %q", detail)
	}
}

// TC-006: the surface declaration is checked before the restatement, so
// a package failing both reports the surface code first.
func TestFreezeOrderSurfaceBeforeRestatement(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "rest-order")
	ctx := env.ctx(t, "rest-order")
	truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
	raw := readFile(t, truthPath)
	updated := strings.Replace(raw, "surfaces: [api]", "surfaces: \"\"", 1)
	if updated == raw {
		t.Fatalf("fixture carries no surfaces line:\n%s", raw)
	}
	if err := os.WriteFile(truthPath, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(restTestPath(t, env, "rest-order")); err != nil {
		t.Fatal(err)
	}
	view, err := env.eng.FreezeTruthPackage(ctx, "qa-bot")
	assertRestBlocked(t, view, err, core.ReasonTruthSurfaceRequired)
	if len(view.BlockReasons) == 0 || view.BlockReasons[0] != core.ReasonTruthSurfaceRequired {
		t.Fatalf("first block reason = %v, want %s", view.BlockReasons, core.ReasonTruthSurfaceRequired)
	}
}

// TC-007: an AC pointing at a section that does not exist is untraceable.
func TestFreezeRejectsUntraceableAC(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "rest-ac")
	ctx := env.ctx(t, "rest-ac")
	truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
	raw := readFile(t, truthPath)
	renamed := strings.Replace(raw, "restates=1", "restates=9", 1)
	if renamed == raw {
		t.Fatalf("fixture AC row carries no restates token:\n%s", raw)
	}
	if err := os.WriteFile(truthPath, []byte(renamed), 0o644); err != nil {
		t.Fatal(err)
	}
	view, err := env.eng.FreezeTruthPackage(ctx, "qa-bot")
	assertRestBlocked(t, view, err, core.ReasonACNotTraceableToRestatement)

	if err := os.WriteFile(truthPath, []byte(strings.Replace(raw, " restates=1", "", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	view, err = env.eng.FreezeTruthPackage(ctx, "qa-bot")
	assertRestBlocked(t, view, err, core.ReasonACNotTraceableToRestatement)
}

// TC-008: five sections, a signature and a matching hash pass.
func TestFreezeAcceptsConfirmedRestatement(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "rest-ok")
	view, err := env.eng.FreezeTruthPackage(env.ctx(t, "rest-ok"), "qa-bot")
	if err != nil {
		t.Fatalf("freeze with a signed restatement: %v", err)
	}
	if view.Status != "frozen" || !view.Frozen {
		t.Fatalf("frozen view = %+v", view)
	}
	if view.Blocked || len(view.BlockReasons) != 0 {
		t.Fatalf("passing view must not be blocked: %+v", view)
	}
	if view.Restatement == nil || !view.Restatement.Matches {
		t.Fatalf("restatement block = %+v, want matches", view.Restatement)
	}
}

// TC-009: a refreeze keeps the legacy waiver, with a warning only.
func TestRefreezeLegacyWaiverWithoutRestatement(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "rest-legacy")
	ctx := env.ctx(t, "rest-legacy")
	if _, err := env.eng.FreezeTruthPackage(ctx, "qa-bot"); err != nil {
		t.Fatalf("first freeze: %v", err)
	}
	if err := os.Remove(restTestPath(t, env, "rest-legacy")); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.RequestTruthChange(ctx, "edit AC-001", "alice"); err != nil {
		t.Fatalf("change: %v", err)
	}
	truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
	raw := readFile(t, truthPath)
	updated := strings.Replace(raw, "作者写下第一条可验证的完成标准", "作者写下第一条可验证的完成标准（含 SSO）", 1)
	if updated == raw {
		t.Fatalf("fixture AC row missing:\n%s", raw)
	}
	if err := os.WriteFile(truthPath, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	view, err := env.eng.RefreezeTruthPackage(ctx, "qa-bot")
	if err != nil {
		t.Fatalf("legacy refreeze without a restatement must pass: %v", err)
	}
	if view.Blocked || view.Status != "frozen" {
		t.Fatalf("refrozen view = %+v", view)
	}
	warned := false
	for _, w := range view.Warnings {
		if strings.Contains(w, "restatement: missing") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("legacy waiver must be reported as a warning: %v", view.Warnings)
	}
}

// TC-010: truth show carries the restatement block, and a legacy
// package omits it from the JSON entirely.
func TestTruthShowRestatementBlock(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "rest-show")
	ctx := env.ctx(t, "rest-show")
	view, err := env.eng.TruthPackage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if view.Restatement == nil {
		t.Fatal("truth show must carry a restatement block when the file exists")
	}
	r := view.Restatement
	if !strings.HasSuffix(r.Path, "restatement.md") {
		t.Fatalf("restatement path = %q", r.Path)
	}
	if r.ConfirmedBy != "justin-mc-lai (unit fixture)" || r.ConfirmedAt == "" || r.Hash == "" || !r.Matches {
		t.Fatalf("restatement block = %+v", r)
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"restatement", "confirmed_by", "confirmed_at", "hash", "matches"} {
		if !strings.Contains(string(raw), "\""+key+"\"") {
			t.Fatalf("truth show JSON missing %q: %s", key, raw)
		}
	}
	if err := os.Remove(restTestPath(t, env, "rest-show")); err != nil {
		t.Fatal(err)
	}
	legacy, err := env.eng.TruthPackage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Restatement != nil {
		t.Fatalf("legacy package must omit the restatement block: %+v", legacy.Restatement)
	}
	raw, err = json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "\"restatement\"") {
		t.Fatalf("legacy JSON must not name a restatement block: %s", raw)
	}
}
