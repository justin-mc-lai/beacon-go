package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/intenttree"
)

func TestTruthNodeReopenWithdrawsClaims(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	for _, id := range []string{"t1", "t2", "f", "m", "root"} {
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatalf("freeze %s: %v", id, err)
		}
	}
	for _, id := range []string{"t1", "t2"} {
		mustRun(t, "implement", "report", "--project", proj, "--node", id, "--by", "dev",
			"--artifact", writeEvidence(t, proj, id), "--json")
		mustRun(t, "verify", "leaf", "--project", proj, "--node", id, "--by", "qa",
			"--artifact", writeEvidence(t, proj, id), "--json")
	}
	mustRun(t, "verify", "rollup", "--project", proj, "--node", "f", "--json")

	dir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree")
	read := func(id string) intenttree.Node {
		t.Helper()
		n, err := intenttree.ReadNode(dir, id)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	mustRun(t, "truth", "node", "reopen", "--project", proj, "--node", "f", "--to", "implemented",
		"--by", "audit", "--reason", "evidence withdrawn", "--json")
	if n := read("f"); n.Status != intenttree.StatusImplemented {
		t.Fatalf("verified feature reopen = %s, want implemented", n.Status)
	}

	mustRun(t, "truth", "node", "reopen", "--project", proj, "--node", "t1", "--to", "implemented",
		"--by", "audit", "--reason", "evidence withdrawn", "--json")
	n := read("t1")
	if n.Status != intenttree.StatusImplemented || n.VerificationEvidence != nil {
		t.Fatalf("reopen to implemented = %s evidence=%v", n.Status, n.VerificationEvidence)
	}

	mustRun(t, "truth", "node", "reopen", "--project", proj, "--node", "t1", "--to", "frozen",
		"--by", "audit", "--reason", "not implemented", "--json")
	n = read("t1")
	if n.Status != intenttree.StatusFrozen || n.Implementation != nil {
		t.Fatalf("reopen to frozen = %s implementation=%v", n.Status, n.Implementation)
	}

	again := mustRun(t, "truth", "node", "reopen", "--project", proj, "--node", "t1",
		"--to", "frozen", "--by", "audit", "--json")
	if !strings.Contains(again, "already") {
		t.Fatalf("repeat reopen must be idempotent: %s", again)
	}
}
