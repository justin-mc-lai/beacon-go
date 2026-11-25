package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/gate"
	"beacon-go/internal/intenttree"
)

func newTruthNodeLinkCmd() *cobra.Command {
	var project, version, nodeID, truthPath string
	var asJSON bool
	c := &cobra.Command{
		Use:   "link",
		Short: "Link a feature node to its requirement package (truth_path pointer)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			dir, projRoot, _, err := resolveTree(st, project, version)
			if err != nil {
				return err
			}
			if nodeID == "" || truthPath == "" {
				return fail("tree_truth_path_missing", "truth node link", "--node and --truth are required")
			}
			n, err := intenttree.ReadNode(dir, nodeID)
			if err != nil {
				return err
			}
			if n.Kind != intenttree.KindFeature {
				return fail("tree_truth_path_invalid", "truth node link", "truth_path belongs to feature nodes")
			}
			rel, abs, err := normalizeTruthPath(projRoot, truthPath)
			if err != nil {
				return err
			}
			raw, err := os.ReadFile(abs)
			if err != nil {
				return fail("tree_truth_path_invalid", "truth node link", err.Error())
			}
			scalars, _, ok := gate.ParseFrontmatter(string(raw))
			if !ok {
				return fail("tree_truth_path_invalid", "truth node link", "truth.md has no frontmatter")
			}
			slug := strings.TrimSpace(scalars["slug"])
			base := filepath.Base(n.Path)
			if slug == "" || (slug != base && slug != n.Title) {
				return fail("tree_truth_slug_mismatch", "truth node link",
					"truth slug "+slug+" does not match node slug "+base)
			}
			linked, err := intenttree.LinkTruthPath(st, dir, nodeID, rel)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, linked)
			}
			_, err = cmd.OutOrStdout().Write([]byte("linked: " + linked.ID + " truth_path=" + linked.TruthPath + " (node " + string(linked.Status) + ")\n"))
			return err
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().StringVar(&nodeID, "node", "", "feature node id (required)")
	c.Flags().StringVar(&truthPath, "truth", "", "truth.md path relative to project root (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}

func newTruthNodeSyncStatusCmd() *cobra.Command {
	var project, version, nodeID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "sync-status",
		Short: "Report node vs linked truth-package status (read-only; never mutates state)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				return fail("store_not_found", "truth node sync-status", "global store not found")
			}
			defer st.Close()
			dir, projRoot, idx, err := resolveTree(st, project, version)
			if err != nil {
				return err
			}
			var out []truthSyncStatus
			for _, e := range idx.Nodes {
				if e.Kind != intenttree.KindFeature {
					continue
				}
				if nodeID != "" && e.ID != nodeID {
					continue
				}
				n, err := intenttree.ReadNode(dir, e.ID)
				if err != nil {
					return err
				}
				if n.EffectiveTruthPath() == "" {
					continue
				}
				status := readTruthStatus(projRoot, n)
				status.NodeID = n.ID
				status.Title = n.Title
				status.NodeStatus = string(n.Status)
				status.Match = status.SlugOK && statusMatches(n.Status, status.PackageStatus)
				out = append(out, status)
			}
			res := map[string]any{"ok": true, "nodes": out}
			for _, s := range out {
				if !s.Match || s.Error != "" {
					res["ok"] = false
				}
			}
			if asJSON {
				return printJSON(cmd, res)
			}
			for _, s := range out {
				state := "ok"
				if !s.Match || s.Error != "" {
					state = "mismatch"
				}
				line := s.NodeID + " " + s.Title + ": node=" + s.NodeStatus + " truth=" + s.PackageStatus + " [" + state + "]"
				if s.Error != "" {
					line += " " + s.Error
				}
				if _, err := cmd.OutOrStdout().Write([]byte(line + "\n")); err != nil {
					return err
				}
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().StringVar(&nodeID, "node", "", "only this feature node")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}

type truthSyncStatus struct {
	NodeID        string `json:"node_id"`
	Title         string `json:"title"`
	TruthPath     string `json:"truth_path"`
	PackageStatus string `json:"package_status"`
	PackageSlug   string `json:"package_slug"`
	NodeStatus    string `json:"node_status"`
	SlugOK        bool   `json:"slug_ok"`
	Match         bool   `json:"match"`
	Error         string `json:"error,omitempty"`
}

func readTruthStatus(projRoot string, n intenttree.Node) truthSyncStatus {
	p := n.EffectiveTruthPath()
	out := truthSyncStatus{TruthPath: p, SlugOK: false}
	if p == "" {
		out.Error = "truth_path missing"
		return out
	}
	abs := p
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(projRoot, p)
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	scalars, _, ok := gate.ParseFrontmatter(string(raw))
	if !ok {
		out.Error = "truth.md has no frontmatter"
		return out
	}
	out.PackageStatus = strings.TrimSpace(scalars["status"])
	out.PackageSlug = strings.TrimSpace(scalars["slug"])
	base := filepath.Base(n.Path)
	out.SlugOK = out.PackageSlug != "" && (out.PackageSlug == base || out.PackageSlug == n.Title)
	return out
}

func statusMatches(node intenttree.Status, pkg string) bool {
	switch pkg {
	case "frozen":
		return node == intenttree.StatusFrozen || node == intenttree.StatusImplemented || node == intenttree.StatusVerified
	case "draft":
		return node == intenttree.StatusDraft
	case "change_pending":
		return node == intenttree.StatusChangePending
	}
	return false
}

func normalizeTruthPath(projRoot, truthPath string) (string, string, error) {
	abs := truthPath
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(projRoot, truthPath)
	}
	abs, err := filepath.Abs(abs)
	if err != nil {
		return "", "", err
	}
	rel, err := filepath.Rel(projRoot, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fail("tree_truth_path_invalid", "truth node link", "truth path must stay inside the project")
	}
	return filepath.ToSlash(rel), abs, nil
}
