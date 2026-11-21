package orchestrator

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"

	"beacon-go/internal/core"
)

// QAExecutedCase is one machine-observed test case. Assertions is
// only populated by the custom adapter; JUnit's tests attribute is a
// test count, never called an assertion count.
type QAExecutedCase struct {
	Name       string `json:"name"`
	Package    string `json:"package,omitempty"`
	Status     string `json:"status"`
	Assertions int    `json:"assertions,omitempty"`
}

// QAExecutionResult is the shared machine-execution judgement for one
// TC invocation.
type QAExecutionResult struct {
	Adapter       string           `json:"adapter"`
	CandidateHash string           `json:"candidate_hash,omitempty"`
	TestCount     int              `json:"test_count"`
	Cases         []QAExecutedCase `json:"cases"`
}

// QAExecutionInput carries the fresh outputs of one TC invocation.
type QAExecutionInput struct {
	RunID         string
	TC            string
	CandidateHash string
	Stdout        string
	Stderr        string
	JUnitPath     string
	CustomPath    string
	Required      []string
}

// evaluateQAExecution is the single place that decides whether an
// exit-0 command produced real machine-execution evidence. Adapters
// use Go, JUnit, custom priority. A malformed supplied adapter
// is never ignored in favour of another successful output.
func evaluateQAExecution(in QAExecutionInput) (QAExecutionResult, error) {
	var (
		res QAExecutionResult
		err error
	)
	if hasGoJSONEvents(in.Stdout) {
		res, err = parseQAGoJSON(in.Stdout)
	} else if hasGoVerboseEvents(in.Stdout) {
		res, err = parseQAGoVerbose(in.Stdout)
	}
	if err != nil {
		return res, err
	}
	if _, serr := os.Stat(in.JUnitPath); serr == nil {
		j, jerr := parseQAJUnit(in.JUnitPath)
		if jerr != nil {
			return j, jerr
		}
		if err := checkQACases(j, in.Required); err != nil {
			return j, err
		}
		if res.Adapter == "" {
			res = j
		}
	} else if !os.IsNotExist(serr) {
		return res, qaError(core.ReasonQAExecutionResultInvalid, serr.Error())
	}
	if raw, rerr := os.ReadFile(in.CustomPath); rerr == nil {
		c, cerr := parseQACustomJSON(raw, in)
		if cerr != nil {
			return c, cerr
		}
		if err := checkQACases(c, in.Required); err != nil {
			return c, err
		}
		if res.Adapter == "" {
			res = c
		}
	} else if !os.IsNotExist(rerr) {
		return res, qaError(core.ReasonQAExecutionResultInvalid, rerr.Error())
	}
	if res.Adapter == "" && looksLikeGoOutput(in.Stdout) {
		return QAExecutionResult{}, core.NewError(core.ReasonQAExecutionNoCases,
			"qa.evaluateQAExecution", "command exited 0 but Go reported no executed test case")
	} else if res.Adapter == "" {
		return QAExecutionResult{}, core.NewError(core.ReasonQAExecutionResultMissing,
			"qa.evaluateQAExecution", "command exited 0 but produced no Go -json/-v, JUnit, or BEACON_TEST_RESULT_PATH machine result")
	}
	if err := checkQACases(res, in.Required); err != nil {
		return QAExecutionResult{}, err
	}
	return res, nil
}

func checkQACases(res QAExecutionResult, required []string) error {
	if len(res.Cases) == 0 {
		return core.NewError(core.ReasonQAExecutionNoCases,
			"qa.evaluateQAExecution", "machine result contains zero executed test cases")
	}
	byName := map[string]QAExecutedCase{}
	for _, c := range res.Cases {
		byName[c.Name] = c
		switch c.Status {
		case "failed":
			return core.NewError(core.ReasonQAExecutionCaseFailed,
				"qa.evaluateQAExecution", "test case failed: "+c.Name)
		case "skipped":
			return core.NewError(core.ReasonQAExecutionCaseSkipped,
				"qa.evaluateQAExecution", "test case skipped: "+c.Name)
		case "passed":
		default:
			return qaError(core.ReasonQAExecutionResultInvalid, "unknown test case status: "+c.Name)
		}
	}
	for _, want := range required {
		want = strings.TrimSpace(want)
		if want == "" {
			continue
		}
		if _, ok := byName[want]; !ok {
			return core.NewError(core.ReasonQAExecutionRequiredMiss,
				"qa.evaluateQAExecution", "required test case did not execute: "+want)
		}
	}
	return nil
}

