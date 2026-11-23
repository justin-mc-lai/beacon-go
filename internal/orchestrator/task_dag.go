package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

const (
	EventTaskRun     = "implement_tasks_run"
	EventTaskAttempt = "implement_task_attempt"
)

// TaskAttempt is immutable once registered, including failed and repair attempts.
type TaskAttempt struct {
	TaskID          string   `json:"task_id"`
	Phase           string   `json:"phase"`
	Argv            []string `json:"argv"`
	Status          string   `json:"status"`
	Detail          string   `json:"detail,omitempty"`
	ExitCode        int      `json:"exit_code"`
	TimedOut        bool     `json:"timed_out"`
	Cancelled       bool     `json:"cancelled"`
	DurationMS      int64    `json:"duration_ms"`
	ContractBefore  string   `json:"contract_before"`
	ContractAfter   string   `json:"contract_after"`
	CandidateBefore string   `json:"candidate_before"`
	CandidateAfter  string   `json:"candidate_after"`
	ResultPath      string   `json:"result_path"`
	StdoutPath      string   `json:"stdout_path"`
	StderrPath      string   `json:"stderr_path"`
}

type TaskNodeResult struct {
	TaskID   string        `json:"task_id"`
	Status   string        `json:"status"`
	Evidence string        `json:"evidence,omitempty"`
	Attempts []TaskAttempt `json:"attempts"`
}

// TaskRunReport is a persisted invocation snapshot, not a mutable task checkbox ledger.
type TaskRunReport struct {
	Schema              string           `json:"schema"`
	RunID               string           `json:"run_id"`
	FeatureSlug         string           `json:"feature_slug"`
	InvocationID        string           `json:"invocation_id"`
	Workdir             string           `json:"workdir"`
	Status              string           `json:"status"`
	Detail              string           `json:"detail,omitempty"`
	ContractFingerprint string           `json:"contract_fingerprint"`
	CandidateBefore     string           `json:"candidate_before"`
	CandidateAfter      string           `json:"candidate_after"`
	SourceExclusions    []string         `json:"source_exclusions"`
	ExecutableBaseline  string           `json:"executable_baseline"`
	SourceExecutables   []string         `json:"source_executables"`
	StartedAt           string           `json:"started_at"`
	FinishedAt          string           `json:"finished_at"`
	Repairs             int              `json:"repairs"`
	Results             []TaskNodeResult `json:"results"`
	ReportPath          string           `json:"report_path"`
	Files               []qaFile         `json:"files"`
}

type taskRunEvent struct {
	Status       string `json:"status"`
	InvocationID string `json:"invocation_id"`
	ReportPath   string `json:"report_path"`
	ReportHash   string `json:"report_hash"`
}

func taskFingerprint(pkg gate.PackageFiles) string {
	return core.PackageContractFingerprint([]byte(pkg.Truth.Text), []byte(pkg.Tests.Text), []byte(pkg.Tasks.Text), []byte(pkg.Restatement.Text))
}

func (e *Engine) taskPackage(run store.Run, m *runMeta) (store.FeatureContext, gate.PackageFiles, string, error) {
	fc, err := e.featureContext(run.FeatureID)
	if err != nil {
		return fc, gate.PackageFiles{}, "", err
	}
	canonical := e.packageFiles(fc)
	root, err := e.executorWorkdir(run, m)
	if err != nil {
		return fc, canonical, "", err
	}
	fc.Project.RootPath = root
	pkg := e.packageFiles(fc)
	if !pkg.Truth.Present || !pkg.Tasks.Present || taskFingerprint(pkg) != taskFingerprint(canonical) {
		return fc, pkg, root, fmt.Errorf("task_contract_stale: bound workdir must contain the same feature package")
	}
	return fc, pkg, root, nil
}

