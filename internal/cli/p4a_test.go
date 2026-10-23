package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"beacon-go/internal/store"
)

// seedP4AProject registers a fresh project (CLI) in a temp store and
// returns its root. The version's docs root defaults to
// docs/beacon/v2.0.0.
func seedP4AProject(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	t.Setenv(store.EnvStorePath, dbPath)
	root := filepath.Join(t.TempDir(), "p4a-proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, "projects", "register", "--path", root, "--version", "v2.0.0", "--json")
	if err != nil {
		t.Fatalf("projects register: %v (%s)", err, out)
	}
	return root
}

func truthCreate(t *testing.T, root, slug string) map[string]any {
	t.Helper()
	out, err := runCLI(t, "truth", "create", "--project", root, "--slug", slug, "--json")
	if err != nil {
		t.Fatalf("truth create %s: %v (%s)", slug, err, out)
	}
	// Fill the one-sentence human summary (the intent gate refuses
	// the skeleton placeholder), then drive the real ask-before-sign
	// flow so lifecycle tests keep testing their own subject.
	truthPath := filepath.Join(root, "docs", "beacon", "v2.0.0", "features", slug, "truth.md")
	if raw, err := os.ReadFile(truthPath); err == nil {
		raw = []byte(strings.Replace(string(raw),
			"（一句话说清这个需求解决什么问题、给谁用、做到什么算完。冻结前由作者补齐。）",
			"测试需求：目标、用户与完成标准。", 1))
		if err := os.WriteFile(truthPath, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if out, err := runCLI(t, "truth", "intent", "--project", root, "--feature", slug,
		"--utterance", "test utterance", "--goal", "test goal",
		"--non-goals", "test non-goals", "--constraints", "test constraints", "--json"); err != nil {
		t.Fatalf("truth intent %s: %v (%s)", slug, err, out)
	}
	if out, err := runCLI(t, "truth", "answer", "--project", root, "--feature", slug,
		"--id", "Q1", "--answer", "test answer", "--json"); err != nil {
		t.Fatalf("truth answer %s: %v (%s)", slug, err, out)
	}
	if out, err := runCLI(t, "truth", "confirm", "--project", root, "--feature", slug, "--json"); err != nil {
		t.Fatalf("truth confirm %s: %v (%s)", slug, err, out)
	}
	return parseDoc(t, out)
}

func truthFreeze(t *testing.T, root, slug string) map[string]any {
	t.Helper()
	out, err := runCLI(t, "truth", "freeze", "--project", root, "--feature", slug, "--by", "qa-bot", "--json")
	if err != nil {
		t.Fatalf("truth freeze %s: %v (%s)", slug, err, out)
	}
	return parseDoc(t, out)
}

// TestTruthCLIChain walks the whole truth lifecycle on the real CLI:
// create -> show/list -> freeze -> change -> refreeze, plus the
// duplicate-create refusal and a blocked freeze.
func TestTruthCLIChain(t *testing.T) {
	root := seedP4AProject(t)

	doc := truthCreate(t, root, "login-2fa")
	assertKeys(t, doc, "slug", "status", "package_dir", "truth_path", "lifecycle_stage")
	if doc["status"] != "draft" || doc["lifecycle_stage"] != "Think" {
		t.Fatalf("created doc = %v", doc)
	}
	pkgDir := filepath.Join(root, "docs", "beacon", "v2.0.0", "features", "login-2fa")
	for _, f := range []string{"truth.md", "tests.md", "tasks.md", "evidence.md"} {
		if _, err := os.Stat(filepath.Join(pkgDir, f)); err != nil {
			t.Fatalf("skeleton file %s: %v", f, err)
		}
	}
	if _, err := runCLI(t, "truth", "create", "--project", root, "--slug", "login-2fa", "--json"); err == nil {
		t.Fatal("duplicate create must fail")
	}

	out, err := runCLI(t, "truth", "list", "--project", root, "--json")
	if err != nil {
		t.Fatalf("truth list: %v", err)
	}
	list := parseDoc(t, out)
	feats, ok := list["features"].([]any)
	if !ok || len(feats) != 1 {
		t.Fatalf("list doc = %v", list)
	}

	doc = truthFreeze(t, root, "login-2fa")
	if doc["status"] != "frozen" || doc["frozen"] != true {
		t.Fatalf("frozen doc = %v", doc)
	}
	// Idempotent re-freeze through the CLI.
	doc = truthFreeze(t, root, "login-2fa")
	if doc["already_frozen"] != true {
		t.Fatalf("re-freeze doc = %v, want already_frozen", doc)
	}

	out, err = runCLI(t, "truth", "change", "--project", root, "--feature", "login-2fa",
		"--summary", "widen AC-001 to cover SSO", "--by", "alice", "--json")
	if err != nil {
		t.Fatalf("truth change: %v (%s)", err, out)
	}
	doc = parseDoc(t, out)
	if doc["status"] != "change_pending" || doc["change_id"] == "" || doc["change_id"] == nil {
		t.Fatalf("change doc = %v", doc)
	}

	out, err = runCLI(t, "truth", "show", "--project", root, "--feature", "login-2fa", "--json")
	if err != nil {
		t.Fatalf("truth show: %v", err)
	}
	if doc := parseDoc(t, out); doc["status"] != "change_pending" {
		t.Fatalf("show during pending = %v", doc)
	}

	// The change must actually edit a requirement surface before
	// refreezing (S1, AC-137: a refreeze with no content movement
	// is refused as empty_refreeze_reason_only).
	truthPath := filepath.Join(pkgDir, "truth.md")
	raw, err := os.ReadFile(truthPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(truthPath, []byte(strings.Replace(string(raw),
		"作者写下第一条可验证的完成标准", "作者写下第一条可验证的完成标准（含 SSO 登录覆盖）", 1)), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err = runCLI(t, "truth", "refreeze", "--project", root, "--feature", "login-2fa", "--by", "qa-bot", "--json")
	if err != nil {
		t.Fatalf("truth refreeze: %v (%s)", err, out)
	}
	if doc := parseDoc(t, out); doc["status"] != "frozen" || doc["change_pending"] != false {
		t.Fatalf("refrozen doc = %v", doc)
	}

	// Blocked freeze: an emptied tests.md stops the freeze cold.
	truthCreate(t, root, "feat-bad")
	if err := os.WriteFile(filepath.Join(root, "docs", "beacon", "v2.0.0", "features", "feat-bad", "tests.md"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = runCLI(t, "truth", "freeze", "--project", root, "--feature", "feat-bad", "--by", "qa-bot", "--json")
	if err == nil {
		t.Fatal("freeze of an incomplete package must fail")
	}
	if doc := parseDoc(t, out); doc["blocked"] != true {
		t.Fatalf("blocked freeze doc = %v", doc)
	}
}

// TestThinFacesAgreeWithGoalAndGate pins the P4a convergence
// contract: the thin ability faces return the SAME verdicts as the
// goal/gate entries, because both call one implementation.
func TestThinFacesAgreeWithGoalAndGate(t *testing.T) {
	root := seedP4AProject(t)
	truthCreate(t, root, "feat-a")
	truthFreeze(t, root, "feat-a")

	out, err := runCLI(t, "goal", "run", "add login", "--project", root, "--feature", "feat-a",
		"--ux", "--executor-id", "host-1", "--json")
	if err != nil {
		t.Fatalf("goal run: %v (%s)", err, out)
	}
	runID := strField(t, parseDoc(t, out), "run_id")

	// Design face vs goal design-stage: same status document.
	faceOut, err := runCLI(t, "design", "status", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("design status: %v", err)
	}
	goalOut, err := runCLI(t, "goal", "design-stage", "status", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("goal design-stage status: %v", err)
	}
	faceDoc, goalDoc := parseDoc(t, faceOut), parseDoc(t, goalOut)
	if !reflect.DeepEqual(faceDoc, goalDoc) {
		t.Fatalf("design face != goal design-stage:\nface %v\ngoal %v", faceDoc, goalDoc)
	}
	if faceDoc["bound"] != false || faceDoc["reason_code"] != "design_primitive_not_bound" {
		t.Fatalf("unbound design doc = %v", faceDoc)
	}

	// Implement face vs goal admit: the same admission verdict.
	faceOut, faceErr := runCLI(t, "implement", "admit", "--run-id", runID, "--json")
	goalOut, goalErr := runCLI(t, "goal", "admit", "--run-id", runID, "--json")
	if faceErr == nil || goalErr == nil {
		t.Fatalf("admission must block unbound UX work: face err=%v goal err=%v", faceErr, goalErr)
	}
	faceDoc, goalDoc = parseDoc(t, faceOut), parseDoc(t, goalOut)
	if faceDoc["reason_code"] != goalDoc["reason_code"] || faceDoc["allow_implement"] != goalDoc["allow_implement"] {
		t.Fatalf("admission verdicts differ:\nface %v\ngoal %v", faceDoc, goalDoc)
	}
	if faceDoc["allow_implement"] != false {
		t.Fatalf("admission doc = %v, want allow_implement=false", faceDoc)
	}

	// Bind through the face; the goal entry must see the same binding.
	out, err = runCLI(t, "design", "bind", "--run-id", runID, "--primitive-set", "acme-ds", "--version", "v1", "--json")
	if err != nil {
		t.Fatalf("design bind: %v (%s)", err, out)
	}
	goalOut, err = runCLI(t, "goal", "design-stage", "status", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("goal design-stage status after bind: %v", err)
	}
	if doc := parseDoc(t, goalOut); doc["bound"] != true {
		t.Fatalf("goal sees binding = %v, want bound", doc)
	}

	// QA face status vs gate status: identical gate tables.
	qaOut, err := runCLI(t, "qa", "status", "--project", root, "--feature", "feat-a", "--json")
	if err != nil {
		t.Fatalf("qa status: %v", err)
	}
	gateOut, err := runCLI(t, "gate", "status", "--project", root, "--feature", "feat-a", "--json")
	if err != nil {
		t.Fatalf("gate status: %v", err)
	}
	if !reflect.DeepEqual(parseDoc(t, qaOut)["gates"], parseDoc(t, gateOut)["gates"]) {
		t.Fatalf("qa gates != gate gates:\nqa %s\ngate %s", qaOut, gateOut)
	}

	// Release face evaluate vs gate evaluate release: same verdict.
	relOut, relErr := runCLI(t, "release", "evaluate", "--project", root, "--feature", "feat-a",
		"--evaluator", "qa-bot", "--json")
	gateRelOut, gateRelErr := runCLI(t, "gate", "evaluate", "release", "--project", root, "--feature", "feat-a",
		"--evaluator", "qa-bot", "--json")
	if relErr == nil || gateRelErr == nil {
		t.Fatalf("release gate must block with no prior gates: face err=%v gate err=%v", relErr, gateRelErr)
	}
	relDoc, gateRelDoc := parseDoc(t, relOut), parseDoc(t, gateRelOut)
	if relDoc["verdict"] != gateRelDoc["verdict"] ||
		!reflect.DeepEqual(relDoc["reason_codes"], gateRelDoc["reason_codes"]) {
		t.Fatalf("release verdicts differ:\nface %v\ngate %v", relDoc, gateRelDoc)
	}
	if relDoc["verdict"] != "blocked" {
		t.Fatalf("release verdict = %v, want blocked", relDoc["verdict"])
	}
}

// TestGoalEntryContractAndResumePackCLI covers the two P2 debts at
// the command surface: entry-contract (G-01) and resume-pack (G-08)
// return complete structured documents.
func TestGoalEntryContractAndResumePackCLI(t *testing.T) {
	root := seedP4AProject(t)
	truthCreate(t, root, "feat-a")
	truthFreeze(t, root, "feat-a")

	out, err := runCLI(t, "goal", "run", "add login", "--project", root, "--feature", "feat-a",
		"--executor-id", "host-1", "--json")
	if err != nil {
		t.Fatalf("goal run: %v (%s)", err, out)
	}
	runID := strField(t, parseDoc(t, out), "run_id")

	out, err = runCLI(t, "goal", "entry-contract", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("entry-contract: %v (%s)", err, out)
	}
	doc := parseDoc(t, out)
	assertKeys(t, doc, "schema_version", "run_id", "feature_slug", "public_cli", "single_entry",
		"stages", "current_stage", "verifier_required", "release_requires_human")
	stages, ok := doc["stages"].([]any)
	if !ok || len(stages) != 5 {
		t.Fatalf("contract stages = %v", doc["stages"])
	}
	first, ok := stages[0].(map[string]any)
	if !ok || first["stage"] != "plan" {
		t.Fatalf("first stage = %v", stages[0])
	}
	if skills, ok := first["skills"].([]any); !ok || len(skills) == 0 {
		t.Fatalf("first stage skills = %v", first["skills"])
	}

	out, err = runCLI(t, "goal", "checkpoint", "--run-id", runID, "--next-action", "write the tests", "--json")
	if err != nil {
		t.Fatalf("checkpoint: %v (%s)", err, out)
	}
	out, err = runCLI(t, "goal", "resume-pack", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("resume-pack: %v (%s)", err, out)
	}
	doc = parseDoc(t, out)
	assertKeys(t, doc, "run", "resume", "latest_checkpoint", "open_blockers")
	resume, ok := doc["resume"].(map[string]any)
	if !ok || resume["resume_action"] != "load_next_skill" {
		t.Fatalf("pack resume = %v", doc["resume"])
	}
	ckpt, ok := doc["latest_checkpoint"].(map[string]any)
	if !ok {
		t.Fatalf("pack checkpoint = %v", doc["latest_checkpoint"])
	}
	state, ok := ckpt["state"].(map[string]any)
	if !ok || state["next_action"] != "write the tests" {
		t.Fatalf("pack checkpoint state = %v", ckpt["state"])
	}
	if !strings.Contains(out, "run_id") {
		t.Fatal("pack output malformed")
	}
}
