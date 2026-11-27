package intenttree

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// EvidenceRef is the registered proof that verified a leaf.
type EvidenceRef struct {
	ArtifactID  string `json:"artifact_id"`
	ContentHash string `json:"content_hash"`
	By          string `json:"by"`
	At          string `json:"at"`
}

// VerifyReport is the outcome of a leaf verification or a rollup.
type VerifyReport struct {
	NodeID   string `json:"node_id"`
	Status   string `json:"status"`
	Verified bool   `json:"verified"`
	Children int    `json:"children_verified,omitempty"`
}

// MarkVerified turns an implemented leaf into verified after its
// evidence artifact was registered under the node scope.
func MarkVerified(st *store.Store, dir, nodeID, artifactID, contentHash, by string) (VerifyReport, error) {
	idx, err := ReadIndex(dir)
	if err != nil {
		return VerifyReport{}, err
	}
	entry, ok := findEntry(idx, nodeID)
	if !ok {
		return VerifyReport{}, core.NewError(ReasonNodeNotFound, "intenttree.MarkVerified", "node not found: "+nodeID)
	}
	if entry.Kind != KindTouchpoint {
		return VerifyReport{}, core.NewError(ReasonInvalidChildKind, "intenttree.MarkVerified",
			"only touchpoints carry verification evidence: "+nodeID)
	}
	if entry.Status != StatusImplemented {
		return VerifyReport{}, core.NewError("tree_node_not_implemented", "intenttree.MarkVerified",
			"leaf must be implemented before verification: "+nodeID+" is "+string(entry.Status))
	}
	n, err := ReadNode(dir, nodeID)
	if err != nil {
		return VerifyReport{}, err
	}
	if err := validateRegisteredDelivery(st, dir, n, artifactID, contentHash); err != nil {
		return VerifyReport{}, err
	}
	if err := ValidateVerifier(n, by); err != nil {
		return VerifyReport{}, err
	}
	now := time.Now().UTC()
	n.Status = StatusVerified
	n.VerificationEvidence = &EvidenceRef{ArtifactID: artifactID, ContentHash: contentHash, By: by, At: now.Format(time.RFC3339Nano)}
	n, err = Stamp(n)
	if err != nil {
		return VerifyReport{}, err
	}
	if err := WriteNode(dir, n); err != nil {
		return VerifyReport{}, err
	}
	entry.Status = StatusVerified
	entry.Fingerprint = n.Fingerprint
	entry.FingerprintAlg = n.FingerprintAlg
	for i := range idx.Nodes {
		if idx.Nodes[i].ID == nodeID {
			idx.Nodes[i] = entry
		}
	}
	if err := WriteIndex(dir, idx); err != nil {
		return VerifyReport{}, err
	}
	if err := st.UpdateTreeNodeStatus(nodeID, string(StatusVerified), n.Fingerprint, now.Format(time.RFC3339Nano)); err != nil {
		return VerifyReport{}, err
	}
	return VerifyReport{NodeID: nodeID, Status: string(StatusVerified), Verified: true}, nil
}

func ValidateVerifier(n Node, by string) error {
	if strings.TrimSpace(by) == "" || n.Implementation == nil || strings.EqualFold(strings.TrimSpace(by), strings.TrimSpace(n.Implementation.By)) {
		return deliveryError("delivery_review_invalid", "distinct verifier identity required")
	}
	return nil
}

func validateRegisteredDelivery(st *store.Store, dir string, n Node, artifactID, hash string) error {
	arts, err := st.Artifacts().ListByScope("node", n.ID)
	if err != nil {
		return err
	}
	for _, art := range arts {
		if art.ID != artifactID {
			continue
		}
		raw, err := os.ReadFile(art.AbsPath)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(raw)
		if hash == "" || hash != art.ContentHash || hash != hex.EncodeToString(sum[:]) {
			return deliveryError(ReasonEvidenceHashMismatch, n.ID)
		}
		root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(dir))))
		if _, err = ValidateEvidenceFile(root, n.ID, n, art.AbsPath); err != nil {
			return err
		}
		return ValidateCaptureDelivery(st, root, n, art.AbsPath)
	}
	return deliveryError(ReasonEvidenceArtifactNotFound, n.ID)
}