// TaskStatus reads the latest invocation and verifies its artifact and candidate bindings.
// No execution, event, directory or artifact writes occur on this path.
func (e *Engine) TaskStatus(runID string) (TaskRunReport, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return TaskRunReport{}, err
	}
	_, pkg, root, err := e.taskPackage(run, m)
	if err != nil {
		return TaskRunReport{}, err
	}
	events, err := e.store.ListEventsByRun(run.ID)
	if err != nil {
		return TaskRunReport{}, err
	}
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].EventType != EventTaskRun {
			continue
		}
		var event taskRunEvent
		if err := json.Unmarshal(events[i].Payload, &event); err != nil {
			return TaskRunReport{}, err
		}
		if event.Status == "running" {
			return TaskRunReport{RunID: run.ID, InvocationID: event.InvocationID, Status: "interrupted"}, fmt.Errorf("task_run_interrupted: previous invocation has no final report")
		}
		var report TaskRunReport
		if err := qaRead(event.ReportPath, &report); err != nil {
			return report, err
		}
		if report.RunID != run.ID || report.InvocationID != event.InvocationID || report.Workdir != root {
			return report, fmt.Errorf("task_report_invalid: report binding differs")
		}
		h, err := qaHash(event.ReportPath)
		if err != nil {
			return report, err
		}
		if h != event.ReportHash {
			return report, fmt.Errorf("task_report_stale: registered report changed")
		}
		if _, err := e.qaRegisteredFile(run.ID, event.ReportPath); err != nil {
			return report, err
		}
		for _, file := range report.Files {
			if err := qaFileMatches(root, file); err != nil {
				return report, err
			}
			if _, err := e.qaRegisteredFile(run.ID, file.Path); err != nil {
				return report, err
			}
		}
		candidate, err := taskCandidate(root, report.SourceExclusions, report.SourceExecutables)
		if err != nil {
			return report, err
		}
		if taskFingerprint(pkg) != report.ContractFingerprint || candidate != report.CandidateAfter {
			return report, fmt.Errorf("task_candidate_stale: contract or source changed; do not resume stale evidence")
		}
		return report, nil
	}
	return TaskRunReport{RunID: run.ID, FeatureSlug: pkg.Slug, Status: "not_started"}, nil
}

