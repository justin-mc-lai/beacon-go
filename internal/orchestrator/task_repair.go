package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"beacon-go/internal/gate"
	"beacon-go/internal/harness"
	"beacon-go/internal/store"
)

const MaxTaskRepairs = 5

// TaskRunOptions configures real local subprocesses, not a container sandbox.
// MaxRepairs bounds the entire invocation, not each individual task.
type TaskRunOptions struct {
	By            string
	RepairCommand string
	RepairArgs    []string
	MaxRepairs    int
	Timeout       time.Duration
}

func (o TaskRunOptions) validate() error {
	if o.MaxRepairs < 0 || o.MaxRepairs > MaxTaskRepairs {
		return fmt.Errorf("task_repair_invalid: max repairs must be between 0 and %d", MaxTaskRepairs)
	}
	if o.Timeout < 0 {
		return fmt.Errorf("task_repair_invalid: timeout must not be negative")
	}
	if o.RepairCommand == "" {
		if len(o.RepairArgs) > 0 || o.MaxRepairs > 0 {
			return fmt.Errorf("task_repair_invalid: repair bounds/arguments require an explicit repair program")
		}
		return nil
	}
	if strings.TrimSpace(o.RepairCommand) != o.RepairCommand || strings.ContainsAny(o.RepairCommand, "\x00\n\r") {
		return fmt.Errorf("task_repair_invalid: repair-cmd must name one executable, not a command line")
	}
	return validateTaskArgv(append([]string{o.RepairCommand}, o.RepairArgs...))
}

// validateTaskArgv rejects deployment/install lanes and explicit shell evaluation.
// Executable scripts remain trusted user code; this is workdir isolation only.
func validateTaskArgv(argv []string) error {
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return fmt.Errorf("task_command_invalid: empty argv")
	}
	program := strings.ToLower(filepath.Base(argv[0]))
	switch program {
	case "sh", "bash", "zsh", "dash", "fish", "csh", "ksh", "cmd", "cmd.exe", "powershell", "pwsh", "env", "sudo", "doas":
		return fmt.Errorf("task_command_forbidden: shell/wrapper program %s", program)
	}
	switch program {
	case "npm", "npm.cmd", "pnpm", "pnpm.cmd", "yarn", "yarn.cmd", "bun":
		for _, arg := range argv[1:] {
			switch strings.ToLower(arg) {
			case "i", "ci", "add", "install", "install-test", "it":
				return fmt.Errorf("task_command_forbidden: package installation alias %s %s", program, arg)
			}
		}
	}
	for _, arg := range argv {
		switch strings.ToLower(filepath.Base(arg)) {
		case "release", "install", "uninstall", "deploy", "publish", "push", "merge":
			return fmt.Errorf("task_command_forbidden: release/install operation %s", arg)
		}
	}
	return nil
}

func taskArgv(root, command string) ([]string, error) {
	argv, err := SplitCommandWords(command)
	if err != nil {
		return nil, err
	}
	if err := validateTaskArgv(argv); err != nil {
		return nil, err
	}
	// LookPath runs before the harness sets cwd, so anchor explicit relative programs.
	if !filepath.IsAbs(argv[0]) && strings.ContainsAny(argv[0], `/\\`) {
		argv[0] = filepath.Join(root, argv[0])
	}
	return argv, nil
}

// taskSourceCandidate hashes every project input, without an extension allowlist.
// Only explicit runtime directory components and exact contract/output paths are
// excluded. PATH/system executables outside the project remain a trusted baseline;
// local scripts are trusted code, not a sandbox, and their bytes are hashed here.
func taskSourceCandidate(root string, exclusions ...string) (string, error) {
	var files []qaFile
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		for _, excluded := range exclusions {
			if path == resolvePkgPath(root, excluded) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if d.IsDir() {
			if path != root {
				switch d.Name() {
				case ".git", ".beacon", ".beacon-worktrees", "node_modules", "vendor":
					return filepath.SkipDir
				}
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("task_candidate_invalid: symlink source %s", path)
		}
		h, err := qaHash(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, qaFile{Path: filepath.ToSlash(rel), SHA256: h})
		return nil
	})
	if err != nil {
		return "", err
	}
	head := ""
	if harness.IsGitRepo(context.Background(), root) {
		head, err = harness.HeadCommit(context.Background(), root)
		if err != nil {
			return "", err
		}
	}
	raw, err := json.Marshal(struct {
		Commit string   `json:"source_commit"`
		Files  []qaFile `json:"source_files"`
	}{head, files})
	if err != nil {
		return "", err
	}
	return digestOf(string(raw)), nil
}

