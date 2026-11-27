package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/orchestrator"
	"beacon-go/internal/store"
	"beacon-go/internal/version"
)

func newVerifyCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "verify",
		Short: "Verification group: audit, evidence, gate, score, autotest",
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
	hideAllBut(c, "audit", "leaf", "rollup", "status", "evaluate")
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
			dir, projRoot, idx, err := resolveTree(st, project, version)
			if err != nil {
				return err
			}
			findings := intenttree.ValidateIndex(idx)
			var advisories []string
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
				if n.Kind == intenttree.KindTouchpoint {
					if len(n.TestCases) == 0 {
						// Legacy trees predate test cases. Missing coverage is
						// reported, but it must not break rendering or projects
						// that never opted into the new contract.
						advisories = append(advisories, intenttree.ReasonTestCaseMissing+": "+n.ID)
					} else {
						findings = append(findings, intenttree.ValidateTestCases(n.ACs, n.TestCases)...)
					}
				}
				if n.Kind == intenttree.KindFeature && len(n.TestCases) > 0 {
					findings = append(findings, intenttree.ValidateTestCases(n.ACs, n.TestCases)...)
				}
				if n.Kind != intenttree.KindTouchpoint && n.Verification != nil {
					findings = append(findings, "tree_verification_scope: "+n.ID)
				}
				if n.Kind == intenttree.KindModule && childCount[n.ID] == 0 {
					findings = append(findings, intenttree.ReasonModuleEmpty+": "+n.ID)
				}
			}
			findings = append(findings, auditVerifiedEvidence(st, projRoot, dir, idx)...)
			report := map[string]any{
				"ok": len(findings) == 0, "reads_only": true,
				"schema": idx.Schema, "version": idx.Version,
				"node_count": len(idx.Nodes), "findings": findings,
				"advisories": advisories,
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

// auditVerifiedEvidence refuses "verified" claims whose proof cannot be
// reproduced: the registered artifact must still exist inside the repo,
// hash to the registered content, and carry source/binary/fingerprint
// metadata bound to the current requirement contract.
func auditVerifiedEvidence(st *store.Store, projRoot, dir string, idx intenttree.Index) []string {
	var out []string
	for _, e := range idx.Nodes {
		if e.Kind != intenttree.KindTouchpoint || e.Status != intenttree.StatusVerified {
			continue
		}
		n, err := intenttree.ReadNode(dir, e.ID)
		if err != nil {
			continue
		}
		ref := n.VerificationEvidence
		if ref == nil || ref.ArtifactID == "" {
			out = append(out, intenttree.ReasonEvidenceArtifactNotFound+": "+e.ID+" (verified leaf has no evidence ref)")
			continue
		}
		arts, err := st.Artifacts().ListByScope("node", e.ID)
		if err != nil {
			out = append(out, intenttree.ReasonEvidenceArtifactNotFound+": "+e.ID+" ("+err.Error()+")")
			continue
		}
		var found *store.ArtifactRef
		for i := range arts {
			if arts[i].ID == ref.ArtifactID {
				found = &arts[i]
				break
			}
		}
		if found == nil {
			out = append(out, intenttree.ReasonEvidenceArtifactNotFound+": "+e.ID+" ("+ref.ArtifactID+")")
			continue
		}
		raw, err := os.ReadFile(found.AbsPath)
		if err != nil {
			out = append(out, intenttree.ReasonEvidenceFileMissing+": "+e.ID+" ("+filepath.ToSlash(found.AbsPath)+")")
			continue
		}
		sum := sha256.Sum256(raw)
		got := hex.EncodeToString(sum[:])
		if got != found.ContentHash || got != ref.ContentHash {
			out = append(out, intenttree.ReasonEvidenceHashMismatch+": "+e.ID)
			continue
		}
		if _, err := intenttree.ValidateEvidenceFile(projRoot, e.ID, n, found.AbsPath); err != nil {
			out = append(out, err.Error())
		}
	}
	return out
}

// ingestEvidence turns a raw transcript into a committed-evidence
// artifact under docs/beacon/<version>/evidence/<node>/ and stamps the
// source commit, running binary and requirement fingerprint. An
// artifact that already carries the full header is copied as-is.
func ingestEvidence(projRoot, treeDir string, n intenttree.Node, artifact, by string) (string, error) {
	abs, err := filepath.Abs(artifact)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	meta := intenttree.ParseEvidenceMeta(string(raw))
	if len(meta.Missing()) == 0 {
		if rel, inside := repoRel(projRoot, abs); inside && !strings.HasPrefix(rel, ".beacon/") {
			return abs, nil
		}
		dest, err := evidenceDestination(treeDir, n.ID, abs)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(dest, raw, 0o644); err != nil {
			return "", err
		}
		return dest, nil
	}
	dest, err := evidenceDestination(treeDir, n.ID, abs)
	if err != nil {
		return "", err
	}
	head, err := stampedEvidenceHeader(projRoot, filepath.Dir(treeDir), n, by)
	if err != nil {
		return "", err
	}
	out := append(append([]byte{}, head...), raw...)
	if err := os.WriteFile(dest, out, 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

func evidenceDestination(treeDir, nodeID, artifact string) (string, error) {
	dir := filepath.Join(filepath.Dir(treeDir), "evidence", nodeID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, filepath.Base(artifact)), nil
}

func repoRel(root, abs string) (string, bool) {
	r, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	if real, rerr := filepath.EvalSymlinks(r); rerr == nil {
		r = real
	}
	a, err := filepath.Abs(abs)
	if err != nil {
		return "", false
	}
	if real, rerr := filepath.EvalSymlinks(a); rerr == nil {
		a = real
	}
	rel, err := filepath.Rel(r, a)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func stampedEvidenceHeader(projRoot, versionDir string, n intenttree.Node, by string) ([]byte, error) {
	commit, err := gitOutput(projRoot, "rev-parse", "HEAD")
	if err != nil {
		return nil, fail("tree_evidence_source_commit", "verify leaf",
			"cannot stamp source_commit (is this a git repository?): "+err.Error())
	}
	bin := version.Runtime().BinarySHA
	if bin == "" {
		return nil, fail("tree_evidence_binary_sha", "verify leaf", "cannot hash the running beacon binary")
	}
	state := "committed"
	// The verification writes tree status + evidence files itself, so
	// those are excluded: the pin covers the code and the requirement
	// packages, not the bookkeeping churn produced by verifying.
	verRel, ok := repoRel(projRoot, versionDir)
	if !ok {
		verRel = versionDir
	}
	dirty, _ := gitOutput(projRoot, "status", "--porcelain", "--untracked-files=no", "--",
		"internal", "cmd", filepath.ToSlash(filepath.Join(verRel, "features")))
	if strings.TrimSpace(dirty) != "" {
		state = "dirty"
	}
	if state == "dirty" {
		return nil, fail("tree_evidence_source_dirty", "verify leaf",
			"source tree is dirty; commit internal/, cmd/ and docs/beacon/ before registering evidence")
	}
	fp, err := intenttree.ContractFingerprint(n)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(by) == "" {
		by = "unknown"
	}
	head := fmt.Sprintf("# Verification evidence — %s\n\n"+
		"- node: `%s`\n- feature_node_id: `%s`\n- source_commit: `%s`\n"+
		"- binary_sha256: `%s`\n- requirement_fingerprint: `%s`\n"+
		"- source_state: %s\n- registered_by: %s\n- registered_at: %s\n\n---\n\n",
		n.Title, n.ID, n.ParentID, commit, bin, fp, state, by, time.Now().UTC().Format(time.RFC3339))
	return []byte(head), nil
}

func gitOutput(projRoot string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", projRoot}, args...)...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func newVerifyLeafCmd() *cobra.Command {
	var project, version, nodeID, artifact, by, kind string
	var asJSON bool
	c := &cobra.Command{
		Use:   "leaf",
		Short: "WRITES: register leaf evidence under node scope and mark it verified",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			dir, projRoot, idx, err := resolveTree(st, project, version)
			if err != nil {
				return err
			}
			if nodeID == "" || artifact == "" {
				return fail("tree_verification_required", "verify leaf", "--node and --artifact are required")
			}
			if kind == "" {
				kind = "test_result"
			}
			node, err := intenttree.ReadNode(dir, nodeID)
			if err != nil {
				return err
			}
			artifact, err = ingestEvidence(projRoot, dir, node, artifact, by)
			if err != nil {
				return err
			}
			if _, err := intenttree.ValidateEvidenceFile(projRoot, nodeID, node, artifact); err != nil {
				return err
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
		Short: "WRITES: mark a parent verified only when every child is verified",
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
