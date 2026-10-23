// Auto-test execution (R2c, parity audit H8): the missing EXECUTION
// half of QA. Before this slice the qa gate could only JUDGE
// evidence that a human had registered by hand; nothing in the Go
// kernel ever ran the tests. This is the port of Python
// commands/auto_test.py's core chain, scoped to command-line tests:
//
//	discover -> execute -> record per-TC evidence -> register
//
// Discovery: the feature package's tests.md coverage matrix
// (gate.ParseCoverageRows — the same rows integrity judges), one
// command per TC. A TC row whose command cell is empty falls back
// to the project's configured test command (.beacon/config key
// `test_command`, or the `autotest_command` project config alias),
// then to the generated qa9 strategy's default command
// (.beacon/qa-strategy.json, R2d);
// a TC with no command anywhere is reported with Python's own code
// qa9_command_missing_auto_case, never silently skipped. Rows with
// a `BLOCKED:` escape are manual by declaration and reported as
// blocked without executing.
//
// Execution: harness.RunCommand — argv only, never a shell (shell
// syntax in a command cell is refused with
// autotest_shell_syntax_unsupported rather than misrun), the whole
// process group is killed on timeout. Browser/extension cases of
// the Python original are out of scope (audit L13, external host
// conditions); a tests.md that names one is just a command like any
// other and will fail honestly if its runner is absent.
//
// Evidence: per-TC result files under .beacon/autotest/<run-id>/
// (result.json + stdout.log + stderr.log, mirroring the Python
// layer dirs), each registered through the SAME artifact channel
// humans use (kind test_result), plus one aggregate report
// registered as test_results — so the formal qa gate consumes
// auto-test evidence with zero special-casing.
package orchestrator

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/harness"
)

// EventAutoTestExecuted is appended once per RunAutoTest call with
// the pass/fail/blocked tallies.
const EventAutoTestExecuted = "autotest_executed"

// Auto-test per-TC outcome statuses.
const (
	AutoTestPassed  = "passed"
	AutoTestFailed  = "failed"
	AutoTestBlocked = "blocked"
)

// AutoTestTCResult is one test case's execution record. It is also
// the on-disk shape of the per-TC result.json evidence file.
type AutoTestTCResult struct {
	TC             string `json:"tc"`
	AC             string `json:"ac"`
	Command        string `json:"command"`
	Status         string `json:"status"` // passed | failed | blocked
	ReasonCode     string `json:"reason_code,omitempty"`
	Detail         string `json:"detail,omitempty"`
	ExitCode       int    `json:"exit_code"`
	TimedOut       bool   `json:"timed_out,omitempty"`
	DurationMS     int64  `json:"duration_ms"`
	AssertionCount *int   `json:"assertion_count,omitempty"` // from a junit file the command wrote at BEACON_JUNIT_PATH
	ResultPath     string `json:"result_path,omitempty"`     // project-relative
	ArtifactID     string `json:"artifact_id,omitempty"`
}

// AutoTestReport is one RunAutoTest outcome.
type AutoTestReport struct {
	RunID           string             `json:"run_id"`
	FeatureSlug     string             `json:"feature_slug"`
	ProjectRoot     string             `json:"project_root"`
	StartedAt       string             `json:"started_at"`
	FinishedAt      string             `json:"finished_at"`
	Total           int                `json:"total"`
	Passed          int                `json:"passed"`
	Failed          int                `json:"failed"`
	Blocked         int                `json:"blocked"`
	Results         []AutoTestTCResult `json:"results"`
	ReportPath      string             `json:"report_path,omitempty"`
	ReportArtifact  string             `json:"report_artifact_id,omitempty"`
	FallbackCommand string             `json:"fallback_command,omitempty"`
}

// AutoTestOptions carries one RunAutoTest invocation.
type AutoTestOptions struct {
	By      string        // registrant identity for the evidence (required)
	Timeout time.Duration // per-TC timeout; <=0 uses the harness default
}