// ValidateVerifiedDelivery rechecks persisted claims before rollup or delivery.
func ValidateVerifiedDelivery(st *store.Store, dir string, n Node) error {
	if n.VerificationEvidence == nil {
		return deliveryError("delivery_receipt_missing", n.ID)
	}
	return validateRegisteredDelivery(st, dir, n, n.VerificationEvidence.ArtifactID, n.VerificationEvidence.ContentHash)
}

// ValidateFeatureDelivery validates the explicit feature scope, never latest.
func ValidateFeatureDelivery(st *store.Store, dir, slug string) error {
	idx, err := ReadIndex(dir)
	if err != nil {
		return err
	}
	if findings := ValidateIndex(idx); len(findings) > 0 {
		return deliveryError(ReasonInvalidIndex, findings[0])
	}
	feature, ok := featureEntry(dir, idx, slug)
	if !ok {
		return deliveryError("tree_feature_not_found", slug)
	}
	var walk func(string) error
	walk = func(id string) error {
		for _, entry := range idx.Nodes {
			if entry.ParentID != id {
				continue
			}
			if entry.Kind == KindTouchpoint {
				n, err := ReadNode(dir, entry.ID)
				if err != nil {
					return err
				}
				if err := ValidateVerifiedDelivery(st, dir, n); err != nil {
					return err
				}
			} else if err := walk(entry.ID); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(feature.ID)
}

// RollupVerified marks a parent verified only when every child already
// is; it walks post-order so one call can close a whole branch.
func RollupVerified(st *store.Store, dir, nodeID string) (VerifyReport, error) {
	idx, err := ReadIndex(dir)
	if err != nil {
		return VerifyReport{}, err
	}
	children := map[string][]IndexEntry{}
	for _, n := range idx.Nodes {
		children[n.ParentID] = append(children[n.ParentID], n)
	}
	// Validate and plan the whole post-order transition before any writes.
	var updates []Node
	var walk func(string) error
	walk = func(id string) error {
		for _, ch := range children[id] {
			if err := walk(ch.ID); err != nil {
				return err
			}
		}
		entry, ok := findEntry(idx, id)
		if !ok {
			return core.NewError(ReasonNodeNotFound, "intenttree.RollupVerified", "node not found: "+id)
		}
		if entry.Kind == KindTouchpoint || len(children[id]) == 0 {
			if entry.Kind == KindTouchpoint && (entry.Status == StatusVerified || entry.Status == StatusDelivered) {
				n, err := ReadNode(dir, id)
				if err != nil {
					return err
				}
				return ValidateVerifiedDelivery(st, dir, n)
			}
			return nil
		}
		all := true
		for _, ch := range children[id] {
			e, _ := findEntry(idx, ch.ID)
			if e.Status != StatusVerified {
				all = false
				break
			}
		}
		if !all {
			return core.NewError("tree_parent_not_verified", "intenttree.RollupVerified",
				fmt.Sprintf("child of %s is not verified", id))
		}
		if entry.Status == StatusVerified {
			return nil
		}
		n, err := ReadNode(dir, id)
		if err != nil {
			return err
		}
		n.Status = StatusVerified
		var bound []string
		for _, ch := range children[id] {
			bound = append(bound, ch.Fingerprint)
		}
		n.ChildFingerprints = bound
		n, err = Stamp(n)
		if err != nil {
			return err
		}
		updates = append(updates, n)
		entry.Status = StatusVerified
		entry.Fingerprint = n.Fingerprint
		entry.FingerprintAlg = n.FingerprintAlg
		for i := range idx.Nodes {
			if idx.Nodes[i].ID == id {
				idx.Nodes[i] = entry
			}
		}
		return nil
	}
	if err := walk(nodeID); err != nil {
		return VerifyReport{}, err
	}
	for _, n := range updates {
		if err := WriteNode(dir, n); err != nil {
			return VerifyReport{}, err
		}
		if err := st.UpdateTreeNodeStatus(n.ID, string(StatusVerified), n.Fingerprint, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return VerifyReport{}, err
		}
	}
	if err := WriteIndex(dir, idx); err != nil {
		return VerifyReport{}, err
	}
	entry, _ := findEntry(idx, nodeID)
	return VerifyReport{NodeID: nodeID, Status: string(entry.Status), Verified: entry.Status == StatusVerified}, nil
}
