package cli

import (
	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCaptureCustomProtocolRedGreenAndRejections(t *testing.T) {
	for _, mode := range []string{"valid", "valid_alias", "zero", "wrong_binding", "skip"} {
		t.Run(mode, func(t *testing.T) {
			proj := captureFixture(t, []string{"go", "test", "."})
			program := `#!/usr/bin/env python3
import json,os,sys
ok='return true' in open('behavior.go').read()
case={'name':'behavior','status':'passed' if ok else 'failed','assertions':1}
mode=` + fmt.Sprintf("%q", mode) + `
if mode=='skip':case['status']='skipped'
result={'schema':'beacon-capture-tests.v1','invocation':os.environ['BEACON_CAPTURE_RESULT'],'node_id':os.environ['BEACON_CAPTURE_NODE'],'ac_id':os.environ['BEACON_CAPTURE_AC'],'phase':os.environ['BEACON_CAPTURE_PHASE'],'cases':[] if mode=='zero' else [case]}
if mode=='wrong_binding':result['invocation']='old'
print(json.dumps(result))
sys.exit(0 if ok else 1)
`
			if err := os.WriteFile(filepath.Join(proj, "custom.py"), []byte(program), 0700); err != nil {
				t.Fatal(err)
			}
			command := "./custom.py"
			if mode == "valid_alias" {
				if err := os.MkdirAll(filepath.Join(proj, ".beacon"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(proj, "custom.py"), filepath.Join(proj, ".beacon/custom-link")); err != nil {
					t.Fatal(err)
				}
				command = "./.beacon/custom-link"
			}
			mustRun(t, "change", "request", "--project", proj, "--node", "t1", "--base-fingerprint", captureNodeFingerprint(t, proj), "--summary", "custom fixture", "--by", "fixture")
			st, err := store.Open(storePathFromEnv(t))
			if err != nil {
				t.Fatal(err)
			}
			spec, _ := json.Marshal(map[string]any{"schema": "beacon-capture-command.v1", "adapter": "custom-json", "argv": []string{command}, "test_files": []string{"custom.py"}, "required_tests": []string{"behavior"}})
			_, err = intenttree.UpdateNode(st, filepath.Join(proj, "docs/beacon/v0.1.0/tree"), "t1", intenttree.NodePatch{Verification: &intenttree.Verification{Type: "test", Spec: string(spec)}})
			st.Close()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := freeze(t, proj, "t1"); err != nil {
				t.Fatal(err)
			}
			out, err := runCLI(t, captureArgs(proj, "red")...)
			if mode != "valid" && mode != "valid_alias" {
				if err == nil {
					t.Fatalf("invalid custom accepted: %s", out)
				}
				if parseDoc(t, out)["artifact_id"] == "" {
					t.Fatal("failure not retained")
				}
				return
			}
			if err != nil {
				t.Fatalf("custom red: %v %s", err, out)
			}
			red := parseDoc(t, out)
			if err := os.WriteFile(filepath.Join(proj, "behavior.go"), []byte("package capturefixture\nfunc Ready()bool{return true}\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if out, err := runCLI(t, append(captureArgs(proj, "green"), "--red", red["artifact_id"].(string))...); err != nil {
				t.Fatalf("custom green: %v %s", err, out)
			}
		})
	}
}
