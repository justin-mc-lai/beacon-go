// Tree commands: plan/truth tree views, migrate tree, plan expand, verify audit.
package cli

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/core"
	"beacon-go/internal/intenttree"
	"beacon-go/internal/orchestrator"
	"beacon-go/internal/store"
)

func newPlanCmd() *cobra.Command {
	c := &cobra.Command{Use: "plan", Short: "Plan group: build and validate intent trees"}
	c.AddCommand(newPlanInitCmd(), newPlanTreeCmd(), newPlanExpandCmd(), newPlanApplyCmd())
	return c
}

func newPlanTreeCmd() *cobra.Command {
	return newTreeReadCmd("tree", "plan tree", "Render the intent tree index (read-only)")
}

func newTruthTreeCmd() *cobra.Command {
	return newTreeReadCmd("tree", "truth tree", "Render the requirement tree index (read-only)")
}

func newTreeReadCmd(use, op, short string) *cobra.Command {
	var project, version, format, outPath, nodeID, snapshot string
	var asJSON bool
	c := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				return fail("store_not_found", op, "global store not found; run migrate tree first")
			}
			defer st.Close()
			dir, projRoot, idx, err := resolveTree(st, project, version)
			if err != nil {
				return err
			}
			if snapshot != "" {
				snap, serr := intenttree.ReadSnapshot(dir, snapshot)
				if serr != nil {
					return serr
				}
				return printJSON(cmd, map[string]any{
					"view": "snapshot", "snapshot_id": snapshot, "schema": snap.Schema,
					"version": snap.Version, "root_id": snap.RootID, "created_at": snap.CreatedAt,
					"source_hash": snap.SourceHash, "node_count": len(snap.Nodes), "nodes": snap.Nodes,
				})
			}
			if nodeID != "" {
				n, nerr := intenttree.ReadNode(dir, nodeID)
				if nerr != nil {
					return nerr
				}
				return printJSON(cmd, n)
			}
			if format == "" || format == "json" {
				report := treeReport(dir, idx)
				if asJSON || format == "json" || format == "" {
					return printJSON(cmd, report)
				}
			}
			if format == "md" {
				data := intenttree.RenderMarkdown(idx)
				p, werr := writeTreeRender(outPath, dir, "tree.md", data)
				if werr != nil {
					return werr
				}
				fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (%d bytes, sha256=%s, source=%s)\n", p, len(data), hashHex(data), intenttree.SourceHash(idx))
				return nil
			}
			if format == "html" {
				data := intenttree.RenderHTML(idx)
				def := filepath.Join(projRoot, ".beacon", "artifacts", "tree", idx.Version, "index.html")
				p, werr := writeTreeRender(outPath, "", def, data)
				if werr != nil {
					return werr
				}
				fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (%d bytes, sha256=%s, source=%s)\n", p, len(data), hashHex(data), intenttree.SourceHash(idx))
				return nil
			}
			return fail("tree_format_unknown", op, "format must be json, md, or html")
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().StringVar(&format, "format", "json", "output format: json, md, html")
	c.Flags().StringVar(&outPath, "out", "", "render output path (for md/html)")
	c.Flags().StringVar(&nodeID, "node", "", "print a single node body (canonical JSON)")
	c.Flags().StringVar(&snapshot, "snapshot", "", "read an immutable snapshot (id or latest)")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON (default)")
	return c
}

func resolveTree(st *store.Store, project, version string) (string, string, intenttree.Index, error) {
	root := store.ResolveRootPath(project)
	proj, err := st.Projects().GetByRootPath(root)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") || strings.Contains(err.Error(), "not found") {
			return "", "", intenttree.Index{}, core.NewError(intenttree.ReasonVersionNotFound, "resolveTree", "project not registered: "+root)
		}
		return "", "", intenttree.Index{}, err
	}
	ver, err := resolveProjectVersion(st, proj.ID, version)
	if err != nil {
		return "", "", intenttree.Index{}, err
	}
	dir := intenttree.TreeDir(proj.RootPath, ver.DocsRoot)
	idx, err := intenttree.ReadIndex(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || strings.Contains(err.Error(), "no such file") {
			return "", "", intenttree.Index{}, core.NewError("tree_not_migrated", "resolveTree", "tree not migrated; run: beacon migrate tree --project "+root+" --version "+ver.Version)
		}
		return "", "", intenttree.Index{}, err
	}
	return dir, proj.RootPath, idx, nil
}

