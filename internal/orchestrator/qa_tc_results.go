package orchestrator

import (
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// qaTCEligibilityChecks projects the shared eligibility error onto TC checks.
func (e *Engine) qaTCEligibilityChecks(run store.Run, checks []gate.CheckItem, eligibilityErr error) []gate.CheckItem {
	if eligibilityErr != nil {
		code, _ := core.ReasonCodeOf(eligibilityErr)
		status := "blocked"
		if strings.Contains(code, "stale") {
			status = "stale"
		}
		if len(checks) == 0 {
			if fc, err := e.featureContext(run.FeatureID); err == nil {
				for _, row := range gate.ParseCoverageRows(e.packageFiles(fc).Tests.Text) {
					checks = append(checks, gate.QATCCheck(row.TC, status, code, eligibilityErr.Error()))
				}
			}
		} else if status == "stale" {
			for i := range checks {
				checks[i] = gate.QATCCheck(strings.SplitN(checks[i].Check, ":", 2)[0], status, code, eligibilityErr.Error())
			}
		}
	}
	return checks
}

// validateQATCResults consumes the report already loaded by currentQAEligibility.
func (e *Engine) validateQATCResults(run store.Run, root, candidateHash, reportPath string, report AutoTestReport, rows []gate.TCRow) ([]gate.CheckItem, error) {
	var checks []gate.CheckItem
	// Validate provenance before exposing even failed execution outcomes.
	for _, f := range report.RawFiles {
		if err := qaFileMatches(root, f); err != nil {
			return checks, qaError("qa_execution_stale", err.Error())
		}
		if _, err := e.qaRegisteredFile(run.ID, resolvePkgPath(root, f.Path)); err != nil {
			return checks, err
		}
	}
	ordered := report.RunID == run.ID && report.CandidateHash == candidateHash && len(rows) > 0 && len(report.Results) == len(rows) && report.Total == len(rows) && len(report.RawFiles) > 0
	seen := map[string]bool{}
	for i, row := range rows {
		if seen[row.TC] || i >= len(report.Results) || report.Results[i].TC != row.TC {
			ordered = false
		}
		seen[row.TC] = true
	}
	if ordered {
		for _, res := range report.Results {
			status := "blocked"
			if res.Status == AutoTestFailed {
				status = "fail"
			}
			// A pass is exposed only after the full existing TC validation below.
			checks = append(checks, gate.QATCCheck(res.TC, status, "qa_execution_invalid", "TC execution has not passed validation"))
		}
	}
	if !ordered {
		return checks, qaError("qa_execution_invalid", "latest invocation did not pass the entire current TC contract")
	}
	for i, res := range report.Results {
		row := rows[i]
		if res.Status == AutoTestFailed || res.Status == AutoTestBlocked {
			continue
		}
		if res.TC != row.TC || res.Status != AutoTestPassed || res.CandidateHash != candidateHash || res.TestCount == nil || *res.TestCount <= 0 || *res.TestCount != len(res.Cases) {
			return checks, qaError("qa_execution_invalid", "invalid TC result: "+row.TC)
		}
		if err := checkQACases(QAExecutionResult{Cases: res.Cases}, row.RequiredTests); err != nil {
			return checks, err
		}
		if _, err := e.qaRegisteredFile(run.ID, resolvePkgPath(root, res.ResultPath)); err != nil {
			return checks, err
		}
		if ordered {
			checks[i] = gate.QATCCheck(row.TC, "pass", "", "validated current invocation: "+reportPath)
		}
	}
	if report.Failed != 0 || report.Blocked != 0 || report.Passed != len(rows) {
		return checks, qaError("qa_execution_invalid", "latest invocation did not pass the entire current TC contract")
	}
	for _, res := range report.Results {
		if res.Status != AutoTestPassed {
			return checks, qaError("qa_execution_invalid", "invalid TC result: "+res.TC)
		}
	}
	return checks, nil
}
