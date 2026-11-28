package cli

import (
	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
	"bytes"
	"encoding/json"
	"github.com/spf13/cobra"
	"io"
	"os"
	"strings"
)

func newChangeReconcileCmd() *cobra.Command { return newControlledTreeChangeCmd(false) }

func newControlledTreeChangeCmd(move bool) *cobra.Command {
	var project, version, node, parent, summary, by, base, file string
	name := "reconcile-index"
	short := "Reconcile explicit projections; file path requests repair draft module bodies (action repair-path)"
	if move {
		name = "move"
		short = "Move a draft module or feature to an explicit parent"
	}
	c := &cobra.Command{Use: name, Short: short, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		for _, v := range []string{version, summary, by} {
			if strings.TrimSpace(v) == "" {
				return fail("tree_change_input_required", "change "+name, "explicit nonblank version, node, summary, by and base-fingerprint required")
			}
		}
		targets := []intenttree.ReconcileTarget{{NodeID: node, BaseFingerprint: base}}
		if !move && cmd.Flags().Changed("file") {
			if cmd.Flags().Changed("node") || cmd.Flags().Changed("base-fingerprint") {
				return fail("tree_change_input_required", "change reconcile-index", "--file is mutually exclusive with --node/--base-fingerprint")
			}
			raw, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			var input struct {
				Nodes []intenttree.ReconcileTarget `json:"nodes"`
			}
			dec := json.NewDecoder(bytes.NewReader(raw))
			dec.DisallowUnknownFields()
			if err = dec.Decode(&input); err != nil {
				return err
			}
			if err = dec.Decode(new(any)); err != io.EOF {
				return fail("tree_change_input_required", "change reconcile-index", "one JSON document required")
			}
			targets = input.Nodes
		} else if strings.TrimSpace(node) == "" || strings.TrimSpace(base) == "" {
			return fail("tree_change_input_required", "change "+name, "nonblank node and base-fingerprint required")
		}
		if move && strings.TrimSpace(parent) == "" {
			return fail("tree_change_input_required", "change move", "explicit parent required")
		}
		st, err := openWriteStore()
		if err != nil {
			return err
		}
		defer st.Close()
		dir, _, idx, err := resolveTree(st, project, version)
		if err != nil {
			return err
		}
		proj, err := st.Projects().GetByRootPath(store.ResolveRootPath(project))
		if err != nil {
			return err
		}
		ver, err := resolveProjectVersion(st, proj.ID, idx.Version)
		if err != nil {
			return err
		}
		if move {
			rec, err := intenttree.MoveChange(st, dir, ver.ID, node, parent, summary, by, base)
			if err != nil {
				return err
			}
			return printJSON(cmd, rec)
		}
		rec, err := intenttree.ReconcileIndexes(st, dir, ver.ID, targets, summary, by)
		if err != nil {
			return err
		}
		return printJSON(cmd, rec)
	}}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "explicit docs version")
	c.Flags().StringVar(&node, "node", "", "explicit node ID")
	c.Flags().StringVar(&summary, "summary", "", "change summary")
	c.Flags().StringVar(&by, "by", "", "requesting identity")
	c.Flags().StringVar(&base, "base-fingerprint", "", "reviewed canonical body fingerprint")
	c.Flags().Bool("json", true, "emit JSON")
	if move {
		c.Flags().StringVar(&parent, "parent", "", "new parent ID")
		_ = c.MarkFlagRequired("parent")
		_ = c.MarkFlagRequired("node")
		_ = c.MarkFlagRequired("base-fingerprint")
	} else {
		c.Flags().StringVar(&file, "file", "", "explicit repair JSON {nodes:[{node_id,base_fingerprint,path?}]}; path changes draft module/subtree bodies")
	}
	for _, f := range []string{"version", "summary", "by"} {
		_ = c.MarkFlagRequired(f)
	}
	return c
}
