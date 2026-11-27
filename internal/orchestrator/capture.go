package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"beacon-go/internal/harness"
	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
	"beacon-go/internal/version"
)

func captureError(code, detail string) error { return qaError(code, detail) }

func captureFile(root, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", captureError("capture_path_invalid", "blank path")
	}
	p := path
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	p, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", captureError("capture_path_invalid", path)
	}
	i, err := os.Stat(p)
	if err != nil || !i.Mode().IsRegular() {
		return "", captureError("capture_path_invalid", path)
	}
	return p, nil
}

func captureTestHash(root string, names []string) (string, error) {
	return intenttree.CaptureTestHash(root, names)
}

// Source snapshot includes all project input files except canonical bookkeeping
// and runtime output. Exclusions are fixed here, never caller-controlled.
func captureSourceHash(root string) (string, error) {
	return intenttree.CaptureSourceHash(root)
}

func normalizeCaptureCommand(spec CaptureCommand) ([]string, []string, error) {
	bad := func() ([]string, []string, error) {
		return nil, nil, captureError("capture_command_invalid", "requires bound go test, local package paths, optional -run and -timeout")
	}
	if spec.Schema == "beacon-capture-command.v1" && spec.Adapter == "custom-json" {
		if len(spec.Argv) == 0 || !strings.HasPrefix(spec.Argv[0], "./") || len(spec.RequiredTests) == 0 || len(spec.TestFiles) == 0 {
			return bad()
		}
		seen := map[string]bool{}
		for _, name := range spec.RequiredTests {
			if strings.TrimSpace(name) == "" || seen[name] {
				return bad()
			}
			seen[name] = true
		}
		for _, arg := range spec.Argv {
			if strings.TrimSpace(arg) == "" {
				return bad()
			}
		}
		return append([]string{}, spec.Argv...), nil, nil
	}
	if spec.Adapter != "" && spec.Adapter != "go-json" {
		return bad()
	}
	if spec.Schema != "beacon-capture-command.v1" || len(spec.Argv) < 3 || spec.Argv[0] != "go" || spec.Argv[1] != "test" || len(spec.RequiredTests) == 0 {
		return bad()
	}
	args := []string{"go", "test", "-json", "-count=1"}
	var packages []string
	for i := 2; i < len(spec.Argv); i++ {
		a := spec.Argv[i]
		if a == "-run" || a == "-timeout" {
			if i+1 >= len(spec.Argv) || strings.TrimSpace(spec.Argv[i+1]) == "" {
				return bad()
			}
			args = append(args, a, spec.Argv[i+1])
			i++
			continue
		}
		if a != "." && !strings.HasPrefix(a, "./") {
			return bad()
		}
		if strings.Contains(a, "..") || strings.ContainsAny(a, ";\n\r") {
			return bad()
		}
		packages = append(packages, a)
		args = append(args, a)
	}
	seen := map[string]bool{}
	for _, name := range spec.RequiredTests {
		if strings.TrimSpace(name) == "" || seen[name] {
			return bad()
		}
		seen[name] = true
	}
	if len(packages) == 0 {
		return bad()
	}
	return args, packages, nil
}

func captureCheckPackages(ctx context.Context, root string, packages, files []string, timeout time.Duration, observed *intenttree.CapturePreflight) error {
	argv := append([]string{"go", "list", "-e", "-json"}, packages...)
	out := harness.RunCommand(ctx, argv, root, captureEnv(), timeout)
	*observed = intenttree.CapturePreflight{Command: argv, ExitCode: out.ExitCode, Stdout: out.Stdout, Stderr: out.Stderr, TimedOut: out.TimedOut}
	if out.Err != nil {
		observed.Error = out.Err.Error()
	}
	if out.Err != nil || out.ExitCode != 0 || out.TimedOut {
		if out.TimedOut {
			return capturePreflightTimeout{}
		}
		return captureError("capture_package_invalid", out.Stderr)
	}
	dec := json.NewDecoder(strings.NewReader(out.Stdout))
	expected := map[string]bool{}
	for dec.More() {
		var p struct {
			Dir                       string
			TestGoFiles, XTestGoFiles []string
		}
		if err := dec.Decode(&p); err != nil {
			return err
		}
		for _, file := range append(p.TestGoFiles, p.XTestGoFiles...) {
			path, err := captureFile(root, filepath.Join(p.Dir, file))
			if err != nil {
				return err
			}
			expected[path] = true
		}
	}
	if len(expected) != len(files) {
		return captureError("capture_test_files_invalid", "test_files must cover every selected package test file")
	}
	for _, file := range files {
		p, err := captureFile(root, file)
		if err != nil {
			return err
		}
		if !expected[p] {
			return captureError("capture_test_files_invalid", file)
		}
	}
	return nil
}

