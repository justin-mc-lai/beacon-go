package cli

import (
	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaptureHistoricalReplayRedGreen(t *testing.T) {
	proj := captureFixture(t, []string{"go", "test", "."})
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = proj
		b, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git: %v %s", err, b)
		}
		return strings.TrimSpace(string(b))
	}
	git("init")
	git("add", "go.mod", "behavior.go", "behavior_test.go")
	git("-c", "user.name=fixture", "-c", "user.email=fixture@example.test", "commit", "-m", "baseline")
	baseline := git("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(proj, "behavior.go"), []byte("package capturefixture\nfunc Ready() bool{return true}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "behavior.go")
	git("-c", "user.name=fixture", "-c", "user.email=fixture@example.test", "commit", "-m", "implementation")
	out, err := runCLI(t, append(captureArgs(proj, "red"), "--baseline", baseline)...)
	if err != nil {
		t.Fatalf("historical Red: %v %s", err, out)
	}
	red := parseDoc(t, out)
	if red["source_commit"] != baseline || red["baseline_commit"] != baseline || red["status"] != "accepted" {
		t.Fatalf("baseline binding: %v", red)
	}
	current, err := os.ReadFile(filepath.Join(proj, "behavior.go"))
	if err != nil || !strings.Contains(string(current), "true") {
		t.Fatal("replay changed live implementation")
	}
	if out, err := runCLI(t, append(captureArgs(proj, "green"), "--red", red["artifact_id"].(string))...); err != nil {
		t.Fatalf("Green: %v %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(proj, "extra.txt"), []byte("new candidate input"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, append(captureArgs(proj, "green"), "--red", red["artifact_id"].(string))...); err == nil || !strings.Contains(err.Error(), "capture_pair_invalid") {
		t.Fatalf("changed replay target accepted: %v", err)
	}
	for _, base := range []string{"HEAD", strings.Repeat("a", 40)} {
		if _, err := runCLI(t, append(captureArgs(proj, "red"), "--baseline", base)...); err == nil {
			t.Fatalf("invalid baseline accepted: %s", base)
		}
	}
	if _, err := runCLI(t, append(captureArgs(proj, "green"), "--baseline", baseline)...); err == nil {
		t.Fatal("baseline on Green accepted")
	}
}

func TestCaptureHistoricalVerifiedRejectsCommitDrift(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	prepareCaptureTree(t, proj)
	if _, err := freeze(t, proj, "t1"); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = proj
		b, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, b)
		}
		return strings.TrimSpace(string(b))
	}
	git("init")
	git("add", "go.mod", "behavior.go", "behavior_test.go")
	git("-c", "user.name=fixture", "-c", "user.email=fixture@example.test", "commit", "-m", "baseline")
	base := git("rev-parse", "HEAD")
	greenSource := []byte("package capturefixture\nfunc Ready() bool{return true}\n")
	if err := os.WriteFile(filepath.Join(proj, "behavior.go"), greenSource, 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "behavior.go")
	git("-c", "user.name=fixture", "-c", "user.email=fixture@example.test", "commit", "-m", "implementation")
	mustRun(t, "implement", "report", "--project", proj, "--node", "t1", "--by", "dev", "--artifact", writeEvidence(t, proj, "t1"))
	runner := func(args ...string) string {
		for i, arg := range args {
			if arg == "--phase" && i+1 < len(args) && args[i+1] == "red" {
				if err := os.WriteFile(filepath.Join(proj, "behavior.go"), greenSource, 0600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--baseline", base)
				break
			}
		}
		return mustRun(t, args...)
	}
	p := writeCapturedEvidenceWithRunner(t, proj, "t1", runner)
	mustRun(t, "verify", "leaf", "--project", proj, "--node", "t1", "--by", "qa", "--artifact", p)
	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	dir := filepath.Join(proj, "docs/beacon/v0.1.0/tree")
	n, err := intenttree.ReadNode(dir, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if err := intenttree.ValidateVerifiedDelivery(st, dir, n); err != nil {
		t.Fatal(err)
	}
	git("-c", "user.name=fixture", "-c", "user.email=fixture@example.test", "commit", "--allow-empty", "-m", "commit drift")
	if err := intenttree.ValidateVerifiedDelivery(st, dir, n); err == nil || !strings.Contains(err.Error(), "delivery_capture_stale") {
		t.Fatalf("empty commit drift accepted: %v", err)
	}
}

func TestCaptureHistoricalReplayCompilationIsNotRed(t *testing.T) {
	proj := captureFixture(t, []string{"go", "test", "."})
	git := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = proj
		b, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, b)
		}
		return strings.TrimSpace(string(b))
	}
	if err := os.WriteFile(filepath.Join(proj, "behavior.go"), []byte("package capturefixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("init")
	git("add", "go.mod", "behavior.go")
	git("-c", "user.name=fixture", "-c", "user.email=fixture@example.test", "commit", "-m", "old interface")
	base := git("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(proj, "behavior.go"), []byte("package capturefixture\nfunc Ready()bool{return true}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "behavior.go", "behavior_test.go")
	git("-c", "user.name=fixture", "-c", "user.email=fixture@example.test", "commit", "-m", "interface implemented")
	out, err := runCLI(t, append(captureArgs(proj, "red"), "--baseline", base)...)
	if err == nil {
		t.Fatal("baseline compilation failure counted as Red")
	}
	r := parseDoc(t, out)
	if r["status"] != "rejected" || r["artifact_id"] == "" {
		t.Fatalf("failure not recorded: %v", r)
	}
}