func (e *Engine) executeTaskAttempt(ctx context.Context, run store.Run, root, dir string, node gate.TaskNode, phase string, argv []string, failure *TaskAttempt, opts TaskRunOptions, fingerprint func() string, candidate func() (string, error)) (TaskAttempt, error) {
	a := TaskAttempt{TaskID: node.ID, Phase: phase, Argv: append([]string(nil), argv...), Status: "failed", ExitCode: -1, ContractBefore: fingerprint()}
	var err error
	a.CandidateBefore, err = candidate()
	if err != nil {
		return a, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return a, err
	}
	a.ResultPath = filepath.Join(dir, "result.json")
	a.StdoutPath = filepath.Join(dir, "stdout.log")
	a.StderrPath = filepath.Join(dir, "stderr.log")
	env := harness.IterationEnv(harness.IterationRequest{RunID: run.ID, FeatureID: run.FeatureID, Stage: StageImplement})
	env = append(env, "BEACON_TASK_ID="+node.ID, "BEACON_PROJECT_ROOT="+root, "BEACON_TASK_PHASE="+phase,
		"BEACON_TASK_CANDIDATE_HASH="+a.CandidateBefore, "BEACON_CONTRACT_FINGERPRINT="+a.ContractBefore,
		"BEACON_TASK_RESULT_PATH="+a.ResultPath)
	if failure != nil {
		env = append(env, "BEACON_FAILURE_CONTEXT="+failure.ResultPath, "BEACON_FAILURE_STDOUT="+failure.StdoutPath, "BEACON_FAILURE_STDERR="+failure.StderrPath)
	}
	if (phase == "verify" || phase == "final-verify") && len(argv) > 1 && filepath.Base(argv[0]) == "go" && argv[1] == "test" {
		argv = taskGoVerificationArgv(argv)
		a.Argv = argv
	}
	out := harness.RunCommand(ctx, argv, root, env, opts.Timeout)
	a.ExitCode, a.TimedOut, a.DurationMS = out.ExitCode, out.TimedOut, out.Duration.Milliseconds()
	switch {
	case ctx.Err() != nil:
		a.Detail = ctx.Err().Error()
		a.Cancelled = true
	case out.TimedOut:
		a.Detail = "task command timed out"
	case out.Err != nil:
		a.Detail = out.Err.Error()
	case out.ExitCode != 0:
		a.Detail = "task command exited non-zero"
	default:
		a.Status = "passed"
	}
	if a.Status == "passed" && (phase == "verify" || phase == "final-verify") && len(argv) > 1 && filepath.Base(argv[0]) == "go" && argv[1] == "test" {
		execution, parseErr := parseQAGoVerbose(out.Stdout)
		if parseErr != nil || execution.TestCount == 0 {
			a.Status, a.Detail = "failed", "task_tests_not_executed: Go verification must run non-skipped tests"
		}
	}
	a.ContractAfter = fingerprint()
	a.CandidateAfter, err = candidate()
	if err != nil {
		a.Status, a.Detail = "failed", err.Error()
	}
	if a.ContractAfter != a.ContractBefore {
		a.Status, a.Detail = "failed", "task_contract_stale: contract changed during execution"
	}
	if (phase == "verify" || phase == "final-verify") && a.CandidateAfter != a.CandidateBefore {
		a.Status, a.Detail = "failed", "task_candidate_stale: verification changed its source candidate"
	}
	for _, f := range []struct{ path, content string }{{a.StdoutPath, out.Stdout}, {a.StderrPath, out.Stderr}} {
		if err := os.WriteFile(f.path, []byte(f.content), 0o644); err != nil {
			return a, err
		}
		if _, err := e.RegisterArtifact(run.ID, "log", f.path, opts.By); err != nil {
			return a, err
		}
	}
	if err := writeTaskJSON(a.ResultPath, a); err != nil {
		return a, err
	}
	if _, err := e.RegisterArtifact(run.ID, "report", a.ResultPath, opts.By); err != nil {
		return a, err
	}
	if err := e.appendEvent(run, EventTaskAttempt, map[string]any{"task_id": a.TaskID, "phase": a.Phase, "status": a.Status, "result_path": a.ResultPath, "candidate_before": a.CandidateBefore, "candidate_after": a.CandidateAfter, "contract_before": a.ContractBefore, "contract_after": a.ContractAfter}); err != nil {
		return a, err
	}
	return a, nil
}

func writeTaskJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	// Every path belongs to a fresh invocation; never overwrite registered evidence.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(raw)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func taskExecutableInputs(root string, commands, verifies map[string][]string, opts TaskRunOptions) []string {
	seen := map[string]bool{}
	add := func(program string) {
		if program == "" {
			return
		}
		if !filepath.IsAbs(program) && strings.ContainsAny(program, `/\\`) {
			program = filepath.Join(root, program)
		}
		if !filepath.IsAbs(program) {
			resolved, err := exec.LookPath(program)
			if err != nil {
				return
			} // Missing programs are captured as failed attempts.
			program = resolved
		}
		rel, err := filepath.Rel(root, program)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			seen[program] = true
		}
	}
	for _, argv := range commands {
		add(argv[0])
	}
	for _, argv := range verifies {
		add(argv[0])
	}
	add(opts.RepairCommand)
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func taskCandidate(root string, exclusions, executables []string) (string, error) {
	source, err := taskSourceCandidate(root, exclusions...)
	if err != nil {
		return "", err
	}
	parts := []string{source}
	for _, executable := range executables {
		h, err := qaHash(executable)
		if os.IsNotExist(err) {
			h, err = "missing", nil
		}
		if err != nil {
			return "", err
		}
		parts = append(parts, executable, h)
	}
	return digestOf(parts...), nil
}

func taskGoVerificationArgv(argv []string) []string {
	out := make([]string, 0, len(argv)+2)
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if arg == "-args" {
			out = append(out, "-v", "-count=1")
			return append(out, argv[i:]...)
		}
		if i > 1 && (arg == "-json" || strings.HasPrefix(arg, "-json=") || strings.HasPrefix(arg, "-count=") || strings.HasPrefix(arg, "-v=")) {
			continue
		}
		if i > 1 && arg == "-count" && i+1 < len(argv) {
			i++
			continue
		}
		out = append(out, arg)
	}
	return append(out, "-v", "-count=1")
}