// RunAutoTest executes the run's feature tests.md TC commands and
// registers the evidence. It returns the full report even when some
// TCs fail; the returned error is non-nil (autotest_failed) when
// any TC failed or was blocked, and a hard refusal (no tests doc /
// nothing executable anywhere) before anything ran.
func (e *Engine) RunAutoTest(ctx context.Context, runID string, opts AutoTestOptions) (AutoTestReport, error) {
	if strings.TrimSpace(opts.By) == "" {
		return AutoTestReport{}, core.NewError(ReasonArtifactRegistrarRequired,
			"qa.RunAutoTest", "an executor identity (--by) is required: auto-test evidence must name who ran it")
	}
	run, _, slug, err := e.loadRun(runID)
	if err != nil {
		return AutoTestReport{}, err
	}
	fctx, err := e.featureContext(run.FeatureID)
	if err != nil {
		return AutoTestReport{}, err
	}
	root := fctx.Project.RootPath
	pkg := e.packageFiles(fctx)
	if !pkg.Tests.Present {
		return AutoTestReport{}, core.NewError(core.ReasonAutoTestTestsDocMissing,
			"qa.RunAutoTest", "feature "+slug+" has no tests document on disk ("+pkg.Tests.Path+"); nothing to execute")
	}
	rows := gate.ParseCoverageRows(pkg.Tests.Text)
	fallback := projectConfigValue(root, "test_command")
	if fallback == "" {
		fallback = projectConfigValue(root, "autotest_command")
	}
	if fallback == "" {
		// R2d (M2): the qa9 strategy the project generated for
		// itself (.beacon/qa-strategy.json) is the last
		// discovery step before qa9_command_missing_auto_case.
		fallback = qaStrategyDefaultCommand(root)
	}
	type job struct {
		idx     int
		row     gate.TCRow
		command string
	}
	// Results keep tests.md row order (Python walks the matrix in
	// order): blocked rows hold their slot via their row index.
	var jobs []job
	blockedAt := map[int]AutoTestTCResult{}
	for i, row := range rows {
		cmd := strings.Trim(strings.TrimSpace(row.Command), "`")
		switch {
		case strings.Contains(row.Command, "BLOCKED:"):
			blockedAt[i] = AutoTestTCResult{
				TC: row.TC, AC: row.AC, Command: cmd, Status: AutoTestBlocked,
				ReasonCode: core.ReasonAutoTestDeclaredBlocked, Detail: "tests.md declares this TC BLOCKED (manual escape); not executed",
			}
		case cmd == "" && fallback == "":
			blockedAt[i] = AutoTestTCResult{
				TC: row.TC, AC: row.AC, Status: AutoTestBlocked,
				ReasonCode: core.ReasonAutoTestCommandMissing,
				Detail:     "no command in tests.md and no project test_command configured",
			}
		case cmd == "":
			jobs = append(jobs, job{idx: i, row: row, command: fallback})
		default:
			jobs = append(jobs, job{idx: i, row: row, command: cmd})
		}
	}
	if len(jobs) == 0 {
		return AutoTestReport{}, core.NewError(core.ReasonAutoTestNoExecutable,
			"qa.RunAutoTest", "tests.md has no executable TC commands and no project test_command is configured (.beacon/config) — refusing to fake a run")
	}

	report := AutoTestReport{
		RunID: run.ID, FeatureSlug: slug, ProjectRoot: root,
		StartedAt: time.Now().UTC().Format(time.RFC3339), FallbackCommand: fallback,
	}
	baseDir := filepath.Join(root, ".beacon", "autotest", run.ID)
	executedAt := map[int]AutoTestTCResult{}
	for _, j := range jobs {
		res := e.executeOneTC(ctx, run.ID, root, baseDir, j.row, j.command, opts)
		if res.Status == AutoTestPassed {
			report.Passed++
		} else {
			report.Failed++
		}
		executedAt[j.idx] = res
	}
	results := make([]AutoTestTCResult, 0, len(rows))
	for i := range rows {
		if r, ok := blockedAt[i]; ok {
			results = append(results, r)
		} else if r, ok := executedAt[i]; ok {
			results = append(results, r)
		}
	}
	report.Blocked = len(blockedAt)
	report.Results = results
	report.Total = len(results)
	report.FinishedAt = time.Now().UTC().Format(time.RFC3339)

	// Aggregate report: the path is fixed BEFORE marshaling so the
	// bytes on disk are final at registration time — the artifact
	// registry hashes file content, and any post-registration
	// rewrite would make the gate judge the evidence stale. The
	// registration id itself lives only in the returned report and
	// the registry, never inside the registered file.
	report.ReportPath = filepath.ToSlash(filepath.Join(".beacon", "autotest", run.ID, "report.json"))
	if err := os.MkdirAll(baseDir, 0o755); err == nil {
		raw, merr := json.MarshalIndent(report, "", "  ")
		if merr == nil {
			reportAbs := filepath.Join(baseDir, "report.json")
			if werr := os.WriteFile(reportAbs, raw, 0o644); werr == nil {
				if reg, rerr := e.RegisterArtifact(run.ID, "test_results", reportAbs, opts.By); rerr == nil {
					report.ReportArtifact = reg.Artifact.ID
				}
			}
		}
	}
	_ = e.appendEvent(run, EventAutoTestExecuted, map[string]any{
		"total": report.Total, "passed": report.Passed,
		"failed": report.Failed, "blocked": report.Blocked,
		"by": opts.By, "report_path": report.ReportPath,
	})
	if report.Failed > 0 || report.Blocked > 0 {
		return report, core.NewError(core.ReasonAutoTestFailed,
			"qa.RunAutoTest", "auto-test finished with failures or blocked TCs (see report)")
	}
	return report, nil
}

