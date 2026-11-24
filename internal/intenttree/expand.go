package intenttree

import (
	"fmt"
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
	Intent       string        `json:"intent,omitempty"`
	ACs          []AC          `json:"acs,omitempty"`
	Verification *Verification `json:"verification,omitempty"`
}

// Proposal is one expansion request for an existing draft parent.
type Proposal struct {
	Children []ProposalChild `json:"children"`
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
	if len(p.Children) == 0 {
		return nil, core.NewError(ReasonInvalidIndex, "intenttree.ApplyProposal", "children must not be empty")
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
		if strings.TrimSpace(ch.Title) == "" {
			return nil, core.NewError(ReasonInvalidIndex, "intenttree.ApplyProposal", "child title is required")
		}
		if err := ValidateChild(parent.Kind, ch.Kind); err != nil {
			return nil, err
		}
		if ch.Kind == KindTouchpoint && (ch.Verification == nil || strings.TrimSpace(ch.Verification.Spec) == "") {
			return nil, core.NewError(ReasonVerificationMissing, "intenttree.ApplyProposal",
				"touchpoint proposal needs a verification spec: "+ch.Title)
		}
		path := parent.Path + "/" + slugify(ch.Title)
		if usedPaths[path] {
			path = fmt.Sprintf("%s-%d", path, i+1)
		}
		usedPaths[path] = true
		n := Node{
			ID:           shortID("n_", parentID, string(ch.Kind), ch.Title, fmt.Sprint(i)),
			Kind:         ch.Kind,
			Title:        ch.Title,
			Intent:       ch.Intent,
			ParentID:     parentID,
			Path:         path,
			Status:       StatusDraft,
			ACs:          ch.ACs,
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
	return nodes, nil
}

func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_' || r == '/':
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "node"
	}
	return out
}
