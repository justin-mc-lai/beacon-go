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
			if format == "" {
				format = "summary"
			}
			if format == "json" || asJSON {
				report, rerr := treeReport(dir, idx)
				if rerr != nil {
					return rerr
				}
				return printJSON(cmd, report)
			}
			if format == "summary" {
				bodies := map[string]intenttree.Node{}
				for _, e := range idx.Nodes {
					n, nerr := intenttree.ReadNode(dir, e.ID)
					if nerr != nil {
						return nerr
					}
					bodies[e.ID] = n
				}
				return printTreeSummary(cmd, idx, bodies, dir, projRoot)
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
				bodies := map[string]intenttree.Node{}
				for _, e := range idx.Nodes {
					n, nerr := intenttree.ReadNode(dir, e.ID)
					if nerr != nil {
						return nerr
					}
					bodies[e.ID] = n
				}
				truth := map[string]intenttree.TruthLinkStatus{}
				for _, e := range idx.Nodes {
					n := bodies[e.ID]
					if n.EffectiveTruthPath() == "" {
						continue
					}
					st := readTruthStatus(projRoot, n)
					st.NodeStatus = ""
					truth[e.ID] = intenttree.TruthLinkStatus{
						Path: st.TruthPath, PackageStatus: st.PackageStatus,
						PackageSlug: st.PackageSlug, SlugOK: st.SlugOK,
						Match: st.Match, Error: st.Error,
					}
				}
				data := intenttree.RenderHTMLTreeWithTruth(idx, bodies, truth)
				def := filepath.Join(projRoot, ".beacon", "artifacts", "tree", idx.Version, "index.html")
				p, werr := writeTreeRender(outPath, "", def, data)
				if werr != nil {
					return werr
				}
				if outPath == "" {
					if err := ensureArtifactsIgnored(projRoot); err != nil {
						return err
					}
					if localOnlyMode(projRoot) {
						if err := ensureDocsIgnored(projRoot); err != nil {
							return err
						}
					}
				}
				fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (%d bytes, sha256=%s, source=%s)\n", p, len(data), hashHex(data), intenttree.SourceHash(idx))
				fmt.Fprintln(cmd.OutOrStdout(), "派生产物，勿提交；权威源见 docs/beacon/"+idx.Version+"/")
				if outPath != "" && !isInside(projRoot, p) {
					fmt.Fprintln(cmd.OutOrStdout(), "note: --out 指向项目外；该文件不会被项目的 .gitignore 覆盖")
				}
				return nil
			}
			return fail("tree_format_unknown", op, "format must be summary, json, md, or html")
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().StringVar(&format, "format", "summary", "output format: summary, json, md, html")
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
			msg := "tree is not materialized for version " + ver.Version + "; run: beacon migrate tree --project " + root + " --version " + ver.Version
			if !ver.IsCurrent {
				msg = "tree only covers materialized versions; version " + ver.Version + " has no tree. Old versions are not materialized by default; run: beacon migrate tree --project " + root + " --version " + ver.Version + " to create it"
			}
			return "", "", intenttree.Index{}, core.NewError("tree_version_not_materialized", "resolveTree", msg)
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

func treeReport(dir string, idx intenttree.Index) (map[string]any, error) {
	counts := map[string]int{}
	for _, n := range idx.Nodes {
		counts[string(n.Kind)]++
	}
	out := map[string]any{
		"schema":        idx.Schema,
		"version":       idx.Version,
		"root_id":       idx.RootID,
		"tree_dir":      dir,
		"node_count":    len(idx.Nodes),
		"counts":        counts,
		"status_counts": statusCounts(idx),
		"nodes":         idx.Nodes,
	}
	unlinked, err := unlinkedFeatureSlugs(dir, idx)
	if err != nil {
		return nil, err
	}
	out["unlinked"] = unlinked
	out["tree_native"] = unlinked
	out["delivered_count"] = statusCounts(idx)[string(intenttree.StatusImplemented)]
	return out, nil
}

func statusCounts(idx intenttree.Index) map[string]int {
	out := map[string]int{}
	for _, n := range idx.Nodes {
		out[string(n.Status)]++
	}
	return out
}

func unlinkedFeatureSlugs(dir string, idx intenttree.Index) ([]string, error) {
	out := []string{}
	for _, e := range idx.Nodes {
		if e.Kind != intenttree.KindFeature {
			continue
		}
		n, err := intenttree.ReadNode(dir, e.ID)
		if err != nil {
			return nil, err
		}
		if n.EffectiveTruthPath() == "" {
			out = append(out, filepath.Base(e.Path))
		}
	}
	return out, nil
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

// printTreeSummary is the human default: what the project is, how much is
// covered, and where the readable view is. Machine data stays behind --json.
func printTreeSummary(cmd *cobra.Command, idx intenttree.Index, bodies map[string]intenttree.Node, dir, projRoot string) error {
	var modules, features, leaves, acs, fsm, planned, existing, linked, mismatch int
	var draft, frozen, implemented, verified, changePending, deprecated int
	var unlinked []string
	for _, e := range idx.Nodes {
		switch e.Kind {
		case intenttree.KindModule:
			modules++
		case intenttree.KindFeature:
			features++
		case intenttree.KindTouchpoint:
			leaves++
		}
		switch e.Status {
		case intenttree.StatusDraft:
			draft++
		case intenttree.StatusFrozen:
			frozen++
		case intenttree.StatusImplemented:
			implemented++
		case intenttree.StatusVerified:
			verified++
		case intenttree.StatusChangePending:
			changePending++
		case intenttree.StatusDeprecated:
			deprecated++
		}
		n, ok := bodies[e.ID]
		if !ok {
			continue
		}
		acs += len(n.ACs)
		if n.DomainFSM != nil {
			fsm++
		}
		for _, tc := range n.TestCases {
			if tc.Status == "existing" {
				existing++
			} else {
				planned++
			}
		}
		if n.EffectiveTruthPath() != "" {
			linked++
			st := readTruthStatus(projRoot, n)
			if !st.Match || st.Error != "" {
				mismatch++
			}
		} else if e.Kind == intenttree.KindFeature {
			unlinked = append(unlinked, filepath.Base(e.Path))
		}
	}
	root := bodies[idx.RootID]
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "需求简报 · %s\n\n", idx.Version)
	if root.Intent != "" {
		fmt.Fprintf(out, "%s\n\n", root.Intent)
	}
	fmt.Fprintf(out, "%d 模块 · %d 功能 · %d 触达点 · %d 条 AC · %d 个状态机\n", modules, features, leaves, acs, fsm)
	fmt.Fprintf(out, "状态：草稿 %d · 冻结 %d · 已交付/实现 %d · 已验证 %d · 变更中 %d · 废弃 %d\n", draft, frozen, implemented, verified, changePending, deprecated)
	fmt.Fprintf(out, "测试：%d 已绑定 · %d 计划中；已验证节点 %d/%d\n", existing, planned, verified, len(idx.Nodes))
	fmt.Fprintf(out, "legacy 包链（可选）：%d/%d 个功能节点已 link，状态不一致 %d\n", linked, features, mismatch)
	if len(unlinked) > 0 {
		fmt.Fprintf(out, "tree-native: %d 个功能无 legacy 包（可选）\n", len(unlinked))
	}
	if planned > 0 {
		fmt.Fprintf(out, "\n还不能宣称已验证：%d 条测试仍是计划状态。\n", planned)
	}
	fmt.Fprintf(out, "\n阅读版：beacon truth tree --version %s --format html\n", idx.Version)
	return nil
}
