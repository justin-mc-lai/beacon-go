package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/orchestrator"
	"beacon-go/internal/store"
)

func captureFixture(t *testing.T, command []string) string {
	t.Helper()
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	for name, body := range map[string]string{
		"go.mod":           "module capturefixture\n\ngo 1.23\n",
		"behavior.go":      "package capturefixture\nfunc Ready() bool { return false }\n",
		"behavior_test.go": "package capturefixture\nimport \"testing\"\nfunc TestBehavior(t *testing.T) { if !Ready() { t.Fatal(\"behavior missing\") } }\n",
	} {
		if err := os.WriteFile(filepath.Join(proj, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	spec, err := json.Marshal(map[string]any{"schema": "beacon-capture-command.v1", "argv": command,
		"test_files": []string{"behavior_test.go"}, "required_tests": []string{"TestBehavior"}})
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	dir := filepath.Join(proj, "docs/beacon/v0.1.0/tree")
	_, err = intenttree.UpdateNode(st, dir, "t1", intenttree.NodePatch{Verification: &intenttree.Verification{Type: "test", Spec: string(spec)}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := freeze(t, proj, "t1"); err != nil {
		t.Fatal(err)
	}
	return proj
}

func captureArgs(proj, phase string) []string {
	return []string{"implement", "capture", "--project", proj, "--version", "v0.1.0", "--node", "t1", "--ac", "t1-1", "--phase", phase, "--by", "maker", "--timeout-sec", "30"}
}

func TestCaptureRealProcessAndTestHash(t *testing.T) {
	proj := captureFixture(t, []string{"go", "test", ".", "-run", "^TestBehavior$"})
	out, err := runCLI(t, captureArgs(proj, "red")...)
	if err != nil {
		t.Fatalf("real Red capture: %v\n%s", err, out)
	}
	red := parseDoc(t, out)
	if red["status"] != "accepted" || int(red["exit_code"].(float64)) != 1 {
		t.Fatalf("Red: %v", red)
	}
	if red["test_sha256"] == "" || red["artifact_id"] == "" {
		t.Fatalf("unregistered capture: %v", red)
	}
	log, err := os.ReadFile(red["stdout_path"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), "behavior missing") {
		t.Fatalf("not actual stdout: %s", log)
	}
	if err := os.WriteFile(filepath.Join(proj, "behavior.go"), []byte("package capturefixture\nfunc Ready() bool { return true }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	args := append(captureArgs(proj, "green"), "--red", red["artifact_id"].(string))
	out, err = runCLI(t, args...)
	if err != nil {
		t.Fatalf("Green: %v\n%s", err, out)
	}
	green := parseDoc(t, out)
	if green["status"] != "accepted" || int(green["exit_code"].(float64)) != 0 || green["test_sha256"] != red["test_sha256"] || green["source_sha256"] == red["source_sha256"] {
		t.Fatalf("bad pair: red=%v green=%v", red, green)
	}
	if green["result_path"] == red["result_path"] {
		t.Fatal("capture history overwritten")
	}
}

func TestCaptureRejectsUnboundCommand(t *testing.T) {
	proj := captureFixture(t, []string{"false"})
	_, err := runCLI(t, captureArgs(proj, "red")...)
	if err == nil || !strings.Contains(err.Error(), "capture_command_invalid") {
		t.Fatalf("want command rejection, got %v", err)
	}
}

func TestCaptureExplicitRedPair(t *testing.T) {
	proj := captureFixture(t, []string{"go", "test", ".", "-run", "^TestBehavior$"})
	_, err := runCLI(t, captureArgs(proj, "green")...)
	if err == nil || !strings.Contains(err.Error(), "capture_red_required") {
		t.Fatalf("Green without Red: %v", err)
	}
}

func TestCapturePreflightFailureRetainsEvidence(t *testing.T) {
	proj := captureFixture(t, []string{"go", "test", "."})
	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, err := orchestrator.NewEngine(st).CaptureTest(ctx, orchestrator.CaptureRequest{Root: proj, TreeDir: filepath.Join(proj, "docs/beacon/v0.1.0/tree"), NodeID: "t1", ACID: "t1-1", Phase: "red", By: "maker", Timeout: time.Second})
	if err == nil || r.ResultPath == "" || r.ArtifactID == "" || r.Status != "rejected" || !r.Cancelled {
		t.Fatalf("preflight not retained: %+v %v", r, err)
	}
	if _, err := os.Stat(r.StderrPath); err != nil {
		t.Fatal(err)
	}
	if r.Preflight == nil || len(r.Preflight.Command) < 2 || r.Preflight.Command[1] != "list" || r.Preflight.ExitCode == 0 {
		t.Fatalf("actual preflight absent: %+v", r.Preflight)
	}
}

func TestCaptureConcurrentProcessesUseBarrier(t *testing.T) {
	proj := captureFixture(t, []string{"go", "test", "."})
	body := `package capturefixture
import("testing";"os";"path/filepath";"time")
func TestBehavior(t *testing.T){
 os.MkdirAll(".beacon/barrier",0700)
 f,err:=os.CreateTemp(".beacon/barrier","worker-");if err!=nil{t.Fatal(err)};f.Close()
 deadline:=time.Now().Add(5*time.Second)
 for {files,_:=filepath.Glob(".beacon/barrier/worker-*");if len(files)>=2{break};if time.Now().After(deadline){t.Fatal("barrier timeout")};time.Sleep(10*time.Millisecond)}
 t.Fatal("behavior missing after two-worker barrier")
}

`
	if err := os.WriteFile(filepath.Join(proj, "behavior_test.go"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	type outcome struct {
		r   orchestrator.CaptureResult
		err error
	}
	results := make(chan outcome, 2)
	for i := 0; i < 2; i++ {
		go func() {
			r, err := orchestrator.NewEngine(st).CaptureTest(context.Background(), orchestrator.CaptureRequest{Root: proj, TreeDir: filepath.Join(proj, "docs/beacon/v0.1.0/tree"), NodeID: "t1", ACID: "t1-1", Phase: "red", By: "maker", Timeout: 15 * time.Second})
			results <- outcome{r, err}
		}()
	}
	a, b := <-results, <-results
	if a.err != nil || b.err != nil {
		t.Fatalf("concurrent captures: %v %v", a.err, b.err)
	}
	if a.r.ResultPath == b.r.ResultPath || a.r.ArtifactID == b.r.ArtifactID {
		t.Fatal("concurrent history collision")
	}
	for _, r := range []orchestrator.CaptureResult{a.r, b.r} {
		raw, err := os.ReadFile(r.StdoutPath)
		if err != nil || !strings.Contains(string(raw), "after two-worker barrier") {
			t.Fatalf("barrier not observed: %s %v", raw, err)
		}
	}
}

func TestCaptureRejectsDuplicateAliasAndEmptyTests(t *testing.T) {
	for _, files := range [][]string{nil, {"behavior_test.go", "./behavior_test.go"}, {"../outside_test.go"}} {
		proj := captureFixture(t, []string{"go", "test", "."})
		// Change via the governed draft path before freeze, not by editing JSON.
		mustRun(t, "change", "request", "--project", proj, "--node", "t1", "--base-fingerprint", captureNodeFingerprint(t, proj), "--summary", "fixture tests", "--by", "fixture")
		st, err := store.Open(storePathFromEnv(t))
		if err != nil {
			t.Fatal(err)
		}
		spec, _ := json.Marshal(map[string]any{"schema": "beacon-capture-command.v1", "argv": []string{"go", "test", "."}, "test_files": files, "required_tests": []string{"TestBehavior"}})
		_, err = intenttree.UpdateNode(st, filepath.Join(proj, "docs/beacon/v0.1.0/tree"), "t1", intenttree.NodePatch{Verification: &intenttree.Verification{Type: "test", Spec: string(spec)}})
		st.Close()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := freeze(t, proj, "t1"); err != nil {
			t.Fatal(err)
		}
		if _, err := runCLI(t, captureArgs(proj, "red")...); err == nil {
			t.Fatalf("invalid files accepted: %v", files)
		}
	}
}

func captureNodeFingerprint(t *testing.T, proj string) string {
	t.Helper()
	n, err := intenttree.ReadNode(filepath.Join(proj, "docs/beacon/v0.1.0/tree"), "t1")
	if err != nil {
		t.Fatal(err)
	}
	return n.Fingerprint
}

func TestCaptureRejectsZeroSkipAndEnvironmentFailure(t *testing.T) {
	for _, tc := range []struct {
		name, test string
		command    []string
	}{
		{"zero", "", []string{"go", "test", ".", "-run", "^NotPresent$"}},
		{"skip", "package capturefixture\nimport \"testing\"\nfunc TestBehavior(t *testing.T){t.Skip(\"skip\")}\n", []string{"go", "test", "."}},
		{"compile", "this is not go source", []string{"go", "test", "."}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proj := captureFixture(t, tc.command)
			if tc.test != "" {
				if err := os.WriteFile(filepath.Join(proj, "behavior_test.go"), []byte(tc.test), 0600); err != nil {
					t.Fatal(err)
				}
			}
			out, err := runCLI(t, captureArgs(proj, "red")...)
			if err == nil {
				t.Fatalf("invalid Red accepted: %s", out)
			}
			doc := parseDoc(t, out)
			if doc["status"] != "rejected" || doc["artifact_id"] == "" {
				t.Fatalf("failure evidence not retained: %v", doc)
			}
		})
	}
}

func TestCapturePairRejectsChangedTestsAndStaleLogs(t *testing.T) {
	proj := captureFixture(t, []string{"go", "test", "."})
	out, err := runCLI(t, captureArgs(proj, "red")...)
	if err != nil {
		t.Fatal(err)
	}
	red := parseDoc(t, out)
	p := filepath.Join(proj, "behavior_test.go")
	original, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(original, []byte("\n// altered test input\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	args := append(captureArgs(proj, "green"), "--red", red["artifact_id"].(string))
	if _, err := runCLI(t, args...); err == nil || !strings.Contains(err.Error(), "capture_pair_invalid") {
		t.Fatalf("changed test: %v", err)
	}
	if err := os.WriteFile(p, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(red["stdout_path"].(string), []byte("forged"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, args...); err == nil || !strings.Contains(err.Error(), "capture_stale") {
		t.Fatalf("stale log: %v", err)
	}
	_, err = runCLI(t, "goal", "artifacts", "register", "--scope-kind", "node", "--scope-id", "t1", "--kind", "capture_result", "--path", red["result_path"].(string), "--by", "maker")
	if err == nil || !strings.Contains(err.Error(), "artifact_kind_not_allowed") {
		t.Fatalf("manual capture impersonation: %v", err)
	}
}

func TestCaptureTimeoutCancellationAndMutation(t *testing.T) {
	for _, mode := range []string{"timeout", "cancel", "mutation"} {
		t.Run(mode, func(t *testing.T) {
			proj := captureFixture(t, []string{"go", "test", "."})
			body := "package capturefixture\nimport (\"testing\";\"time\")\nfunc TestBehavior(t *testing.T){time.Sleep(10*time.Second);t.Fatal(\"missing\")}\n"
			if mode == "mutation" {
				body = "package capturefixture\nimport (\"testing\";\"os\")\nfunc TestBehavior(t *testing.T){if err:=os.WriteFile(\"behavior.go\",[]byte(\"package capturefixture\\nfunc Ready() bool{return true}\\n\"),0600);err!=nil{t.Fatal(err)};t.Fatal(\"mutation executed\")}\n"
			}
			if err := os.WriteFile(filepath.Join(proj, "behavior_test.go"), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			st, err := store.Open(storePathFromEnv(t))
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			// Mutation needs compilation/startup time; only timeout mode tests a short deadline.
			timeout := 30 * time.Second
			if mode == "timeout" {
				timeout = time.Second
			}
			if mode == "cancel" {
				go func() { time.Sleep(500 * time.Millisecond); cancel() }()
			}
			r, err := orchestrator.NewEngine(st).CaptureTest(ctx, orchestrator.CaptureRequest{Root: proj, TreeDir: filepath.Join(proj, "docs/beacon/v0.1.0/tree"), NodeID: "t1", ACID: "t1-1", Phase: "red", By: "maker", Timeout: timeout})
			if err == nil {
				t.Fatal("invalid capture accepted")
			}
			if mode == "mutation" {
				log, logErr := os.ReadFile(r.StdoutPath)
				source, sourceErr := os.ReadFile(filepath.Join(proj, "behavior.go"))
				if logErr != nil || sourceErr != nil || !strings.Contains(string(log), "mutation executed") || !strings.Contains(string(source), "return true") {
					t.Fatalf("mutation process did not reach assertion: log=%s source=%s read errors=%v/%v capture=%+v err=%v", log, source, logErr, sourceErr, r, err)
				}
				if !strings.Contains(err.Error(), "capture_input_changed") || r.ArtifactID == "" || r.TimedOut || r.Cancelled {
					t.Fatalf("mutation not retained: %+v %v", r, err)
				}
			}
			if mode == "timeout" && r.ResultPath != "" && !r.TimedOut {
				t.Fatalf("timeout not recorded: %+v", r)
			}
		})
	}
}
