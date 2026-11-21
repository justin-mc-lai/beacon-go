package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/core"
)

func TestQAExecutionEvidence(t *testing.T) {
	const (
		runID     = "run_test"
		tcID      = "TC-001"
		candidate = "sha256:candidate"
	)
	cases := []struct {
		name     string
		setup    func(t *testing.T, dir string) (stdout, stderr string)
		required []string
		wantErr  bool
		wantCode string
	}{
		{name: "go-verbose-overlapping-name", setup: func(t *testing.T, dir string) (string, string) {
			return "=== RUN   Same\n=== RUN   Same\n--- PASS: Same (0s)\n", ""
		}, wantErr: true},
		{name: "go-json-overlapping-name", setup: func(t *testing.T, dir string) (string, string) {
			return "{\"Action\":\"run\",\"Package\":\"p\",\"Test\":\"Same\"}\n{\"Action\":\"run\",\"Package\":\"p\",\"Test\":\"Same\"}\n{\"Action\":\"pass\",\"Package\":\"p\",\"Test\":\"Same\"}\n", ""
		}, wantErr: true},
		{name: "malformed-go-json", setup: func(t *testing.T, dir string) (string, string) {
			return "{\"Action\":\"run\",\"Test\":\"Same\"}\n{\"Action\":bad}\n{\"Action\":\"pass\",\"Test\":\"Same\"}\n", ""
		}, wantErr: true},
		{name: "junit-trailing-document", setup: func(t *testing.T, dir string) (string, string) {
			writeQAFile(t, filepath.Join(dir, "junit.xml"), `<testsuite tests="1"><testcase name="one"/></testsuite><testsuite tests="1"><testcase name="two"/></testsuite>`)
			return "", ""
		}, wantErr: true},
		{
			name: "go-package-collision",
			setup: func(t *testing.T, dir string) (string, string) {
				return "{\"Action\":\"run\",\"Package\":\"a\",\"Test\":\"Same\"}\n{\"Action\":\"pass\",\"Package\":\"b\",\"Test\":\"Same\"}\n", ""
			}, wantErr: true,
		},
		{
			name: "go-fail-then-pass",
			setup: func(t *testing.T, dir string) (string, string) {
				return "=== RUN   Same\n--- FAIL: Same (0s)\n=== RUN   Same\n--- PASS: Same (0s)\n", ""
			}, wantErr: true,
		},
		{
			name: "go-incomplete-verbose",
			setup: func(t *testing.T, dir string) (string, string) {
				return "=== RUN   Missing\n=== RUN   Same\n--- PASS: Same (0s)\n", ""
			}, wantErr: true,
		},
		{
			name: "junit-wrong-totals",
			setup: func(t *testing.T, dir string) (string, string) {
				writeQAFile(t, filepath.Join(dir, "junit.xml"), `<testsuite tests="2"><testcase name="one"/></testsuite>`)
				return "", ""
			}, wantErr: true,
		},
		{
			name: "empty-custom-no-fallback",
			setup: func(t *testing.T, dir string) (string, string) {
				writeQAFile(t, filepath.Join(dir, "custom.json"), "")
				return "=== RUN   Same\n--- PASS: Same (0s)\n", ""
			}, wantErr: true,
		},
		{
			name: "go-json-pass",
			setup: func(t *testing.T, dir string) (string, string) {
				return `{"Action":"run","Package":"p","Test":"TestAlpha"}
{"Action":"pass","Package":"p","Test":"TestAlpha","Elapsed":0.01}
{"Action":"pass","Package":"p","Elapsed":0.02}
`, ""
			},
		},
		{
			name: "go-verbose-pass",
			setup: func(t *testing.T, dir string) (string, string) {
				return "=== RUN   TestAlpha\n--- PASS: TestAlpha (0.00s)\nPASS\nok  \tp\t0.1s\n", ""
			},
		},
		{
			name: "go-no-tests-pass",
			setup: func(t *testing.T, dir string) (string, string) {
				return "PASS\nok  \tp\t(cached)\n", ""
			},
			wantErr:  true,
			wantCode: "qa_execution_no_cases",
		},
		{
			name: "go-skip",
			setup: func(t *testing.T, dir string) (string, string) {
				return "=== RUN   TestAlpha\n--- SKIP: TestAlpha (0.00s)\nPASS\n", ""
			},
			wantErr:  true,
			wantCode: "qa_execution_case_skipped",
		},
		{
			name: "go-fail",
			setup: func(t *testing.T, dir string) (string, string) {
				return "=== RUN   TestAlpha\n--- FAIL: TestAlpha (0.00s)\nFAIL\n", ""
			},
			wantErr:  true,
			wantCode: "qa_execution_case_failed",
		},
		{
			name: "junit-empty-summary",
			setup: func(t *testing.T, dir string) (string, string) {
				writeQAFile(t, filepath.Join(dir, "junit.xml"), `<?xml version="1.0"?><testsuites tests="3"></testsuites>`)
				return "", ""
			},
			wantErr:  true,
			wantCode: "qa_execution_result_invalid",
		},
		{
			name: "junit-valid",
			setup: func(t *testing.T, dir string) (string, string) {
				writeQAFile(t, filepath.Join(dir, "junit.xml"), `<?xml version="1.0"?><testsuites tests="2"><testsuite name="s" tests="2"><testcase name="TestAlpha" classname="p"/><testcase name="TestBeta" classname="p"/></testsuite></testsuites>`)
				return "", ""
			},
		},
		{
			name: "custom-missing",
			setup: func(t *testing.T, dir string) (string, string) {
				return "PASS\n", ""
			},
			wantErr:  true,
			wantCode: "qa_execution_result_missing",
		},
		{
			name: "custom-valid",
			setup: func(t *testing.T, dir string) (string, string) {
				writeQAFile(t, filepath.Join(dir, "custom.json"), `{"schema":"beacon-test-execution.v1","run_id":"`+runID+`","tc":"`+tcID+`","candidate_hash":"`+candidate+`","cases":[{"name":"alpha","status":"passed","assertions":2}]}`)
				return "", ""
			},
		},
		{
			name: "custom-wrong-candidate",
			setup: func(t *testing.T, dir string) (string, string) {
				writeQAFile(t, filepath.Join(dir, "custom.json"), `{"schema":"beacon-test-execution.v1","run_id":"`+runID+`","tc":"`+tcID+`","candidate_hash":"other","cases":[{"name":"alpha","status":"passed","assertions":1}]}`)
				return "", ""
			},
			wantErr:  true,
			wantCode: "qa_execution_result_invalid",
		},
		{
			name: "required-miss",
			setup: func(t *testing.T, dir string) (string, string) {
				return "=== RUN   TestAlpha\n--- PASS: TestAlpha (0.00s)\n", ""
			},
			required: []string{"TestMissing"},
			wantErr:  true,
			wantCode: "qa_execution_required_case_missing",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			stdout, stderr := tc.setup(t, dir)
			_, err := evaluateQAExecution(QAExecutionInput{
				RunID: runID, TC: tcID, CandidateHash: candidate,
				Stdout: stdout, Stderr: stderr,
				JUnitPath:  filepath.Join(dir, "junit.xml"),
				CustomPath: filepath.Join(dir, "custom.json"),
				Required:   tc.required,
			})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected rejection, got nil")
				}
				if tc.wantCode != "" {
					if rc, _ := core.ReasonCodeOf(err); rc != tc.wantCode {
						t.Fatalf("reason = %q, want %q (err=%v)", rc, tc.wantCode, err)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected rejection: %v", err)
			}
		})
	}
}

func writeQAFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
