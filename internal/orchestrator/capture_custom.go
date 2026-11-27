package orchestrator

import (
	"encoding/json"
	"strings"
)

// The custom program writes this document to stdout. It is trusted project
// test code, not an authenticated external authority. Invocation fields prevent
// accidentally consuming another capture's cached result.
type captureCustomResult struct {
	Schema     string `json:"schema"`
	Invocation string `json:"invocation"`
	NodeID     string `json:"node_id"`
	ACID       string `json:"ac_id"`
	Phase      string `json:"phase"`
	Cases      []struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Assertions int    `json:"assertions"`
	} `json:"cases"`
}

func captureCustomVerdict(stdout string, res CaptureResult, required []string) error {
	var result captureCustomResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		return captureError("capture_result_invalid", err.Error())
	}
	if result.Schema != "beacon-capture-tests.v1" || result.Invocation != res.ResultPath || result.NodeID != res.NodeID || result.ACID != res.ACID || result.Phase != res.Phase || len(result.Cases) == 0 {
		return captureError("capture_result_invalid", "custom binding/empty cases")
	}
	seen := map[string]string{}
	failed := false
	requiredFailure := false
	for _, c := range result.Cases {
		if strings.TrimSpace(c.Name) == "" || seen[c.Name] != "" || c.Assertions <= 0 || (c.Status != "passed" && c.Status != "failed") {
			return captureError("capture_result_invalid", "custom assertion/status/duplicate case")
		}
		seen[c.Name] = c.Status
		if c.Status == "failed" {
			failed = true
		}
	}
	for _, name := range required {
		if seen[name] == "" {
			return captureError("capture_result_invalid", "missing required case")
		}
		if seen[name] == "failed" {
			requiredFailure = true
		}
	}
	if res.Phase == "red" && (res.ExitCode != 1 || !requiredFailure) || res.Phase == "green" && (res.ExitCode != 0 || failed) {
		return captureError("capture_result_invalid", "custom outcome differs")
	}
	return nil
}
