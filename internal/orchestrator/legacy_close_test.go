package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
)

func TestLegacyCloseBackfillsWithoutQA(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	report := filepath.Join(t.TempDir(), "old-qa.md")
	body := []byte("legacy independent report PASS")
	if err := os.WriteFile(report, body, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	sha := hex.EncodeToString(sum[:])

	if _, err := env.eng.LegacyClose(run.RunID, LegacyCloseOptions{
		ConfirmedBy: "justin", ReportPath: report, ReportSHA256: sha, ReasonCode: "free_bypass",
	}); err == nil {
		t.Fatal("non-whitelisted reason must be refused")
	}
	if _, err := env.eng.LegacyClose(run.RunID, LegacyCloseOptions{
		ConfirmedBy: "justin", ReportPath: report, ReportSHA256: strings.Repeat("0", 64),
		ReasonCode: "legacy_evidence_thin",
	}); err == nil {
		t.Fatal("report hash drift must be refused")
	}

	v, err := env.eng.LegacyClose(run.RunID, LegacyCloseOptions{
		ConfirmedBy: "justin", By: "delivery-manager", ReportPath: report, ReportSHA256: sha,
		CandidateSHA256: strings.Repeat("a", 64), ReasonCode: "legacy_evidence_thin",
	})
	if err != nil {
		t.Fatalf("legacy close: %v", err)
	}
	if v.LoopStage != string(core.LoopDone) || v.Status != "completed" {
		t.Fatalf("view = %+v", v)
	}
	row, err := env.st.Runs().GetByID(run.RunID)
	if err != nil || row.Status != core.RunSucceeded || !row.EndedAt.Valid {
		t.Fatalf("run row = %+v err=%v", row, err)
	}
}
