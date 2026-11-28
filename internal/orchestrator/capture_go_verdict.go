package orchestrator

import (
	"encoding/json"
	"strings"
)

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