type qaCustomDoc struct {
	Schema        string `json:"schema"`
	RunID         string `json:"run_id"`
	TC            string `json:"tc"`
	CandidateHash string `json:"candidate_hash"`
	Cases         []struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Assertions int    `json:"assertions"`
	} `json:"cases"`
}

func parseQACustomJSON(raw []byte, in QAExecutionInput) (QAExecutionResult, error) {
	var doc qaCustomDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return QAExecutionResult{}, core.NewError(core.ReasonQAExecutionResultInvalid,
			"qa.evaluateQAExecution", "custom result is not valid JSON: "+err.Error())
	}
	if doc.Schema != "beacon-test-execution.v1" {
		return QAExecutionResult{}, core.NewError(core.ReasonQAExecutionResultInvalid,
			"qa.evaluateQAExecution", "custom result schema must be beacon-test-execution.v1")
	}
	if strings.TrimSpace(doc.RunID) != in.RunID || strings.TrimSpace(doc.TC) != in.TC {
		return QAExecutionResult{}, core.NewError(core.ReasonQAExecutionResultInvalid,
			"qa.evaluateQAExecution", "custom result run_id/tc does not match this invocation")
	}
	if in.CandidateHash != "" && doc.CandidateHash != in.CandidateHash {
		return QAExecutionResult{}, core.NewError(core.ReasonQAExecutionResultInvalid,
			"qa.evaluateQAExecution", "custom result candidate_hash does not match the current candidate")
	}
	res := QAExecutionResult{Adapter: "custom", CandidateHash: doc.CandidateHash}
	seen := map[string]bool{}
	for _, c := range doc.Cases {
		name := strings.TrimSpace(c.Name)
		if name == "" {
			return QAExecutionResult{}, core.NewError(core.ReasonQAExecutionResultInvalid,
				"qa.evaluateQAExecution", "custom result case has no name")
		}
		if seen[name] {
			return QAExecutionResult{}, core.NewError(core.ReasonQAExecutionResultInvalid,
				"qa.evaluateQAExecution", "custom result case name is not unique: "+name)
		}
		seen[name] = true
		status := strings.ToLower(strings.TrimSpace(c.Status))
		switch status {
		case "passed", "failed", "skipped":
		default:
			return QAExecutionResult{}, core.NewError(core.ReasonQAExecutionResultInvalid,
				"qa.evaluateQAExecution", "custom result case has unknown status: "+c.Status)
		}
		if c.Assertions <= 0 {
			return QAExecutionResult{}, core.NewError(core.ReasonQAExecutionResultInvalid,
				"qa.evaluateQAExecution", "custom result passed case has no positive assertion count: "+name)
		}
		res.Cases = append(res.Cases, QAExecutedCase{Name: name, Status: status, Assertions: c.Assertions})
	}
	res.TestCount = len(res.Cases)
	return res, nil
}

type qaGoEvent struct {
	Action  string `json:"Action"`
	Test    string `json:"Test"`
	Package string `json:"Package"`
	Output  string `json:"Output"`
}

func hasGoJSONEvents(stdout string) bool {
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var ev qaGoEvent
		if json.Unmarshal([]byte(line), &ev) == nil && ev.Action != "" {
			return true
		}
	}
	return false
}

