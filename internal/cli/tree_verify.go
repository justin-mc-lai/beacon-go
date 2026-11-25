package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/orchestrator"
)

func newVerifyCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "verify",
		Short: "Verification group: audit, evidence, qa gate, score, autotest",
		Long: "The single verification entry. Tree audit/leaf/rollup live here, and\n" +
			"the former qa face is folded in: status, evaluate, planner-review,\n" +
			"score, score-history, autotest, strategy, agentscore, agenteval,\n" +
			"verify-harness. `qa` is not a command anymore.",
	}
	c.AddCommand(
		newVerifyAuditCmd(), newVerifyLeafCmd(), newVerifyRollupCmd(),
		newGateStatusCmd(), newGateEvaluateFixedCmd("qa"), newGoalPlannerReviewCmd(),
		newQAScoreCmd(), newQAScoreHistoryCmd(), newQAAutoTestCmd(),
		newQAStrategyCmd(), newQAAgentScoreCmd(), newQAAgentEvalCmd(),
		newQAVerifyHarnessCmd(),
	)
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

func newVerifyLeafCmd() *cobra.Command {
	var project, version, nodeID, artifact, by, kind string
	var asJSON bool
	c := &cobra.Command{
		Use:   "leaf",
		Short: "Register leaf evidence under the node scope and mark it verified",
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
			if nodeID == "" || artifact == "" {
				return fail("tree_verification_required", "verify leaf", "--node and --artifact are required")
			}
			if kind == "" {
				kind = "test_result"
			}
			eng := orchestrator.NewEngine(st)
			reg, err := eng.RegisterScopedArtifact("node", nodeID, kind, artifact, by)
			if err != nil {
				return err
			}
			rep, err := intenttree.MarkVerified(st, dir, nodeID, reg.Artifact.ID, reg.Artifact.ContentHash, by)
			if err != nil {
				return err
			}
			_ = idx
			if asJSON {
				return printJSON(cmd, map[string]any{"verify": rep, "artifact": reg.Artifact})
			}
			_, err = cmd.OutOrStdout().Write([]byte("verified: " + rep.NodeID + " artifact " + reg.Artifact.ID + "\n"))
			return err
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().StringVar(&nodeID, "node", "", "touchpoint node id (required)")
	c.Flags().StringVar(&artifact, "artifact", "", "evidence file path (required)")
	c.Flags().StringVar(&by, "by", "", "verifier identity (required)")
	c.Flags().StringVar(&kind, "kind", "test_result", "evidence kind")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}

func newVerifyRollupCmd() *cobra.Command {
	var project, version, nodeID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "rollup",
		Short: "Mark a parent verified only when every child is verified",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			dir, _, _, err := resolveTree(st, project, version)
			if err != nil {
				return err
			}
			if nodeID == "" {
				return fail("tree_node_required", "verify rollup", "--node is required")
			}
			rep, err := intenttree.RollupVerified(st, dir, nodeID)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, rep)
			}
			_, err = cmd.OutOrStdout().Write([]byte("rollup: " + rep.NodeID + " " + rep.Status + "\n"))
			return err
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().StringVar(&nodeID, "node", "", "parent node id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}
