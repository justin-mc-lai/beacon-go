package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// runCLI executes a fresh root command and returns stdout + error.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCmd()
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

// parseDoc asserts the output is exactly one JSON document.
func parseDoc(t *testing.T, s string) map[string]any {
	t.Helper()
	var doc map[string]any
	dec := json.NewDecoder(bytes.NewReader([]byte(s)))
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, s)
	}
	if dec.More() {
		t.Fatalf("stdout contains more than one JSON document:\n%s", s)
	}
	return doc
}

func assertKeys(t *testing.T, doc map[string]any, keys ...string) {
	t.Helper()
	for _, k := range keys {
		if _, ok := doc[k]; !ok {
			t.Errorf("JSON missing key %q (have %v)", k, doc)
		}
	}
}

func strField(t *testing.T, doc map[string]any, key string) string {
	t.Helper()
	v, ok := doc[key].(string)
	if !ok {
		t.Fatalf("field %q = %v, want string", key, doc[key])
	}
	return v
}

// seedOneProject creates (via DAO) one project/version/feature with a
// blocked lifecycle, one running run, two artifact pointers (one real
// blob, one missing) and three gate rows (qa blocked->pass, release
// blocked). It returns the ids/paths a CLI test needs.
type seedResult struct {
	dbPath, projRoot, blobPath string
	project                    store.Project
	feature                    store.FeaturePackage
	run                        store.Run
}

