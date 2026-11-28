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
	seenPackages := map[string]bool{}
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
		key := filepath.Clean(a)
		if seenPackages[key] {
			return bad()
		}
		seenPackages[key] = true
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
	discovered, err := discoverCaptureTests(ctx, root, packages, timeout, observed)
	if err != nil {
		return err
	}
	expected := map[string]bool{}
	for _, file := range discovered {
		expected[filepath.Join(root, file)] = true
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
		delete(expected, p)
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
	if req.Baseline != "" && req.Phase != "red" {
		return res, captureError("capture_baseline_invalid", "baseline only allowed for Red")
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
	if spec.DiscoverTests {
		if spec.Adapter == "custom-json" || len(spec.TestFiles) != 0 {
			return res, captureError("capture_command_invalid", "discovery is Go-only and cannot also declare test_files")
		}
	}
	// Allocate before discovery so even the first failed process has one receipt.
	fp, err := intenttree.ContractFingerprint(n)
	if err != nil {
		return res, err
	}
	liveSource, err := captureSourceHash(root)
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
	res = CaptureResult{Schema: "beacon-capture.v1", NodeID: n.ID, ACID: req.ACID, Phase: req.Phase, By: req.By, RedID: req.RedID, Fingerprint: fp, Command: argv, StartedAt: time.Now().UTC().Format(time.RFC3339Nano), Status: "rejected", ResultPath: filepath.Join(dir, "result.json"), StdoutPath: filepath.Join(dir, "stdout.log"), StderrPath: filepath.Join(dir, "stderr.log"), Workdir: root, BinarySHA256: version.Runtime().BinarySHA}
	if spec.Adapter != "custom-json" {
		res.Preflight = &intenttree.CapturePreflight{}
		if spec.DiscoverTests {
			spec.TestFiles, err = discoverCaptureTests(ctx, root, packages, req.Timeout, res.Preflight)
		} else {
			err = captureCheckPackages(ctx, root, packages, spec.TestFiles, req.Timeout, res.Preflight)
		}
		if err != nil {
			res.ExitCode = -1
			res.TimedOut = res.Preflight.TimedOut
			res.Cancelled = ctx.Err() != nil
			res.Reason = err.Error()
			res.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
			return e.persistCapture(res, res.Preflight.Stdout, res.Preflight.Stderr, err)
		}
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
	liveRoot := root
	replayCandidate := ""
	if req.Baseline != "" {
		if spec.Adapter == "custom-json" {
			return res, captureError("capture_baseline_invalid", "historical replay currently requires Go tests")
		}
		var cleanup func()
		root, replayCandidate, cleanup, err = prepareCaptureReplay(ctx, liveRoot, req.Baseline, spec.TestFiles)
		if err != nil {
			res.ExitCode = -1
			res.TestFiles = append([]string{}, spec.TestFiles...)
			res.TestSHA256 = testHash
			res.SourceSHA256 = liveSource
			res.Reason = err.Error()
			res.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
			return e.persistCapture(res, "", "", err)
		}
		defer cleanup()
		// Resolve the same relative test set inside the historical snapshot.
		var relative []string
		for _, name := range spec.TestFiles {
			p, e := captureFile(liveRoot, name)
			if e != nil {
				return res, e
			}
			rel, e := filepath.Rel(liveRoot, p)
			if e != nil {
				return res, e
			}
			relative = append(relative, rel)
		}
		spec.TestFiles = relative
		if replayHash, e := captureTestHash(root, relative); e != nil || replayHash != testHash {
			return res, captureError("capture_baseline_invalid", "overlay test hash differs")
		}
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
		if red.BaselineCommit != "" {
			head, e := qaGit(liveRoot, "rev-parse", "HEAD")
			if e != nil || strings.TrimSpace(head) != red.ReplayCandidate {
				return res, captureError("capture_pair_invalid", "candidate commit differs from replay target")
			}
			if liveSource != red.ReplayTargetSHA256 {
				return res, captureError("capture_pair_invalid", "live replay target inputs changed")
			}
		}
	}
	source, err := captureSourceHash(root)
	if err != nil {
		return res, err
	}
	if req.Baseline == "" {
		source = liveSource
	}
	res.Command, res.TestSHA256, res.SourceSHA256 = argv, testHash, source
	commit, _ := qaGit(root, "rev-parse", "HEAD")
	res.SourceCommit = strings.TrimSpace(commit)
	if req.Baseline != "" {
		res.SourceCommit = req.Baseline
		res.BaselineCommit = req.Baseline
		res.ReplayCandidate = replayCandidate
		res.ReplayTargetSHA256 = liveSource
		res.ReplayOverlay = append([]string{}, spec.TestFiles...)
		res.ReplayDeleted = []string{}
		res.ExecutionRoot = root
	}
	res.BinarySHA256 = version.Runtime().BinarySHA
	res.Workdir = liveRoot
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
	if req.Baseline != "" {
		after, e := captureSourceHash(liveRoot)
		if e != nil || after != liveSource {
			runErr = captureError("capture_input_changed", "live replay target changed during execution")
		}
	}
	if runErr == nil {
		res.Status = "accepted"
	} else {
		res.Reason = runErr.Error()
	}
	return e.persistCapture(res, out.Stdout, out.Stderr, runErr)
}

func (e *Engine) persistCapture(res CaptureResult, stdout, stderr string, runErr error) (CaptureResult, error) {
	for path, body := range map[string]string{res.StdoutPath: stdout, res.StderrPath: stderr} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			return res, err
		}
		if _, err := e.RegisterScopedArtifact("node", res.NodeID, "log", path, res.By); err != nil {
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
	reg, _, err := e.registerArtifactScopedCaptured(storeScope(res.NodeID), "capture_result", res.ResultPath, res.By, true)
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