func parseQAGoJSON(stdout string) (QAExecutionResult, error) {
	res := QAExecutionResult{Adapter: "go-json"}
	seenRun := map[string]bool{}
	seenCase := map[string]bool{}
	order := []string{}
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var ev qaGoEvent
		if json.Unmarshal([]byte(line), &ev) != nil {
			return res, qaError(core.ReasonQAExecutionResultInvalid, "malformed Go JSON event")
		}
		if strings.Contains(ev.Output, "(cached)") {
			return res, qaError(core.ReasonQAExecutionResultInvalid, "cached Go output is not this invocation's execution")
		}
		name := strings.TrimSpace(ev.Test)
		key := ev.Package + "\x00" + name
		if name == "" {
			if ev.Action == "fail" || ev.Action == "skip" {
				return res, qaError(core.ReasonQAExecutionCaseFailed, "Go package failed or skipped: "+ev.Package)
			}
			continue
		}
		switch ev.Action {
		case "run":
			if seenRun[key] {
				return res, qaError(core.ReasonQAExecutionResultInvalid, "Go RUN reused before terminal result: "+key)
			}
			seenRun[key] = true
			if !seenCase[key] {
				seenCase[key] = true
				order = append(order, key)
			}
		case "pass":
			if !seenRun[key] {
				return QAExecutionResult{}, core.NewError(core.ReasonQAExecutionResultInvalid,
					"qa.evaluateQAExecution", "Go JSON pass without a preceding run event: "+name)
			}
			res.Cases = append(res.Cases, QAExecutedCase{Name: name, Package: ev.Package, Status: "passed"})
			seenRun[key] = false
		case "fail":
			return res, qaError(core.ReasonQAExecutionCaseFailed, "Go case failed: "+key)
		case "skip":
			return res, qaError(core.ReasonQAExecutionCaseSkipped, "Go case skipped: "+key)
		}
	}
	// Every run case must reach a terminal pass/fail/skip event.
	for _, name := range order {
		if seenRun[name] {
			return QAExecutionResult{}, core.NewError(core.ReasonQAExecutionResultInvalid,
				"qa.evaluateQAExecution", "Go JSON case has no terminal result: "+name)
		}
	}
	res.TestCount = len(res.Cases)
	return res, nil
}

func hasGoVerboseEvents(stdout string) bool {
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "=== RUN ") || strings.HasPrefix(line, "--- PASS: ") ||
			strings.HasPrefix(line, "--- FAIL: ") || strings.HasPrefix(line, "--- SKIP: ") {
			return true
		}
	}
	return false
}

func parseQAGoVerbose(stdout string) (QAExecutionResult, error) {
	res := QAExecutionResult{Adapter: "go-v"}
	seenRun := map[string]bool{}
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "=== RUN "):
			name := strings.TrimSpace(strings.TrimPrefix(line, "=== RUN "))
			name = strings.TrimSuffix(name, " (")
			if name != "" {
				if seenRun[name] {
					return res, qaError(core.ReasonQAExecutionResultInvalid, "Go -v overlapping RUN: "+name)
				}
				seenRun[name] = true
			}
		case strings.HasPrefix(line, "--- PASS: "):
			name := qaVerboseCaseName(strings.TrimPrefix(line, "--- PASS: "))
			if name != "" {
				if !seenRun[name] {
					return QAExecutionResult{}, core.NewError(core.ReasonQAExecutionResultInvalid,
						"qa.evaluateQAExecution", "Go -v pass without RUN line: "+name)
				}
				res.Cases = append(res.Cases, QAExecutedCase{Name: name, Status: "passed"})
				seenRun[name] = false
			}
		case strings.HasPrefix(line, "--- FAIL: "):
			return res, qaError(core.ReasonQAExecutionCaseFailed, line)
		case strings.HasPrefix(line, "--- SKIP: "):
			return res, qaError(core.ReasonQAExecutionCaseSkipped, line)
		case line == "FAIL" || strings.HasPrefix(line, "FAIL\t") || strings.HasPrefix(line, "FAIL "):
			return res, qaError(core.ReasonQAExecutionCaseFailed, line)
		}
	}
	for name, pending := range seenRun {
		if pending {
			return res, qaError(core.ReasonQAExecutionResultInvalid, "Go -v RUN has no PASS: "+name)
		}
	}
	res.TestCount = len(res.Cases)
	return res, nil
}

