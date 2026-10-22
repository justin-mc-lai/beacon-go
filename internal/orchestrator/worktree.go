// Real git worktrees (P3c, AC-099): the P2c parallel plan becomes
// real directories and branches.
//
// Modes (AC-099's three): "worktree" (satellite pool: one git
// worktree + one branch per lake, under <project>/.beacon-worktrees/),
// "run-branch" (the project root itself, switched to a
// beacon-run/<run-id> branch) and "current-branch" (the project
// root on whatever branch is checked out). Illegal combinations
// fail closed: current-branch + isolated pool has no meaning and is
// refused, and satellite pool creation without a pool plan is
// refused.
//
// Safety rails (all fail-closed, all events on the run trail):
//   - not a git repository root -> refuse (worktree_not_a_git_repository)
//   - main work tree dirty -> refuse unless --allow-dirty (the
//     override itself is recorded in the creation event)
//   - merge direction is satellite -> main_star ONLY; any other
//     direction is refused before git is invoked
//   - merge conflicts stop the merge (git abort) and report the
//     conflicted files; nothing is force-resolved
//   - cleanup refuses a satellite with uncommitted or unmerged
//     work unless --discard, which is itself recorded in the event
package orchestrator

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/harness"
	"beacon-go/internal/store"
)

// Worktree modes (AC-099).
const (
	WorktreeModePool          = "worktree"
	WorktreeModeRunBranch     = "run-branch"
	WorktreeModeCurrentBranch = "current-branch"
)

// WorktreeDirName is the project-relative directory holding the
// pool's real satellite worktrees.
const WorktreeDirName = ".beacon-worktrees"

// WorktreeCreateOptions carries the create inputs.
type WorktreeCreateOptions struct {
	Mode       string // worktree (default) | run-branch | current-branch
	Lake       string // pool mode: one lake ("" = every satellite)
	Isolated   bool   // caller asked for isolation (with current-branch = invalid combo)
	AllowDirty bool   // override the dirty-main refusal (recorded)
}

// WorktreeView is the worktree surface of one run.
type WorktreeView struct {
	RunID       string           `json:"run_id"`
	ProjectRoot string           `json:"project_root"`
	PlanMode    string           `json:"plan_mode,omitempty"`
	MainBranch  string           `json:"main_branch,omitempty"`
	Records     []WorktreeRecord `json:"records"`
	// Quarantines: S1 (AC-140) stale-truth isolations recorded
	// against this run's worktrees.
	Quarantines []TruthQuarantine `json:"quarantines"`
	Note        string            `json:"note,omitempty"`
}

// worktreeBook returns the run's worktree state (creating the shell).
func worktreeBook(m *runMeta) *WorktreeState {
	if m.Implement == nil {
		m.Implement = &ImplementState{}
	}
	if m.Implement.Worktrees == nil {
		m.Implement.Worktrees = &WorktreeState{}
	}
	return m.Implement.Worktrees
}

func (e *Engine) worktreeView(run store.Run, m *runMeta, root, note string) WorktreeView {
	book := worktreeBook(m)
	records := book.Records
	if records == nil {
		records = []WorktreeRecord{}
	}
	quarantines := book.Quarantines
	if quarantines == nil {
		quarantines = []TruthQuarantine{}
	}
	planMode := ""
	if m.ParallelPlan != nil {
		planMode = m.ParallelPlan.Mode
	}
	return WorktreeView{
		RunID: run.ID, ProjectRoot: root, PlanMode: planMode,
		MainBranch: book.MainBranch, Records: records, Quarantines: quarantines, Note: note,
	}
}