type capturePreflightTimeout struct{}

func (capturePreflightTimeout) Error() string { return "capture_package_invalid: preflight timed out" }

func captureEnv() []string {
	var env []string
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if key != "GOFLAGS" && key != "GOWORK" {
			env = append(env, item)
		}
	}
	return append(env, "GOFLAGS=", "GOWORK=off")
}

// captureGoVerdict requires observed run + terminal events for every contracted
// test and rejects package-only failures, skipped or incomplete executions.
func captureGoVerdict(stdout, phase string, exit int, required []string) error {
	started := map[string]bool{}
	terminal := map[string]string{}
	failed := 0
	packageFailures := map[string]bool{}
	testFailures := map[string]bool{}
	for _, line := range strings.Split(stdout, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var ev qaGoEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			return captureError("capture_result_invalid", "not Go JSON")
		}
		if strings.Contains(ev.Output, "[build failed]") || strings.Contains(ev.Output, "(cached)") {
			return captureError("capture_result_invalid", "build failure/cached result")
		}
		if ev.Test == "" {
			if ev.Action == "fail" {
				packageFailures[ev.Package] = true
			}
			continue
		}
		key := ev.Package + "\x00" + ev.Test
		switch ev.Action {
		case "run":
			if started[key] {
				return captureError("capture_result_invalid", "duplicate run")
			}
			started[key] = true
		case "pass", "fail", "skip":
			if !started[key] || terminal[key] != "" {
				return captureError("capture_result_invalid", "terminal without run")
			}
			terminal[key] = ev.Action
			if ev.Action == "skip" {
				return captureError("capture_result_invalid", "skipped test")
			}
			if ev.Action == "fail" {
				failed++
				testFailures[ev.Package] = true
			}
		}
	}
	for key := range started {
		if terminal[key] == "" {
			return captureError("capture_result_invalid", "incomplete test")
		}
	}
	requiredFailed := false
	for _, name := range required {
		found := false
		for key := range terminal {
			if strings.HasSuffix(key, "\x00"+name) {
				found = true
				if terminal[key] == "fail" {
					requiredFailed = true
				}
			}
		}
		if !found {
			return captureError("capture_result_invalid", "required test not executed: "+name)
		}
	}
	for pkg := range packageFailures {
		if !testFailures[pkg] {
			return captureError("capture_result_invalid", "package failed without observed test failure")
		}
	}
	if len(terminal) == 0 || (phase == "red" && (exit != 1 || failed == 0 || !requiredFailed)) || (phase == "green" && (exit != 0 || failed != 0)) {
		return captureError("capture_result_invalid", "expected behavior Red or all passing Green")
	}
	return nil
}

