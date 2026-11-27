package cli

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// TestChangeRequestTwoExecutors proves that two independent writers that
// read the same frozen base cannot both commit. Both executors read the
// fingerprint, meet at a sync barrier, then issue the request. Exactly one
// commits; the other gets tree_change_base_mismatch. The canonical tree and
// its index stay internally consistent after the race.
func TestChangeRequestTwoExecutors(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	for _, id := range []string{"t1", "t2", "f", "m", "root"} {
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatalf("freeze %s: %v", id, err)
		}
	}
	doc := parseDoc(t, mustRun(t, "plan", "tree", "--project", proj, "--json"))
	base := ""
	for _, n := range doc["nodes"].([]any) {
		m := n.(map[string]any)
		if m["id"] == "t1" {
			base = m["fingerprint"].(string)
		}
	}
	if base == "" {
		t.Fatal("no base fingerprint for t1")
	}

	const executors = 2
	var wg sync.WaitGroup
	barrier := make(chan struct{})
	results := make([]string, executors)
	errs := make([]error, executors)
	for i := range executors {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-barrier
			args := []string{"change", "request", "--project", proj, "--node", "t1",
				"--summary", fmt.Sprintf("writer %d", i), "--by", fmt.Sprintf("executor-%d", i),
				"--base-fingerprint", base, "--json"}
			results[i], errs[i] = runCLI(t, args...)
		}(i)
	}
	close(barrier)
	wg.Wait()

	accepted, rejected := 0, 0
	for i := range executors {
		if errs[i] == nil {
			accepted++
			continue
		}
		rejected++
		if !strings.Contains(errs[i].Error(), "tree_change_base_mismatch") {
			t.Fatalf("executor %d rejected for wrong reason: %v\n%s", i, errs[i], results[i])
		}
	}
	if accepted != 1 || rejected != 1 {
		t.Fatalf("want exactly one accepted and one rejected, got accepted=%d rejected=%d", accepted, rejected)
	}

	after := parseDoc(t, mustRun(t, "plan", "tree", "--project", proj, "--json"))
	status := map[string]string{}
	fps := map[string]string{}
	for _, n := range after["nodes"].([]any) {
		m := n.(map[string]any)
		status[m["id"].(string)] = m["status"].(string)
		fps[m["id"].(string)] = m["fingerprint"].(string)
	}
	if status["t1"] != "draft" || status["f"] != "change_pending" || status["m"] != "change_pending" || status["root"] != "change_pending" {
		t.Fatalf("cascade statuses after race: %v", status)
	}
	if got := parseDoc(t, mustRun(t, "change", "list", "--project", proj, "--json")); len(got["changes"].([]any)) != 1 {
		t.Fatalf("want exactly one change record, got %s", got)
	}
	if fps["t1"] == base {
		t.Fatal("committed target fingerprint must differ from the shared base")
	}
	if out := mustRun(t, "verify", "audit", "--project", proj, "--json"); !strings.Contains(out, `"ok": true`) {
		t.Fatalf("tree audit after race failed: %s", out)
	}
}

// TestChangeRequestMissingBaseRejected makes the optimistic-concurrency
// base mandatory: no caller may overwrite frozen truth without declaring
// the fingerprint it read.
func TestChangeRequestMissingBaseRejected(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	for _, id := range []string{"t1", "t2", "f", "m", "root"} {
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatalf("freeze %s: %v", id, err)
		}
	}
	if _, err := runCLI(t, "change", "request", "--project", proj, "--node", "t1",
		"--summary", "no base", "--by", "x", "--json"); err == nil ||
		!strings.Contains(err.Error(), "tree_change_base_mismatch") {
		t.Fatalf("missing base must be refused, got %v", err)
	}
	if got := parseDoc(t, mustRun(t, "change", "list", "--project", proj, "--json")); got["changes"] != nil && len(got["changes"].([]any)) != 0 {
		t.Fatalf("missing base wrote a change record: %s", got)
	}
}

// TestChangeRecoveryRestoresInterruptedWrite proves that a journal left by
// an interrupted change rolls the tree back to the complete old version
// before any later writer proceeds.
func TestChangeRecoveryRestoresInterruptedWrite(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	for _, id := range []string{"t1", "t2", "f", "m", "root"} {
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatalf("freeze %s: %v", id, err)
		}
	}
	before := parseDoc(t, mustRun(t, "plan", "tree", "--project", proj, "--json"))
	beforeFPS := map[string]string{}
	for _, n := range before["nodes"].([]any) {
		m := n.(map[string]any)
		beforeFPS[m["id"].(string)] = m["fingerprint"].(string)
	}

	dir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree")
	indexRaw, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	nodeRaw, err := os.ReadFile(filepath.Join(dir, "nodes", "t1.json"))
	if err != nil {
		t.Fatal(err)
	}
	journal := fmt.Sprintf(`{"tree_dir":%q,"files":[{"path":%q,"old_base64":%q},{"path":%q,"old_base64":%q}]}`,
		dir,
		filepath.Join(dir, "index.json"), b64(string(indexRaw)),
		filepath.Join(dir, "nodes", "t1.json"), b64(string(nodeRaw)))
	lockDir := filepath.Join(proj, ".beacon", "locks")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lockDir, "tree-change-journal.json"), []byte(journal), 0o600); err != nil {
		t.Fatal(err)
	}
	// Simulate the interruption: index still old, node body half-written.
	if err := os.WriteFile(filepath.Join(dir, "nodes", "t1.json"), []byte(`{"interrupted":true}`), 0o600); err != nil {
		t.Fatal(err)
	}

	base := beforeFPS["t1"]
	mustRun(t, "change", "request", "--project", proj, "--node", "t1",
		"--summary", "after recovery", "--by", "x", "--base-fingerprint", base, "--json")
	after := parseDoc(t, mustRun(t, "plan", "tree", "--project", proj, "--json"))
	for _, n := range after["nodes"].([]any) {
		m := n.(map[string]any)
		id := m["id"].(string)
		// The unrelated sibling must keep its frozen fingerprint.
		if id == "t2" && m["fingerprint"].(string) != beforeFPS[id] {
			t.Fatalf("sibling t2 changed unexpectedly during recovery")
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "nodes", "t1.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "interrupted") {
		t.Fatal("interrupted node body was not rolled back")
	}
	if _, err := os.Stat(filepath.Join(lockDir, "tree-change-journal.json")); !os.IsNotExist(err) {
		t.Fatalf("journal must be removed after recovery, got %v", err)
	}
}