func seedOneProject(t *testing.T) seedResult {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	t.Setenv(store.EnvStorePath, dbPath)
	projRoot := filepath.Join(t.TempDir(), "demo-proj")
	if err := os.MkdirAll(projRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	projRoot = store.ResolveRootPath(projRoot)
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p, err := st.Projects().Register(store.Project{RootPath: projRoot, Name: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := st.Projects().AddVersion(store.ProjectVersion{ProjectID: p.ID, Version: "v2.0.0", DocsRoot: "docs/beacon/v2.0.0", IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	f, err := st.Features().CreatePackage(store.FeaturePackage{ProjectVersionID: v.ID, Slug: "demo-feature", TruthPath: "truth.md", TestsPath: "tests.md"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Features().UpdateLifecycle(store.Lifecycle{FeatureID: f.ID, Stage: core.StageQa, Status: core.StatusBlocked, ReasonCodes: []string{"evidence_missing"}}); err != nil {
		t.Fatal(err)
	}
	r, err := st.Runs().CreateRun(store.Run{FeatureID: f.ID, RunKind: "goal", Status: core.RunRunning, Iteration: 2})
	if err != nil {
		t.Fatal(err)
	}
	blobPath := filepath.Join(t.TempDir(), "evidence.json")
	if err := os.WriteFile(blobPath, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(`{"ok":true}`))
	if _, err := st.Artifacts().Add(store.ArtifactRef{RunID: r.ID, Kind: "evidence", AbsPath: blobPath, ContentHash: hex.EncodeToString(digest[:]), SizeBytes: 10}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Artifacts().Add(store.ArtifactRef{RunID: r.ID, Kind: "log", AbsPath: filepath.Join(t.TempDir(), "gone.log"), ContentHash: "deadbeef"}); err != nil {
		t.Fatal(err)
	}
	for _, g := range []store.GateResult{
		{FeatureID: f.ID, GateType: "qa", Verdict: "blocked", ReasonCodes: []string{"evidence_missing"}, EvaluatedAt: "2026-10-21T01:00:00Z"},
		{FeatureID: f.ID, GateType: "qa", Verdict: "pass", EvaluatedAt: "2026-10-21T02:00:00Z"},
		{FeatureID: f.ID, GateType: "release", Verdict: "blocked", ReasonCodes: []string{"qa_not_passed"}, EvaluatedAt: "2026-10-21T01:30:00Z"},
	} {
		if _, err := st.Gates().Record(g); err != nil {
			t.Fatal(err)
		}
	}
	return seedResult{dbPath: dbPath, projRoot: projRoot, blobPath: blobPath, project: p, feature: f, run: r}
}

// AC-021/TC-021: register -> list -> show closure, idempotent register,
// show carries project_key, versions[] and is_current.
func TestProjectsRegisterListShowClosure(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	t.Setenv(store.EnvStorePath, dbPath)
	projDir := filepath.Join(t.TempDir(), "my-app")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	resolved := store.ResolveRootPath(projDir)

	out, err := runCLI(t, "projects", "register", "--path", projDir, "--version", "v2.0.0", "--json")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	doc := parseDoc(t, out)
	assertKeys(t, doc, "id", "project_key", "root_path", "name", "registered_at", "current_version", "versions")
	if got := strField(t, doc, "project_key"); got != store.ProjectKey(projDir) {
		t.Errorf("project_key = %q want %q", got, store.ProjectKey(projDir))
	}
	if got := strField(t, doc, "root_path"); got != resolved {
		t.Errorf("root_path = %q want resolved %q", got, resolved)
	}
	if got := strField(t, doc, "current_version"); got != "v2.0.0" {
		t.Errorf("current_version = %q", got)
	}
	versions, ok := doc["versions"].([]any)
	if !ok || len(versions) != 1 {
		t.Fatalf("versions = %v want 1 entry", doc["versions"])
	}
	v0 := versions[0].(map[string]any)
	if v0["is_current"] != true || v0["version"] != "v2.0.0" {
		t.Errorf("versions[0] = %v", v0)
	}
	firstID := strField(t, doc, "id")

	// Idempotent: same root path returns the same record, no duplicate.
	out, err = runCLI(t, "projects", "register", "--path", projDir, "--version", "v2.0.0", "--json")
	if err != nil {
		t.Fatalf("re-register: %v", err)
	}
	if got := strField(t, parseDoc(t, out), "id"); got != firstID {
		t.Errorf("re-register id = %q want same %q", got, firstID)
	}
	out, err = runCLI(t, "projects", "list", "--json")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	listDoc := parseDoc(t, out)
	projects, ok := listDoc["projects"].([]any)
	if !ok || len(projects) != 1 {
		t.Fatalf("projects list = %v want exactly 1", listDoc["projects"])
	}
	out, err = runCLI(t, "projects", "show", "--project", projDir, "--json")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	showDoc := parseDoc(t, out)
	if strField(t, showDoc, "project_key") != store.ProjectKey(projDir) {
		t.Errorf("show project_key mismatch: %v", showDoc)
	}
}

// AC-017/019/020/065 + TC-015/017/019/020: spine show full fields, gate
// read-only view immune to project JSON files (AC-006), artifacts stale.
func TestSpineShowGateArtifacts(t *testing.T) {
	seed := seedOneProject(t)

	out, err := runCLI(t, "spine", "show", "--project", seed.projRoot, "--feature", "demo-feature", "--json")
	if err != nil {
		t.Fatalf("spine show: %v", err)
	}
	doc := parseDoc(t, out)
	assertKeys(t, doc, "run_id", "project_key", "project_root", "version", "feature_slug",
		"lifecycle_stage", "lifecycle_status", "phase", "phases_completed", "phase_promoted",
		"next_harness", "next_action", "progress_status", "active_run_id", "run_status",
		"iteration", "gate_results", "artifact_refs", "blocking_reason_codes", "updated_at")
	if got := strField(t, doc, "feature_slug"); got != "demo-feature" {
		t.Errorf("feature_slug = %q", got)
	}
	if got := strField(t, doc, "lifecycle_stage"); got != "Qa" {
		t.Errorf("lifecycle_stage = %q", got)
	}
	if got := strField(t, doc, "run_status"); got != "Running" {
		t.Errorf("run_status = %q", got)
	}
	if _, ok := doc["phase_promoted"].(bool); !ok {
		t.Errorf("phase_promoted type = %T", doc["phase_promoted"])
	}
	if _, ok := doc["phases_completed"].([]any); !ok {
		t.Errorf("phases_completed type = %T", doc["phases_completed"])
	}
	gates, _ := doc["gate_results"].([]any)
	if len(gates) != 2 {
		t.Errorf("gate_results = %v want latest-per-type (2)", gates)
	}
	refs, _ := doc["artifact_refs"].([]any)
	if len(refs) != 2 {
		t.Errorf("artifact_refs = %v want 2", refs)
	}

	// Human output smoke.
	out, err = runCLI(t, "spine", "show", "--project", seed.projRoot, "--feature", "demo-feature")
	if err != nil || !bytes.Contains([]byte(out), []byte("demo-feature")) {
		t.Errorf("human show = %q, %v", out, err)
	}

	// Gate: latest qa verdict is pass in DB.
	out, err = runCLI(t, "spine", "gate", "demo-feature", "--json")
	if err != nil {
		t.Fatalf("spine gate: %v", err)
	}
	gateDoc := parseDoc(t, out)
	assertKeys(t, gateDoc, "feature_slug", "version", "project_key", "gate_results")
	verdicts := map[string]string{}
	for _, g := range gateDoc["gate_results"].([]any) {
		gm := g.(map[string]any)
		verdicts[gm["gate_type"].(string)] = gm["verdict"].(string)
	}
	if verdicts["qa"] != "pass" || verdicts["release"] != "blocked" {
		t.Errorf("gate verdicts = %v", verdicts)
	}
	// AC-006: a contradicting project JSON file must not change the
	// DB-only result, and the read must not write anything back.
	poison := filepath.Join(seed.projRoot, ".beacon", "gate-report.json")
	if err := os.MkdirAll(filepath.Dir(poison), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(poison, []byte(`{"qa":"fail","release":"pass"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = runCLI(t, "spine", "gate", "demo-feature", "--json")
	if err != nil {
		t.Fatalf("spine gate after poison file: %v", err)
	}
	verdicts2 := map[string]string{}
	for _, g := range parseDoc(t, out)["gate_results"].([]any) {
		gm := g.(map[string]any)
		verdicts2[gm["gate_type"].(string)] = gm["verdict"].(string)
	}
	if verdicts2["qa"] != "pass" || verdicts2["release"] != "blocked" {
		t.Errorf("gate verdicts changed by project JSON file: %v", verdicts2)
	}
	st, err := store.OpenWithOptions(seed.dbPath, store.OpenOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := st.Gates().ListByFeature(seed.feature.ID)
	st.Close()
	if err != nil || len(rows) != 3 {
		t.Errorf("gate rows after read commands = %d, %v want 3 (read-only)", len(rows), err)
	}
	// Version filter miss -> feature_not_found.
	_, err = runCLI(t, "spine", "gate", "demo-feature", "--version", "v9.9.9", "--json")
	if rc, _ := core.ReasonCodeOf(err); rc != ReasonFeatureNotFound {
		t.Errorf("gate --version miss reason = %q (err %v)", rc, err)
	}

	// Artifacts: real blob fresh, missing blob stale(file_missing).
	out, err = runCLI(t, "spine", "artifacts", seed.run.ID, "--json")
	if err != nil {
		t.Fatalf("spine artifacts: %v", err)
	}
	artDoc := parseDoc(t, out)
	if strField(t, artDoc, "run_id") != seed.run.ID {
		t.Errorf("artifacts run_id = %v", artDoc["run_id"])
	}
	staleByKind := map[string]map[string]any{}
	for _, a := range artDoc["artifact_refs"].([]any) {
		am := a.(map[string]any)
		staleByKind[am["kind"].(string)] = am
	}
	if staleByKind["evidence"]["stale"] != false {
		t.Errorf("evidence artifact = %v want stale=false", staleByKind["evidence"])
	}
	if staleByKind["log"]["stale"] != true || staleByKind["log"]["stale_reason"] != "file_missing" {
		t.Errorf("log artifact = %v want stale/file_missing", staleByKind["log"])
	}

	// TC-015 DB-first: delete the only real blob; show still returns
	// lifecycle + gates.
	if err := os.Remove(seed.blobPath); err != nil {
		t.Fatal(err)
	}
	out, err = runCLI(t, "spine", "show", "--project", seed.projRoot, "--feature", "demo-feature", "--json")
	if err != nil {
		t.Fatalf("spine show without blobs: %v", err)
	}
	doc = parseDoc(t, out)
	if strField(t, doc, "lifecycle_stage") != "Qa" || strField(t, doc, "lifecycle_status") != "Blocked" {
		t.Errorf("show without blobs lost lifecycle: %v", doc)
	}
}

// AC-018/082 via CLI: list --status hits on either axis and misses clean.
func TestSpineListStatusFilterCLI(t *testing.T) {
	seedOneProject(t)
	out, err := runCLI(t, "spine", "list", "--status", "Running", "--json")
	if err != nil {
		t.Fatalf("list Running: %v", err)
	}
	snaps, _ := parseDoc(t, out)["snapshots"].([]any)
	if len(snaps) != 1 || snaps[0].(map[string]any)["feature_slug"] != "demo-feature" {
		t.Errorf("list --status Running = %v", snaps)
	}
	out, err = runCLI(t, "spine", "list", "--status", "Blocked", "--json")
	if err != nil {
		t.Fatalf("list Blocked: %v", err)
	}
	if snaps, _ := parseDoc(t, out)["snapshots"].([]any); len(snaps) != 1 {
		t.Errorf("list --status Blocked (lifecycle axis) = %d want 1", len(snaps))
	}
	out, err = runCLI(t, "spine", "list", "--status", "Closed", "--json")
	if err != nil {
		t.Fatalf("list Closed: %v", err)
	}
	if snaps, _ := parseDoc(t, out)["snapshots"].([]any); len(snaps) != 0 {
		t.Errorf("list --status Closed = %d want 0", len(snaps))
	}
	// Unknown project filter -> project_not_registered.
	_, err = runCLI(t, "spine", "list", "--project", filepath.Join(t.TempDir(), "nope"), "--json")
	if rc, _ := core.ReasonCodeOf(err); rc != ReasonProjectNotRegistered {
		t.Errorf("list unknown project reason = %q (err %v)", rc, err)
	}
}

// Not-found surface: every lookup failure is a non-zero, reason-coded
// error — never a panic, never partial JSON on stdout.
func TestNotFoundReasonCodes(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	t.Setenv(store.EnvStorePath, dbPath)

	// No DB file at all yet.
	out, err := runCLI(t, "spine", "show", "--project", "/tmp/definitely-not-registered", "--json")
	if rc, _ := core.ReasonCodeOf(err); rc != ReasonProjectNotRegistered || out != "" {
		t.Errorf("show without DB = out %q reason %q (err %v)", out, rc, err)
	}
	if _, err := runCLI(t, "spine", "artifacts", "run_nope", "--json"); err == nil {
		t.Error("artifacts without DB must fail")
	} else if rc, _ := core.ReasonCodeOf(err); rc != ReasonRunNotFound {
		t.Errorf("artifacts reason = %q", rc)
	}
	if _, err := runCLI(t, "spine", "gate", "nope", "--json"); err == nil {
		t.Error("gate without DB must fail")
	} else if rc, _ := core.ReasonCodeOf(err); rc != ReasonFeatureNotFound {
		t.Errorf("gate reason = %q", rc)
	}
	if _, err := runCLI(t, "projects", "show", "--project", "/tmp/definitely-not-registered", "--json"); err == nil {
		t.Error("projects show without DB must fail")
	} else if rc, _ := core.ReasonCodeOf(err); rc != ReasonProjectNotRegistered {
		t.Errorf("projects show reason = %q", rc)
	}
	// projects list on an empty machine is a valid empty list, not an error.
	out, err = runCLI(t, "projects", "list", "--json")
	if err != nil {
		t.Fatalf("projects list without DB: %v", err)
	}
	if got := parseDoc(t, out)["projects"].([]any); len(got) != 0 {
		t.Errorf("projects list without DB = %v", got)
	}

	// With a seeded DB: unknown feature / run inside a known project.
	seed := seedOneProject(t)
	_ = seed
	_, err = runCLI(t, "spine", "show", "--project", seed.projRoot, "--feature", "nope", "--json")
	if rc, _ := core.ReasonCodeOf(err); rc != ReasonFeatureNotFound {
		t.Errorf("show unknown feature reason = %q (err %v)", rc, err)
	}
	_, err = runCLI(t, "spine", "artifacts", "run_unknown", "--json")
	if rc, _ := core.ReasonCodeOf(err); rc != ReasonRunNotFound {
		t.Errorf("artifacts unknown run reason = %q (err %v)", rc, err)
	}
}

// spine show with no selectors resolves the single registered project.
func TestSpineShowSingleProjectDefault(t *testing.T) {
	seedOneProject(t)
	out, err := runCLI(t, "spine", "show", "--json")
	if err != nil {
		t.Fatalf("spine show default: %v", err)
	}
	if got := strField(t, parseDoc(t, out), "feature_slug"); got != "demo-feature" {
		t.Errorf("default show feature = %q", got)
	}
}

func TestHarnessVerifyJSON(t *testing.T) {
	cmd := newQAVerifyHarnessCmd()
	cmd.SetArgs([]string{"--slug", "test-feature", "--json"})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error executing verify-harness: %v", err)
	}
	var res map[string]any
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse JSON from verify-harness: %v, raw: %s", err, buf.String())
	}
	if res["status"] != "passed" || res["slug"] != "test-feature" || res["exit_code"].(float64) != 0 {
		t.Errorf("unexpected json result: %v", res)
	}
	if res["evidence_hash"] == "" {
		t.Errorf("expected non-empty evidence_hash")
	}
}
