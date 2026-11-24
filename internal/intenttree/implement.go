package intenttree

import (
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
	idx, err := ReadIndex(dir)
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
	if entry.Status == StatusImplemented && outcome == "done" {
		return ImplementationReport{NodeID: nodeID, Status: string(entry.Status), Already: true, Outcome: outcome}, nil
	}
	if entry.Status != StatusFrozen {
		return ImplementationReport{}, core.NewError(ReasonNodeNotFrozen, "intenttree.ReportImplementation",
			"leaf must be frozen before implement: "+nodeID+" is "+string(entry.Status))
	}
	n, err := ReadNode(dir, nodeID)
	if err != nil {
		return ImplementationReport{}, err
	}
	if n.Verification == nil || strings.TrimSpace(n.Verification.Spec) == "" {
		return ImplementationReport{}, core.NewError(ReasonVerificationMissing, "intenttree.ReportImplementation",
			"leaf has no executable verification spec: "+nodeID)
	}
	now := time.Now().UTC()
	rec := &ImplementationRecord{By: strings.TrimSpace(by), Outcome: outcome, At: now.Format(time.RFC3339Nano)}
	if outcome == "done" {
		if strings.TrimSpace(artifact) == "" {
			return ImplementationReport{}, core.NewError(ReasonArtifactRequired, "intenttree.ReportImplementation", "--artifact is required for done")
		}
		raw, err := os.ReadFile(artifact)
		if err != nil {
			return ImplementationReport{}, err
		}
		sum := sha256.Sum256(raw)
		rec.ArtifactPath = artifact
		rec.ArtifactHash = hex.EncodeToString(sum[:])
		n.Status = StatusImplemented
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
	if err := WriteNode(dir, n); err != nil {
		return ImplementationReport{}, err
	}
	entry.Status = n.Status
	entry.Fingerprint = n.Fingerprint
	entry.FingerprintAlg = n.FingerprintAlg
	for i := range idx.Nodes {
		if idx.Nodes[i].ID == nodeID {
			idx.Nodes[i] = entry
		}
	}
	if err := WriteIndex(dir, idx); err != nil {
		return ImplementationReport{}, err
	}
	if err := st.UpdateTreeNodeStatus(n.ID, string(n.Status), n.Fingerprint, now.Format(time.RFC3339Nano)); err != nil {
		return ImplementationReport{}, err
	}
	event := map[string]any{
		"type": "implementation_" + outcome, "node_id": nodeID, "by": rec.By,
		"outcome": outcome, "artifact": rec.ArtifactPath, "artifact_hash": rec.ArtifactHash,
		"reason_code": rec.ReasonCode, "at": rec.At,
	}
	raw, _ := json.MarshalIndent(event, "", "  ")
	_ = atomicWrite(filepath.Join(dir, "events", nodeID+"."+now.Format("20060102T150405.000000000Z")+".json"), append(raw, '\n'))
	return ImplementationReport{NodeID: nodeID, Status: string(n.Status), Outcome: outcome, Artifact: rec.ArtifactPath}, nil
}
