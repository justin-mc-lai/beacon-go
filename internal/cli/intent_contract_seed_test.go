// intent-contract-bridge (plan stage): executable acceptance seed.
//
// This test is the feature's acceptance seed, not its proof of
// delivery: it FAILS on main today (the record command does not exist)
// and must be turned green by the implement round without editing what
// it asserts.
//
//	AC-001  the aligned briefing lands in <pkg>/intent.json and, when a
//	        run exists, in an intent_recorded event on that run
//
// It goes through the public CLI only, so it compiles against main and
// fails by assertion.
package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"beacon-go/internal/store"
)

const (
	seedIntentRecordEvent = "intent_recorded"
	seedIntentDocSchema   = "beacon-intent-v1"
	seedIntentRelPath     = "docs/beacon/v2.0.0/features/intent-demo/intent.json"
	seedIntentRestated    = "把 AlignIntentStream 的结构化结果写进包内 intent.json，并记录到 run 事件"
	seedIntentOriginal    = "让意图对齐落地：intent.json 加 run 事件"
	seedIntentRisk        = "没有意图记录的自动跑无法复核"
	seedIntentBoundary    = "不改 release 语义"
	seedIntentSlug        = "intent-contract-bridge"
)

func seedStr(t *testing.T, m map[string]any, key string) string {
	t.Helper()
	v, ok := m[key].(string)
	if !ok {
		t.Fatalf("field %q = %v, want string (document: %v)", key, m[key], m)
	}
	return v
}

func seedChild(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	v, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("field %q = %v, want object (document: %v)", key, m[key], m)
	}
	return v
}

func seedListLen(m map[string]any, key string) int {
	v, _ := m[key].([]any)
	return len(v)
}

// seedStrList asserts the field is a string array and returns it, so
// the seed can compare values and not just counts.
func seedStrList(t *testing.T, m map[string]any, key string) []string {
	t.Helper()
	items, ok := m[key].([]any)
	if !ok {
		t.Fatalf("field %q = %v, want an array of strings (document: %v)", key, m[key], m)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			t.Fatalf("field %q item = %v, want string", key, item)
		}
		out = append(out, s)
	}
	return out
}

func seedWantList(t *testing.T, m map[string]any, key string, want ...string) {
	t.Helper()
	got := seedStrList(t, m, key)
	if len(got) != len(want) {
		t.Fatalf("field %q = %v, want %v", key, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("field %q[%d] = %q, want %q (all: %v)", key, i, got[i], want[i], got)
		}
	}
}