// WorktreeCreate materializes the plan (pool modes) or records the
// root binding (branch modes) for a run.
func (e *Engine) WorktreeCreate(ctx context.Context, runID string, opts WorktreeCreateOptions) (WorktreeView, error) {
	mode := opts.Mode
	if mode == "" {
		mode = WorktreeModePool
	}
	switch mode {
	case WorktreeModePool, WorktreeModeRunBranch, WorktreeModeCurrentBranch:
	default:
		return WorktreeView{}, core.NewError(core.ReasonWorktreeModeUnknown, "goal.WorktreeCreate",
			"unknown worktree mode "+mode+" (want worktree|run-branch|current-branch)")
	}
	if mode == WorktreeModeCurrentBranch && opts.Isolated {
		return WorktreeView{}, core.NewError(core.ReasonWorktreeInvalidCombo, "goal.WorktreeCreate",
			"current-branch cannot be combined with an isolated worktree (AC-099 fail-closed combination)")
	}
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return WorktreeView{}, err
	}
	root, err := e.runProjectRoot(run)
	if err != nil {
		return WorktreeView{}, err
	}
	if !harness.IsGitRepoRoot(ctx, root) {
		return WorktreeView{}, core.NewError(core.ReasonWorktreeNotGitRepo, "goal.WorktreeCreate",
			"project root "+root+" is not the top level of a git work tree")
	}
	dirty, err := harness.MainDirty(ctx, root)
	if err != nil {
		return WorktreeView{}, err
	}
	if dirty && !opts.AllowDirty {
		return WorktreeView{}, core.NewError(core.ReasonWorktreeMainDirty, "goal.WorktreeCreate",
			"main work tree has uncommitted changes; commit/stash first or pass --allow-dirty (recorded)")
	}
	mainBranch, err := harness.CurrentBranch(ctx, root)
	if err != nil {
		return WorktreeView{}, err
	}
	book := worktreeBook(m)
	book.MainBranch = mainBranch

	switch mode {
	case WorktreeModeCurrentBranch, WorktreeModeRunBranch:
		branch := mainBranch
		if mode == WorktreeModeRunBranch {
			branch = "beacon-run/" + runID
			if branch != mainBranch {
				if harness.BranchExists(ctx, root, branch) {
					if _, err := harness.SwitchBranch(ctx, root, branch); err != nil {
						return WorktreeView{}, err
					}
				} else if _, err := harness.CreateAndSwitchBranch(ctx, root, branch); err != nil {
					return WorktreeView{}, err
				}
			}
		}
		if rec := book.find(slug); rec != nil && rec.State == WorktreeActive {
			return e.worktreeView(run, m, root, "root binding already recorded"), nil
		}
		rec := WorktreeRecord{
			Lake: slug, Branch: branch, Path: root, Ref: ".",
			BaseRef: mainBranch, State: WorktreeActive, CreatedAt: nowISO(),
		}
		book.Records = append(book.Records, rec)
		if err := e.appendEvent(run, "worktree_created", map[string]any{
			"lake": slug, "mode": mode, "branch": branch, "path": root, "ref": ".",
			"allow_dirty": opts.AllowDirty, "real": true,
		}); err != nil {
			return WorktreeView{}, err
		}
		if err := e.saveRun(run, m); err != nil {
			return WorktreeView{}, err
		}
		return e.worktreeView(run, m, root, mode+" binding recorded at the project root"), nil
	}

	// Pool mode: a main-star/satellite plan is required.
	if m.ParallelPlan == nil || m.ParallelPlan.Mode != ParallelPool {
		return WorktreeView{}, core.NewError(core.ReasonWorktreePlanRequired, "goal.WorktreeCreate",
			"satellite worktrees need a parallel plan in "+ParallelPool+" mode; run goal parallel-plan first")
	}
	baseCommit, err := harness.HeadCommit(ctx, root)
	if err != nil {
		return WorktreeView{}, err
	}
	var targets []SatellitePlan
	for _, s := range m.ParallelPlan.Satellites {
		if opts.Lake != "" && s.Lake != opts.Lake {
			continue
		}
		targets = append(targets, s)
	}
	if len(targets) == 0 {
		return WorktreeView{}, core.NewError(core.ReasonWorktreeMissing, "goal.WorktreeCreate",
			"no satellite named "+opts.Lake+" in the run's parallel plan")
	}
	for _, s := range targets {
		if book.activeFor(s.Lake) != nil {
			return WorktreeView{}, core.NewError(core.ReasonWorktreeExists, "goal.WorktreeCreate",
				"worktree for lake "+s.Lake+" already exists and is active")
		}
		branch := "beacon-run/" + runID + "/" + s.Lake
		path := filepath.Join(root, WorktreeDirName, s.Lake)
		if harness.BranchExists(ctx, root, branch) {
			return WorktreeView{}, core.NewError(core.ReasonWorktreeExists, "goal.WorktreeCreate",
				"branch "+branch+" already exists")
		}
		if _, err := harness.WorktreeAdd(ctx, root, path, branch, ""); err != nil {
			return WorktreeView{}, core.NewError(core.ReasonWorktreeMissing, "goal.WorktreeCreate",
				"git worktree add failed for lake "+s.Lake+": "+strings.TrimSpace(err.Error()))
		}
		book.Records = append(book.Records, WorktreeRecord{
			Lake: s.Lake, Branch: branch, Path: path, Ref: s.Worktree,
			BaseRef: baseCommit, State: WorktreeActive, CreatedAt: nowISO(),
		})
		if err := e.appendEvent(run, "worktree_created", map[string]any{
			"lake": s.Lake, "mode": mode, "branch": branch, "path": path, "ref": s.Worktree,
			"base_ref": baseCommit, "allow_dirty": opts.AllowDirty, "real": true,
		}); err != nil {
			return WorktreeView{}, err
		}
	}
	if err := e.saveRun(run, m); err != nil {
		return WorktreeView{}, err
	}
	// S1 (AC-140): a freshly materialized worktree can already
	// carry a stale local freeze (its branch tip was cut from a
	// commit that had one) — scan and record at creation too.
	note := "satellite worktree(s) created"
	if _, err := e.scanAndRecordQuarantines(run, m, root); err != nil {
		return WorktreeView{}, err
	}
	if n := len(worktreeBook(m).Quarantines); n > 0 {
		note += "; " + strconv.Itoa(n) + " stale truth quarantine(s) on record (AC-140)"
	}
	return e.worktreeView(run, m, root, note), nil
}

