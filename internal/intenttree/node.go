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
	ReasonSchemaUnknown        = "tree_schema_unknown"
	ReasonNodeNotFound         = "tree_node_not_found"
	ReasonDuplicateID          = "tree_duplicate_node_id"
	ReasonOrphanNode           = "tree_orphan_node"
	ReasonInvalidChildKind     = "tree_invalid_child_kind"
	ReasonChildLimit           = "tree_child_limit_exceeded"
	ReasonInvalidIndex         = "tree_index_invalid"
	ReasonStateConflict        = "tree_state_conflict"
	ReasonVersionNotFound      = "tree_version_not_found"
	ReasonAlreadyMigrated      = "tree_already_migrated"
	ReasonVerificationMissing  = "tree_verification_missing"
	ReasonModuleEmpty          = "tree_module_empty"
	ReasonNodeNotDraft         = "tree_node_not_draft"
	ReasonParentNotFrozen      = "tree_parent_not_frozen"
	ReasonACMissing            = "tree_ac_missing"
	ReasonLeafHasChildren      = "tree_leaf_has_children"
	ReasonChangeNotAllowed     = "tree_change_not_allowed"
	ReasonChangeBaseMismatch   = "tree_change_base_mismatch"
	ReasonChangeSummaryMissing = "tree_change_summary_missing"
	ReasonNodeNotFrozen        = "tree_node_not_frozen"
	ReasonArtifactRequired     = "tree_artifact_required"
	ReasonOutcomeInvalid       = "tree_outcome_invalid"
	ReasonFSMScope             = "tree_fsm_scope"
	ReasonFSMInvalid           = "tree_fsm_invalid"
	ReasonFSMUnknownState      = "tree_fsm_unknown_state"
	ReasonFSMDuplicate         = "tree_fsm_duplicate_state"
	ReasonFSMConflict          = "tree_fsm_illegal_conflict"
	ReasonTestCaseMissing      = "tree_test_case_missing"
	ReasonTestCaseInvalid      = "tree_test_case_invalid"
	ReasonACCoverage           = "tree_ac_test_coverage"
	ReasonTruthPathMissing     = "tree_truth_path_missing"
	ReasonTruthPathInvalid     = "tree_truth_path_invalid"
	ReasonTruthSlugMismatch    = "tree_truth_slug_mismatch"
	ReasonTruthLinkNotDraft    = "tree_truth_link_not_draft"
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
	StatusDraft         Status = "draft"
	StatusFrozen        Status = "frozen"
	StatusImplemented   Status = "implemented"
	StatusVerified      Status = "verified"
	StatusChangePending Status = "change_pending"
	StatusDeprecated    Status = "deprecated"
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

// ImplementationRecord is the leaf execution report.
type ImplementationRecord struct {
	By           string `json:"by"`
	Outcome      string `json:"outcome"`
	ArtifactPath string `json:"artifact_path,omitempty"`
	ArtifactHash string `json:"artifact_hash,omitempty"`
	ReasonCode   string `json:"reason_code,omitempty"`
	At           string `json:"at"`
}

// FSMTransition is one legal or illegal domain transition.
type FSMTransition struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Event string `json:"event,omitempty"`
}

// DomainFSM is the business state machine carried by a feature node.
type DomainFSM struct {
	States      []string        `json:"states"`
	Transitions []FSMTransition `json:"transitions"`
	Illegal     []FSMTransition `json:"illegal,omitempty"`
}

// TestCase binds one AC to a concrete test or manual check.
type TestCase struct {
	ID     string `json:"id"`
	ACID   string `json:"ac_id"`
	Kind   string `json:"kind"`
	Ref    string `json:"ref"`
	Status string `json:"status"`
}

// Node is one canonical node body. Fingerprint fields are excluded from
// the hashed bytes; everything else is contract surface.
type Node struct {
	ID                   string                `json:"id"`
	Kind                 Kind                  `json:"kind"`
	Title                string                `json:"title"`
	Intent               string                `json:"intent,omitempty"`
	ParentID             string                `json:"parent_id,omitempty"`
	Path                 string                `json:"path"`
	Status               Status                `json:"status"`
	ACs                  []AC                  `json:"acs,omitempty"`
	TestCases            []TestCase            `json:"test_cases,omitempty"`
	DomainFSM            *DomainFSM            `json:"domain_fsm,omitempty"`
	Verification         *Verification         `json:"verification,omitempty"`
	ChildFingerprints    []string              `json:"child_fingerprints,omitempty"`
	Implementation       *ImplementationRecord `json:"implementation,omitempty"`
	VerificationEvidence *EvidenceRef          `json:"verification_evidence,omitempty"`
	TruthPath            string                `json:"truth_path,omitempty"`
	LegacyTruthPath      string                `json:"legacy_truth_path,omitempty"`
	LegacyTestsPath      string                `json:"legacy_tests_path,omitempty"`
	Fingerprint          string                `json:"fingerprint"`
	FingerprintAlg       string                `json:"fingerprint_alg"`
}

