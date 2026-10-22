// Standard flow S2 (AC-141): the pre-implement baseline checks —
// the feature worktree's HEAD must already contain the canonical
// truth_baseline_commit, and the worktree must not carry a dirty
// state that violates the dirty policy.
//
// Both checks judge ONE binding, resolved once here and shared by
// the admission items (admission.go) and the admission bundle's
// branch-governance / diff items (bundle.go) — one judgement, no
// second derivation:
//
//   - a REAL satellite worktree record of the run (active, not the
//     root binding): judge that directory and its branch;
//   - otherwise the project root itself (single_worktree /
//     current-branch binding): judge the root and its HEAD;
//   - a planned-only pool binding (a parallel plan names a
//     satellite ref but no real worktree was ever created) judges
//     NOTHING: there is no real worktree or branch to check, so the
//     items report not_applicable instead of inventing a verdict.
//
// Non-git projects are likewise not_applicable — AC-141 governs
// git worktrees; a non-git project must never be painted red by a
// git-only check (the task's explicit caliber, consistent with
// S1's P4b frontmatter-authority rule for non-git projects).
package orchestrator

import (
	"context"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/harness"
	"beacon-go/internal/store"
)

// Admission item names of the S2 checks (admission.go items 6/7).
const (
	AdmissionCheckBaselineMerged = "baseline_merged"
	AdmissionCheckDirtyPolicy    = "dirty_policy"
)

// worktreeBinding is the resolved AC-141 judgement target.
type worktreeBinding struct {
	Applicable bool
	Dir        string // absolute directory whose HEAD/status is judged
	Branch     string // branch checked out at Dir ("" when unknown)
	Root       string // project root (package layout anchor)
	Feature    store.FeaturePackage
	Note       string // why not applicable (when Applicable=false)
}

// bindingOf resolves the judgement target for one run. Read-only:
// it never creates the worktree bookkeeping it consults.
func (e *Engine) bindingOf(run store.Run, m *runMeta, slug string) (worktreeBinding, error) {
	fc, ok, err := e.featureContextOf(run.FeatureID)
	if err != nil {
		return worktreeBinding{}, err
	}
	if !ok {
		return worktreeBinding{Note: "feature is not registered in projects/spine"}, nil
	}
	root := fc.Project.RootPath
	b := worktreeBinding{Root: root, Feature: fc.Feature}
	if !harness.IsGitRepo(context.Background(), root) {
		b.Note = "not a git repository: AC-141 baseline/dirty checks do not apply (frontmatter is the authority, P4b caliber)"
		return b, nil
	}
	if m.Implement != nil && m.Implement.Worktrees != nil {
		if rec := m.Implement.Worktrees.activeFor(slug); rec != nil && rec.Ref != "." && rec.Path != "" {
			b.Applicable, b.Dir, b.Branch = true, rec.Path, rec.Branch
			return b, nil
		}
	}
	if expectedWorktree(m, slug) != "." {
		b.Note = "run is bound to a planned-only satellite ref (" + expectedWorktree(m, slug) +
			"); no real worktree/branch exists to judge (AC-141 judges real bindings)"
		return b, nil
	}
	b.Applicable, b.Dir = true, root
	b.Branch, _ = harness.CurrentBranch(context.Background(), root)
	return b, nil
}

// baselineCommitOf returns the run feature's consumable baseline
// commit (S1's recorded/probed value, via the shared FrozenFact)
// and its baseline status. "" commit when none exists.
func (e *Engine) baselineCommitOf(featureID string) (commit, status string, frozen bool, err error) {
	ff, err := e.frozenFactByID(featureID)
	if err != nil {
		return "", "", false, err
	}
	return ff.BaselineCommit, ff.BaselineStatus, ff.Frozen, nil
}

// evalBaselineMerged is the AC-141 ancestor judgement: the bound
// worktree's HEAD must contain truth_baseline_commit
// (git merge-base --is-ancestor). Frozen codes: a frozen package
// with no committed baseline on record reports
// truth_baseline_commit_missing; a HEAD that does not contain the
// recorded baseline reports truth_baseline_not_merged (R10/TC-141).
func (e *Engine) evalBaselineMerged(run store.Run, m *runMeta, slug string) (AdmissionItem, string, error) {
	b, err := e.bindingOf(run, m, slug)
	if err != nil {
		return AdmissionItem{}, "", err
	}
	if !b.Applicable {
		return AdmissionItem{Check: AdmissionCheckBaselineMerged, OK: true, Applicable: false, Detail: b.Note}, "", nil
	}
	commit, status, frozen, err := e.baselineCommitOf(run.FeatureID)
	if err != nil {
		return AdmissionItem{}, "", err
	}
	if !frozen {
		return AdmissionItem{Check: AdmissionCheckBaselineMerged, OK: true, Applicable: false,
			Detail: "truth is not frozen; no baseline exists to merge yet (the truth_frozen item governs)"}, "", nil
	}
	if status == BaselineNoDocument {
		// S1 caliber (stdflow-s1 report §3): baseline discipline
		// only ever probes packages with a real truth document on
		// disk; a no_document (seeded) registration is not blocked.
		return AdmissionItem{Check: AdmissionCheckBaselineMerged, OK: true, Applicable: false,
			Detail: "no truth document on disk (seeded registration): baseline-merged not applicable — S1 no_document caliber"}, "", nil
	}
	if status != BaselineCommitted || commit == "" {
		return AdmissionItem{Check: AdmissionCheckBaselineMerged, Applicable: true,
			ReasonCode: core.ReasonTruthBaselineCommitMissing,
			Detail: "no truth_baseline_commit on record (baseline_status=" + status +
				"); freeze, commit on the canonical branch and run `beacon truth baseline` first (AC-141/AC-139)"}, "", nil
	}
	head, err := harness.HeadCommit(context.Background(), b.Dir)
	if err != nil {
		return AdmissionItem{}, "", err
	}
	anc, err := harness.IsAncestor(context.Background(), b.Dir, commit, "HEAD")
	if err != nil {
		return AdmissionItem{}, "", err
	}
	if !anc {
		return AdmissionItem{Check: AdmissionCheckBaselineMerged, Applicable: true,
			ReasonCode: core.ReasonTruthBaselineNotMerged,
			Detail: "worktree HEAD " + shortSHA(head) + " does not contain truth_baseline_commit " + shortSHA(commit) +
				"; merge the canonical branch into the worktree before implementing (AC-141)"}, commit, nil
	}
	return AdmissionItem{Check: AdmissionCheckBaselineMerged, OK: true, Applicable: true,
		Detail: "worktree HEAD " + shortSHA(head) + " contains truth_baseline_commit " + shortSHA(commit)}, commit, nil
}

