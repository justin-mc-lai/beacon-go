package intenttree

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// ImplementationReport is the outcome of one leaf implement call.
type ImplementationReport struct {
	NodeID   string `json:"node_id"`
	Status   string `json:"status"`
	Already  bool   `json:"already"`
	Outcome  string `json:"outcome"`
	Artifact string `json:"artifact,omitempty"`
}

// ReportImplementation records a leaf execution. Success moves
// frozen→implemented once; a failure keeps the frozen fact and stores
// the reason instead of pretending progress.
func ReportImplementation(st *store.Store, dir, nodeID, by, outcome, artifact, reason string) (ImplementationReport, error) {
	if strings.TrimSpace(by) == "" {
		return ImplementationReport{}, core.NewError(ReasonInvalidIndex, "intenttree.ReportImplementation", "--by is required")
	}
	if outcome != "done" && outcome != "failed" {
		return ImplementationReport{}, core.NewError(ReasonOutcomeInvalid, "intenttree.ReportImplementation", "outcome must be done or failed")
	}
	idx, n, err := loadMutationNode(st, dir, nodeID)
	if err != nil {
		return ImplementationReport{}, err
	}
	entry, ok := findEntry(idx, nodeID)
	if !ok {
		return ImplementationReport{}, core.NewError(ReasonNodeNotFound, "intenttree.ReportImplementation", "node not found: "+nodeID)
	}
	if entry.Kind != KindTouchpoint {
		return ImplementationReport{}, core.NewError(ReasonInvalidChildKind, "intenttree.ReportImplementation",
			"only touchpoint leaves can report execution: "+nodeID+" is "+string(entry.Kind))
	}
	if entry.Status != StatusFrozen && !(entry.Status == StatusImplemented && outcome == "done") {
		return ImplementationReport{}, core.NewError(ReasonNodeNotFrozen, "intenttree.ReportImplementation",
			"leaf must be frozen before implement: "+nodeID+" is "+string(entry.Status))
	}
	if n.Verification == nil || strings.TrimSpace(n.Verification.Spec) == "" {
		return ImplementationReport{}, core.NewError(ReasonVerificationMissing, "intenttree.ReportImplementation",
			"leaf has no executable verification spec: "+nodeID)
	}
	initialContract, err := ContractFingerprint(n)
	if err != nil {
		return ImplementationReport{}, err
	}
	var raw []byte
	if outcome == "done" {
		if strings.TrimSpace(artifact) == "" {
			return ImplementationReport{}, core.NewError(ReasonArtifactRequired, "intenttree.ReportImplementation", "--artifact is required for done")
		}
		artifact, err = implementationArtifactPath(artifact)
		if err != nil {
			return ImplementationReport{}, err
		}
		raw, err = os.ReadFile(artifact)
		if err != nil {
			return ImplementationReport{}, err
		}
		if len(bytes.TrimSpace(raw)) == 0 {
			return ImplementationReport{}, core.NewError(ReasonArtifactRequired, "intenttree.ReportImplementation", "artifact must contain non-whitespace evidence")
		}
	}
	unlock, err := lockTreeChange(dir)
	if err != nil {
		return ImplementationReport{}, err
	}
	defer unlock()
	if err := recoverTreeChange(dir, st); err != nil {
		return ImplementationReport{}, err
	}
	idx, n, err = readMutationNode(dir, nodeID)
	if err != nil {
		return ImplementationReport{}, err
	}
	currentContract, err := ContractFingerprint(n)
	if err != nil {
		return ImplementationReport{}, err
	}
	if initialContract != currentContract {
		return ImplementationReport{}, core.NewError(ReasonChangeBaseMismatch, "intenttree.ReportImplementation", "requirement changed while reading implementation evidence")
	}
	if n.Status != StatusFrozen && !(n.Status == StatusImplemented && outcome == "done") {
		return ImplementationReport{}, core.NewError(ReasonNodeNotFrozen, "intenttree.ReportImplementation", "leaf is no longer frozen")
	}
	now := time.Now().UTC()
	rec := &ImplementationRecord{By: strings.TrimSpace(by), Outcome: outcome, At: now.Format(time.RFC3339Nano)}
	already := n.Status == StatusImplemented
	if outcome == "done" {
		sum := sha256.Sum256(raw)
		if already {
			if n.Implementation == nil || n.Implementation.ArtifactHash != hex.EncodeToString(sum[:]) {
				return ImplementationReport{}, core.NewError(ReasonEvidenceHashMismatch, "intenttree.ReportImplementation", "artifact differs from the recorded implementation")
			}
			recordedPath, err := implementationArtifactPath(n.Implementation.ArtifactPath)
			if err != nil || recordedPath != artifact {
				return ImplementationReport{}, core.NewError(ReasonEvidenceHashMismatch, "intenttree.ReportImplementation", "artifact path differs from the recorded implementation")
			}
			rec = n.Implementation
		} else {
			rec.ArtifactPath = artifact
			rec.ArtifactHash = hex.EncodeToString(sum[:])
			n.Status = StatusImplemented
		}
	} else {
		rec.ReasonCode = strings.TrimSpace(reason)
		if rec.ReasonCode == "" {
			rec.ReasonCode = "implementation_failed"
		}
	}
	n.Implementation = rec
	n, err = Stamp(n)
	if err != nil {
		return ImplementationReport{}, err
	}
	event := map[string]any{
		"type": "implementation_" + outcome, "node_id": nodeID, "by": rec.By,
		"outcome": outcome, "artifact": rec.ArtifactPath, "artifact_hash": rec.ArtifactHash,
		"reason_code": rec.ReasonCode, "at": rec.At,
	}
	eventTime, err := time.Parse(time.RFC3339Nano, rec.At)
	if err != nil {
		return ImplementationReport{}, err
	}
	raw, err = json.MarshalIndent(event, "", "  ")
	if err != nil {
		return ImplementationReport{}, err
	}
	eventPath := filepath.Join(dir, "events", nodeID+"."+eventTime.Format("20060102T150405.000000000Z")+".json")
	if err := persistMutation(st, dir, idx, n, map[string][]byte{eventPath: append(raw, '\n')}, ""); err != nil {
		return ImplementationReport{}, err
	}
	return ImplementationReport{NodeID: nodeID, Status: string(n.Status), Already: already, Outcome: outcome, Artifact: rec.ArtifactPath}, nil
}

func implementationArtifactPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}