// RunTasks executes every node afresh in topological order. Prior checkboxes and
// successes never skip execution. A stale prior invocation is rejected explicitly.
func (e *Engine) RunTasks(ctx context.Context, runID string, opts TaskRunOptions) (TaskRunReport, error) {
	if err := opts.validate(); err != nil {
		return TaskRunReport{}, err
	}
	admission, err := e.CheckImplementAdmission(runID)
	if err != nil {
		return TaskRunReport{}, err
	}
	if admission.Status != "pass" {
		return TaskRunReport{}, fmt.Errorf("task_admission_blocked: %s", strings.Join(admission.ReasonCodes, ", "))
	}
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return TaskRunReport{}, err
	}
	fc, pkg, root, err := e.taskPackage(run, m)
	if err != nil {
		return TaskRunReport{}, err
	}
	dag, err := gate.ParseTaskDAG(pkg.Tasks.Text, gate.DefinedACs(pkg.Truth.Text))
	if err != nil {
		return TaskRunReport{}, err
	}
	commands, verifies := make(map[string][]string, len(dag.Nodes)), make(map[string][]string, len(dag.Nodes))
	exclusions := taskSourceExclusions(root, pkg, dag)
	for _, node := range dag.Nodes {
		if !node.Auto {
			return TaskRunReport{}, fmt.Errorf("task_not_auto: %s requires manual execution", node.ID)
		}
		commands[node.ID], err = taskArgv(root, node.Command)
		if err != nil {
			return TaskRunReport{}, err
		}
		verifies[node.ID], err = taskArgv(root, node.Verify)
		if err != nil {
			return TaskRunReport{}, err
		}
		if _, err := taskEvidencePath(root, node.Evidence, pkg); err != nil {
			return TaskRunReport{}, err
		}
	}
	executables := taskExecutableInputs(root, commands, verifies, opts)
	candidate := func() (string, error) { return taskCandidate(root, exclusions, executables) }
	if _, err := e.TaskStatus(runID); err != nil {
		return TaskRunReport{}, err
	}
	if err := ctx.Err(); err != nil {
		return TaskRunReport{}, err
	}
	if opts.By == "" {
		opts.By = "kernel_task_runner"
	}
	parent := filepath.Join(root, ".beacon", "task-runs", run.ID)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return TaskRunReport{}, err
	}
	lock := filepath.Join(parent, ".running")
	if err := os.Mkdir(lock, 0o700); err != nil {
		return TaskRunReport{}, fmt.Errorf("task_run_locked: %w", err)
	}
	defer os.Remove(lock)
	base, err := os.MkdirTemp(parent, "invocation-")
	if err != nil {
		return TaskRunReport{}, err
	}
	report := TaskRunReport{Schema: "beacon-task-run.v1", RunID: run.ID, FeatureSlug: slug, InvocationID: filepath.Base(base), Workdir: root, Status: "failed", ContractFingerprint: taskFingerprint(pkg), StartedAt: time.Now().UTC().Format(time.RFC3339Nano), ReportPath: filepath.Join(base, "report.json"), SourceExclusions: exclusions, SourceExecutables: executables, ExecutableBaseline: "Project-local executable bytes are candidate inputs, even under excluded output/runtime paths; PATH/system executables outside the project are a trusted baseline. Scripts are trusted code, not a sandbox."}
	report.CandidateBefore, err = candidate()
	if err != nil {
		return report, err
	}
	if err := e.appendEvent(run, EventTaskRun, map[string]any{"status": "running", "invocation_id": report.InvocationID, "report_path": report.ReportPath}); err != nil {
		return report, err
	}
	fingerprint := func() string { return taskFingerprint(e.packageFiles(fc)) }
	verifiedCandidate := ""
	finish := func(cause error) (TaskRunReport, error) {
		report.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		var hashErr error
		report.CandidateAfter, hashErr = candidate()
		cause = errors.Join(cause, hashErr)
		if verifiedCandidate != "" && report.CandidateAfter != verifiedCandidate {
			cause = errors.Join(cause, fmt.Errorf("task_candidate_stale: final verified candidate changed before report"))
		}
		if fingerprint() != report.ContractFingerprint {
			cause = errors.Join(cause, fmt.Errorf("task_contract_stale: requirement contract changed"))
		}
		if cause == nil {
			report.Status = "passed"
		} else {
			report.Detail = cause.Error()
		}
		if err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			h, err := qaHash(path)
			if err != nil {
				return err
			}
			report.Files = append(report.Files, qaFile{Path: path, SHA256: h})
			return nil
		}); err != nil {
			cause = errors.Join(cause, err)
			report.Status = "failed"
			report.Detail = cause.Error()
		}
		if err := writeTaskJSON(report.ReportPath, report); err != nil {
			return report, errors.Join(cause, err)
		}
		if _, err := e.RegisterArtifact(run.ID, "report", report.ReportPath, opts.By); err != nil {
			return report, errors.Join(cause, err)
		}
		h, err := qaHash(report.ReportPath)
		if err != nil {
			return report, errors.Join(cause, err)
		}
		if err := e.appendEvent(run, EventTaskRun, map[string]any{"status": report.Status, "invocation_id": report.InvocationID, "report_path": report.ReportPath, "report_hash": h}); err != nil {
			return report, errors.Join(cause, err)
		}
		return report, cause
	}
	for _, node := range dag.Nodes {
		report.Results = append(report.Results, TaskNodeResult{TaskID: node.ID, Status: "pending"})
	}
	for idx, node := range dag.Nodes {
		result := &report.Results[idx]
		result.Status = "failed"
		for {
			var failure *TaskAttempt
			for _, phase := range []string{"command", "verify"} {
				if err := ctx.Err(); err != nil {
					return finish(err)
				}
				if fingerprint() != report.ContractFingerprint {
					return finish(fmt.Errorf("task_contract_stale: contract changed before %s", node.ID))
				}
				argv := commands[node.ID]
				if phase == "verify" {
					argv = verifies[node.ID]
				}
				dir := filepath.Join(base, node.ID, fmt.Sprintf("attempt-%03d-%s", len(result.Attempts)+1, phase))
				a, err := e.executeTaskAttempt(ctx, run, root, dir, node, phase, argv, nil, opts, fingerprint, candidate)
				result.Attempts = append(result.Attempts, a)
				if err != nil {
					return finish(err)
				}
				if a.Status != "passed" {
					failure = &result.Attempts[len(result.Attempts)-1]
					break
				}
			}
			if failure == nil {
				break
			}
			if failure.Cancelled || failure.TimedOut || strings.Contains(failure.Detail, "task_candidate_stale") || fingerprint() != report.ContractFingerprint || opts.RepairCommand == "" || report.Repairs >= opts.MaxRepairs {
				return finish(fmt.Errorf("task_failed: %s %s: %s (repair budget %d/%d)", node.ID, failure.Phase, failure.Detail, report.Repairs, opts.MaxRepairs))
			}
			repairArgv := append([]string{opts.RepairCommand}, opts.RepairArgs...)
			if !filepath.IsAbs(repairArgv[0]) && strings.ContainsAny(repairArgv[0], `/\\`) {
				repairArgv[0] = filepath.Join(root, repairArgv[0])
			}
			report.Repairs++
			dir := filepath.Join(base, node.ID, fmt.Sprintf("attempt-%03d-repair", len(result.Attempts)+1))
			a, err := e.executeTaskAttempt(ctx, run, root, dir, node, "repair", repairArgv, failure, opts, fingerprint, candidate)
			result.Attempts = append(result.Attempts, a)
			if err != nil {
				return finish(err)
			}
			if a.Status != "passed" {
				return finish(fmt.Errorf("task_repair_failed: %s: %s", node.ID, a.Detail))
			}
		}
		snapshot, err := e.snapshotTaskEvidence(run.ID, root, filepath.Join(base, node.ID, "evidence-initial"), node.Evidence, pkg, opts.By)
		if err != nil {
			return finish(fmt.Errorf("task_evidence_missing: %s: %w", node.ID, err))
		}
		result.Evidence, result.Status = snapshot, "passed"
	}
	// A later command/repair can invalidate an earlier node. Every Verify must
	// pass again on one final candidate; none of these attempts may alter it.
	finalCandidate, err := candidate()
	if err != nil {
		return finish(err)
	}
	verifiedCandidate = finalCandidate
	for idx, node := range dag.Nodes {
		result := &report.Results[idx]
		if err := ctx.Err(); err != nil {
			result.Status = "failed"
			return finish(err)
		}
		if fingerprint() != report.ContractFingerprint {
			result.Status = "failed"
			return finish(fmt.Errorf("task_contract_stale: contract changed before final verification"))
		}
		dir := filepath.Join(base, node.ID, fmt.Sprintf("attempt-%03d-final-verify", len(result.Attempts)+1))
		a, err := e.executeTaskAttempt(ctx, run, root, dir, node, "final-verify", verifies[node.ID], nil, opts, fingerprint, candidate)
		result.Attempts = append(result.Attempts, a)
		if err != nil {
			result.Status = "failed"
			return finish(err)
		}
		if a.Status != "passed" || a.CandidateBefore != finalCandidate || a.CandidateAfter != finalCandidate {
			result.Status = "failed"
			return finish(fmt.Errorf("task_final_verify_failed: %s: %s", node.ID, a.Detail))
		}
		snapshot, err := e.snapshotTaskEvidence(run.ID, root, filepath.Join(base, node.ID, "evidence-final"), node.Evidence, pkg, opts.By)
		if err != nil {
			result.Status = "failed"
			return finish(err)
		}
		result.Evidence = snapshot
	}
	return finish(ctx.Err())
}

