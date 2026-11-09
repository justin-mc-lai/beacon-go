// artifact-upsert TC-AU-003..005: the qa evidence check judges only the
// newest pointer per (run, path). Re-running a stage rewrites the stage's
// artifact files, so a legacy DB can hold several rows for one path whose
// older hashes no longer match disk — validating those would make the qa
// gate permanently stale. No migration: the read path dedupes.
package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// hashOf returns the sha256 hex digest the store's staleness check uses.
func hashOf(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// artifactRow writes an artifact_refs row straight through SQL, the way
// a pre-upsert build left it behind: several rows for one (run, path).
// Add would now collapse them, so legacy fixtures must bypass it.
func artifactRow(t *testing.T, env testEnv, id, runID, kind, absPath, hash, createdAt string, size int64) {
	t.Helper()
	if _, err := env.st.DB().Exec(
		`INSERT INTO artifact_refs(id, run_id, kind, abs_path, content_hash, size_bytes, created_at) VALUES(?,?,?,?,?,?,?)`,
		id, runID, kind, absPath, hash, size, createdAt); err != nil {
		t.Fatalf("insert legacy artifact row %s: %v", id, err)
	}
}

// qaFixture builds a git-free temp fixture: one run with the qa stage
// executed on record and one qa evidence file on disk.
func qaFixture(t *testing.T) (testEnv, RunView, string) {
	t.Helper()
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	appendRealStageExecuted(t, env, run.RunID, StageQA)
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "qa", "report.json")
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		t.Fatal(err)
	}
	return env, run, reportPath
}

func qaCtx(t *testing.T, env testEnv) store.FeatureContext {
	t.Helper()
	ctx, err := env.st.ResolveFeature(env.root, "feat-a", "")
	if err != nil {
		t.Fatalf("ResolveFeature: %v", err)
	}
	return ctx
}