func qaVerboseCaseName(rest string) string {
	rest = strings.TrimSpace(rest)
	if i := strings.Index(rest, " ("); i > 0 {
		rest = rest[:i]
	}
	return strings.TrimSpace(rest)
}

func looksLikeGoOutput(stdout string) bool {
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ok  \t") || strings.HasPrefix(line, "ok \t") ||
			strings.Contains(line, "testing: warning: no tests to run") {
			return true
		}
	}
	return false
}

func parseQAJUnit(path string) (QAExecutionResult, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return QAExecutionResult{}, core.NewError(core.ReasonQAExecutionResultMissing,
			"qa.evaluateQAExecution", "junit file is not readable: "+err.Error())
	}
	var doc qaJUnitNode
	decoder := xml.NewDecoder(strings.NewReader(string(raw)))
	if err := decoder.Decode(&doc); err != nil {
		return QAExecutionResult{}, qaError(core.ReasonQAExecutionResultInvalid, err.Error())
	}
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return QAExecutionResult{}, qaError(core.ReasonQAExecutionResultInvalid, err.Error())
		}
		if b, ok := tok.(xml.CharData); ok && strings.TrimSpace(string(b)) == "" {
			continue
		}
		return QAExecutionResult{}, qaError(core.ReasonQAExecutionResultInvalid, "JUnit trailing document")
	}
	cases, err := doc.cases()
	if err != nil {
		return QAExecutionResult{}, err
	}
	if len(cases) == 0 {
		return QAExecutionResult{}, core.NewError(core.ReasonQAExecutionResultInvalid,
			"qa.evaluateQAExecution", "junit summary has no testcase nodes; totals alone are not execution evidence")
	}
	return QAExecutionResult{Adapter: "junit", TestCount: len(cases), Cases: cases}, nil
}

type qaJUnitNode struct {
	XMLName  xml.Name
	Tests    *int          `xml:"tests,attr"`
	Failures int           `xml:"failures,attr"`
	Errors   int           `xml:"errors,attr"`
	Skipped  int           `xml:"skipped,attr"`
	Name     string        `xml:"name,attr"`
	Children []qaJUnitNode `xml:",any"`
}

func (n qaJUnitNode) cases() ([]QAExecutedCase, error) {
	if n.XMLName.Local != "testsuites" && n.XMLName.Local != "testsuite" {
		return nil, qaError(core.ReasonQAExecutionResultInvalid, "JUnit requires testsuite/testcase nodes")
	}
	var cases []QAExecutedCase
	for _, child := range n.Children {
		if child.XMLName.Local == "testcase" {
			if strings.TrimSpace(child.Name) == "" {
				return nil, qaError(core.ReasonQAExecutionResultInvalid, "unnamed JUnit case")
			}
			status := "passed"
			for _, v := range child.Children {
				if v.XMLName.Local == "failure" || v.XMLName.Local == "error" {
					status = "failed"
				}
				if v.XMLName.Local == "skipped" {
					status = "skipped"
				}
			}
			cases = append(cases, QAExecutedCase{Name: child.Name, Status: status})
		} else if child.XMLName.Local == "testsuite" || child.XMLName.Local == "testsuites" {
			c, err := child.cases()
			if err != nil {
				return nil, err
			}
			cases = append(cases, c...)
		}
	}
	if n.Tests != nil && *n.Tests != len(cases) || n.Failures != 0 || n.Errors != 0 || n.Skipped != 0 {
		return nil, qaError(core.ReasonQAExecutionResultInvalid, "JUnit totals differ from actual successful testcase nodes")
	}
	return cases, nil
}

// qaExecutionDetail renders a compact audit line for the TC result.
func qaExecutionDetail(res QAExecutionResult) string {
	return fmt.Sprintf("%s: %d case(s) executed", res.Adapter, res.TestCount)
}
