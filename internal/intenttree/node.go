// Package intenttree implements the canonical JSON tree, its fingerprint
// and the read/render/import surfaces of the intent-tree truth model.
// Bodies live in the repo as JSON; SQLite only caches the index.
package intenttree

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"beacon-go/internal/core"
)

// Stable contract strings.
const (
	Schema         = "beacon-intent-tree-v1"
	FingerprintAlg = "tree-json-v1"
	MaxChildren    = 7
)

// Reason codes owned by this package.
const (
	ReasonSchemaUnknown       = "tree_schema_unknown"
	ReasonNodeNotFound        = "tree_node_not_found"
	ReasonDuplicateID         = "tree_duplicate_node_id"
	ReasonOrphanNode          = "tree_orphan_node"
	ReasonInvalidChildKind    = "tree_invalid_child_kind"
	ReasonChildLimit          = "tree_child_limit_exceeded"
	ReasonInvalidIndex        = "tree_index_invalid"
	ReasonStateConflict       = "tree_state_conflict"
	ReasonVersionNotFound     = "tree_version_not_found"
	ReasonAlreadyMigrated     = "tree_already_migrated"
	ReasonVerificationMissing = "tree_verification_missing"
	ReasonModuleEmpty         = "tree_module_empty"
)

// Kind is one of the four frozen node kinds.
type Kind string

const (
	KindIntent     Kind = "intent"
	KindModule     Kind = "module"
	KindFeature    Kind = "feature"
	KindTouchpoint Kind = "touchpoint"
)

// Status is the node lifecycle (orthogonal to goal's run statuses).
type Status string

const (
	StatusDraft       Status = "draft"
	StatusFrozen      Status = "frozen"
	StatusImplemented Status = "implemented"
	StatusVerified    Status = "verified"
	StatusDeprecated  Status = "deprecated"
)

// AC is one acceptance criterion on a node.
type AC struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Verification is the executable proof required before a touchpoint
// can freeze. Type is "test" or "manual".
type Verification struct {
	Type string `json:"type"`
	Spec string `json:"spec"`
}

// Node is one canonical node body. Fingerprint fields are excluded from
// the hashed bytes; everything else is contract surface.
type Node struct {
	ID              string        `json:"id"`
	Kind            Kind          `json:"kind"`
	Title           string        `json:"title"`
	Intent          string        `json:"intent,omitempty"`
	ParentID        string        `json:"parent_id,omitempty"`
	Path            string        `json:"path"`
	Status          Status        `json:"status"`
	ACs             []AC          `json:"acs,omitempty"`
	Verification    *Verification `json:"verification,omitempty"`
	LegacyTruthPath string        `json:"legacy_truth_path,omitempty"`
	LegacyTestsPath string        `json:"legacy_tests_path,omitempty"`
	Fingerprint     string        `json:"fingerprint"`
	FingerprintAlg  string        `json:"fingerprint_alg"`
}

// IndexEntry is the reviewable summary of one node. A review never
// needs to open the node body files.
type IndexEntry struct {
	ID             string `json:"id"`
	ParentID       string `json:"parent_id,omitempty"`
	Kind           Kind   `json:"kind"`
	Status         Status `json:"status"`
	Title          string `json:"title"`
	Path           string `json:"path"`
	Fingerprint    string `json:"fingerprint"`
	FingerprintAlg string `json:"fingerprint_alg"`
}

// Index is the single-document tree view.
type Index struct {
	Schema  string       `json:"schema"`
	Version string       `json:"version"`
	RootID  string       `json:"root_id"`
	Nodes   []IndexEntry `json:"nodes"`
}

// ValidateChild enforces the fixed kind matrix.
func ValidateChild(parent, child Kind) error {
	if child == KindIntent && parent == KindIntent {
		return nil
	}
	if (child == KindModule) && (parent == KindIntent || parent == KindModule) {
		return nil
	}
	if child == KindFeature && parent == KindModule {
		return nil
	}
	if child == KindTouchpoint && parent == KindFeature {
		return nil
	}
	return core.NewError(ReasonInvalidChildKind, "intenttree.ValidateChild",
		fmt.Sprintf("kind %s cannot be a child of %s", child, parent))
}

// CanonicalBytes returns the deterministic JSON for fingerprinting:
// struct order fixed, no indentation, fingerprint fields cleared.
func CanonicalBytes(n Node) ([]byte, error) {
	copy := n
	copy.Fingerprint = ""
	copy.FingerprintAlg = ""
	if copy.ACs == nil {
		copy.ACs = []AC{}
	}
	return json.Marshal(copy)
}

// Fingerprint computes the tree-json-v1 content hash.
func Fingerprint(n Node) (string, error) {
	raw, err := CanonicalBytes(n)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// Stamp computes and stamps the fingerprint + algorithm on a node.
func Stamp(n Node) (Node, error) {
	fp, err := Fingerprint(n)
	if err != nil {
		return n, err
	}
	n.Fingerprint = fp
	n.FingerprintAlg = FingerprintAlg
	return n, nil
}

// ValidateIndex checks structural invariants on an index. It is
// read-only and safe for the verify audit seam.
func ValidateIndex(idx Index) []string {
	var findings []string
	if idx.Schema != Schema {
		findings = append(findings, ReasonSchemaUnknown+": "+idx.Schema)
		return findings
	}
	byID := map[string]IndexEntry{}
	for _, n := range idx.Nodes {
		if _, dup := byID[n.ID]; dup {
			findings = append(findings, ReasonDuplicateID+": "+n.ID)
		}
		byID[n.ID] = n
	}
	for _, n := range idx.Nodes {
		if n.ParentID == "" {
			continue
		}
		p, ok := byID[n.ParentID]
		if !ok {
			findings = append(findings, ReasonOrphanNode+": "+n.ID+" parent "+n.ParentID)
			continue
		}
		if err := ValidateChild(p.Kind, n.Kind); err != nil {
			findings = append(findings, ReasonInvalidChildKind+": "+n.ID+" under "+p.ID)
		}
	}
	if idx.RootID == "" {
		findings = append(findings, ReasonInvalidIndex+": empty root_id")
	} else if root, ok := byID[idx.RootID]; !ok {
		findings = append(findings, ReasonNodeNotFound+": root "+idx.RootID)
	} else if root.Kind != KindIntent || root.ParentID != "" {
		findings = append(findings, ReasonInvalidIndex+": root must be a parentless intent")
	}
	return findings
}

// SortedIndex returns an index with nodes ordered by path then id,
// so byte-for-byte writes are stable across runs.
func SortedIndex(idx Index) Index {
	out := idx
	out.Nodes = append([]IndexEntry(nil), idx.Nodes...)
	sort.Slice(out.Nodes, func(i, j int) bool {
		if out.Nodes[i].Path == out.Nodes[j].Path {
			return out.Nodes[i].ID < out.Nodes[j].ID
		}
		return strings.Compare(out.Nodes[i].Path, out.Nodes[j].Path) < 0
	})
	return out
}
