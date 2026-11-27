package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

func prepareCaptureTree(t *testing.T, proj string) {
	t.Helper()
	for name, body := range map[string]string{
		"go.mod":           "module capturefixture\n\ngo 1.23\n",
		"behavior.go":      "package capturefixture\nfunc Ready() bool{return false}\n",
		"behavior_test.go": "package capturefixture\nimport \"testing\"\nfunc TestBehavior(t *testing.T){if !Ready(){t.Fatal(\"behavior missing\")}}\n",
	} {
		if err := os.WriteFile(filepath.Join(proj, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	spec, _ := json.Marshal(map[string]any{"schema": "beacon-capture-command.v1", "argv": []string{"go", "test", "."}, "test_files": []string{"behavior_test.go"}, "required_tests": []string{"TestBehavior"}})
	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, id := range []string{"t1", "t2"} {
		_, err := intenttree.UpdateNode(st, filepath.Join(proj, "docs/beacon/v0.1.0/tree"), id, intenttree.NodePatch{Verification: &intenttree.Verification{Type: "test", Spec: string(spec)}})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// Real captures supply provenance; review is a synthetic test receipt, not a
// claim that this test launched an independent agent.
func writeCapturedEvidence(t *testing.T, proj, id string) string {
	return writeCapturedEvidenceWithRunner(t, proj, id, func(args ...string) string { return mustRun(t, args...) })
}

func writeCapturedEvidenceWithRunner(t *testing.T, proj, id string, run func(...string) string) string {
	t.Helper()
	n, err := intenttree.ReadNode(filepath.Join(proj, "docs/beacon/v0.1.0/tree"), id)
	if err != nil {
		t.Fatal(err)
	}
	by := n.Implementation.By
	results := map[string]map[string]any{}
	for _, phase := range []string{"red", "green"} {
		ready := "false"
		if phase == "green" {
			ready = "true"
		}
		if err := os.WriteFile(filepath.Join(proj, "behavior.go"), []byte("package capturefixture\nfunc Ready() bool{return "+ready+"}\n"), 0600); err != nil {
			t.Fatal(err)
		}
		args := []string{"implement", "capture", "--project", proj, "--version", "v0.1.0", "--node", id, "--ac", n.ACs[0].ID, "--phase", phase, "--by", by}
		if phase == "green" {
			args = append(args, "--red", results["red"]["artifact_id"].(string))
		}
		results[phase] = parseDoc(t, run(args...))
	}
	p := writeEvidence(t, proj, id)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(string(raw), "delivery_receipt: ", 2)
	var receipt intenttree.DeliveryReceipt
	if err := json.Unmarshal([]byte(parts[1]), &receipt); err != nil {
		t.Fatal(err)
	}
	green := results["green"]
	receipt.BinarySHA256 = green["binary_sha256"].(string)
	if commit, _ := green["source_commit"].(string); commit != "" {
		receipt.SourceCommit = commit
	}
	for _, pair := range []struct {
		phase string
		run   *intenttree.DeliveryRun
	}{{"red", receipt.Tasks[0].Red}, {"green", receipt.Tasks[0].Green}} {
		c := results[pair.phase]
		r := pair.run
		r.CaptureID = c["artifact_id"].(string)
		r.StartedAt = c["started_at"].(string)
		r.FinishedAt = c["finished_at"].(string)
		r.TestSHA256 = c["test_sha256"].(string)
		exit := int(c["exit_code"].(float64))
		r.ExitCode = &exit
		r.Command = nil
		for _, a := range c["command"].([]any) {
			r.Command = append(r.Command, a.(string))
		}
		log, err := os.ReadFile(c["stdout_path"].(string))
		if err != nil {
			t.Fatal(err)
		}
		r.LogPath = id + "-" + pair.phase + ".log"
		sum := sha256.Sum256(log)
		r.LogSHA256 = fmt.Sprintf("%x", sum)
		if err := os.WriteFile(filepath.Join(filepath.Dir(p), r.LogPath), log, 0600); err != nil {
			t.Fatal(err)
		}
	}
	review := intenttree.IndependentReview{NodeID: id, RequirementFingerprint: receipt.RequirementFingerprint, SourceCommit: receipt.SourceCommit, BinarySHA256: receipt.BinarySHA256, Reviewer: "qa", Verdict: "pass", Findings: []string{}, ReviewedAt: time.Now().UTC().Format(time.RFC3339Nano), SourceSHA256: green["source_sha256"].(string), Captures: map[string]string{n.ACs[0].ID: green["artifact_id"].(string)}}
	b, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	if err := os.WriteFile(filepath.Join(filepath.Dir(p), receipt.Review.ReceiptPath), b, 0600); err != nil {
		t.Fatal(err)
	}
	receipt.Review.ReceiptSHA256 = fmt.Sprintf("%x", sum)
	b, err = json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	header := fmt.Sprintf("source_commit: %s\nbinary_sha256: %s\nrequirement_fingerprint: %s\n", receipt.SourceCommit, receipt.BinarySHA256, receipt.RequirementFingerprint)
	if err := os.WriteFile(p, []byte(header+"delivery_receipt: "+string(b)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