// WorktreeStatus returns the run's recorded worktrees. Since S1
// it also runs the AC-140 stale-truth scan over the satellite
// worktrees and persists any NEW quarantine it finds (an old
// finding is reported from the book without a duplicate record);
// with no new finding the call writes nothing.
func (e *Engine) WorktreeStatus(runID string) (WorktreeView, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return WorktreeView{}, err
	}
	root, err := e.runProjectRoot(run)
	if err != nil {
		return WorktreeView{}, err
	}
	if _, err := e.scanAndRecordQuarantines(run, m, root); err != nil {
		return WorktreeView{}, err
	}
	return e.worktreeView(run, m, root, ""), nil
}

// WorktreeMerge merges one satellite into the main star. Direction
// is satellite -> main_star ONLY: from=main_star, or any to other
// than main_star, is refused before git runs.
func (e *Engine) WorktreeMerge(ctx context.Context, runID, lake, from, to string) (WorktreeView, error) {
	if from == "main_star" || (to != "" && to != "main_star") {
		return WorktreeView{}, core.NewError(core.ReasonWorktreeMergeDirection, "goal.WorktreeMerge",
			"merges run satellite -> main_star only, never the reverse and never satellite -> satellite ("+core.ReasonWorktreeMergeDirection+")")
	}
	if from != "" && from != lake {
		return WorktreeView{}, core.NewError(core.ReasonWorktreeMergeDirection, "goal.WorktreeMerge",
			"--from must name the satellite lake being merged")
	}
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return WorktreeView{}, err
	}
	root, err := e.runProjectRoot(run)
	if err != nil {
		return WorktreeView{}, err
	}
	book := worktreeBook(m)
	rec := book.activeFor(lake)
	if rec == nil {
		return WorktreeView{}, core.NewError(core.ReasonWorktreeMissing, "goal.WorktreeMerge",
			"no active worktree for lake "+lake)
	}
	if rec.Ref == "." {
		return WorktreeView{}, core.NewError(core.ReasonWorktreeMergeDirection, "goal.WorktreeMerge",
			"lake "+lake+" is bound at the project root (branch mode); there is no satellite to merge")
	}
	current, err := harness.CurrentBranch(ctx, root)
	if err != nil {
		return WorktreeView{}, err
	}
	if book.MainBranch != "" && current != book.MainBranch {
		return WorktreeView{}, core.NewError(core.ReasonWorktreeMergeDirection, "goal.WorktreeMerge",
			"main star is on branch "+current+", not its recorded main branch "+book.MainBranch+"; refusing to merge into the wrong branch")
	}
	// S1 (AC-140): a FRESH stale-truth scan of the source
	// worktree gates the merge — a satellite carrying a
	// quarantined (locally frozen, canonically unfrozen or
	// divergent) package may not land its truth on the main star.
	// The scan result is authoritative; an old record for a since-
	// repaired file does not block, and a new finding is recorded
	// before the refusal.
	{
		found := e.scanWorktreeQuarantines(run, m, root)
		var stale []TruthQuarantine
		for _, q := range found {
			if q.WorktreePath == rec.Path {
				stale = append(stale, q)
			}
		}
		if len(stale) > 0 {
			if _, err := e.recordQuarantines(run, m, stale); err != nil {
				return WorktreeView{}, err
			}
			if err := e.saveRun(run, m); err != nil {
				return WorktreeView{}, err
			}
			q := stale[0]
			_ = e.appendEvent(run, "worktree_merge_blocked", map[string]any{
				"lake": lake, "branch": rec.Branch, "feature_slug": q.FeatureSlug,
				"quarantine_reason": q.Reason, "reason_code": core.ReasonStaleFeatureLocalRefreeze,
			})
			return e.worktreeView(run, m, root, "merge blocked: stale feature-local refreeze quarantined in "+rec.Path+" ("+q.Reason+")"),
				core.NewError(core.ReasonStaleFeatureLocalRefreeze, "goal.WorktreeMerge",
					"satellite "+lake+" carries a stale local freeze of package "+q.FeatureSlug+
						" ("+q.Reason+"); its truth is quarantined and may not merge into the main star (AC-140) — reconcile the package on the canonical branch instead")
		}
	}
	// Protected-path write guard (P4c; v1.6.5 denylist, a
	// cross-version fail-closed principle): the merge is the one
	// automatic write that lands satellite content on the main
	// star, so it is where the guard bites. If the satellite would
	// bring a protected path (.env, secrets, auth, payments,
	// migrations, key material) into the main tree, the merge is
	// blocked with protected_path_denylist and an explicit human
	// route — never silently allowed.
	base := book.MainBranch
	if base == "" {
		base = current
	}
	changed, err := harness.ChangedFiles(ctx, root, base, rec.Branch)
	if err != nil {
		return WorktreeView{}, err
	}
	for _, f := range changed {
		if cat, hit := core.ProtectedPathMatch(f); hit {
			route := core.HumanRouteForProtectedPath(cat, f)
			_ = e.appendEvent(run, "worktree_merge_blocked", map[string]any{
				"lake": lake, "branch": rec.Branch, "file": f, "category": cat,
				"reason_code": core.ReasonProtectedPathDenylist, "human_route": route,
			})
			return e.worktreeView(run, m, root, "merge blocked: "+route),
				core.NewError(core.ReasonProtectedPathDenylist, "goal.WorktreeMerge",
					"satellite "+rec.Branch+" would write protected path "+f+" ("+cat+") into the main star; "+route)
		}
	}
	conflicts, _, mergeErr := harness.MergeInto(ctx, root, rec.Branch)
	if len(conflicts) > 0 {
		_ = e.appendEvent(run, "worktree_merge_conflict", map[string]any{
			"lake": lake, "branch": rec.Branch, "files": conflicts,
			"reason_code": core.ReasonWorktreeMergeConflict,
		})
		return e.worktreeView(run, m, root, "merge conflict in: "+strings.Join(conflicts, ", ")),
			core.NewError(core.ReasonWorktreeMergeConflict, "goal.WorktreeMerge",
				"merging "+rec.Branch+" conflicts in "+strings.Join(conflicts, ", ")+"; merge aborted, nothing force-resolved")
	}
	if mergeErr != nil {
		return WorktreeView{}, core.NewError(core.ReasonWorktreeMergeConflict, "goal.WorktreeMerge",
			"git merge failed: "+strings.TrimSpace(mergeErr.Error()))
	}
	rec.State = WorktreeMerged
	rec.MergedAt = nowISO()
	_ = e.appendEvent(run, "worktree_merged", map[string]any{
		"lake": lake, "branch": rec.Branch, "into": book.MainBranch, "direction": "satellite->main_star",
	})
	if err := e.saveRun(run, m); err != nil {
		return WorktreeView{}, err
	}
	return e.worktreeView(run, m, root, "lake "+lake+" merged into "+book.MainBranch), nil
}