// executeOneTC runs one TC command and lands its evidence files +
// artifact registration. It never returns an error: every outcome
// is a result row (the aggregate decides).
func (e *Engine) executeOneTC(ctx context.Context, runID, root, baseDir string, row gate.TCRow, command string, opts AutoTestOptions) AutoTestTCResult {
	res := AutoTestTCResult{TC: row.TC, AC: row.AC, Command: command, ExitCode: -1}
	tcDir := filepath.Join(baseDir, sanitizeTCDir(row.TC))
	argv, err := SplitCommandWords(command)
	if err != nil {
		res.Status = AutoTestBlocked
		if rc, ok := core.ReasonCodeOf(err); ok {
			res.ReasonCode = rc
		} else {
			res.ReasonCode = core.ReasonAutoTestUnparseable
		}
		res.Detail = err.Error()
		e.writeTCResult(root, tcDir, &res, "", "")
		return res
	}
	if len(argv) == 0 {
		res.Status = AutoTestBlocked
		res.ReasonCode = core.ReasonAutoTestCommandMissing
		res.Detail = "empty command after parsing"
		e.writeTCResult(root, tcDir, &res, "", "")
		return res
	}
	junitPath := filepath.Join(tcDir, "junit.xml")
	env := harness.IterationEnv(harness.IterationRequest{RunID: runID, Stage: "qa"})
	env = append(env,
		"BEACON_TC_ID="+row.TC, "BEACON_AC_ID="+row.AC,
		"BEACON_PROJECT_ROOT="+root, "BEACON_JUNIT_PATH="+junitPath,
		"BEACON_TC_DIR="+tcDir,
	)
	out := harness.RunCommand(ctx, argv, root, env, opts.Timeout)
	res.ExitCode = out.ExitCode
	res.TimedOut = out.TimedOut
	res.DurationMS = out.Duration.Milliseconds()
	res.AssertionCount = junitAssertionCount(junitPath)
	switch {
	case out.Err != nil:
		res.Status = AutoTestFailed
		if rc, ok := core.ReasonCodeOf(out.Err); ok {
			res.ReasonCode = rc
		} else {
			res.ReasonCode = core.ReasonAutoTestLayerFailed
		}
		res.Detail = out.Err.Error()
	case out.TimedOut:
		res.Status = AutoTestFailed
		res.ReasonCode = core.ReasonExecutorTimedOut
		res.Detail = "command timed out and its process group was killed"
	case out.ExitCode != 0:
		res.Status = AutoTestFailed
		res.ReasonCode = core.ReasonAutoTestLayerFailed
		res.Detail = "command exited non-zero"
	default:
		res.Status = AutoTestPassed
	}
	e.writeTCResult(root, tcDir, &res, out.Stdout, out.Stderr)
	// Register the result file as qa-consumable evidence. The file
	// is final at this point (writeTCResult fixed its path before
	// marshaling) and is NOT rewritten afterwards: the registry
	// hashes content, so a rewrite would read as stale evidence to
	// the gate. The artifact id therefore lives only on this
	// in-memory row (and in the aggregate report), not in the file.
	// A failed registration does not rewrite the execution outcome;
	// it is recorded on the row instead.
	resultAbs := filepath.Join(tcDir, "result.json")
	if reg, rerr := e.RegisterArtifact(runID, "test_result", resultAbs, opts.By); rerr == nil {
		res.ArtifactID = reg.Artifact.ID
	} else {
		res.Detail = strings.TrimSpace(res.Detail + " artifact registration failed: " + rerr.Error())
	}
	return res
}

// writeTCResult persists one TC's stdout/stderr/result.json under
// its evidence dir. The project-relative result path is stamped
// BEFORE marshaling, so the file's bytes are complete and final
// the moment they land — callers register the file by content hash
// and must never rewrite it afterwards.
func (e *Engine) writeTCResult(root, tcDir string, res *AutoTestTCResult, stdout, stderr string) {
	if err := os.MkdirAll(tcDir, 0o755); err != nil {
		return
	}
	if rel, rerr := filepath.Rel(root, filepath.Join(tcDir, "result.json")); rerr == nil {
		res.ResultPath = filepath.ToSlash(rel)
	}
	_ = os.WriteFile(filepath.Join(tcDir, "stdout.log"), []byte(stdout), 0o644)
	_ = os.WriteFile(filepath.Join(tcDir, "stderr.log"), []byte(stderr), 0o644)
	raw, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(tcDir, "result.json"), raw, 0o644)
}

// sanitizeTCDir keeps a TC id filesystem-safe for its evidence dir.
func sanitizeTCDir(tc string) string {
	tc = strings.TrimSpace(tc)
	if tc == "" {
		return "tc-unnamed"
	}
	var b strings.Builder
	for _, r := range tc {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// junitAssertionCount reads the tests= total from a junit XML file
// (the Python chain counts assertions from junit first). nil when
// no readable junit exists — the count is reported, never invented.
func junitAssertionCount(path string) *int {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc struct {
		XMLName xml.Name `xml:"testsuites"`
		Tests   *int     `xml:"tests,attr"`
		Suites  []struct {
			Tests *int `xml:"tests,attr"`
		} `xml:"testsuite"`
	}
	if err := xml.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	if doc.Tests != nil {
		return doc.Tests
	}
	total, found := 0, false
	for _, s := range doc.Suites {
		if s.Tests != nil {
			total += *s.Tests
			found = true
		}
	}
	if found {
		return &total
	}
	return nil
}
