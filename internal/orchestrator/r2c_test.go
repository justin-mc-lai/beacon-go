package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// ---- cmdsplit ----

func TestSplitCommandWords(t *testing.T) {
	argv, err := SplitCommandWords(`go test ./... -run 'Test A' -name "x y"`)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	want := []string{"go", "test", "./...", "-run", "Test A", "-name", "x y"}
	if len(argv) != len(want) {
		t.Fatalf("argv = %v, want %v", argv, want)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Fatalf("argv = %v, want %v", argv, want)
		}
	}
	for _, shell := range []string{"ls | grep x", "echo a > f", "echo $(whoami)", "a && b", "cat `f`"} {
		if _, err := SplitCommandWords(shell); err == nil {
			t.Errorf("shell syntax %q must be refused", shell)
		} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonAutoTestShellSyntax {
			t.Errorf("shell syntax %q code = %q, want %q", shell, rc, core.ReasonAutoTestShellSyntax)
		}
	}
	if _, err := SplitCommandWords("echo 'unbalanced"); err == nil {
		t.Error("unbalanced quote must fail")
	}
}

// ---- H8 auto-test ----

const autotestTestsDoc = `---
status: draft
---

# Tests

| AC ID | TC ID | Layer | Command | Assertion |
|---|---|---|---|---|
| AC-001 | TC-001 | cli | true | exit code 0 |
| AC-002 | TC-002 | cli | false | exit code 0 |
| AC-003 | TC-003 | cli |  | exit code 0 |
`

