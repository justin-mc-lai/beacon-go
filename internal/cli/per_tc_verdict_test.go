package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPerTCVerdictCLI(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "beacon")
	build := exec.Command("go", "build", "-o", bin, "./cmd/beacon")
	build.Dir = "../.."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v %s", err, out)
	}
	for _, scenario := range []string{"pass", "missing", "failed", "blocked", "stale", "raw-stale", "duplicate-tc", "zero"} {
		t.Run(scenario, func(t *testing.T) {
			q := newKernelQA(t, "python3 case.py", "actual-case")
			b, err := os.ReadFile(q.tests)
			if err != nil {
				t.Fatal(err)
			}
			text := strings.ReplaceAll(string(b), "TC-001", "TC-002") + "| AC-001 | TC-001 | behavior | `python3 case.py` | actual assertion | actual-case |\n"
			if scenario == "zero" {
				text = "# No TC contract\n"
			}
			if scenario == "duplicate-tc" {
				text = strings.ReplaceAll(text, "TC-002", "TC-001")
			}
			q.write(q.tests, text)
			q.candidate()
			if scenario == "failed" {
				q.write(filepath.Join(q.root, "case.py"), "assert False, 'negative control'\n")
			}
			if scenario == "blocked" {
				q.write(q.tests, strings.Replace(text, "python3 case.py", "BLOCKED: manual", 1))
				q.candidate()
			}
			var report map[string]any
			if scenario != "missing" && scenario != "zero" {
				report = q.autotest(scenario == "pass" || scenario == "stale" || scenario == "raw-stale")
			}
			if scenario == "pass" || scenario == "stale" || scenario == "raw-stale" {
				p := q.review("product", "pass", "checker")
				q.call(true, "goal", "attach-verifier", "--run-id", q.run, "--source", "checker", "--ref", p)
			}
			if scenario == "stale" {
				q.write(filepath.Join(q.root, report["report_path"].(string)), "changed")
			}
			if scenario == "raw-stale" {
				q.write(filepath.Join(q.root, filepath.Dir(report["report_path"].(string)), "TC-001", "stdout.log"), "changed")
			}
			doc := q.call(scenario == "pass", "gate", "evaluate", "qa", "--project", q.root, "--feature", q.slug, "--run", q.run, "--evaluator", "checker")
			var got []string
			for _, raw := range doc["checks"].([]any) {
				check := raw.(map[string]any)
				name := check["check"].(string)
				if strings.HasPrefix(name, "TC-") {
					got = append(got, name)
					if check["ok"] != (scenario == "pass" || scenario == "blocked" && name == "TC-001:pass") {
						t.Fatalf("incorrect ok: %v", check)
					}
				}
			}
			status := map[string]string{"pass": "pass", "missing": "blocked", "failed": "fail", "blocked": "blocked", "stale": "stale", "raw-stale": "stale", "duplicate-tc": "blocked"}[scenario]
			var want []string
			if scenario != "zero" {
				want = []string{"TC-002:" + status, "TC-001:" + status}
			}
			if scenario == "blocked" {
				want[1] = "TC-001:pass"
			}
			if scenario == "duplicate-tc" {
				want[0] = "TC-001:blocked"
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("checks = %v, want %v; gate=%v", got, want, doc)
			}
			t.Logf("real CLI scenario=%s verdict=%v checks=%v", scenario, doc["verdict"], got)
			cmd := exec.Command(bin, "gate", "evaluate", "qa", "--project", q.root, "--feature", q.slug, "--run", q.run, "--evaluator", "checker", "--json")
			out, cliErr := cmd.Output()
			if (cliErr == nil) != (scenario == "pass") {
				t.Fatalf("compiled CLI: %v %s", cliErr, out)
			}
			compiled := parseDoc(t, string(out))
			if !reflect.DeepEqual(compiled["checks"], doc["checks"]) {
				t.Fatalf("compiled checks drift: %s", out)
			}
		})
	}
}
