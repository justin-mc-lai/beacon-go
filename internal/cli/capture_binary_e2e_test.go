package cli

import (
	"beacon-go/internal/store"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Real executable entry, process captures and verification. The reviewer document
// is a synthetic fixture; this test does not claim to launch a reviewing agent.
func TestCaptureBinaryE2ESuccessAndRefusal(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "beacon")
	build := exec.Command("go", "build", "-o", bin, "./cmd/beacon")
	build.Dir = "../.."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	prepareCaptureTree(t, proj)
	for _, id := range []string{"t1", "t2", "f", "m", "root"} {
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatal(err)
		}
	}
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("binary %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	for _, id := range []string{"t1", "t2"} {
		run("implement", "report", "--project", proj, "--node", id, "--by", "dev", "--artifact", writeEvidence(t, proj, id))
		p := writeCapturedEvidenceWithRunner(t, proj, id, run)
		run("verify", "leaf", "--project", proj, "--node", id, "--by", "qa", "--artifact", p, "--json")
	}
	run("verify", "rollup", "--project", proj, "--node", "root", "--json")
	run("truth", "node", "deliver", "--project", proj, "--node", "root", "--by", "owner", "--json")
	created := parseDoc(t, run("goal", "run", "--project", proj, "--feature", "f", "--executor", "manual", "--executor-id", "dev", "--json"))
	runID := created["run_id"].(string)
	// Set stage only in fixture: bypassing earlier stages is not a product action.
	setStage := func(stage string, index int) {
		t.Helper()
		st, err := store.Open(storePathFromEnv(t))
		if err != nil {
			t.Fatal(err)
		}
		defer st.Close()
		row, err := st.Runs().GetByID(runID)
		if err != nil {
			t.Fatal(err)
		}
		var meta map[string]any
		if err := json.Unmarshal(row.Metadata, &meta); err != nil {
			t.Fatal(err)
		}
		meta["loop_stage"] = stage
		meta["current_stage_idx"] = index
		b, err := json.Marshal(meta)
		if err != nil {
			t.Fatal(err)
		}
		ok, err := st.Runs().UpdateRunStateCAS(runID, row.Metadata, row.Status, b, nil)
		if err != nil || !ok {
			t.Fatalf("fixture stage: %v %v", ok, err)
		}
	}
	if err := os.WriteFile(filepath.Join(proj, "behavior.go"), []byte("package capturefixture\nfunc Ready()bool{return false}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "verify", "rollup", "--project", proj, "--node", "root", "--json")
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "delivery_capture_stale") {
		t.Fatalf("binary stale candidate not refused: %v %s", err, out)
	}
	refuse := func(args ...string) {
		t.Helper()
		out, err := exec.Command(bin, args...).CombinedOutput()
		if err == nil || !strings.Contains(string(out), "delivery_capture_stale") {
			t.Fatalf("%v: %v %s", args, err, out)
		}
	}
	for _, group := range []string{"verify", "release"} {
		refuse(group, "evaluate", "--project", proj, "--feature", "f", "--run", runID, "--evaluator", "qa", "--json")
	}
	setStage("qa_routing", 3)
	refuse("goal", "tick", "--run-id", runID, "--by", "qa", "--json")
	setStage("release_gate", 4)
	refuse("goal", "complete", "--run-id", runID, "--json")
}