func (e *Engine) CaptureTest(ctx context.Context, req CaptureRequest) (CaptureResult, error) {
	var res CaptureResult
	if strings.TrimSpace(req.By) == "" || (req.Phase != "red" && req.Phase != "green") || req.Timeout <= 0 {
		return res, captureError("capture_request_invalid", "identity, phase and positive timeout required")
	}
	if req.Phase == "green" && strings.TrimSpace(req.RedID) == "" {
		return res, captureError("capture_red_required", "explicit Red artifact ID required")
	}
	if req.Phase == "red" && req.RedID != "" {
		return res, captureError("capture_request_invalid", "Red cannot reference Red")
	}
	root, err := filepath.EvalSymlinks(req.Root)
	if err != nil {
		return res, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return res, err
	}
	tree, err := filepath.EvalSymlinks(req.TreeDir)
	if err != nil {
		return res, err
	}
	tree, err = filepath.Abs(tree)
	if err != nil {
		return res, err
	}
	if filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(tree)))) != root {
		return res, captureError("capture_scope_invalid", "tree does not belong to project")
	}
	req.By = strings.TrimSpace(req.By)
	n, err := intenttree.ReadNode(req.TreeDir, req.NodeID)
	if err != nil {
		return res, err
	}
	if n.Kind != intenttree.KindTouchpoint || (n.Status != intenttree.StatusFrozen && n.Status != intenttree.StatusImplemented) || n.Verification == nil {
		return res, captureError("capture_node_invalid", "frozen/implemented touchpoint with tests required")
	}
	found := false
	for _, ac := range n.ACs {
		if ac.ID == req.ACID {
			found = true
		}
	}
	if !found {
		return res, captureError("capture_ac_invalid", req.ACID)
	}
	var spec CaptureCommand
	if err := json.Unmarshal([]byte(n.Verification.Spec), &spec); err != nil {
		return res, captureError("capture_command_invalid", err.Error())
	}
	argv, packages, err := normalizeCaptureCommand(spec)
	if err != nil {
		return res, err
	}
	if spec.Adapter == "custom-json" {
		p, e := filepath.Abs(filepath.Join(root, argv[0]))
		if e != nil {
			return res, e
		}
		// Canonicalize once before both pair comparison and execution.
		if real, resolveErr := captureFile(root, p); resolveErr == nil {
			p = real
		}
		argv[0] = p
	}
	testHash, err := captureTestHash(root, spec.TestFiles)
	if err != nil {
		return res, err
	}
	fp, err := intenttree.ContractFingerprint(n)
	if err != nil {
		return res, err
	}
	if req.Phase == "green" {
		red, err := e.ReadCapture(req.NodeID, req.RedID)
		if err != nil {
			return res, err
		}
		a, _ := json.Marshal(red.Command)
		b, _ := json.Marshal(argv)
		if red.Phase != "red" || red.Status != "accepted" || red.ACID != req.ACID || red.By != req.By || red.Fingerprint != fp || red.TestSHA256 != testHash || string(a) != string(b) {
			return res, captureError("capture_pair_invalid", "Red scope/contract/tests/command/maker differs")
		}
	}
	source, err := captureSourceHash(root)
	if err != nil {
		return res, err
	}
	base := filepath.Join(root, ".beacon", "captures")
	if err := os.MkdirAll(base, 0700); err != nil {
		return res, err
	}
	dir, err := os.MkdirTemp(base, "attempt-")
	if err != nil {
		return res, err
	}
	res = CaptureResult{Schema: "beacon-capture.v1", NodeID: n.ID, ACID: req.ACID, Phase: req.Phase, By: req.By, RedID: req.RedID, Fingerprint: fp, Command: argv, TestSHA256: testHash, SourceSHA256: source, StartedAt: time.Now().UTC().Format(time.RFC3339Nano), Status: "rejected", ResultPath: filepath.Join(dir, "result.json"), StdoutPath: filepath.Join(dir, "stdout.log"), StderrPath: filepath.Join(dir, "stderr.log")}
	commit, _ := qaGit(root, "rev-parse", "HEAD")
	res.SourceCommit = strings.TrimSpace(commit)
	res.BinarySHA256 = version.Runtime().BinarySHA
	res.Workdir = root
	res.TestFiles = append([]string{}, spec.TestFiles...)
	var preflightErr error
	if spec.Adapter == "custom-json" {
		executable, resolveErr := captureFile(root, argv[0])
		preflightErr = resolveErr
		bound := false
		for _, name := range spec.TestFiles {
			p, e := captureFile(root, name)
			if e == nil && p == executable {
				bound = true
			}
		}
		if preflightErr == nil && !bound {
			preflightErr = captureError("capture_test_files_invalid", "custom executable must be hashed test input")
		}
		if preflightErr == nil {
			argv = append([]string{executable}, argv[1:]...)
			res.Command = argv
		}
	} else {
		res.Preflight = &intenttree.CapturePreflight{}
		preflightErr = captureCheckPackages(ctx, root, packages, spec.TestFiles, req.Timeout, res.Preflight)
	}
	out := harness.IterationOutput{ExitCode: -1}
	if preflightErr == nil {
		env := append(captureEnv(), "BEACON_CAPTURE_RESULT="+res.ResultPath, "BEACON_CAPTURE_NODE="+n.ID, "BEACON_CAPTURE_AC="+req.ACID, "BEACON_CAPTURE_PHASE="+req.Phase)
		out = harness.RunCommand(ctx, argv, root, env, req.Timeout)
	} else {
		out.Err = preflightErr
		// No test process started. Keep its exit -1; the separate preflight
		// object records the actual go-list command, output and exit status.
		if res.Preflight != nil {
			out.Stdout = res.Preflight.Stdout
			out.Stderr = res.Preflight.Stderr
		}
	}
	res.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	res.ExitCode = out.ExitCode
	res.TimedOut = out.TimedOut
	var preflightTimeout capturePreflightTimeout
	if errors.As(preflightErr, &preflightTimeout) {
		res.TimedOut = true
	}
	res.Cancelled = ctx.Err() != nil
	runErr := out.Err
	if out.TimedOut || res.Cancelled {
		runErr = captureError("capture_interrupted", "timeout/cancellation")
	}
	if runErr == nil {
		if spec.Adapter == "custom-json" {
			runErr = captureCustomVerdict(out.Stdout, res, spec.RequiredTests)
		} else {
			runErr = captureGoVerdict(out.Stdout, req.Phase, out.ExitCode, spec.RequiredTests)
		}
	}
	after, hashErr := captureTestHash(root, spec.TestFiles)
	if hashErr != nil || after != testHash {
		runErr = captureError("capture_input_changed", "test inputs changed")
	}
	current, nodeErr := intenttree.ReadNode(req.TreeDir, n.ID)
	if nodeErr != nil {
		runErr = nodeErr
	} else {
		currentFP, _ := intenttree.ContractFingerprint(current)
		if currentFP != fp {
			runErr = captureError("capture_input_changed", "requirement changed")
		}
	}
	res.SourceAfter, err = captureSourceHash(root)
	if err != nil {
		runErr = err
	} else if res.SourceAfter != source {
		runErr = captureError("capture_input_changed", "project inputs changed during execution")
	}
	if runErr == nil {
		res.Status = "accepted"
	} else {
		res.Reason = runErr.Error()
	}
	for path, body := range map[string]string{res.StdoutPath: out.Stdout, res.StderrPath: out.Stderr} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			return res, err
		}
		if _, err := e.RegisterScopedArtifact("node", n.ID, "log", path, req.By); err != nil {
			return res, err
		}
	}
	res.StdoutSHA256, _ = qaHash(res.StdoutPath)
	res.StderrSHA256, _ = qaHash(res.StderrPath)
	raw, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return res, err
	}
	if err := os.WriteFile(res.ResultPath, raw, 0600); err != nil {
		return res, err
	}
	reg, _, err := e.registerArtifactScopedCaptured(storeScope(n.ID), "capture_result", res.ResultPath, req.By, true)
	if err != nil {
		return res, err
	}
	res.ArtifactID = reg.Artifact.ID
	return res, runErr
}

func storeScope(id string) string { return store.ScopeRunID("node", id) }

// ReadCapture accepts only reserved, node-scoped kernel capture artifacts.
func (e *Engine) ReadCapture(nodeID, id string) (CaptureResult, error) {
	return intenttree.ReadCaptureEvidence(e.store, nodeID, id)
}