// TruthLinkStatus is the read-only comparison between a node and its
// linked requirement package. It is never persisted.
type TruthLinkStatus struct {
	Path          string `json:"truth_path"`
	PackageStatus string `json:"package_status,omitempty"`
	PackageSlug   string `json:"package_slug,omitempty"`
	SlugOK        bool   `json:"slug_ok"`
	Match         bool   `json:"match"`
	Error         string `json:"error,omitempty"`
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
	TruthPath      string `json:"truth_path,omitempty"`
	Fingerprint    string `json:"fingerprint"`
	FingerprintAlg string `json:"fingerprint_alg"`
}

// EffectiveTruthPath returns the canonical link, falling back to the
// legacy migration field for trees written before truth_path existed.
func (n Node) EffectiveTruthPath() string {
	if n.TruthPath != "" {
		return n.TruthPath
	}
	return n.LegacyTruthPath
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

// fingerprintVerifiers dispatches by the alg recorded on the node, so a
// historical algorithm can be verified without rewriting old records.
var fingerprintVerifiers = map[string]func(Node) (string, error){
	FingerprintAlg: Fingerprint,
}

// VerifyFingerprint checks a node with the algorithm it records.
func VerifyFingerprint(n Node) (bool, error) {
	fn, ok := fingerprintVerifiers[n.FingerprintAlg]
	if !ok {
		return false, core.NewError(ReasonSchemaUnknown, "intenttree.VerifyFingerprint",
			"unknown fingerprint alg "+n.FingerprintAlg)
	}
	got, err := fn(n)
	if err != nil {
		return false, err
	}
	return got == n.Fingerprint, nil
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

// ValidateDomainFSM checks a feature's business state machine.
func ValidateDomainFSM(f *DomainFSM) []string {
	if f == nil {
		return nil
	}
	var out []string
	states := map[string]bool{}
	for _, st := range f.States {
		st = strings.TrimSpace(st)
		if st == "" {
			out = append(out, ReasonFSMInvalid+": empty state")
			continue
		}
		if states[st] {
			out = append(out, ReasonFSMDuplicate+": "+st)
		}
		states[st] = true
	}
	if len(f.States) == 0 {
		out = append(out, ReasonFSMInvalid+": states are empty")
	}
	legal := map[string]bool{}
	for _, tr := range f.Transitions {
		if !states[tr.From] || !states[tr.To] {
			out = append(out, ReasonFSMUnknownState+": "+tr.From+" -> "+tr.To)
			continue
		}
		legal[tr.From+"->"+tr.To] = true
	}
	for _, tr := range f.Illegal {
		if !states[tr.From] || !states[tr.To] {
			out = append(out, ReasonFSMUnknownState+": "+tr.From+" -> "+tr.To)
		}
		if legal[tr.From+"->"+tr.To] {
			out = append(out, ReasonFSMConflict+": "+tr.From+" -> "+tr.To)
		}
	}
	return out
}

// ValidateTestCases checks ids, enum values, AC references and that every
// AC is covered by at least one test case.
func ValidateTestCases(acs []AC, cases []TestCase) []string {
	var out []string
	seen := map[string]bool{}
	covered := map[string]bool{}
	for _, tc := range cases {
		if strings.TrimSpace(tc.ID) == "" {
			out = append(out, ReasonTestCaseInvalid+": empty id")
		}
		if seen[tc.ID] {
			out = append(out, ReasonTestCaseInvalid+": duplicate id "+tc.ID)
		}
		seen[tc.ID] = true
		switch tc.Kind {
		case "unit", "e2e", "manual":
		default:
			out = append(out, ReasonTestCaseInvalid+": kind "+tc.Kind)
		}
		switch tc.Status {
		case "existing", "planned":
		default:
			out = append(out, ReasonTestCaseInvalid+": status "+tc.Status)
		}
		if strings.TrimSpace(tc.Ref) == "" {
			out = append(out, ReasonTestCaseInvalid+": empty ref "+tc.ID)
		}
		found := false
		for _, ac := range acs {
			if ac.ID == tc.ACID {
				found = true
				covered[ac.ID] = true
			}
		}
		if !found {
			out = append(out, ReasonTestCaseInvalid+": unknown ac "+tc.ACID)
		}
	}
	for _, ac := range acs {
		if !covered[ac.ID] {
			out = append(out, ReasonACCoverage+": "+ac.ID)
		}
	}
	return out
}
