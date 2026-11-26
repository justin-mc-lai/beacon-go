package intenttree

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// ProposalChild is one child the host agent wants to add. The kernel
// validates it; the model never writes these files directly.
type ProposalChild struct {
	Kind         Kind          `json:"kind"`
	Title        string        `json:"title"`
	Slug         string        `json:"slug,omitempty"`
	Intent       string        `json:"intent,omitempty"`
	Doc          string        `json:"doc,omitempty"`
	Constraint   bool          `json:"constraint,omitempty"`
	ACs          []AC          `json:"acs,omitempty"`
	TestCases    []TestCase    `json:"test_cases,omitempty"`
	DomainFSM    *DomainFSM    `json:"domain_fsm,omitempty"`
	Verification *Verification `json:"verification,omitempty"`
}

// Provenance is the host/LLM record for one expansion. The kernel
// stores it; it never calls the model itself.
type Provenance struct {
	Model      string   `json:"model,omitempty"`
	PromptHash string   `json:"prompt_hash,omitempty"`
	Skill      string   `json:"skill,omitempty"`
	Tokens     int      `json:"tokens,omitempty"`
	CostUSD    float64  `json:"cost_usd,omitempty"`
	By         string   `json:"by,omitempty"`
	Sources    []string `json:"sources,omitempty"`
}

// Proposal is one expansion request for an existing draft parent.
type Proposal struct {
	Children   []ProposalChild `json:"children"`
	Provenance *Provenance     `json:"provenance,omitempty"`
}

// ValidateProposal is the single proposal judgement shared by the
// dry-run `plan expand` and the write path `plan apply`.
func ValidateProposal(parent Kind, p Proposal) error {
	if len(p.Children) == 0 {
		return core.NewError(ReasonInvalidIndex, "intenttree.ValidateProposal", "children must not be empty")
	}
	if len(p.Children) > MaxChildren {
		return core.NewError(ReasonChildLimit, "intenttree.ValidateProposal",
			fmt.Sprintf("%d children proposed (max %d)", len(p.Children), MaxChildren))
	}
	for _, ch := range p.Children {
		if strings.TrimSpace(ch.Title) == "" {
			return core.NewError(ReasonInvalidIndex, "intenttree.ValidateProposal", "child title is required")
		}
		segment := proposalSegment(ch.Title, ch.Slug)
		if segment == "" {
			return core.NewError(ReasonChildSlugMissing, "intenttree.ValidateProposal",
				"child title has no ASCII slug; set the optional slug: "+ch.Title)
		}
		if !nodeSlugRe.MatchString(segment) {
			return core.NewError(ReasonChildSlugInvalid, "intenttree.ValidateProposal",
				"child slug must match "+nodeSlugRe.String()+": "+segment)
		}
		if err := ValidateChild(parent, ch.Kind); err != nil {
			return err
		}
		if ch.DomainFSM != nil {
			if ch.Kind != KindFeature {
				return core.NewError(ReasonFSMScope, "intenttree.ValidateProposal",
					"domain FSM belongs to feature nodes: "+ch.Title)
			}
			if findings := ValidateDomainFSM(ch.DomainFSM); len(findings) > 0 {
				return core.NewError(ReasonFSMInvalid, "intenttree.ValidateProposal", strings.Join(findings, "; "))
			}
		}
		if len(ch.TestCases) > 0 {
			if ch.Kind != KindTouchpoint && ch.Kind != KindFeature {
				return core.NewError(ReasonTestCaseInvalid, "intenttree.ValidateProposal",
					"test cases belong to touchpoints or features: "+ch.Title)
			}
			if findings := ValidateTestCases(ch.ACs, ch.TestCases); len(findings) > 0 {
				return core.NewError(ReasonTestCaseInvalid, "intenttree.ValidateProposal", strings.Join(findings, "; "))
			}
		}
		if ch.Kind == KindTouchpoint {
			if len(ch.ACs) == 0 {
				return core.NewError(ReasonACMissing, "intenttree.ValidateProposal",
					"touchpoint proposal needs at least one AC: "+ch.Title)
			}
			if ch.Verification == nil || strings.TrimSpace(ch.Verification.Spec) == "" {
				return core.NewError(ReasonVerificationMissing, "intenttree.ValidateProposal",
					"touchpoint proposal needs a verification spec: "+ch.Title)
			}
		}
	}
	return nil
}