func shortSHA(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

// dirtyViolations classifies the dirty paths of one judged
// directory under the S2 dirty policy:
//
//   - VIOLATION (always): any dirty path inside the feature's
//     requirement-package directory — the package IS the baseline
//     body; a dirty member means the worktree's requirement text
//     differs from what was frozen and committed;
//   - VIOLATION (always): any dirty path on the P4c protected-path
//     denylist (secrets / auth / payments / migrations / env /
//     key material);
//   - NOT a violation at the pre-implement point (DERIVED caliber,
//     see the S2 report): ordinary code files. The frozen text
//     enumerates no rule for them at implement entry — implement
//     admission is the moment work BEGINS, prior-stage outputs and
//     the caller's scoped work-in-progress are the implementer's
//     canvas, and R11's dirty language ("must not mix unrelated
//     runtime dirty files") binds the canonical_commit step, not
//     this one. They are counted in the detail, never hidden.
//
// entries are slash-separated paths relative to the judged dir;
// pkgRel is the package directory relative to the same dir.
func dirtyViolations(entries []string, pkgRel string) (violations, ordinary []string) {
	pkgRel = path.Clean(filepath.ToSlash(pkgRel))
	for _, p := range entries {
		p = strings.TrimSuffix(path.Clean("/"+filepath.ToSlash(p)), "/")
		p = strings.TrimPrefix(p, "/")
		switch {
		case p == pkgRel || strings.HasPrefix(p, pkgRel+"/") || strings.HasPrefix(pkgRel, p+"/"):
			violations = append(violations, p+" (requirement package)")
		default:
			if cat, hit := core.ProtectedPathMatch(p); hit {
				violations = append(violations, p+" (protected path: "+cat+")")
			} else {
				ordinary = append(ordinary, p)
			}
		}
	}
	return violations, ordinary
}

// evalDirtyPolicy is the AC-141 dirty-policy judgement over the
// bound worktree's working tree.
func (e *Engine) evalDirtyPolicy(run store.Run, m *runMeta, slug string) (AdmissionItem, error) {
	b, err := e.bindingOf(run, m, slug)
	if err != nil {
		return AdmissionItem{}, err
	}
	if !b.Applicable {
		return AdmissionItem{Check: AdmissionCheckDirtyPolicy, OK: true, Applicable: false, Detail: b.Note}, nil
	}
	entries, err := harness.StatusEntries(context.Background(), b.Dir)
	if err != nil {
		return AdmissionItem{}, err
	}
	pkgRel := path.Dir(filepath.ToSlash(b.Feature.TruthPath))
	// Package paths are recorded relative to the project root; a
	// satellite worktree mirrors the project root layout, so the
	// same relative package dir applies inside it. When the judged
	// dir is a sub-path binding (never in the current modes), fall
	// back to the root's own status for the package leg.
	violations, ordinary := dirtyViolations(entries, pkgRel)
	if len(violations) > 0 {
		shown := violations
		if len(shown) > 5 {
			shown = shown[:5]
		}
		return AdmissionItem{Check: AdmissionCheckDirtyPolicy, Applicable: true,
			ReasonCode: core.ReasonUnscopedDirtyState,
			Detail: "dirty state violates the pre-implement dirty policy: " + strings.Join(shown, ", ") +
				"; commit/stash the requirement package and protected paths before implementing (AC-141)"}, nil
	}
	detail := "no package or protected-path dirty state"
	if len(ordinary) > 0 {
		detail += " (" + strconv.Itoa(len(ordinary)) + " ordinary code path(s) dirty — not a violation at the pre-implement point, DERIVED caliber)"
	}
	return AdmissionItem{Check: AdmissionCheckDirtyPolicy, OK: true, Applicable: true, Detail: detail}, nil
}