func taskEvidencePath(root, declared string, pkg gate.PackageFiles) (string, error) {
	p := filepath.Clean(declared)
	if filepath.IsAbs(p) || p == "." || p == ".." || strings.HasPrefix(p, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("task_evidence_invalid: evidence must be project-relative")
	}
	abs := filepath.Join(root, p)
	for _, f := range []gate.PackageFile{pkg.Truth, pkg.Tests, pkg.Tasks, pkg.Restatement} {
		if abs == resolvePkgPath(root, f.Path) {
			return "", fmt.Errorf("task_evidence_invalid: contract documents are not execution evidence")
		}
	}
	// Resolve existing ancestors too, so a not-yet-created file cannot escape via a symlink directory.
	for parent := abs; ; parent = filepath.Dir(parent) {
		real, err := filepath.EvalSymlinks(parent)
		if err == nil {
			rel, err := filepath.Rel(root, real)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return "", fmt.Errorf("task_evidence_invalid: evidence escapes workdir")
			}
			break
		}
		if !os.IsNotExist(err) || filepath.Dir(parent) == parent {
			return "", err
		}
	}
	return abs, nil
}

// Contract members are bound separately by taskFingerprint. Exclusions are exact
// package/output paths, never a global directory-name heuristic for docs/evidence.
func taskSourceExclusions(root string, pkg gate.PackageFiles, dag gate.TaskDAG) []string {
	var paths []string
	for _, file := range []gate.PackageFile{pkg.Truth, pkg.Tests, pkg.Tasks, pkg.Restatement, pkg.Evidence} {
		if file.Path != "" {
			paths = append(paths, resolvePkgPath(root, file.Path))
		}
	}
	for _, node := range dag.Nodes {
		paths = append(paths, filepath.Join(root, node.Evidence))
	}
	return paths
}

func (e *Engine) snapshotTaskEvidence(runID, root, dir, declared string, pkg gate.PackageFiles, by string) (string, error) {
	source, err := taskEvidencePath(root, declared, pkg)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(source)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("task_evidence_invalid: evidence must be a regular file")
	}
	raw, err := os.ReadFile(source)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, filepath.Base(source))
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	_, writeErr := f.Write(raw)
	closeErr := f.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return "", err
	}
	if _, err := e.RegisterArtifact(runID, "evidence", path, by); err != nil {
		return "", err
	}
	return path, nil
}