// ApplyProposal validates and persists a draft expansion. All children
// are written before the index, so a failed write leaves the previous
// index authoritative.
func ApplyProposal(st *store.Store, dir, versionID, parentID string, p Proposal) ([]Node, error) {
	idx, err := ReadIndex(dir)
	if err != nil {
		return nil, err
	}
	parent, ok := findEntry(idx, parentID)
	if !ok {
		return nil, core.NewError(ReasonNodeNotFound, "intenttree.ApplyProposal", "parent not found: "+parentID)
	}
	if parent.Status != StatusDraft {
		return nil, core.NewError(ReasonNodeNotDraft, "intenttree.ApplyProposal",
			"only draft parents can expand: "+parentID+" is "+string(parent.Status))
	}
	if err := ValidateProposal(Kind(parent.Kind), p); err != nil {
		return nil, err
	}
	existing := 0
	usedPaths := map[string]bool{}
	for _, e := range idx.Nodes {
		if e.ParentID == parentID {
			existing++
		}
		usedPaths[e.Path] = true
	}
	if existing+len(p.Children) > MaxChildren {
		return nil, core.NewError(ReasonChildLimit, "intenttree.ApplyProposal",
			fmt.Sprintf("existing %d + proposed %d exceeds the per-expansion cap %d", existing, len(p.Children), MaxChildren))
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var nodes []Node
	var cache []store.TreeNode
	for i, ch := range p.Children {
		explicitSlug := strings.TrimSpace(ch.Slug) != ""
		path := parent.Path + "/" + proposalSegment(ch.Title, ch.Slug)
		if usedPaths[path] {
			if explicitSlug {
				return nil, core.NewError(ReasonChildSlugCollision, "intenttree.ApplyProposal",
					"explicit child slug collides with an existing path: "+path)
			}
			path = fmt.Sprintf("%s-%d", path, i+1)
		}
		usedPaths[path] = true
		n := Node{
			ID:           shortID("n_", parentID, string(ch.Kind), ch.Title, fmt.Sprint(i)),
			Kind:         ch.Kind,
			Title:        ch.Title,
			Intent:       ch.Intent,
			Doc:          ch.Doc,
			Constraint:   ch.Constraint,
			ParentID:     parentID,
			Path:         path,
			Status:       StatusDraft,
			ACs:          ch.ACs,
			TestCases:    ch.TestCases,
			DomainFSM:    ch.DomainFSM,
			Verification: ch.Verification,
		}
		n, err = Stamp(n)
		if err != nil {
			return nil, err
		}
		if err := WriteNode(dir, n); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
		cache = append(cache, store.TreeNode{
			ID: n.ID, ProjectVersionID: versionID, ParentID: n.ParentID, Kind: string(n.Kind),
			Path: n.Path, Status: string(n.Status), Title: n.Title, Fingerprint: n.Fingerprint,
			FingerprintAlg: n.FingerprintAlg, DocPath: NodePath(dir, n.ID),
			CreatedAt: now, UpdatedAt: now,
		})
		idx.Nodes = append(idx.Nodes, IndexEntry{
			ID: n.ID, ParentID: n.ParentID, Kind: n.Kind, Status: n.Status, Title: n.Title,
			Path: n.Path, Fingerprint: n.Fingerprint, FingerprintAlg: n.FingerprintAlg,
		})
	}
	if findings := ValidateIndex(idx); len(findings) > 0 {
		return nil, core.NewError(ReasonInvalidIndex, "intenttree.ApplyProposal", strings.Join(findings, "; "))
	}
	if err := WriteIndex(dir, idx); err != nil {
		return nil, err
	}
	if err := st.AppendTreeNodes(versionID, cache); err != nil {
		return nil, err
	}
	record := map[string]any{
		"parent_id": parentID, "at": now, "children": nodes,
	}
	if p.Provenance != nil {
		record["provenance"] = p.Provenance
	}
	if raw, err := prettyJSON(record); err == nil {
		_ = atomicWrite(filepath.Join(dir, "expansions", shortID("exp_", parentID, now)+".json"), raw)
	}
	return nodes, nil
}

var nodeSlugRe = regexp.MustCompile(`^[a-z0-9]+(?:[-_][a-z0-9]+)*$`)

func proposalSegment(title, explicit string) string {
	if slug := strings.TrimSpace(explicit); slug != "" {
		return slug
	}
	return Slugify(title)
}

// Slugify keeps ASCII letters, digits, spaces, hyphens, underscores and
// slashes; non-ASCII text is ignored. An empty result means the caller
// must provide an explicit slug.
func Slugify(s string) string {
	var b strings.Builder
	separator := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			separator = false
		case r == ' ' || r == '-' || r == '_' || r == '/':
			if b.Len() > 0 && !separator {
				b.WriteByte('-')
				separator = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
