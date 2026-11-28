package cli

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/orchestrator"
	"beacon-go/internal/store"
)

func TestCaptureSelectorDiscoversFilesWithoutContractEdit(t *testing.T) {
	proj := captureFixture(t, []string{"go", "test", "."})
	setCaptureSelector(t, proj)
	before := captureNodeFingerprint(t, proj)
	first := parseDoc(t, mustRun(t, captureArgs(proj, "red")...))
	if err := os.WriteFile(filepath.Join(proj, "unrelated_test.go"), []byte("package capturefixture\nimport \"testing\"\nfunc TestUnrelated(t *testing.T){}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	second := parseDoc(t, mustRun(t, captureArgs(proj, "red")...))
	if captureNodeFingerprint(t, proj) != before {
		t.Fatal("discovery rewrote frozen contract")
	}
	if len(second["test_files"].([]any)) != 2 || second["test_sha256"] == first["test_sha256"] {
		t.Fatalf("snapshot not refreshed: %v", second)
	}
	if _, err := runCLI(t, append(captureArgs(proj, "green"), "--red", first["artifact_id"].(string))...); err == nil {
		t.Fatal("old Red accepted different test inputs")
	}
	if err := os.WriteFile(filepath.Join(proj, "behavior.go"), []byte("package capturefixture\nfunc Ready()bool{return true}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	mustRun(t, append(captureArgs(proj, "green"), "--red", second["artifact_id"].(string))...)
}

func TestCaptureSelectorReplayRejectsHistoricalExtraTests(t *testing.T) {
	for _, mode := range []string{"rename", "delete"} {
		t.Run(mode, func(t *testing.T) {
			proj := captureFixture(t, []string{"go", "test", "."})
			setCaptureSelector(t, proj)
			if mode == "delete" {
				if err := os.WriteFile(filepath.Join(proj, "old_test.go"), []byte("package capturefixture\nimport \"testing\"\nfunc TestOld(t *testing.T){}\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			gitCLI(t, proj, "init")
			gitCLI(t, proj, "add", ".")
			gitCLI(t, proj, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "baseline")
			base := strings.TrimSpace(gitCLI(t, proj, "rev-parse", "HEAD"))
			if mode == "delete" {
				if err := os.Remove(filepath.Join(proj, "old_test.go")); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Rename(filepath.Join(proj, "behavior_test.go"), filepath.Join(proj, "renamed_test.go")); err != nil {
				t.Fatal(err)
			}
			gitCLI(t, proj, "add", ".")
			gitCLI(t, proj, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "rename")
			_, err := runCLI(t, append(captureArgs(proj, "red"), "--baseline", base)...)
			if err == nil || !strings.Contains(err.Error(), "capture_replay_test_set_unsupported") {
				t.Fatalf("must refuse old tests, got %v", err)
			}
		})
	}
}

func setCaptureSelector(t *testing.T, proj string) {
	t.Helper()
	dir := filepath.Join(proj, "docs/beacon/v0.1.0/tree")
	mustRun(t, "change", "request", "--project", proj, "--node", "t1", "--base-fingerprint", captureNodeFingerprint(t, proj), "--summary", "selector fixture", "--by", "owner")
	spec, _ := json.Marshal(map[string]any{"schema": "beacon-capture-command.v1", "argv": []string{"go", "test", ".", "-run", "^TestBehavior$"}, "required_tests": []string{"TestBehavior"}, "discover_tests": true})
	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := intenttree.UpdateNode(st, dir, "t1", intenttree.NodePatch{Verification: &intenttree.Verification{Type: "test", Spec: string(spec)}}); err != nil {
		t.Fatal(err)
	}
	if _, err := freeze(t, proj, "t1"); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureSelectorInterruptedDiscoveryRetainsAttempt(t *testing.T) {
	for _, mode := range []string{"cancel", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			proj := captureFixture(t, []string{"go", "test", "."})
			setCaptureSelector(t, proj)
			st, err := store.Open(storePathFromEnv(t))
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			timeout := time.Second
			if mode == "cancel" {
				cancel()
			} else {
				timeout = time.Nanosecond
			}
			r, err := orchestrator.NewEngine(st).CaptureTest(ctx, orchestrator.CaptureRequest{Root: proj, TreeDir: filepath.Join(proj, "docs/beacon/v0.1.0/tree"), NodeID: "t1", ACID: "t1-1", Phase: "red", By: "maker", Timeout: timeout})
			if err == nil || r.ArtifactID == "" || r.Preflight == nil || r.ExitCode != -1 || (mode == "cancel" && !r.Cancelled) || (mode == "timeout" && !r.TimedOut) {
				t.Fatalf("lost discovery: %+v %v", r, err)
			}
			attempts, _ := filepath.Glob(filepath.Join(proj, ".beacon/captures/attempt-*"))
			if len(attempts) != 1 {
				t.Fatalf("want unique attempt: %v", attempts)
			}
			if _, err := orchestrator.NewEngine(st).ReadCapture("t1", r.ArtifactID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCaptureSelectorOneDiscoveryAndDegenerateInputs(t *testing.T) {
	for _, mode := range []string{"once", "failed", "zero", "skip", "duplicate", "alias"} {
		t.Run(mode, func(t *testing.T) {
			proj := captureFixture(t, []string{"go", "test", "."})
			setCaptureSelector(t, proj)
			goBin, err := exec.LookPath("go")
			if err != nil {
				t.Fatal(err)
			}
			tools := t.TempDir()
			counter := filepath.Join(tools, "calls")
			body := "#!/bin/sh\nif [ \"$1\" = list ]; then\n  echo list >> '" + counter + "'\n"
			if mode == "failed" {
				body += "  echo discovery-broken >&2\n  exit 23\n"
			}
			if mode == "zero" {
				body += "  echo '{\"Dir\":\"" + proj + "\"}'\n  exit 0\n"
			}
			if mode == "duplicate" || mode == "alias" {
				other := proj
				if mode == "alias" {
					other += "/."
				}
				body += "  echo '{\"Dir\":\"" + proj + "\",\"TestGoFiles\":[\"behavior_test.go\"]}'\n"
				body += "  echo '{\"Dir\":\"" + other + "\",\"TestGoFiles\":[\"behavior_test.go\"]}'\n  exit 0\n"
			}
			body += "fi\nexec '" + goBin + "' \"$@\"\n"
			if err := os.WriteFile(filepath.Join(tools, "go"), []byte(body), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
			if mode == "skip" {
				if err := os.WriteFile(filepath.Join(proj, "behavior_test.go"), []byte("package capturefixture\nimport \"testing\"\nfunc TestBehavior(t *testing.T){t.Skip(\"no execution\")}\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			st, err := store.Open(storePathFromEnv(t))
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			r, err := orchestrator.NewEngine(st).CaptureTest(context.Background(), orchestrator.CaptureRequest{Root: proj, TreeDir: filepath.Join(proj, "docs/beacon/v0.1.0/tree"), NodeID: "t1", ACID: "t1-1", Phase: "red", By: "maker", Timeout: 30 * time.Second})
			if (mode == "once") != (err == nil) || r.ArtifactID == "" {
				t.Fatalf("mode %s: %+v %v", mode, r, err)
			}
			calls, err := os.ReadFile(counter)
			if err != nil || string(calls) != "list\n" {
				t.Fatalf("discovery repeated: %q %v", calls, err)
			}
			if mode == "failed" && (r.Preflight.ExitCode != 23 || !strings.Contains(r.Preflight.Stderr, "discovery-broken") || r.ExitCode != -1) {
				t.Fatalf("invented preflight: %+v", r)
			}
		})
	}
}