// seedIntentProject registers one project + v2.0.0 feature package with
// a real truth.md on disk and returns root + package dir.
func seedIntentProject(t *testing.T) (string, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	t.Setenv(store.EnvStorePath, dbPath)

	root := filepath.Join(t.TempDir(), "intent-proj")
	pkgDir := filepath.Join(root, "docs", "beacon", "v2.0.0", "features", "intent-demo")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	truthDoc := "---\nslug: intent-demo\nversion: v2.0.0\nstatus: draft\n---\n\n# Requirement Truth: intent-demo\n"
	for _, name := range []string{"truth.md", "tests.md", "tasks.md", "evidence.md"} {
		body := "# " + name + "\n"
		if name == "truth.md" {
			body = truthDoc
		}
		if err := os.WriteFile(filepath.Join(pkgDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p, err := st.Projects().Register(store.Project{RootPath: root, Name: "intent-proj"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := st.Projects().AddVersion(store.ProjectVersion{
		ProjectID: p.ID, Version: "v2.0.0", DocsRoot: "docs/beacon/v2.0.0", IsCurrent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	rel := "docs/beacon/v2.0.0/features/intent-demo/"
	if _, err := st.Features().CreatePackage(store.FeaturePackage{
		ProjectVersionID: v.ID, Slug: "intent-demo",
		TruthPath: rel + "truth.md", TestsPath: rel + "tests.md", Status: "draft",
	}); err != nil {
		t.Fatal(err)
	}
	return root, pkgDir
}

// seedBriefingFile writes the structured result AlignIntentStream would
// return and returns its path, bytes, and sha256. confirmed selects the
// confirmation state the briefing carries, so a seed can prove the
// recorded document mirrors it instead of assuming true.
func seedBriefingFile(t *testing.T, confirmed bool) (string, []byte, string) {
	t.Helper()
	briefing := map[string]any{
		"original_prompt":  seedIntentOriginal,
		"restated_intent":  seedIntentRestated,
		"identified_risks": []string{seedIntentRisk},
		"boundaries":       []string{seedIntentBoundary},
		"proposed_slugs":   []string{seedIntentSlug},
		"confirmed":        confirmed,
		"degraded":         true,
	}
	briefRaw, err := json.MarshalIndent(briefing, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	briefPath := filepath.Join(t.TempDir(), "briefing.json")
	if err := os.WriteFile(briefPath, briefRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(briefRaw)
	return briefPath, briefRaw, hex.EncodeToString(sum[:])
}

// seedCreateManualRun creates one run through the public CLI and
// returns its id.
func seedCreateManualRun(t *testing.T, root string) string {
	t.Helper()
	out, err := runCLI(t, "goal", "run", "--project", root, "--feature", "intent-demo",
		"--executor", "manual", "--executor-id", "seed", "--json")
	if err != nil {
		t.Fatalf("goal run: %v (stdout %q)", err, out)
	}
	return strField(t, parseDoc(t, out), "run_id")
}

// seedIntentRecordEvents returns every decoded intent_recorded payload
// on one run, in trace order.
func seedIntentRecordEvents(t *testing.T, runID string) []map[string]any {
	t.Helper()
	out, err := runCLI(t, "goal", "trace", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("goal trace %s: %v (stdout %q)", runID, err, out)
	}
	var trace map[string]any
	if err := json.Unmarshal([]byte(out), &trace); err != nil {
		t.Fatalf("goal trace %s is not one JSON document: %v\n%s", runID, err, out)
	}
	events, _ := trace["events"].([]any)
	var payloads []map[string]any
	for _, item := range events {
		ev, ok := item.(map[string]any)
		if !ok || ev["event_type"] != seedIntentRecordEvent {
			continue
		}
		p, ok := ev["payload_json"].(map[string]any)
		if !ok {
			t.Fatalf("%s on run %s has no decoded payload: %v", seedIntentRecordEvent, runID, ev)
		}
		payloads = append(payloads, p)
	}
	return payloads
}

// AC-001: recording the aligned briefing writes the package file and
// the run event, and both carry the same bytes. Without a run the
// command only writes the file and reports an empty run_id.
func TestIntentRecordLandsBriefingInPackageAndRun(t *testing.T) {
	root, pkgDir := seedIntentProject(t)

	out, err := runCLI(t, "goal", "run", "--project", root, "--feature", "intent-demo",
		"--executor", "manual", "--executor-id", "seed", "--json")
	if err != nil {
		t.Fatalf("goal run: %v (stdout %q)", err, out)
	}
	runID := strField(t, parseDoc(t, out), "run_id")

	briefPath, briefRaw, briefHash := seedBriefingFile(t, true)

	out, err = runCLI(t, "truth", "intent", "record", "--project", root, "--feature", "intent-demo",
		"--briefing", briefPath, "--run-id", runID, "--by", "chief-of-staff", "--json")
	if err != nil {
		t.Fatalf("truth intent record: %v (stdout %q); want the aligned briefing landed in the package and the run", err, out)
	}
	rec := parseDoc(t, out)
	if got := seedStr(t, rec, "run_id"); got != runID {
		t.Fatalf("record output run_id = %q, want the target run %q", got, runID)
	}

	intentPath := filepath.Join(pkgDir, "intent.json")
	raw, err := os.ReadFile(intentPath)
	if err != nil {
		t.Fatalf("intent.json not written at %s: %v", intentPath, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("intent.json is not valid JSON: %v\n%s", err, raw)
	}
	if got := seedStr(t, doc, "schema"); got != seedIntentDocSchema {
		t.Fatalf("intent.json schema = %q, want %q", got, seedIntentDocSchema)
	}
	if seedStr(t, doc, "feature_slug") != "intent-demo" || seedStr(t, doc, "version") != "v2.0.0" {
		t.Fatalf("intent.json feature/version = %v/%v", doc["feature_slug"], doc["version"])
	}
	if got := seedStr(t, doc, "truth_path"); got != "docs/beacon/v2.0.0/features/intent-demo/truth.md" {
		t.Fatalf("intent.json truth_path = %q", got)
	}
	recordedAt := seedStr(t, doc, "recorded_at")
	parsedAt, err := time.Parse(time.RFC3339, recordedAt)
	if err != nil {
		t.Fatalf("intent.json recorded_at %v is not RFC3339: %v", recordedAt, err)
	}
	if parsedAt.Location() != time.UTC || !strings.HasSuffix(recordedAt, "Z") {
		t.Fatalf("intent.json recorded_at = %q, want RFC3339 in UTC (Z)", recordedAt)
	}
	if seedStr(t, doc, "recorded_by") != "chief-of-staff" || doc["confirmed"] != true {
		t.Fatalf("intent.json recorded_by/confirmed = %v/%v", doc["recorded_by"], doc["confirmed"])
	}
	brief := seedChild(t, doc, "briefing")
	if got := seedStr(t, brief, "original_prompt"); got != seedIntentOriginal {
		t.Fatalf("intent.json briefing.original_prompt = %q, want %q", got, seedIntentOriginal)
	}
	if got := seedStr(t, brief, "restated_intent"); got != seedIntentRestated {
		t.Fatalf("intent.json briefing.restated_intent = %q", got)
	}
	seedWantList(t, brief, "identified_risks", seedIntentRisk)
	seedWantList(t, brief, "boundaries", seedIntentBoundary)
	seedWantList(t, brief, "proposed_slugs", seedIntentSlug)
	if degraded, ok := brief["degraded"].(bool); !ok || !degraded {
		t.Fatalf("intent.json briefing.degraded = %v, want the briefing value true preserved", brief["degraded"])
	}
	if brief["confirmed"] != true {
		t.Fatalf("intent.json briefing.confirmed = %v, want true", brief["confirmed"])
	}

	sum := sha256.Sum256(raw)
	intentHash := hex.EncodeToString(sum[:])

	traceOut, err := runCLI(t, "goal", "trace", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("goal trace: %v (stdout %q)", err, traceOut)
	}
	var trace map[string]any
	if err := json.Unmarshal([]byte(traceOut), &trace); err != nil {
		t.Fatalf("goal trace is not one JSON document: %v\n%s", err, traceOut)
	}
	events, _ := trace["events"].([]any)
	var payload map[string]any
	for _, item := range events {
		ev, ok := item.(map[string]any)
		if !ok || seedStr(t, ev, "event_type") != seedIntentRecordEvent {
			continue
		}
		if p, ok := ev["payload_json"].(map[string]any); ok {
			payload = p
		}
	}
	if payload == nil {
		t.Fatalf("run %s has no %s event after recording the briefing (events: %d)", runID, seedIntentRecordEvent, len(events))
	}
	if got := seedStr(t, payload, "intent_sha256"); got != intentHash {
		t.Fatalf("%s intent_sha256 = %q, want sha256 of the file bytes %q", seedIntentRecordEvent, got, intentHash)
	}
	if got := seedStr(t, payload, "briefing_sha256"); got != briefHash {
		t.Fatalf("%s briefing_sha256 = %q, want sha256 of the briefing bytes %q", seedIntentRecordEvent, got, briefHash)
	}
	if got := seedStr(t, payload, "intent_path"); got != seedIntentRelPath {
		t.Fatalf("%s intent_path = %q", seedIntentRecordEvent, got)
	}
	if got := seedStr(t, payload, "restated_intent"); got != seedIntentRestated || payload["confirmed"] != true {
		t.Fatalf("%s payload does not mirror the briefing: %v", seedIntentRecordEvent, payload)
	}
	seedWantList(t, payload, "risks", seedIntentRisk)
	seedWantList(t, payload, "boundaries", seedIntentBoundary)
	seedWantList(t, payload, "proposed_slugs", seedIntentSlug)
	if seedStr(t, payload, "recorded_by") != "chief-of-staff" {
		t.Fatalf("%s payload recorded_by = %v", seedIntentRecordEvent, payload["recorded_by"])
	}
	if briefRaw == nil {
		t.Fatal("briefing bytes were not produced")
	}

	// No run: the same command must only write the file, return an
	// empty run_id, and never create a run as a side effect.
	t.Run("without_run_only_writes_file", func(t *testing.T) {
		root, pkgDir := seedIntentProject(t)
		briefPath, _, briefHash := seedBriefingFile(t, true)

		out, err := runCLI(t, "truth", "intent", "record", "--project", root, "--feature", "intent-demo",
			"--briefing", briefPath, "--by", "chief-of-staff", "--json")
		if err != nil {
			t.Fatalf("truth intent record without a run: %v (stdout %q); want the file written and run_id empty", err, out)
		}
		rec := parseDoc(t, out)
		gotRun, ok := rec["run_id"]
		if !ok {
			t.Fatalf("record output has no run_id field: %v", rec)
		}
		if gotRun != "" {
			t.Fatalf("record output run_id = %v without a run, want empty", gotRun)
		}
		raw, err := os.ReadFile(filepath.Join(pkgDir, "intent.json"))
		if err != nil {
			t.Fatalf("intent.json not written without a run: %v", err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("intent.json is not valid JSON: %v", err)
		}
		if got := seedStr(t, doc, "schema"); got != seedIntentDocSchema {
			t.Fatalf("intent.json schema = %q, want %q", got, seedIntentDocSchema)
		}
		if got := seedStr(t, seedChild(t, doc, "briefing"), "restated_intent"); got != seedIntentRestated {
			t.Fatalf("intent.json briefing.restated_intent = %q", got)
		}
		if briefHash == "" {
			t.Fatal("briefing sha256 was not computed")
		}

		st, err := store.Open(os.Getenv(store.EnvStorePath))
		if err != nil {
			t.Fatal(err)
		}
		defer st.Close()
		ctx, err := st.ResolveFeature(root, "intent-demo", "v2.0.0")
		if err != nil {
			t.Fatal(err)
		}
		runs, err := st.Runs().ListByFeature(ctx.Feature.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(runs) != 0 {
			t.Fatalf("recording without a run created %d run row(s); want none", len(runs))
		}
	})

	// Run selection: --run-id wins over the latest run, and without it
	// the event lands on the most recently created run. Two runs are
	// needed, otherwise an implementation that ignores --run-id and
	// always takes the newest run cannot be told apart from a correct
	// one.
	t.Run("run_selection_prefers_flag_then_latest", func(t *testing.T) {
		root, _ := seedIntentProject(t)
		runA := seedCreateManualRun(t, root)
		runB := seedCreateManualRun(t, root)
		if runA == runB {
			t.Fatalf("two goal runs returned the same id %q", runA)
		}
		briefPath, _, _ := seedBriefingFile(t, true)

		if out, err := runCLI(t, "truth", "intent", "record", "--project", root, "--feature", "intent-demo",
			"--briefing", briefPath, "--run-id", runA, "--by", "chief-of-staff", "--json"); err != nil {
			t.Fatalf("truth intent record --run-id %s: %v (stdout %q)", runA, err, out)
		}
		if got := len(seedIntentRecordEvents(t, runA)); got != 1 {
			t.Fatalf("run A has %d %s event(s) after --run-id A, want 1", got, seedIntentRecordEvent)
		}
		if got := len(seedIntentRecordEvents(t, runB)); got != 0 {
			t.Fatalf("run B has %d %s event(s) after --run-id A, want 0 (--run-id must win)", got, seedIntentRecordEvent)
		}

		if out, err := runCLI(t, "truth", "intent", "record", "--project", root, "--feature", "intent-demo",
			"--briefing", briefPath, "--by", "chief-of-staff", "--json"); err != nil {
			t.Fatalf("truth intent record without --run-id: %v (stdout %q)", err, out)
		}
		if got := len(seedIntentRecordEvents(t, runA)); got != 1 {
			t.Fatalf("run A has %d %s event(s) after the unflagged record, want still 1", got, seedIntentRecordEvent)
		}
		if got := len(seedIntentRecordEvents(t, runB)); got != 1 {
			t.Fatalf("run B has %d %s event(s) after the unflagged record, want 1 (latest run)", got, seedIntentRecordEvent)
		}
	})

	// Confirmation mirroring: an unconfirmed briefing must not be
	// recorded as confirmed. The command still exits 0 and writes the
	// document; only the mirror changes, so hardcoded true cannot pass.
	t.Run("unconfirmed_briefing_mirrors_false", func(t *testing.T) {
		root, pkgDir := seedIntentProject(t)
		runID := seedCreateManualRun(t, root)
		briefPath, _, _ := seedBriefingFile(t, false)

		out, err := runCLI(t, "truth", "intent", "record", "--project", root, "--feature", "intent-demo",
			"--briefing", briefPath, "--run-id", runID, "--by", "chief-of-staff", "--json")
		if err != nil {
			t.Fatalf("truth intent record with an unconfirmed briefing: %v (stdout %q)", err, out)
		}
		if got := seedStr(t, parseDoc(t, out), "run_id"); got != runID {
			t.Fatalf("record output run_id = %q, want %q", got, runID)
		}
		raw, err := os.ReadFile(filepath.Join(pkgDir, "intent.json"))
		if err != nil {
			t.Fatalf("intent.json not written for an unconfirmed briefing: %v", err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("intent.json is not valid JSON: %v", err)
		}
		if got, ok := doc["confirmed"].(bool); !ok || got {
			t.Fatalf("intent.json confirmed = %v, want false mirrored from briefing.confirmed", doc["confirmed"])
		}
		brief := seedChild(t, doc, "briefing")
		if got, ok := brief["confirmed"].(bool); !ok || got {
			t.Fatalf("intent.json briefing.confirmed = %v, want false", brief["confirmed"])
		}
		payloads := seedIntentRecordEvents(t, runID)
		if len(payloads) != 1 {
			t.Fatalf("run has %d %s event(s), want 1", len(payloads), seedIntentRecordEvent)
		}
		if got, ok := payloads[0]["confirmed"].(bool); !ok || got {
			t.Fatalf("%s payload confirmed = %v, want false", seedIntentRecordEvent, payloads[0]["confirmed"])
		}
	})
}
