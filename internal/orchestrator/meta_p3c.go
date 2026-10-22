// P3c real-integration state types: the run's executor configuration
// (subprocess / profile / ralph commands, recorded when the run is
// created with one), the REAL git worktrees created from the
// parallel plan, and the planner-review record. Like every other
// goal state these live inside runs.metadata_json — the single store
// snapshot owned by the engine — never in package-owned JSON side
// files (charter rule 7, frozen AC-016). Secrets are never recorded
// here: only argv shapes and paths.
package orchestrator

import "time"

// ExecutorConfig is the run's subprocess-family executor
// configuration, captured at `goal run --exec-cmd` (or assembled by
// the CLI for tick/driver/delegate). Argv is an argument ARRAY —
// never a shell string (anti-injection rule of this slice).
type ExecutorConfig struct {
	Argv            []string `json:"argv,omitempty"`
	TimeoutSec      int      `json:"timeout_sec,omitempty"`
	RalphArgv       []string `json:"ralph_argv,omitempty"`
	RalphMaxIter    int      `json:"ralph_max_iterations,omitempty"`
	Profile         string   `json:"profile,omitempty"`
	ProfileArgv     []string `json:"profile_argv,omitempty"`
	ReviewerProfile string   `json:"reviewer_profile,omitempty"`
	ReviewerArgv    []string `json:"reviewer_argv,omitempty"`
}

// timeout returns the per-iteration timeout (<=0 = harness default).
func (c *ExecutorConfig) timeout() time.Duration {
	if c == nil || c.TimeoutSec <= 0 {
		return 0
	}
	return time.Duration(c.TimeoutSec) * time.Second
}

// WorktreeRecord is one REAL worktree created from the run's
// parallel plan (P3c, AC-099): the satellite lake, its branch, its
// directory, and its lifecycle state. Records live on ImplementState
// (the implement workstation binding); the directory itself lives
// under <project>/.beacon-worktrees/<lake> (AC-099 worktree mode).
type WorktreeRecord struct {
	Lake      string `json:"lake"`
	Branch    string `json:"branch"`
	Path      string `json:"path"` // absolute
	Ref       string `json:"ref"`  // plan-relative ref (.beacon/worktrees/satellite/<lake>)
	BaseRef   string `json:"base_ref,omitempty"`
	State     string `json:"state"` // active | merged | removed
	CreatedAt string `json:"created_at"`
	MergedAt  string `json:"merged_at,omitempty"`
	RemovedAt string `json:"removed_at,omitempty"`
}

// Worktree lifecycle states.
const (
	WorktreeActive  = "active"
	WorktreeMerged  = "merged"
	WorktreeRemoved = "removed"
)

// WorktreeState is the run's real-worktree book (kept on
// ImplementState): every satellite created for this run plus the
// main star's branch at creation time.
type WorktreeState struct {
	MainBranch string           `json:"main_branch,omitempty"`
	Records    []WorktreeRecord `json:"records,omitempty"`
	// Quarantines are the S1 (AC-140) stale-truth isolations
	// recorded against this run's worktrees (marker records
	// only — quarantining never modifies a worktree's files).
	Quarantines []TruthQuarantine `json:"quarantines,omitempty"`
}

// TruthQuarantine is one AC-140 isolation record: a satellite
// worktree carries a package whose frontmatter declares frozen
// while the canonical package is not frozen, or whose frozen
// body fingerprint differs from the canonical baseline. The
// record freezes the evidence (file hash + frontmatter status +
// fingerprints) so the merge gate can refuse without trusting
// the worktree later.
type TruthQuarantine struct {
	FeatureID         string `json:"feature_id"`
	FeatureSlug       string `json:"feature_slug"`
	WorktreePath      string `json:"worktree_path"`
	TruthRelPath      string `json:"truth_rel_path"`
	FrontmatterStatus string `json:"frontmatter_status,omitempty"`
	TruthSHA256       string `json:"truth_sha256"`
	Reason            string `json:"reason"` // local_frozen_canonical_unfrozen | fingerprint_mismatch
	RecordedAt        string `json:"recorded_at"`
}

// find returns the record for a lake (any state), or nil.
func (s *WorktreeState) find(lake string) *WorktreeRecord {
	if s == nil {
		return nil
	}
	for i := range s.Records {
		if s.Records[i].Lake == lake {
			return &s.Records[i]
		}
	}
	return nil
}

// activeFor returns the record when it exists AND is active.
func (s *WorktreeState) activeFor(lake string) *WorktreeRecord {
	r := s.find(lake)
	if r == nil || r.State != WorktreeActive {
		return nil
	}
	return r
}

// PlannerReviewRecord is the run's latest planner review (AC-102):
// the aggregated verdict over the reviewer lanes. The full lanes
// ride in the view and in the gate_results row; the record keeps
// the headline so status surfaces can show it.
type PlannerReviewRecord struct {
	Verdict    string `json:"verdict"` // pass | blocked
	ReasonCode string `json:"reason_code,omitempty"`
	Reviewers  int    `json:"reviewers"`
	Invalid    int    `json:"invalid"`
	Mode       string `json:"execution_mode"`
	At         string `json:"at"`
	Summary    string `json:"summary,omitempty"`
}
