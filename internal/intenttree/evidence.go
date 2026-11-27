package intenttree

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"beacon-go/internal/core"
)

// Evidence metadata reason codes.
const (
	ReasonEvidenceMetaMissing      = "tree_evidence_meta_missing"
	ReasonEvidenceFingerprint      = "tree_evidence_fingerprint_mismatch"
	ReasonEvidenceOutsideRepo      = "tree_evidence_outside_repo"
	ReasonEvidenceLocalRuntimePath = "tree_evidence_local_runtime_path"
	ReasonEvidenceHashMismatch     = "tree_evidence_hash_mismatch"
	ReasonEvidenceArtifactNotFound = "tree_evidence_artifact_not_found"
	ReasonEvidenceFileMissing      = "tree_evidence_file_missing"
	ReasonEvidenceSourceDirty      = "tree_evidence_source_dirty"
)

var (
	commitRe = regexp.MustCompile(`^[0-9a-f]{40}$`)
	sha256Re = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// EvidenceMeta is the machine-checkable header every committed
// verification artifact must carry. source_state is "committed" when
// source_commit contains the artifact and every input it describes.
type EvidenceMeta struct {
	SourceCommit           string
	BinarySHA256           string
	RequirementFingerprint string
	SourceState            string
}

// ParseEvidenceMeta reads flat `key: value` lines, with or without a
// leading "- ". Unknown keys are ignored.
func ParseEvidenceMeta(text string) EvidenceMeta {
	var m EvidenceMeta
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.Trim(strings.TrimSpace(v), "`\"'")
		switch strings.TrimSpace(k) {
		case "source_commit":
			m.SourceCommit = v
		case "binary_sha256":
			m.BinarySHA256 = v
		case "requirement_fingerprint":
			m.RequirementFingerprint = v
		case "source_state":
			m.SourceState = v
		}
	}
	return m
}

// Missing lists the header fields a verification artifact still needs.
func (m EvidenceMeta) Missing() []string {
	var out []string
	if m.SourceCommit == "" {
		out = append(out, "source_commit")
	}
	if m.BinarySHA256 == "" {
		out = append(out, "binary_sha256")
	}
	if m.RequirementFingerprint == "" {
		out = append(out, "requirement_fingerprint")
	}
	return out
}

// ContractBytes is the requirement-contract view of a node used to
// bind evidence. Lifecycle claims (status, implementation, verification
// evidence, child bindings) are excluded so the requirement identity is
// stable from freeze through verified.
func ContractBytes(n Node) ([]byte, error) {
	copy := n
	copy.Status = ""
	copy.ChildFingerprints = nil
	copy.Implementation = nil
	copy.VerificationEvidence = nil
	copy.Fingerprint = ""
	copy.FingerprintAlg = ""
	if copy.ACs == nil {
		copy.ACs = []AC{}
	}
	return json.Marshal(copy)
}

// ContractFingerprint is the stable requirement identity evidence binds.
func ContractFingerprint(n Node) (string, error) {
	raw, err := ContractBytes(n)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// ValidateEvidenceFile enforces the committed-evidence contract before a
// leaf may be marked verified: the file lives inside the repository, is
// not runtime-only state, carries source/binary/fingerprint metadata,
// and binds the CURRENT requirement contract.
func ValidateEvidenceFile(projRoot, nodeID string, n Node, absPath string) (EvidenceMeta, error) {
	root, err := filepath.Abs(projRoot)
	if err != nil {
		return EvidenceMeta{}, err
	}
	if real, rerr := filepath.EvalSymlinks(root); rerr == nil {
		root = real
	}
	abs, err := filepath.Abs(absPath)
	if err != nil {
		return EvidenceMeta{}, err
	}
	if real, rerr := filepath.EvalSymlinks(abs); rerr == nil {
		abs = real
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return EvidenceMeta{}, core.NewError(ReasonEvidenceOutsideRepo, "intenttree.ValidateEvidenceFile",
			"evidence must live inside the repository: "+absPath)
	}
	clean := filepath.ToSlash(rel)
	if strings.HasPrefix(clean, ".beacon/") || strings.HasPrefix(clean, ".loopx/") {
		return EvidenceMeta{}, core.NewError(ReasonEvidenceLocalRuntimePath, "intenttree.ValidateEvidenceFile",
			"evidence must not live under gitignored runtime state: "+clean)
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return EvidenceMeta{}, core.NewError(ReasonEvidenceFileMissing, "intenttree.ValidateEvidenceFile",
			"evidence file unreadable: "+absPath)
	}
	meta := ParseEvidenceMeta(string(raw))
	if missing := meta.Missing(); len(missing) > 0 {
		return meta, core.NewError(ReasonEvidenceMetaMissing, "intenttree.ValidateEvidenceFile",
			"evidence is missing "+strings.Join(missing, ", ")+"; every artifact records source_commit, binary_sha256 and requirement_fingerprint")
	}
	if !commitRe.MatchString(meta.SourceCommit) {
		return meta, core.NewError(ReasonEvidenceMetaMissing, "intenttree.ValidateEvidenceFile",
			"source_commit must be a full 40-hex git commit")
	}
	if !sha256Re.MatchString(meta.BinarySHA256) {
		return meta, core.NewError(ReasonEvidenceMetaMissing, "intenttree.ValidateEvidenceFile",
			"binary_sha256 must be a full 64-hex sha256")
	}
	if strings.EqualFold(meta.SourceState, "dirty") {
		return meta, core.NewError(ReasonEvidenceSourceDirty, "intenttree.ValidateEvidenceFile",
			"evidence was registered from a dirty source tree; commit first")
	}
	want, err := ContractFingerprint(n)
	if err != nil {
		return meta, err
	}
	if meta.RequirementFingerprint != want {
		return meta, core.NewError(ReasonEvidenceFingerprint, "intenttree.ValidateEvidenceFile",
			"requirement_fingerprint does not match the current contract for "+nodeID+"; regenerate the evidence")
	}
	return meta, nil
}
