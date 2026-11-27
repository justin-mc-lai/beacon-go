package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"beacon-go/internal/orchestrator"
	"beacon-go/internal/store"
)

func TestCapturePreflightProcessFacts(t *testing.T) {
	for _, mode := range []string{"missing", "timeout", "exit"} {
		t.Run(mode, func(t *testing.T) {
			proj := captureFixture(t, []string{"go", "test", "."})
			tools := t.TempDir()
			if mode != "missing" {
				body := "#!/bin/sh\necho actual-preflight-stdout\necho actual-preflight-stderr >&2\n"
				if mode == "timeout" {
					body += "/bin/sleep 10\n"
				} else {
					body += "exit 7\n"
				}
				if err := os.WriteFile(filepath.Join(tools, "go"), []byte(body), 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", tools)
			st, err := store.Open(storePathFromEnv(t))
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			r, err := orchestrator.NewEngine(st).CaptureTest(context.Background(), orchestrator.CaptureRequest{Root: proj, TreeDir: filepath.Join(proj, "docs/beacon/v0.1.0/tree"), NodeID: "t1", ACID: "t1-1", Phase: "red", By: "maker", Timeout: 2 * time.Second})
			if err == nil || r.ArtifactID == "" || r.Status != "rejected" || r.Preflight == nil {
				t.Fatalf("preflight not retained: %+v %v", r, err)
			}
			if mode == "missing" {
				if r.Preflight.Error == "" {
					t.Fatal("spawn failure absent")
				}
				return
			}
			if !strings.Contains(r.Preflight.Stdout, "actual-preflight-stdout") || !strings.Contains(r.Preflight.Stderr, "actual-preflight-stderr") {
				t.Fatalf("actual outputs lost: %+v", r.Preflight)
			}
			if mode == "timeout" && !r.Preflight.TimedOut {
				t.Fatal("timeout flag lost")
			}
			if mode == "exit" && r.Preflight.ExitCode != 7 {
				t.Fatalf("exit lost: %d", r.Preflight.ExitCode)
			}
		})
	}
}