func resolveProjectVersion(st *store.Store, projectID, version string) (store.ProjectVersion, error) {
	vers, err := st.Projects().ListVersions(projectID)
	if err != nil {
		return store.ProjectVersion{}, err
	}
	if len(vers) == 0 {
		return store.ProjectVersion{}, core.NewError(intenttree.ReasonVersionNotFound, "resolveProjectVersion", "no versions registered")
	}
	if strings.EqualFold(version, "latest") || strings.EqualFold(version, "auto") {
		return store.ProjectVersion{}, core.NewError(intenttree.ReasonVersionNotFound, "resolveProjectVersion",
			"implicit latest is forbidden; pass --version explicitly")
	}
	if version == "" || strings.EqualFold(version, "current") {
		for _, v := range vers {
			if v.IsCurrent {
				return v, nil
			}
		}
		return store.ProjectVersion{}, core.NewError(intenttree.ReasonVersionNotFound, "resolveProjectVersion",
			"no current version; pass --version explicitly")
	}
	norm, err := orchestrator.NormalizeDocsVersion(version)
	if err != nil {
		return store.ProjectVersion{}, err
	}
	for _, v := range vers {
		if v.Version == norm {
			return v, nil
		}
	}
	return store.ProjectVersion{}, core.NewError(intenttree.ReasonVersionNotFound, "resolveProjectVersion", "version not found: "+norm)
}

func treeReport(dir string, idx intenttree.Index) map[string]any {
	counts := map[string]int{}
	for _, n := range idx.Nodes {
		counts[string(n.Kind)]++
	}
	return map[string]any{
		"schema":     idx.Schema,
		"version":    idx.Version,
		"root_id":    idx.RootID,
		"tree_dir":   dir,
		"node_count": len(idx.Nodes),
		"counts":     counts,
		"nodes":      idx.Nodes,
	}
}

func writeTreeRender(outPath, treeDir, defaultName string, data []byte) (string, error) {
	p := outPath
	if p == "" {
		if treeDir != "" {
			p = filepath.Join(treeDir, defaultName)
		} else {
			p = defaultName
		}
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return p, nil
}

func hashHex(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)
}

func newDoctorRebuildTreeCmd() *cobra.Command {
	var project, version string
	var asJSON bool
	c := &cobra.Command{
		Use:   "rebuild-tree",
		Short: "Rebuild the SQLite tree cache from canonical repo JSON (never the reverse)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			dir, _, idx, err := resolveTree(st, project, version)
			if err != nil {
				return err
			}
			if findings := intenttree.ValidateIndex(idx); len(findings) > 0 {
				return fail(intenttree.ReasonInvalidIndex, "doctor rebuild-tree", strings.Join(findings, "; "))
			}
			cache := make([]store.TreeNode, 0, len(idx.Nodes))
			for _, e := range idx.Nodes {
				n, nerr := intenttree.ReadNode(dir, e.ID)
				if nerr != nil {
					return nerr
				}
				cache = append(cache, store.TreeNode{
					ID: n.ID, ParentID: n.ParentID, Kind: string(n.Kind), Path: n.Path,
					Status: string(n.Status), Title: n.Title, Fingerprint: n.Fingerprint,
					FingerprintAlg: n.FingerprintAlg, DocPath: intenttree.NodePath(dir, n.ID),
					CreatedAt: store.NowUTC(), UpdatedAt: store.NowUTC(),
				})
			}
			// Resolve the version id from the registered project/version.
			root := store.ResolveRootPath(project)
			proj, perr := st.Projects().GetByRootPath(root)
			if perr != nil {
				return perr
			}
			v, verr := resolveProjectVersion(st, proj.ID, idx.Version)
			if verr != nil {
				return verr
			}
			for i := range cache {
				cache[i].ProjectVersionID = v.ID
			}
			if err := st.ReplaceTreeNodes(v.ID, cache); err != nil {
				return err
			}
			return printJSON(cmd, map[string]any{"rebuilt": len(cache), "version": idx.Version})
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().BoolVar(&asJSON, "json", false, "JSON output (default)")
	return c
}