// TC-AU-003 (AC-003): with legacy rows present, a mismatching OLD hash
// must not reach the staleness verdict — only the newest row is
// validated, so the gate passes on the live pointer.
func TestQAStalenessUsesLatestPerPath(t *testing.T) {
	env, run, reportPath := qaFixture(t)
	// Disk holds v2; the newest row describes v2, the older rows
	// describe v1 (their hashes no longer match disk).
	if err := os.WriteFile(reportPath, []byte(`{"verdict":"pass","v":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	artifactRow(t, env, "art_legacy_old1", run.RunID, "qa_report", reportPath,
		hashOf(`{"verdict":"pass","v":1}`), "2026-01-01T00:00:00Z", 3)
	artifactRow(t, env, "art_legacy_old2", run.RunID, "qa_report", reportPath,
		hashOf(`{"verdict":"pass","v":1}`), "2026-01-02T00:00:00Z", 3)
	artifactRow(t, env, "art_legacy_live", run.RunID, "qa_report", reportPath,
		hashOf(`{"verdict":"pass","v":2}`), "2026-01-03T00:00:00Z", 3)
	rows, err := env.st.Artifacts().ListByRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("fixture rows = %d, want 3 legacy duplicates", len(rows))
	}

	ctx := qaCtx(t, env)
	ev, err := env.eng.evalQA(ctx, "qa-bot")
	if err != nil {
		t.Fatalf("evalQA: %v", err)
	}
	if ev.Verdict != gate.VerdictPass {
		t.Fatalf("newest row is live, qa must pass; got %+v (%v)", ev, ev.ReasonCodes)
	}
	for _, c := range ev.Checks {
		if c.Check == "qa_artifact_refs_valid" && !c.OK {
			t.Fatalf("qa_artifact_refs_valid = %+v, want ok from the newest row only", c)
		}
	}

	// The control: new bytes on disk with the newest row still holding
	// the old hash must block, so newest-only is not a blanket pass.
	if err := os.WriteFile(reportPath, []byte(`{"verdict":"fail","v":3}`), 0o644); err != nil {
		t.Fatal(err)
	}
	ev, err = env.eng.evalQA(ctx, "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictBlocked || len(ev.ReasonCodes) == 0 || ev.ReasonCodes[0] != core.ReasonQAArtifactStale {
		t.Fatalf("newest row matching no disk bytes must block as stale, got %+v", ev)
	}
}

// TC-AU-004 (AC-004): a database that already holds duplicate rows needs
// no migration — the read path dedupes, the newest row still exists, and
// the gate passes. Rows are neither deleted nor rewritten.
func TestQALegacyDuplicatesNoMigration(t *testing.T) {
	env, run, reportPath := qaFixture(t)
	if err := os.WriteFile(reportPath, []byte(`{"verdict":"pass"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Three pre-upsert leftovers whose hashes can no longer match disk,
	// then the newest row of the same path written by an older build.
	// Nothing here goes through Add, so the legacy hash history survives
	// untouched — exactly the database the fix must tolerate.
	artifactRow(t, env, "art_legacy_a", run.RunID, "qa_report", reportPath, "deadbeef1", "2026-02-01T00:00:00Z", 1)
	artifactRow(t, env, "art_legacy_b", run.RunID, "qa", reportPath, "deadbeef2", "2026-02-02T00:00:00Z", 1)
	artifactRow(t, env, "art_legacy_c", run.RunID, "qa_report", reportPath, "deadbeef3", "2026-02-03T00:00:00Z", 1)
	artifactRow(t, env, "art_legacy_live", run.RunID, "qa_report", reportPath, hashOf(`{"verdict":"pass"}`), "2026-02-04T00:00:00Z", 17)
	before, err := env.st.Artifacts().ListByRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 4 {
		t.Fatalf("fixture rows = %d, want 4 (3 stale + 1 live)", len(before))
	}
	// The schema is what it always was: nothing was added to hold
	// duplicate rows (AC-006) and nothing has to be migrated away.
	if err := env.st.VerifySchemaContract(); err != nil {
		t.Fatalf("schema contract drifted: %v", err)
	}
	ctx := qaCtx(t, env)
	ev, err := env.eng.evalQA(ctx, "qa-bot")
	if err != nil {
		t.Fatalf("evalQA over legacy duplicates: %v", err)
	}
	if ev.Verdict != gate.VerdictPass {
		t.Fatalf("legacy duplicates + live newest row must pass without migration, got %+v (%v)", ev, ev.ReasonCodes)
	}
	// No migration, no cleanup: the rows are still all there.
	after, err := env.st.Artifacts().ListByRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("evaluation must not rewrite history: %d rows before, %d after", len(before), len(after))
	}
	for i := range before {
		if before[i].ID != after[i].ID || before[i].ContentHash != after[i].ContentHash {
			t.Fatalf("row %d mutated by evaluation: %+v -> %+v", i, before[i], after[i])
		}
	}
}

// TC-AU-005 (AC-005): the real sequence that used to poison the gate —
// register, change the file, register again — ends with a passing qa
// verdict instead of a permanent qa_artifact_stale.
func TestQAReRegisterClearsStale(t *testing.T) {
	env, run, reportPath := qaFixture(t)
	writeBlob(t, reportPath, "# qa report v1\nall green\n")
	ctx := qaCtx(t, env)

	if _, err := env.eng.RegisterArtifact(run.RunID, "qa_report", reportPath, "qa-bot"); err != nil {
		t.Fatalf("first register: %v", err)
	}
	ev, err := env.eng.evalQA(ctx, "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictPass {
		t.Fatalf("freshly registered evidence must pass, got %+v", ev)
	}

	// The stage re-runs and rewrites its report: the pointer is now
	// stale, exactly as before the fix.
	writeBlob(t, reportPath, "# qa report v2\nre-run\nall green\n")
	ev, err = env.eng.evalQA(ctx, "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictBlocked || len(ev.ReasonCodes) == 0 || ev.ReasonCodes[0] != core.ReasonQAArtifactStale {
		t.Fatalf("a rewritten file must read stale first, got %+v", ev)
	}

	// Registering the same path again refreshes the pointer in place:
	// the gate clears instead of carrying a dead hash forever.
	res, err := env.eng.RegisterArtifact(run.RunID, "qa_report", reportPath, "qa-bot")
	if err != nil {
		t.Fatalf("re-register: %v", err)
	}
	if res.AlreadyRegistered {
		t.Fatal("changed bytes must not report already_registered")
	}
	ev, err = env.eng.evalQA(ctx, "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictPass {
		t.Fatalf("re-registering the same path must clear qa_artifact_stale, got %+v (%v)", ev, ev.ReasonCodes)
	}
	rows, err := env.st.Artifacts().ListByRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("re-registering the same path must not append rows: %d rows", len(rows))
	}
	if rows[0].ContentHash != hashOf("# qa report v2\nre-run\nall green\n") {
		t.Fatalf("stored hash = %q, want the re-run bytes", rows[0].ContentHash)
	}
}
