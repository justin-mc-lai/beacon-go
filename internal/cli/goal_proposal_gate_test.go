package cli

import (
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/core"
)

func p2dProposalFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "proposal.json")
	raw := `{
  "schema": "lake-proposal.v1",
  "utterance": "ship the platform",
  "lakes": [
    {"slug":"lake-a","outcome":"a","value":"v","metric":"m","operable":"o","size":"S","acceptance_seed":"go test ./internal/cli -run TestProgramProposalGate -count=1"},
    {"slug":"lake-b","outcome":"b","value":"v","metric":"m","operable":"o","size":"S","acceptance_seed":"go test ./internal/cli -run TestProgramProposalGate -count=1","depends_on":["lake-a"]},
    {"slug":"lake-c","outcome":"c","value":"v","metric":"m","operable":"o","size":"S","acceptance_seed":"go test ./internal/cli -run TestProgramProposalGate -count=1","depends_on":["lake-b"]}
  ],
  "not_proposed": [{"slug":"extra","reason":"later"}]
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// AC-009: a new program may not be cut directly; it must start from a
// human-selected proposal. Re-planning an existing program stays open.
func TestProgramPlanRequiresProposal(t *testing.T) {
	root := seedP2DProject(t)
	out, err := runCLI(t, "goal", "run", "ship it", "--project", root,
		"--feature", "feat-a", "--executor-id", "host-1", "--json")
	if err != nil {
		t.Fatalf("goal run: %v", err)
	}
	runID := strField(t, parseDoc(t, out), "run_id")

	_, err = runCLI(t, "goal", "program", "plan", "--run-id", runID,
		"--program", "p", "--lake", "lake-a", "--json")
	if code, _ := core.ReasonCodeOf(err); code != core.ReasonLakeProposalRequired {
		t.Fatalf("first-cut direct plan: want %s, got %v", core.ReasonLakeProposalRequired, err)
	}

	out, err = runCLI(t, "goal", "program", "propose", "--run-id", runID, "--file", p2dProposalFile(t))
	if err != nil {
		t.Fatalf("program propose: %v", err)
	}
	proposalID := strField(t, parseDoc(t, out), "proposal_id")
	if _, err := runCLI(t, "goal", "program", "select", "--run-id", runID, "--proposal", proposalID,
		"--lake", "lake-a", "--lake", "lake-b", "--lake", "lake-c", "--by", "test"); err != nil {
		t.Fatalf("program select: %v", err)
	}
	if _, err := runCLI(t, "goal", "program", "plan", "--run-id", runID, "--from-proposal", proposalID); err != nil {
		t.Fatalf("plan --from-proposal: %v", err)
	}
	if _, err := runCLI(t, "goal", "program", "plan", "--run-id", runID,
		"--program", "p2", "--lake", "lake-a", "--lake", "lake-b", "--lake", "lake-c",
		"--depends", "lake-b=lake-a", "--json"); err != nil {
		t.Fatalf("re-plan of an existing program must be allowed: %v", err)
	}
}
