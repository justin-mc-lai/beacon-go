package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/intenttree"
)

func newVerifyCmd() *cobra.Command {
	c := &cobra.Command{Use: "verify", Short: "Verification group: read-only audit and evidence"}
	c.AddCommand(newVerifyAuditCmd())
	return c
}

// newVerifyAuditCmd is the analyze half of analyze/converge: strictly
// read-only. Repairs belong to `change`, never to this command.
func newVerifyAuditCmd() *cobra.Command {
	var project, version string
	var asJSON bool
	c := &cobra.Command{
		Use:   "audit",
		Short: "Read-only tree audit (structure, kind rules, leaf verification)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				return fail("store_not_found", "verify audit", "global store not found")
			}
			defer st.Close()
			dir, _, idx, err := resolveTree(st, project, version)
			if err != nil {
				return err
			}
			findings := intenttree.ValidateIndex(idx)
			childCount := map[string]int{}
			for _, n := range idx.Nodes {
				if n.ParentID != "" {
					childCount[n.ParentID]++
				}
			}
			for _, entry := range idx.Nodes {
				n, nerr := intenttree.ReadNode(dir, entry.ID)
				if nerr != nil {
					findings = append(findings, "tree_node_unreadable: "+entry.ID+" ("+nerr.Error()+")")
					continue
				}
				if n.Kind == intenttree.KindTouchpoint && n.Verification == nil {
					findings = append(findings, intenttree.ReasonVerificationMissing+": "+n.ID)
				}
				if n.Kind != intenttree.KindTouchpoint && n.Verification != nil {
					findings = append(findings, "tree_verification_scope: "+n.ID)
				}
				if n.Kind == intenttree.KindModule && childCount[n.ID] == 0 {
					findings = append(findings, intenttree.ReasonModuleEmpty+": "+n.ID)
				}
			}
			report := map[string]any{
				"ok": len(findings) == 0, "reads_only": true,
				"schema": idx.Schema, "version": idx.Version,
				"node_count": len(idx.Nodes), "findings": findings,
			}
			if err := printJSON(cmd, report); err != nil {
				return err
			}
			if len(findings) > 0 {
				return fail("tree_audit_findings", "verify audit",
					fmt.Sprintf("%d finding(s); repairs belong to `change`", len(findings)))
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().BoolVar(&asJSON, "json", false, "JSON output (default)")
	return c
}
