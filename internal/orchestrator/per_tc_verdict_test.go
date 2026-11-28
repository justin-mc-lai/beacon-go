package orchestrator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/gate"
)

func TestPerTCVerdictRejectsMalformedReport(t *testing.T) {
	for _, axis := range []string{"missing", "duplicate", "reordered", "zero-results", "zero-cases", "blank-tc", "wrong-candidate"} {
		t.Run(axis, func(t *testing.T) {
			env := newTestEnv(t, "active")
			s := env.createRun(t)
			text := "| AC ID | TC ID | Layer | Command | Assertion |\n|---|---|---|---|---|\n| AC-001 | TC-002 | behavior | python3 qa-case.py | holds |\n| AC-001 | TC-001 | behavior | python3 qa-case.py | holds |\n"
			if err := os.WriteFile(filepath.Join(env.root, "tests.md"), []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
			prepareQACandidate(t, env, s.RunID)
			r, err := env.eng.RunAutoTest(context.Background(), s.RunID, AutoTestOptions{By: "maker"})
			if err != nil {
				t.Fatal(err)
			}
			switch axis {
			case "missing":
				r.Results = r.Results[:1]
			case "duplicate":
				r.Results[1] = r.Results[0]
			case "reordered":
				r.Results[0], r.Results[1] = r.Results[1], r.Results[0]
			case "zero-results":
				r.Results = nil
				r.Total = 0
				r.Passed = 0
			case "zero-cases":
				n := 0
				r.Results[0].TestCount = &n
				r.Results[0].Cases = nil
			case "blank-tc":
				r.Results[0].TC = " "
			case "wrong-candidate":
				r.Results[0].CandidateHash = "other"
			}
			path := resolvePkgPath(env.root, r.ReportPath)
			b, _ := json.Marshal(r)
			if err := os.WriteFile(path, b, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := env.eng.RegisterArtifact(s.RunID, "test_result", path, "maker"); err != nil {
				t.Fatal(err)
			}
			run, m, _, err := env.eng.loadRun(s.RunID)
			if err != nil {
				t.Fatal(err)
			}
			h, err := qaHash(path)
			if err != nil {
				t.Fatal(err)
			}
			// Negative control: even a live hash-bound malformed report cannot pass.
			if err := env.eng.appendEvent(run, EventAutoTestExecuted, map[string]any{"candidate_hash": r.CandidateHash, "report_path": path, "report_hash": h}); err != nil {
				t.Fatal(err)
			}
			var checks []gate.CheckItem
			_, err = env.eng.currentQAEligibility(run, m, true, &checks)
			if err == nil {
				t.Fatal("malformed report accepted")
			}
			if len(checks) != 2 {
				t.Fatalf("missing contract checks: %v", checks)
			}
			for i, c := range checks {
				want := []string{"TC-002:blocked", "TC-001:blocked"}[i]
				if c.Check != want || c.OK || strings.TrimSpace(c.Detail) == "" {
					t.Fatalf("check=%+v want=%s", c, want)
				}
			}
		})
	}
}