// WorktreeCleanup removes one satellite worktree. Uncommitted
// changes or an unmerged branch refuse the cleanup unless discard
// is set (and the discard is recorded in the removal event).
func (e *Engine) WorktreeCleanup(ctx context.Context, runID, lake string, discard bool) (WorktreeView, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return WorktreeView{}, err
	}
	root, err := e.runProjectRoot(run)
	if err != nil {
		return WorktreeView{}, err
	}
	book := worktreeBook(m)
	rec := book.find(lake)
	if rec == nil || rec.State == WorktreeRemoved {
		return WorktreeView{}, core.NewError(core.ReasonWorktreeMissing, "goal.WorktreeCleanup",
			"no live worktree for lake "+lake)
	}
	if rec.Ref == "." {
		return WorktreeView{}, core.NewError(core.ReasonWorktreeMissing, "goal.WorktreeCleanup",
			"lake "+lake+" is bound at the project root (branch mode); branch-mode bindings are not removed")
	}
	if !discard {
		dirty, err := harness.HasUncommittedChanges(ctx, rec.Path)
		if err != nil {
			return WorktreeView{}, err
		}
		if dirty {
			return WorktreeView{}, core.NewError(core.ReasonWorktreeUnmerged, "goal.WorktreeCleanup",
				"worktree "+rec.Path+" has uncommitted changes; merge/commit them or clean up with --discard")
		}
		merged, err := harness.IsAncestor(ctx, root, rec.Branch, book.MainBranch)
		if err != nil {
			return WorktreeView{}, err
		}
		if !merged {
			return WorktreeView{}, core.NewError(core.ReasonWorktreeUnmerged, "goal.WorktreeCleanup",
				"branch "+rec.Branch+" is not merged into "+book.MainBranch+"; merge it or clean up with --discard")
		}
	}
	if _, err := harness.WorktreeRemove(ctx, root, rec.Path, discard); err != nil {
		return WorktreeView{}, core.NewError(core.ReasonWorktreeMissing, "goal.WorktreeCleanup",
			"git worktree remove failed: "+strings.TrimSpace(err.Error()))
	}
	branchGone := true
	if _, err := harness.DeleteBranch(ctx, root, rec.Branch, discard); err != nil {
		branchGone = false
		_ = e.appendEvent(run, "worktree_branch_kept", map[string]any{
			"lake": lake, "branch": rec.Branch, "detail": strings.TrimSpace(err.Error()),
		})
	}
	rec.State = WorktreeRemoved
	rec.RemovedAt = nowISO()
	_ = e.appendEvent(run, "worktree_removed", map[string]any{
		"lake": lake, "branch": rec.Branch, "path": rec.Path,
		"discard": discard, "branch_deleted": branchGone,
	})
	if err := e.saveRun(run, m); err != nil {
		return WorktreeView{}, err
	}
	return e.worktreeView(run, m, root, "worktree for lake "+lake+" removed (discard="+boolWord(discard)+")"), nil
}

func boolWord(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
