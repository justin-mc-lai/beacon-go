// Subprocess + profile stage executors (P3c, AC-044 / AC-045 shape /
// AC-100): REAL stage execution. The command runs as a child process
// (argv only, never a shell) in the run's working directory — the
// project root, or the run's ACTIVE real worktree when one is bound
// (AC-099 cwd alignment: harness/agent project-root == worktree cwd).
//
// Evidence discipline (the opposite of the RecordingExecutor):
//   - stdout/stderr are captured to files and registered in
//     artifact_refs with real sha256 hashes and sizes;
//   - the stage_executed event carries the REAL exit code, the
//     backend identity and the AC-042 summary;
//   - a non-zero exit (or a timeout) returns a FAILED StageOutcome,
//     which Tick routes into the P2a repair/blocker machinery — a
//     failure is never dressed up as success;
//   - a missing / non-executable / unconfigured command fails closed
//     with a typed error BEFORE any evidence is written.
package orchestrator

import (
	"context"
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

// ---- Subprocess stage executor ----

// SubprocessExecutor is the StageExecutor over a SubprocessBackend:
// one real command per stage attempt.
type SubprocessExecutor struct {
	st      *store.Store
	eng     *Engine
	backend *harness.SubprocessBackend
}

// NewSubprocessExecutor builds the executor. argv is validated at
// execution time (typed fail-closed), not here.
func NewSubprocessExecutor(eng *Engine, argv []string, timeoutSec int) *SubprocessExecutor {
	var timeout = harnessTimeout(timeoutSec)
	return &SubprocessExecutor{st: eng.store, eng: eng, backend: harness.NewSubprocessBackend(argv, timeout)}
}

func harnessTimeout(timeoutSec int) time.Duration {
	if timeoutSec <= 0 {
		return harness.DefaultIterationTimeout
	}
	return time.Duration(timeoutSec) * time.Second
}

// ID returns the backend identity (subprocess:<argv0 base>).
func (x *SubprocessExecutor) ID() string { return x.backend.Name() }

// ExecuteStage runs the command once for the stage.
func (x *SubprocessExecutor) ExecuteStage(ctx context.Context, run RunView, stage string) (StageOutcome, error) {
	full, m, _, err := x.eng.loadRun(run.RunID)
	if err != nil {
		return StageOutcome{}, err
	}
	workdir, err := x.eng.executorWorkdir(full, m)
	if err != nil {
		return StageOutcome{}, err
	}
	res, err := x.backend.Iterate(ctx, harness.IterationRequest{
		RunID: run.RunID, FeatureID: run.FeatureID, Stage: stage, Workdir: workdir,
	})
	if err != nil {
		return StageOutcome{}, err // typed fail-closed (missing/not-executable/not-configured)
	}
	return x.eng.recordIterationEvidence(full, stage, res, "subprocess")
}

// ---- Profile (harness) stage executor ----

// ProfileExecutor is the StageExecutor over a ProfileBackend: it
// first writes the stage-context file (requirement package pointers,
// stage plan, admission verdict when at implement, rollback rules),
// then runs the profile's command template with the context PATH as
// the handoff. This is the ImplementHarness execution face: context
// file + worktree-aligned cwd distinguish it from the bare
// SubprocessExecutor (AC-100's anti-impersonation line).
type ProfileExecutor struct {
	eng     *Engine
	profile harness.Profile
	timeout int // seconds
}

// NewProfileExecutor binds a profile (already command-resolved).
func NewProfileExecutor(eng *Engine, p harness.Profile, timeoutSec int) *ProfileExecutor {
	return &ProfileExecutor{eng: eng, profile: p, timeout: timeoutSec}
}

// ID returns subprocess:<profile>.
func (x *ProfileExecutor) ID() string { return "subprocess:" + x.profile.Name }

// ExecuteStage builds the context file and runs the profile once.
func (x *ProfileExecutor) ExecuteStage(ctx context.Context, run RunView, stage string) (StageOutcome, error) {
	full, m, slug, err := x.eng.loadRun(run.RunID)
	if err != nil {
		return StageOutcome{}, err
	}
	workdir, err := x.eng.executorWorkdir(full, m)
	if err != nil {
		return StageOutcome{}, err
	}
	dir, err := x.eng.artifactDirFor(full)
	if err != nil {
		return StageOutcome{}, err
	}
	sc := harness.StageContext{
		RunID: run.RunID, FeatureID: run.FeatureID, FeatureSlug: slug,
		Stage: stage, Iteration: 1, Workdir: workdir, StagePlan: m.StagePlan,
		Acceptance: AcceptanceRuleText, Rollback: RollbackRuleText,
		Prompt: stagePrompt(m, stage),
	}
	if fc, ok, err := x.eng.featureContextOf(run.FeatureID); err == nil && ok {
		sc.PackageRoot = filepath.Join(fc.Project.RootPath, fc.Version.DocsRoot, "features", fc.Feature.Slug)
		sc.TruthPath = joinPkgPath(sc.PackageRoot, fc.Feature.TruthPath)
		sc.TestsPath = joinPkgPath(sc.PackageRoot, fc.Feature.TestsPath)
	}
	if stage == StageImplement {
		if adm, admErr := x.eng.checkAdmission(full, m); admErr == nil {
			sc.Admission = adm
		}
		if m.Implement != nil {
			sc.Lane = m.Implement.Lane
		}
	}
	ctxPath, err := harness.WriteStageContext(dir, sc)
	if err != nil {
		return StageOutcome{}, err
	}
	if _, err := x.eng.store.Artifacts().Add(store.ArtifactRef{
		RunID: run.RunID, Kind: "stage_context", AbsPath: ctxPath,
	}); err != nil {
		return StageOutcome{}, err
	}
	backend := harness.NewProfileBackend(x.profile, ctxPath, sc.Prompt, harnessTimeout(x.timeout))
	res, err := backend.Iterate(ctx, harness.IterationRequest{
		RunID: run.RunID, FeatureID: run.FeatureID, Stage: stage, Lane: sc.Lane,
		Iteration: 1, Workdir: workdir, ContextPath: ctxPath,
	})
	if err != nil {
		return StageOutcome{}, err
	}
	return x.eng.recordIterationEvidence(full, stage, res, "implement_harness")
}

// stagePrompt is the one-line iteration prompt for a stage: the
// run's utterance plus the stage's root skill.
func stagePrompt(m *runMeta, stage string) string {
	prompt := m.Utterance
	if skills := ProgressiveSkills[stage]; len(skills) > 0 {
		if prompt != "" {
			prompt += " "
		}
		prompt += "[" + skills[0] + "]"
	}
	if prompt == "" {
		prompt = "run stage " + stage
	}
	return prompt
}
