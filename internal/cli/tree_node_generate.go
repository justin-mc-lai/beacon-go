package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

func newTruthNodeGenerateTestsCmd() *cobra.Command {
	return newGenerateCmd("generate-tests", "Generate planned test cases from the node's ACs and Domain FSM")
}

func newTruthNodeGenerateDocCmd() *cobra.Command {
	return newGenerateCmd("generate-doc", "Generate a requirement-document skeleton from the node's intent, ACs and Domain FSM")
}

func newGenerateCmd(use, short string) *cobra.Command {
	var project, version, nodeID, summary, by string
	var apply, change, asJSON bool
	c := &cobra.Command{
		Use:   use,
		Short: short,
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
			if nodeID == "" {
				return fail("tree_node_required", "truth node "+use, "--node is required")
			}
			n, err := intenttree.ReadNode(dir, nodeID)
			if err != nil {
				return err
			}
			if change {
				if !apply {
					return fail("tree_change_requires_apply", "truth node "+use, "--change requires --apply")
				}
				if n.Status != intenttree.StatusFrozen && n.Status != intenttree.StatusImplemented && n.Status != intenttree.StatusVerified {
					return fail("tree_change_not_allowed", "truth node "+use,
						"--change requires frozen truth; "+nodeID+" is "+string(n.Status))
				}
				if summary == "" {
					return fail("tree_change_summary_missing", "truth node "+use, "--change requires --summary")
				}
				if by == "" {
					by = "cli"
				}
				if _, err := intenttree.RequestChange(st, dir, idx.Version, nodeID, summary, by, ""); err != nil {
					return err
				}
				n, err = intenttree.ReadNode(dir, nodeID)
				if err != nil {
					return err
				}
			}
			var report intenttree.GeneratedReport
			if use == "generate-tests" {
				generated := intenttree.PlannedTestCases(n)
				report = intenttree.GeneratedReport{NodeID: n.ID, Applied: apply, TestCases: generated, Added: len(generated)}
				if apply {
					merged := append(append([]intenttree.TestCase{}, n.TestCases...), generated...)
					n, err = intenttree.UpdateNode(st, dir, nodeID, intenttree.NodePatch{TestCases: merged})
					if err != nil {
						return err
					}
					report.Applied = true
				}
			} else {
				doc := intenttree.GenerateDoc(n)
				report = intenttree.GeneratedReport{NodeID: n.ID, Applied: apply, Doc: doc}
				if apply {
					n, err = intenttree.UpdateNode(st, dir, nodeID, intenttree.NodePatch{Doc: doc})
					if err != nil {
						return err
					}
					report.Applied = true
				}
			}
			if asJSON {
				return printJSON(cmd, report)
			}
			if use == "generate-tests" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %d planned test cases (applied=%t)\n", n.ID, report.Added, report.Applied)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: doc %d bytes (applied=%t)\n", n.ID, len(report.Doc), report.Applied)
			if !apply {
				fmt.Fprintln(cmd.OutOrStdout(), report.Doc)
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().StringVar(&nodeID, "node", "", "node id (required)")
	c.Flags().BoolVar(&apply, "apply", false, "write generated content (default: dry-run)")
	c.Flags().BoolVar(&change, "change", false, "for frozen nodes: request a change before applying")
	c.Flags().StringVar(&summary, "summary", "", "change summary (with --change)")
	c.Flags().StringVar(&by, "by", "", "change actor (with --change; default cli)")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}

var _ = store.ResolveRootPath
