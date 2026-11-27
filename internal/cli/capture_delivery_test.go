package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/orchestrator"
	"beacon-go/internal/store"
)

func TestCaptureRejectsHandwrittenDelivery(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	if _, err := freeze(t, proj, "t1"); err != nil {
		t.Fatal(err)
	}
	p := writeEvidence(t, proj, "t1")
	mustRun(t, "implement", "report", "--project", proj, "--node", "t1", "--by", "dev", "--artifact", p)
	_, err := runCLI(t, "verify", "leaf", "--project", proj, "--node", "t1", "--by", "qa", "--artifact", p)
	if err == nil || !strings.Contains(err.Error(), "delivery_capture_required") {
		t.Fatalf("handwritten receipt must fail: %v", err)
	}
	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	reg, err := orchestrator.NewEngine(st).RegisterScopedArtifact("node", "t1", "test_result", p, "qa")
	if err != nil {
		t.Fatal(err)
	}
	_, err = intenttree.MarkVerified(st, filepath.Join(proj, "docs/beacon/v0.1.0/tree"), "t1", reg.Artifact.ID, reg.Artifact.ContentHash, "qa")
	if err == nil || !strings.Contains(err.Error(), "delivery_capture_required") {
		t.Fatalf("direct service bypass: %v", err)
	}
}

func TestCaptureRejectsPublicRefreshOfReservedResult(t *testing.T) {
	proj := captureFixture(t, []string{"go", "test", "."})
	red := parseDoc(t, mustRun(t, captureArgs(proj, "red")...))
	p := red["result_path"].(string)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	for _, alias := range []string{p, filepath.Join(filepath.Dir(p), ".", filepath.Base(p))} {
		_, err := runCLI(t, "goal", "artifacts", "register", "--scope-kind", "node", "--scope-id", "t1", "--kind", "log", "--path", alias, "--by", "maker")
		if err == nil || !strings.Contains(err.Error(), "artifact_capture_immutable") {
			t.Fatalf("reserved refresh accepted: %v", err)
		}
	}
	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := orchestrator.NewEngine(st).ReadCapture("t1", red["artifact_id"].(string)); err == nil {
		t.Fatal("mutated capture was re-blessed")
	}
}

