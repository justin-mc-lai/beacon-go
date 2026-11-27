package intenttree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContractFingerprintIgnoresLifecycle(t *testing.T) {
	n := Node{
		ID: "n1", Kind: KindTouchpoint, Title: "t", Path: "root/m/f/t",
		Status: StatusFrozen, ACs: []AC{{ID: "AC-001", Text: "x"}},
	}
	frozen, err := ContractFingerprint(n)
	if err != nil {
		t.Fatal(err)
	}
	n.Status = StatusVerified
	n.Implementation = &ImplementationRecord{By: "dev", Outcome: "done"}
	n.VerificationEvidence = &EvidenceRef{ArtifactID: "art_1", ContentHash: "deadbeef"}
	verified, err := ContractFingerprint(n)
	if err != nil {
		t.Fatal(err)
	}
	if frozen != verified {
		t.Fatalf("requirement fingerprint moved with lifecycle: %s != %s", frozen, verified)
	}
	n.ACs[0].Text = "changed"
	changed, err := ContractFingerprint(n)
	if err != nil {
		t.Fatal(err)
	}
	if changed == frozen {
		t.Fatal("requirement fingerprint must move when the contract changes")
	}
}

func TestValidateEvidenceFileContract(t *testing.T) {
	root := t.TempDir()
	n := Node{ID: "n1", Kind: KindTouchpoint, Title: "t", Path: "root/m/f/t", Status: StatusImplemented}
	fp, err := ContractFingerprint(n)
	if err != nil {
		t.Fatal(err)
	}
	good := "source_commit: " + strings.Repeat("a", 40) + "\n" +
		"binary_sha256: " + strings.Repeat("b", 64) + "\n" +
		"requirement_fingerprint: " + fp + "\n\nproof\n"
	write := func(rel, body string) string {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if _, err := ValidateEvidenceFile(root, "n1", n, write("docs/beacon/v0.1.0/evidence/n1.txt", good)); err != nil {
		t.Fatalf("good evidence refused: %v", err)
	}
	if _, err := ValidateEvidenceFile(root, "n1", n, write(".beacon/artifacts/n1.txt", good)); err == nil {
		t.Fatal("gitignored runtime evidence must be refused")
	}
	outside := filepath.Join(t.TempDir(), "n1.txt")
	if err := os.WriteFile(outside, []byte(good), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateEvidenceFile(root, "n1", n, outside); err == nil {
		t.Fatal("evidence outside the repository must be refused")
	}
	if _, err := ValidateEvidenceFile(root, "n1", n, write("docs/beacon/v0.1.0/evidence/no-meta.txt", "proof\n")); err == nil {
		t.Fatal("evidence without source/binary/fingerprint metadata must be refused")
	}
	badFP := strings.Replace(good, fp, strings.Repeat("c", 64), 1)
	if _, err := ValidateEvidenceFile(root, "n1", n, write("docs/beacon/v0.1.0/evidence/bad-fp.txt", badFP)); err == nil {
		t.Fatal("evidence bound to another requirement fingerprint must be refused")
	}
}

func TestParseEvidenceMetaReadsBacktickedValues(t *testing.T) {
	meta := ParseEvidenceMeta("- source_commit: `" + strings.Repeat("a", 40) + "`\n" +
		"- binary_sha256: `" + strings.Repeat("b", 64) + "`\n" +
		"- requirement_fingerprint: `" + strings.Repeat("c", 64) + "`\n")
	if meta.SourceCommit != strings.Repeat("a", 40) {
		t.Fatalf("source commit = %q", meta.SourceCommit)
	}
	if meta.BinarySHA256 != strings.Repeat("b", 64) {
		t.Fatalf("binary sha = %q", meta.BinarySHA256)
	}
	if meta.RequirementFingerprint != strings.Repeat("c", 64) {
		t.Fatalf("fingerprint = %q", meta.RequirementFingerprint)
	}
}
