// Shared execution helpers (workdir, captures, iteration evidence).
// The built-in subprocess/ralph/profile executors were removed; these
// helpers stay because planner review and program delivery still use
// them with host-driven evidence.
package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/harness"
	"beacon-go/internal/store"
)

// runProjectRoot resolves the run's project root from the store.
func (e *Engine) runProjectRoot(run store.Run) (string, error) {
	fc, ok, err := e.featureContextOf(run.FeatureID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", core.NewError(ReasonProjectNotRegistered, "goal.executorWorkdir",
			"no feature context for run "+run.ID)
	}
	return fc.Project.RootPath, nil
}

// executorWorkdir resolves where an executor runs for this run: the
// ACTIVE real worktree bound to the run (its own lake's satellite),
// else the project root. A plan ref with no real worktree behind it
// is plan-only (P2c) and the project root stays the cwd.
func (e *Engine) executorWorkdir(run store.Run, m *runMeta) (string, error) {
	root, err := e.runProjectRoot(run)
	if err != nil {
		return "", err
	}
	if m.Implement != nil && m.Implement.Worktrees != nil {
		if rec := m.Implement.Worktrees.activeFor(slugOf(e, run)); rec != nil {
			return rec.Path, nil
		}
	}
	return root, nil
}

func slugOf(e *Engine, run store.Run) string {
	slug := ""
	if feat, err := e.store.Features().GetByID(run.FeatureID); err == nil {
		slug = feat.Slug
	}
	return slug
}

// artifactDirFor is the run's on-disk evidence directory:
// <project>/.beacon/artifacts/<run-id>/. It holds captured streams
// and stage-context files; the store only ever keeps pointers.
func (e *Engine) artifactDirFor(run store.Run) (string, error) {
	root, err := e.runProjectRoot(run)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, ".beacon", "artifacts", run.ID), nil
}

// writeCapture persists one captured stream and registers it in
// artifact_refs with its real sha256 + size. Returns the ref.
func (e *Engine) writeCapture(run store.Run, dir, kind, name, content string) (store.ArtifactRef, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return store.ArtifactRef{}, err
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return store.ArtifactRef{}, err
	}
	sum := sha256.Sum256([]byte(content))
	ref, err := e.store.Artifacts().Add(store.ArtifactRef{
		RunID: run.ID, Kind: kind, AbsPath: path,
		ContentHash: hex.EncodeToString(sum[:]), SizeBytes: int64(len(content)),
	})
	if err != nil {
		return store.ArtifactRef{}, err
	}
	return ref, nil
}

// recordIterationEvidence writes the full evidence set of one real
// iteration: stdout/stderr captures as artifacts plus ONE
// stage_executed event carrying the real exit code and the AC-042
// summary. Returns the StageOutcome Tick should consume.
func (e *Engine) recordIterationEvidence(run store.Run, stage string, res harness.IterationResult, backendLabel string) (StageOutcome, error) {
	dir, err := e.artifactDirFor(run)
	if err != nil {
		return StageOutcome{}, err
	}
	n := iterationSeqOf(e, run.ID)
	base := fmt.Sprintf("%s-%03d", stage, n)
	if _, err := e.writeCapture(run, dir, "stage_stdout", base+".stdout.txt", res.StdoutText); err != nil {
		return StageOutcome{}, err
	}
	if _, err := e.writeCapture(run, dir, "stage_stderr", base+".stderr.txt", res.StderrText); err != nil {
		return StageOutcome{}, err
	}
	reason := ""
	if res.TimedOut {
		reason = core.ReasonExecutorTimedOut
	} else if res.ExitCode != 0 {
		reason = core.ReasonExecutorExitNonZero
	}
	if err := e.appendEvent(run, "stage_executed", map[string]any{
		"stage": stage, "executor": res.AgentName, "backend": backendLabel,
		"outcome": outcomeWord(res.Success), "exit_code": res.ExitCode,
		"timed_out": res.TimedOut, "reason_code": reason,
		"summary": res.Summary, "placeholder": false, "real": true,
	}); err != nil {
		return StageOutcome{}, err
	}
	if res.Success {
		return StageOutcome{Status: "done", Summary: res.Summary}, nil
	}
	if reason == "" {
		reason = ReasonStageExecutionFailed
	}
	return StageOutcome{Status: "failed", ReasonCode: reason, Summary: res.Summary}, nil
}

func outcomeWord(success bool) string {
	if success {
		return "done"
	}
	return "failed"
}

// iterationSeqOf counts prior stage_executed events so capture file
// names never collide across iterations.
func iterationSeqOf(e *Engine, runID string) int {
	events, err := e.store.ListEventsByRun(runID)
	if err != nil {
		return 0
	}
	n := 0
	for _, ev := range events {
		if ev.EventType == "stage_executed" {
			n++
		}
	}
	return n + 1
}

func harnessTimeout(timeoutSec int) time.Duration {
	if timeoutSec <= 0 {
		return harness.DefaultIterationTimeout
	}
	return time.Duration(timeoutSec) * time.Second
}