func TestCaptureMakerVerifyHasNoRegistrationSideEffect(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	prepareCaptureTree(t, proj)
	if _, err := freeze(t, proj, "t1"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "implement", "report", "--project", proj, "--node", "t1", "--by", "dev", "--artifact", writeEvidence(t, proj, "t1"))
	p := writeCapturedEvidence(t, proj, "t1")
	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	before, err := st.Artifacts().ListByScope("node", "t1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = runCLI(t, "verify", "leaf", "--project", proj, "--node", "t1", "--by", "dev", "--artifact", p)
	if err == nil {
		t.Fatal("self verification accepted")
	}
	after, err := st.Artifacts().ListByScope("node", "t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) {
		t.Fatalf("rejected verification registered evidence: %d -> %d", len(before), len(after))
	}
}

func TestCaptureRollupRefusalHasNoPartialWrites(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	prepareCaptureTree(t, proj)
	// Two feature branches: first can roll up; the second cannot.
	dir := filepath.Join(proj, "docs/beacon/v0.1.0/tree")
	idx, err := intenttree.ReadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	f2 := intenttree.Node{ID: "f2", Kind: intenttree.KindFeature, ParentID: "m", Title: "second", Path: "m/f2", Status: intenttree.StatusFrozen}
	f2, err = intenttree.Stamp(f2)
	if err != nil {
		t.Fatal(err)
	}
	if err := intenttree.WriteNode(dir, f2); err != nil {
		t.Fatal(err)
	}
	idx.Nodes = append(idx.Nodes, intenttree.IndexEntry{ID: f2.ID, Kind: f2.Kind, ParentID: f2.ParentID, Title: f2.Title, Path: f2.Path, Status: f2.Status, Fingerprint: f2.Fingerprint, FingerprintAlg: f2.FingerprintAlg})
	// Move t2 into the refusing branch using fixture-only setup.
	n, err := intenttree.ReadNode(dir, "t2")
	if err != nil {
		t.Fatal(err)
	}
	n.ParentID = "f2"
	n, err = intenttree.Stamp(n)
	if err != nil {
		t.Fatal(err)
	}
	if err := intenttree.WriteNode(dir, n); err != nil {
		t.Fatal(err)
	}
	for i := range idx.Nodes {
		if idx.Nodes[i].ID == "t2" {
			idx.Nodes[i].ParentID = "f2"
			idx.Nodes[i].Fingerprint = n.Fingerprint
		}
	}
	if err := intenttree.WriteIndex(dir, idx); err != nil {
		t.Fatal(err)
	}
	if _, err := freeze(t, proj, "t1"); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "implement", "report", "--project", proj, "--node", "t1", "--by", "dev", "--artifact", writeEvidence(t, proj, "t1"))
	mustRun(t, "verify", "leaf", "--project", proj, "--node", "t1", "--by", "qa", "--artifact", writeCapturedEvidence(t, proj, "t1"))
	before, err := os.ReadFile(intenttree.NodePath(dir, "f"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = runCLI(t, "verify", "rollup", "--project", proj, "--node", "m")
	if err == nil {
		t.Fatal("incomplete branch accepted")
	}
	after, err := os.ReadFile(intenttree.NodePath(dir, "f"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("failed rollup partially mutated earlier branch")
	}
}

func TestCaptureDeliveryRefusesCandidateAndReferenceForgery(t *testing.T) {
	for _, mode := range []string{"unknown", "wrong_phase", "candidate", "assertion", "review_binding", "envelope_exit"} {
		t.Run(mode, func(t *testing.T) {
			proj := treeTestEnv(t)
			seedFreezeTree(t, proj, false)
			prepareCaptureTree(t, proj)
			if _, err := freeze(t, proj, "t1"); err != nil {
				t.Fatal(err)
			}
			mustRun(t, "implement", "report", "--project", proj, "--node", "t1", "--by", "dev", "--artifact", writeEvidence(t, proj, "t1"))
			p := writeCapturedEvidence(t, proj, "t1")
			raw, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			parts := strings.SplitN(string(raw), "delivery_receipt: ", 2)
			var receipt map[string]any
			if err := json.Unmarshal([]byte(parts[1]), &receipt); err != nil {
				t.Fatal(err)
			}
			task := receipt["tasks"].([]any)[0].(map[string]any)
			red := task["red"].(map[string]any)
			green := task["green"].(map[string]any)
			switch mode {
			case "unknown":
				green["capture_id"] = "art_missing"
			case "wrong_phase":
				green["capture_id"] = red["capture_id"]
			case "candidate":
				if err := os.WriteFile(filepath.Join(proj, "behavior.go"), []byte("package capturefixture\nfunc Ready() bool{return false}\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "assertion":
				f := filepath.Join(proj, "behavior_test.go")
				b, err := os.ReadFile(f)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(f, append(b, []byte("\n// changed\n")...), 0600); err != nil {
					t.Fatal(err)
				}
			case "review_binding":
				reviewRef := receipt["review"].(map[string]any)
				reviewPath := filepath.Join(filepath.Dir(p), reviewRef["receipt_path"].(string))
				reviewRaw, readErr := os.ReadFile(reviewPath)
				if readErr != nil {
					t.Fatal(readErr)
				}
				var review map[string]any
				if err := json.Unmarshal(reviewRaw, &review); err != nil {
					t.Fatal(err)
				}
				review["captures"] = map[string]string{"t1-1": "another_green"}
				reviewRaw, err = json.Marshal(review)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(reviewPath, reviewRaw, 0600); err != nil {
					t.Fatal(err)
				}
				h := sha256.Sum256(reviewRaw)
				reviewRef["receipt_sha256"] = fmt.Sprintf("%x", h)
			case "envelope_exit":
				green["command"] = []string{"true"}
				red["command"] = []string{"true"}
			}
			b, err := json.Marshal(receipt)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(parts[0]+"delivery_receipt: "+string(b)+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			_, err = runCLI(t, "verify", "leaf", "--project", proj, "--node", "t1", "--by", "qa", "--artifact", p)
			if err == nil {
				t.Fatalf("%s forgery accepted", mode)
			}
		})
	}
}