func TestRunAutoTestExecutesAndRegistersEvidence(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	if err := os.MkdirAll(env.root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.root, "tests.md"), []byte(autotestTestsDoc), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := env.eng.RunAutoTest(context.Background(), run.RunID, AutoTestOptions{By: "qa-bot"})
	if err == nil {
		t.Fatal("a failing TC must surface autotest_failed")
	}
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonAutoTestFailed {
		t.Fatalf("err code = %q, want %q", rc, core.ReasonAutoTestFailed)
	}
	if report.Passed != 1 || report.Failed != 1 || report.Blocked != 1 || report.Total != 3 {
		t.Fatalf("report tallies = %+v, want 1/1/1 of 3", report)
	}
	byTC := map[string]AutoTestTCResult{}
	for _, r := range report.Results {
		byTC[r.TC] = r
	}
	if byTC["TC-001"].Status != AutoTestPassed || byTC["TC-001"].ArtifactID == "" {
		t.Errorf("TC-001 = %+v, want passed with a registered artifact", byTC["TC-001"])
	}
	if byTC["TC-002"].Status != AutoTestFailed || byTC["TC-002"].ReasonCode != core.ReasonAutoTestLayerFailed {
		t.Errorf("TC-002 = %+v, want failed/auto_test_layer_failed", byTC["TC-002"])
	}
	if byTC["TC-003"].Status != AutoTestBlocked || byTC["TC-003"].ReasonCode != core.ReasonAutoTestCommandMissing {
		t.Errorf("TC-003 = %+v, want blocked/qa9_command_missing_auto_case", byTC["TC-003"])
	}
	// Results keep tests.md row order (blocked rows hold their slot).
	if len(report.Results) == 3 && (report.Results[0].TC != "TC-001" || report.Results[2].TC != "TC-003") {
		t.Errorf("result order = [%s %s %s], want tests.md order TC-001..TC-003",
			report.Results[0].TC, report.Results[1].TC, report.Results[2].TC)
	}
	// Evidence files on disk.
	resultPath := filepath.Join(env.root, ".beacon", "autotest", run.RunID, "TC-001", "result.json")
	if _, err := os.Stat(resultPath); err != nil {
		t.Errorf("per-TC evidence missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(env.root, ".beacon", "autotest", run.RunID, "report.json")); err != nil {
		t.Errorf("aggregate report missing: %v", err)
	}
	// The qa gate consumes the evidence with no special-casing.
	ctx, err := env.st.ResolveFeature(env.root, "feat-a", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.evalQA(ctx, "qa-bot"); err != nil {
		t.Fatalf("qa gate over autotest evidence: %v", err)
	}
	arts, err := env.st.Artifacts().ListByRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, a := range arts {
		kinds[a.Kind]++
		// The registered hash must match the FINAL bytes on disk:
		// the gate re-hashes evidence, and any post-registration
		// rewrite would read as stale.
		if a.Kind == "test_result" || a.Kind == "test_results" {
			raw, rerr := os.ReadFile(a.AbsPath)
			if rerr != nil {
				t.Fatalf("read registered artifact %s: %v", a.AbsPath, rerr)
			}
			sum := sha256.Sum256(raw)
			if hex.EncodeToString(sum[:]) != a.ContentHash {
				t.Errorf("artifact %s hash drift: registered %s, disk %s", a.Kind, a.ContentHash, hex.EncodeToString(sum[:]))
			}
		}
	}
	if kinds["test_result"] != 2 {
		t.Errorf("test_result artifacts = %d, want 2 (TC-001 + TC-002)", kinds["test_result"])
	}
	if kinds["test_results"] != 1 {
		t.Errorf("test_results artifacts = %d, want 1 (aggregate)", kinds["test_results"])
	}
}

func TestRunAutoTestRefusals(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	// No tests.md on disk at all.
	if _, err := env.eng.RunAutoTest(context.Background(), run.RunID, AutoTestOptions{By: "qa"}); err == nil {
		t.Fatal("missing tests doc must refuse")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonAutoTestTestsDocMissing {
		t.Fatalf("code = %q, want %q", rc, core.ReasonAutoTestTestsDocMissing)
	}
	// tests.md with only a BLOCKED row: nothing executable anywhere.
	doc := "| AC ID | TC ID | Layer | Command | Assertion |\n|---|---|---|---|---|\n| AC-001 | TC-001 | cli | BLOCKED: manual only | n/a |\n"
	if err := os.MkdirAll(env.root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.root, "tests.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.RunAutoTest(context.Background(), run.RunID, AutoTestOptions{By: "qa"}); err == nil {
		t.Fatal("no executable TC must refuse (no fake runs)")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonAutoTestNoExecutable {
		t.Fatalf("code = %q, want %q", rc, core.ReasonAutoTestNoExecutable)
	}
	// Fallback: project test_command makes the empty-command TC run.
	doc2 := "| AC ID | TC ID | Layer | Command | Assertion |\n|---|---|---|---|---|\n| AC-001 | TC-001 | cli |  | exit 0 |\n"
	if err := os.WriteFile(filepath.Join(env.root, "tests.md"), []byte(doc2), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(env.root, ".beacon"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.root, ".beacon", "config"), []byte("test_command = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := env.eng.RunAutoTest(context.Background(), run.RunID, AutoTestOptions{By: "qa"})
	if err != nil {
		t.Fatalf("fallback run: %v", err)
	}
	if report.Passed != 1 || report.FallbackCommand != "true" {
		t.Errorf("fallback report = %+v, want 1 passed via test_command", report)
	}
}

// ---- H10 state portability ----

func TestStateExportImportRoundTrip(t *testing.T) {
	env := newTestEnv(t, "active")
	// CreateRun initializes the lifecycle; set the stage after it.
	env.createRun(t)
	if err := env.st.Features().UpdateLifecycle(store.Lifecycle{
		FeatureID: env.feature.ID, Stage: core.StageQa, Status: core.StatusInProgress,
		ReasonCodes: []string{"evidence_pending"},
	}); err != nil {
		t.Fatal(err)
	}
	env.seedQAPass(t)
	env.seedTruthFrozen(t)

	snap, abs, err := env.eng.ExportProjectState(env.root)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("snapshot file: %v", err)
	}
	if len(snap.Features) != 1 || len(snap.Features[0].LatestGates) != 2 {
		t.Fatalf("snapshot = %+v, want 1 feature with 2 gate verdicts", snap.Features)
	}

	// Machine B: fresh DB, same repo directory.
	dbB := filepath.Join(t.TempDir(), "b.db")
	stB, err := store.Open(dbB)
	if err != nil {
		t.Fatal(err)
	}
	defer stB.Close()
	engB := NewEngine(stB)
	if !engB.AutoImportState(env.root) {
		t.Fatal("AutoImportState should recognize the snapshot-carrying repo")
	}
	res, err := engB.ImportProjectState(env.root)
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if res.FeaturesAdded != 0 || res.GatesRestored != 0 {
		t.Errorf("second import must fill nothing (DB wins), got %+v", res)
	}
	ctxB, err := stB.ResolveFeature(env.root, "feat-a", "")
	if err != nil {
		t.Fatalf("resolve on machine B: %v", err)
	}
	snapB, err := stB.Spine().Snapshot(ctxB.Feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snapB.Stage != core.StageQa {
		t.Errorf("machine B lifecycle stage = %q, want qa", snapB.Stage)
	}
	gateTypes := map[string]string{}
	for _, g := range snapB.LatestGates {
		gateTypes[g.GateType] = g.Verdict
	}
	if gateTypes["qa"] != "pass" || gateTypes["truth_freeze"] != "pass" {
		t.Errorf("machine B gates = %v, want qa+truth_freeze pass restored", gateTypes)
	}
	if snapB.RunCount != 0 {
		t.Errorf("runs are summaries only and must not be imported; RunCount = %d", snapB.RunCount)
	}
}

// ---- M10 archive ----

func writePkg(t *testing.T, root, versionRoot, slug, status string) {
	t.Helper()
	dir := filepath.Join(root, versionRoot, "features", slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nstatus: " + status + "\n---\n\n# Truth\n\nAC-001 and AC-002.\n"
	if err := os.WriteFile(filepath.Join(dir, "truth.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveScanLineageApply(t *testing.T) {
	root := t.TempDir()
	writePkg(t, root, filepath.Join("docs", "beacon", "archive", "v1.5.0"), "feat-x", "frozen")
	writePkg(t, root, filepath.Join("docs", "beacon", "v1.6.0"), "feat-x", "frozen")
	writePkg(t, root, filepath.Join("docs", "beacon", "v2.0.0"), "feat-x", "draft")
	writePkg(t, root, filepath.Join("docs", "beacon", "v2.0.0"), "feat-y", "draft")

	entries := ScanArchiveEntries(root)
	if len(entries) != 4 {
		t.Fatalf("entries = %d, want 4", len(entries))
	}
	hops := ArchiveLineage(root, "feat-x")
	if len(hops) != 3 {
		t.Fatalf("lineage hops = %d, want 3", len(hops))
	}
	if hops[0].Version != "v1.5.0" || hops[0].Location != "archive" || hops[2].Version != "v2.0.0" {
		t.Errorf("lineage order/locations wrong: %+v", hops)
	}
	if hops[1].ACCount != 2 {
		t.Errorf("AC count = %d, want 2", hops[1].ACCount)
	}

	if _, err := ApplyArchiveStatus(root, "v1.6.0", "feat-x", "bogus", ""); err == nil {
		t.Error("invalid archive status must refuse")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonArchiveStatusInvalid {
		t.Errorf("code = %q, want %q", rc, core.ReasonArchiveStatusInvalid)
	}
	res, err := ApplyArchiveStatus(root, "v1.6.0", "feat-x", "archived-learnable", "superseded by v2")
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(res.MetadataRef))); err != nil {
		t.Errorf("metadata file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(res.IndexRef))); err != nil {
		t.Errorf("index file: %v", err)
	}
	after := ArchiveLineage(root, "feat-x")
	if after[1].Status != "archived-learnable" || !after[1].Archived || !after[1].Learnable {
		t.Errorf("after apply, v1.6.0 hop = %+v, want archived-learnable", after[1])
	}
	if _, err := ApplyArchiveStatus(root, "v1.0.0", "feat-x", "active", ""); err == nil {
		t.Error("pre-1.3.8 versions must refuse governance")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonArchiveNotSupported {
		t.Errorf("code = %q, want %q", rc, core.ReasonArchiveNotSupported)
	}
}

// ---- M11 init ----

func TestInitProjectAssemblesContext(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	eng := NewEngine(st)
	root := filepath.Join(t.TempDir(), "new-proj")

	report, err := eng.InitProject(root, InitOptions{
		Version: "v1.0.0", ProjectMode: "greenfield", RuntimeVersion: "v-test",
		WithClaude: true, WithAgents: true,
	})
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if report.DocsVersion != "v1.0.0" || report.Status != "completed" {
		t.Fatalf("report = %+v", report)
	}
	summary, err := os.ReadFile(filepath.Join(root, "docs", "beacon", "v1.0.0", "SUMMARY.md"))
	if err != nil || string(summary) != "# Beacon Summary\n\n- status: draft\n" {
		t.Errorf("SUMMARY.md = %q, %v (want the Python template verbatim)", summary, err)
	}
	for _, rel := range []string{
		"docs/beacon/v1.0.0/execution/architecture-blueprint.md",
		"docs/beacon/v1.0.0/execution/version-evolution-blueprint.md",
		"docs/beacon/v1.0.0/release/v1.0.0.md",
		".beacon/state/project-onboarding.json",
		".beacon/state/init-report.json",
		"CLAUDE.md", "AGENTS.md",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Errorf("init file %s: %v", rel, err)
		}
	}
	// Registered with the version current.
	proj, err := st.Projects().GetByRootPath(store.ResolveRootPath(root))
	if err != nil {
		t.Fatalf("project not registered by init: %v", err)
	}
	cur, err := st.Projects().CurrentVersion(proj.ID)
	if err != nil || cur.Version != "v1.0.0" {
		t.Errorf("current version = %+v, %v", cur, err)
	}
	// Idempotent: a second init leaves existing files unchanged.
	report2, err := eng.InitProject(root, InitOptions{Version: "v1.0.0", WithClaude: true, WithAgents: true})
	if err != nil {
		t.Fatal(err)
	}
	unchanged := 0
	for _, a := range report2.Actions {
		if a.Action == "unchanged" {
			unchanged++
		}
	}
	if unchanged == 0 {
		t.Error("second init should report unchanged files")
	}
	// Takeover adds the two current-state files.
	root2 := filepath.Join(t.TempDir(), "takeover-proj")
	if _, err := eng.InitProject(root2, InitOptions{Version: "v1.0.0", ProjectMode: "takeover"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root2, "docs", "beacon", "v1.0.0", "execution", "current-state-service-map.md")); err != nil {
		t.Errorf("takeover service map: %v", err)
	}
	// Bad mode + dry-run discipline.
	if _, err := eng.InitProject(root2, InitOptions{Version: "v1.0.0", ProjectMode: "sideways"}); err == nil {
		t.Error("invalid project mode must refuse")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonProjectModeInvalid {
		t.Errorf("code = %q, want %q", rc, core.ReasonProjectModeInvalid)
	}
	root3 := filepath.Join(t.TempDir(), "dry-proj")
	if _, err := eng.InitProject(root3, InitOptions{Version: "v1.0.0", DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root3, "docs")); !os.IsNotExist(err) {
		t.Error("dry-run init must not write files")
	}
}

// ---- M12 hooks ----

const validHookContract = `---
hook_id: nightly-align
hook_code: post_release
enabled: true
trigger_policy: auto_allowed
failure_route: release
command_template: "true"
produces_evidence:
  - hook_run_record
---

## Purpose
Align the workspace after a release.

## Admission
Runs only when admitted by the hook contract checks.

## Evidence
Writes a run record under the hook state dir.

## Failure
Failures route to release and never rewrite any verdict.
`

func writeHook(t *testing.T, root, id, body string) {
	t.Helper()
	dir := filepath.Join(root, "docs", "beacon", "hooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHookAdmissionAndRun(t *testing.T) {
	root := t.TempDir()
	writeHook(t, root, "nightly-align", validHookContract)

	adm := ValidateHookContract(root, "nightly-align", "", "")
	if adm.Blocked {
		t.Fatalf("valid contract blocked: %+v", adm.Diagnostics)
	}
	// Dry run: planned evidence only, no execution.
	res := RunHook(context.Background(), root, "nightly-align", HookRunOptions{DryRun: true})
	if res.Status != "planned" || res.SideEffectExecuted {
		t.Errorf("dry run = %+v, want planned without side effects", res)
	}
	if _, err := os.Stat(filepath.Join(root, ".beacon", "state", "hooks", "nightly-align", res.RunID+".json")); err != nil {
		t.Errorf("run evidence: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "beacon", ".machine", "hooks", "nightly-align.latest.json")); err != nil {
		t.Errorf("latest read model: %v", err)
	}
	// Unconfirmed real run of an auto_allowed hook is refused.
	res = RunHook(context.Background(), root, "nightly-align", HookRunOptions{})
	if res.ReasonCode != core.ReasonHookHiddenAutoRun {
		t.Errorf("unconfirmed run code = %q, want %q", res.ReasonCode, core.ReasonHookHiddenAutoRun)
	}
	// Confirmed: the template truly executes through the harness.
	res = RunHook(context.Background(), root, "nightly-align", HookRunOptions{Confirm: true})
	if res.Status != "passed" || !res.SideEffectExecuted {
		t.Fatalf("confirmed run = %+v, want passed+executed", res)
	}
	if code, _ := res.Execution["exit_code"].(int); code != 0 {
		t.Errorf("execution = %+v, want exit 0", res.Execution)
	}
	// Status face sees the latest run.
	st := HookStatus(root, "nightly-align", "")
	if st["status"] != "ready" {
		t.Errorf("hook status = %v, want ready", st["status"])
	}
	// Disable flips enabled and admission then blocks.
	if _, code, err := DisableHook(root, "nightly-align"); err != nil || code != "" {
		t.Fatalf("disable: code=%q err=%v", code, err)
	}
	if adm := ValidateHookContract(root, "nightly-align", "", ""); !adm.Blocked {
		t.Error("disabled hook must fail admission")
	}
}

func TestHookAdmissionDiagnostics(t *testing.T) {
	root := t.TempDir()
	broken := "---\nhook_id: other\nhook_code: whenever\nenabled: true\ntrigger_policy: auto_allowed\nfailure_route: release\nskill: some-skill\n---\n\nno sections here\n"
	writeHook(t, root, "broken", broken)
	adm := ValidateHookContract(root, "broken", "", "")
	if !adm.Blocked {
		t.Fatal("broken contract must block")
	}
	codes := map[string]bool{}
	for _, d := range adm.Diagnostics {
		codes[d.ReasonCode] = true
	}
	for _, want := range []string{"hook_id_mismatch", "unknown_hook_code", "required_evidence_missing", "section_missing_purpose"} {
		if !codes[want] {
			t.Errorf("diagnostics %v missing %q", adm.ReasonCodes, want)
		}
	}
	// Authority bypass language is caught.
	evil := strings.Replace(validHookContract, "nightly-align", "evil", 1)
	evil = strings.Replace(evil, "Align the workspace after a release.", "This hook can set qa verdict directly.", 1)
	writeHook(t, root, "evil", evil)
	adm = ValidateHookContract(root, "evil", "", "")
	found := false
	for _, d := range adm.Diagnostics {
		if d.ReasonCode == "hook_authority_bypass" {
			found = true
		}
	}
	if !found {
		t.Errorf("authority bypass not caught: %v", adm.ReasonCodes)
	}
	// Missing file.
	adm = ValidateHookContract(root, "ghost", "", "")
	if !adm.Blocked || len(adm.ReasonCodes) == 0 || adm.ReasonCodes[0] != core.ReasonHookFileMissing {
		t.Errorf("missing hook admission = %+v", adm)
	}
	// An ABSENT hook_id is a mismatch too (Python compares
	// front.get("hook_id") — None — against the file stem).
	noid := strings.Replace(validHookContract, "hook_id: nightly-align\n", "", 1)
	noid = strings.Replace(noid, "nightly-align", "noid", 1)
	writeHook(t, root, "noid", noid)
	adm = ValidateHookContract(root, "noid", "", "")
	found = false
	for _, d := range adm.Diagnostics {
		if d.ReasonCode == "hook_id_mismatch" {
			found = true
		}
	}
	if !found {
		t.Errorf("absent hook_id must mismatch: %v", adm.ReasonCodes)
	}
}

func TestPostReleaseHookTriggerAndBuiltinInstall(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	writeHook(t, env.root, "nightly-align", validHookContract)
	manual := strings.Replace(validHookContract, "nightly-align", "manual-align", 1)
	manual = strings.Replace(manual, "auto_allowed", "manual_confirmed", 1)
	writeHook(t, env.root, "manual-align", manual)

	summary, err := env.eng.RunPostReleaseHooks(run.RunID)
	if err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if len(summary.Hooks) != 2 {
		t.Fatalf("triggered hooks = %+v, want 2", summary.Hooks)
	}
	byID := map[string]HookTriggerEntry{}
	for _, h := range summary.Hooks {
		byID[h.HookID] = h
	}
	if byID["nightly-align"].Action != "executed" || byID["nightly-align"].Status != "passed" {
		t.Errorf("auto hook = %+v, want executed/passed", byID["nightly-align"])
	}
	if byID["manual-align"].Action != "awaiting_confirmation" {
		t.Errorf("manual hook = %+v, want awaiting_confirmation (never silently run)", byID["manual-align"])
	}
	events, err := env.st.ListEventsByRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range events {
		if ev.EventType == EventPostReleaseHooks {
			found = true
		}
	}
	if !found {
		t.Error("post_release_hooks_triggered event missing")
	}

	// The builtin install hook materializes the host skill dirs.
	home := t.TempDir()
	builtin := strings.Replace(validHookContract, "nightly-align", PostReleaseInstallHookID, 1)
	builtin = strings.Replace(builtin, "auto_allowed", "manual_confirmed", 1)
	builtin = strings.Replace(builtin, `command_template: "true"`, "skill: beacon-post-release-local-install", 1)
	writeHook(t, env.root, PostReleaseInstallHookID, builtin)
	res := RunHook(context.Background(), env.root, PostReleaseInstallHookID, HookRunOptions{Confirm: true, Home: home})
	if res.Status != "passed" {
		t.Fatalf("builtin run = %+v, want passed", res)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "beacon-plan", "SKILL.md")); err != nil {
		t.Errorf("claude skill install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "beacon-goal", "SKILL.md")); err != nil {
		t.Errorf("agents skill install: %v", err)
	}
}
