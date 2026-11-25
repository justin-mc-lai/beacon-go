package cli

import (
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/gate"
)

type kernelQA struct {
	t                                             *testing.T
	root, run, truth, tests, manifest, hash, slug string
}

func (q *kernelQA) write(path, text string) {
	q.t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		q.t.Fatal(err)
	}
}
func (q *kernelQA) sum(path string) string {
	q.t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		q.t.Fatal(err)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (q *kernelQA) call(ok bool, args ...string) map[string]any {
	q.t.Helper()
	out, err := runCLI(q.t, append(args, "--json")...)
	if (err == nil) != ok {
		q.t.Fatalf("%v: err=%v stdout=%s", args, err, out)
	}
	if strings.TrimSpace(out) == "" {
		return nil
	}
	return parseDoc(q.t, out)
}
func newKernelQA(t *testing.T, command, required string) *kernelQA {
	t.Helper()
	q := &kernelQA{t: t, root: t.TempDir(), slug: "qa-probe"}
	t.Setenv("BEACON_STORE_PATH", filepath.Join(t.TempDir(), "isolated.db"))
	q.call(true, "projects", "register", "--path", q.root, "--version", "v1.0.0")
	f := q.call(true, "truth", "create", "--project", q.root, "--slug", "qa-probe")
	q.truth = filepath.Join(q.root, f["truth_path"].(string))
	q.tests = filepath.Join(q.root, f["tests_path"].(string))
	b, _ := os.ReadFile(q.truth)
	q.write(q.truth, strings.Replace(string(b), "status: draft", "status: frozen", 1))
	q.write(q.tests, "# Tests\n\n| AC ID | TC ID | Layer | Command | Assertion | Required Tests |\n| --- | --- | --- | --- | --- | --- |\n| AC-001 | TC-001 | behavior | `"+command+"` | actual assertion | "+required+" |\n")
	q.write(filepath.Join(q.root, "source.py"), "print('implementation')\n")
	q.write(filepath.Join(q.root, "artifact.txt"), "candidate\n")
	q.write(filepath.Join(q.root, "case.py"), kernelQARunner)
	r := q.call(true, "goal", "run", "QA fixture", "--project", q.root, "--feature", "qa-probe", "--executor", "external", "--executor-id", "maker")
	q.run = r["run_id"].(string)
	q.manifest = filepath.Join(q.root, "candidate.json")
	q.candidate()
	return q
}

const kernelQARunner = `import os,json,pathlib
assert 2+2==4
pathlib.Path(os.environ['BEACON_TEST_RESULT_PATH']).write_text(json.dumps({'schema':'beacon-test-execution.v1','run_id':os.environ['BEACON_RUN_ID'],'tc':os.environ['BEACON_TC_ID'],'candidate_hash':os.environ['BEACON_QA_CANDIDATE_HASH'],'cases':[{'name':'actual-case','status':'passed','assertions':1}]}))
`

func (q *kernelQA) candidate() {
	v := map[string]any{"schema": "beacon-qa-candidate.v1", "run_id": q.run, "feature_slug": q.slug, "truth_sha256": q.sum(q.truth), "tests_sha256": q.sum(q.tests), "source_commit": "", "source_files": []any{map[string]string{"path": "source.py", "sha256": q.sum(filepath.Join(q.root, "source.py"))}}, "build_command": "fixture source snapshot", "maker": "maker", "artifacts": []any{map[string]string{"path": filepath.Join(q.root, "artifact.txt"), "sha256": q.sum(filepath.Join(q.root, "artifact.txt"))}}}
	b, _ := json.Marshal(v)
	q.write(q.manifest, string(b))
	q.hash = q.sum(q.manifest)
	q.call(true, "goal", "artifacts", "register", "--run-id", q.run, "--kind", "qa_candidate", "--path", q.manifest, "--by", "maker")
}
func (q *kernelQA) autotest(ok bool) map[string]any {
	return q.call(ok, "verify", "autotest", "run", "--run-id", q.run, "--by", "maker")
}
func (q *kernelQA) review(scope, verdict, checker string) string {
	path := filepath.Join(q.root, "review.json")
	raw := filepath.Join(q.root, "review.log")
	q.write(raw, "independent observation\n")
	q.call(true, "goal", "artifacts", "register", "--run-id", q.run, "--kind", "log", "--path", raw, "--by", checker)
	var tcs []string
	bdoc, _ := os.ReadFile(q.tests)
	for _, r := range gate.ParseCoverageRows(string(bdoc)) {
		tcs = append(tcs, r.TC)
	}
	b, _ := json.Marshal(map[string]any{"schema": "beacon-product-qa.v1", "run_id": q.run, "candidate_hash": q.hash, "scope": scope, "verdict": verdict, "maker": "maker", "checker": checker, "covered_tcs": tcs, "evidence_refs": []string{raw}})
	q.write(path, string(b))
	q.call(true, "goal", "artifacts", "register", "--run-id", q.run, "--kind", "qa_report", "--path", path, "--by", checker)
	return path
}
func (q *kernelQA) gate(ok bool) {
	q.t.Helper()
	q.call(ok, "gate", "evaluate", "qa", "--project", q.root, "--feature", q.slug, "--run", q.run, "--evaluator", "checker")
}

func prepareCLIProductQA(t *testing.T, root, slug, runID, checker string) string {
	t.Helper()
	q := &kernelQA{t: t, root: root, run: runID, slug: slug, truth: filepath.Join(root, "truth.md"), tests: filepath.Join(root, "tests.md"), manifest: filepath.Join(root, "candidate.json")}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(q.truth); os.IsNotExist(err) {
		q.write(q.truth, "---\nslug: "+slug+"\nversion: v2.0.0\nlanguage: en\nstatus: frozen\nparser_contract: beacon-feature-package-v2\ncanonical_refs:\n  truth: truth.md\n  tests: tests.md\n  tasks: tasks.md\n  evidence: evidence.md\n---\n# Truth\n\n- **AC-001**: fixture requirement\n")
	}
	for name, text := range map[string]string{"tasks.md": "---\nslug: " + slug + "\n---\n- [ ] implement · ac=AC-001\n", "evidence.md": "# Evidence\n\n| Surface | Authority | Artifact |\n|---|---|---|\n| truth | requirement_truth | truth.md |\n"} {
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			q.write(path, text)
		}
	}
	doc, err := os.ReadFile(q.tests)
	if err != nil {
		doc = []byte("| AC ID | TC ID | Layer | Command | Assertion |\n|---|---|---|---|---|\n| AC-001 | TC-001 | behavior | python3 case.py | actual assertion |\n")
	}
	text := string(doc)
	for _, r := range gate.ParseCoverageRows(text) {
		text = strings.ReplaceAll(text, r.Command, "python3 case.py")
	}
	q.write(q.tests, text)
	q.write(filepath.Join(root, "case.py"), kernelQARunner)
	q.write(filepath.Join(root, "source.py"), "print('implementation')\n")
	q.write(filepath.Join(root, "artifact.txt"), "candidate\n")
	q.candidate()
	q.autotest(true)
	path := q.review("product", "pass", checker)
	q.call(true, "goal", "attach-verifier", "--run-id", runID, "--source", checker, "--ref", path)
	q.gate(true)
	return path
}
func (q *kernelQA) verified() {
	q.autotest(true)
	p := q.review("product", "pass", "checker")
	q.call(true, "goal", "attach-verifier", "--run-id", q.run, "--source", "checker", "--ref", p)
	q.gate(true)
}

func TestKernelQAExecutionEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, cmd, required string
		ok                  bool
	}{
		{"custom", "python3 case.py", "actual-case", true},
		{"empty", "python3 -c 'pass'", "", false},
		{"required", "python3 case.py", "absent", false},
		{"go-json", "go test -json -count=1 .", "TestActual", true},
		{"go-verbose", "go test -v -count=1 .", "TestActual", true},
		{"go-empty", "go test -v -run '^Missing$' .", "", false},
		{"go-skip", "go test -v -run '^TestSkip$' .", "", false},
		{"junit", "python3 case.py", "actual-case", true},
		{"junit-totals", "python3 case.py", "", false},
		{"outside-script", "python3 case.py", "", false},
		{"artifact-mutated-during-test", "python3 case.py", "actual-case", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := newKernelQA(t, tc.cmd, tc.required)
			if strings.HasPrefix(tc.name, "go-") {
				q.write(filepath.Join(q.root, "go.mod"), "module fixture\n\ngo 1.22\n")
				q.write(filepath.Join(q.root, "actual_test.go"), "package fixture\nimport \"testing\"\nfunc TestActual(t *testing.T){if 2+2!=4{t.Fatal(\"sum\")}}\nfunc TestSkip(t *testing.T){t.Skip(\"negative control\")}\n")
				if tc.ok {
					q.write(filepath.Join(q.root, "actual_test.go"), "package fixture\nimport \"testing\"\nfunc TestActual(t *testing.T){if 2+2!=4{t.Fatal(\"sum\")}}\n")
				}
			}
			if strings.HasPrefix(tc.name, "junit") {
				total := "1"
				if tc.name == "junit-totals" {
					total = "2"
				}
				q.write(filepath.Join(q.root, "case.py"), "import os,pathlib\nassert 2+2==4\npathlib.Path(os.environ['BEACON_JUNIT_PATH']).write_text('<testsuite tests=\""+total+"\"><testcase name=\"actual-case\"/></testsuite>')\n")
			}
			if tc.name == "outside-script" {
				outside := filepath.Join(t.TempDir(), "outside.py")
				q.write(outside, kernelQARunner)
				b, _ := os.ReadFile(q.tests)
				q.write(q.tests, strings.ReplaceAll(string(b), "python3 case.py", "python3 "+outside))
				q.candidate()
			}
			if tc.name == "artifact-mutated-during-test" {
				q.write(filepath.Join(q.root, "case.py"), kernelQARunner+"pathlib.Path('artifact.txt').write_text('changed during execution')\n")
			}
			r := q.autotest(tc.ok)
			if tc.ok {
				old := filepath.Join(q.root, r["report_path"].(string))
				sum := q.sum(old)
				next := q.autotest(true)
				if next["report_path"] == r["report_path"] || q.sum(old) != sum {
					t.Fatal("invocation reused or rewrote raw evidence")
				}
			}
		})
	}
}
func TestKernelQACandidateBinding(t *testing.T) {
	t.Run("git-source-provenance", func(t *testing.T) {
		q := newKernelQA(t, "python3 case.py", "actual-case")
		git := func(args ...string) string {
			t.Helper()
			cmd := exec.Command("git", append([]string{"-C", q.root}, args...)...)
			cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@local", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@local")
			b, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("git %v: %v %s", args, err, b)
			}
			return strings.TrimSpace(string(b))
		}
		git("init", "-b", "main")
		git("add", "source.py")
		git("commit", "-m", "implementation")
		impl := git("rev-parse", "HEAD")
		q.write(filepath.Join(q.root, "notes.md"), "documentation\n")
		git("add", "notes.md")
		git("commit", "-m", "QA docs")
		var manifest map[string]any
		b, _ := os.ReadFile(q.manifest)
		_ = json.Unmarshal(b, &manifest)
		register := func(commit string, ok bool) {
			manifest["source_commit"] = commit
			b, _ := json.Marshal(manifest)
			q.write(q.manifest, string(b))
			q.call(ok, "goal", "artifacts", "register", "--run-id", q.run, "--kind", "qa_candidate", "--path", q.manifest, "--by", "maker")
		}
		register(strings.Repeat("f", 40), false)
		register(git("rev-parse", "HEAD"), false)
		register(impl, true)
		// Re-registering an unchanged older path is a new current event.
		oldPath := q.manifest
		oldHash := q.sum(oldPath)
		manifest["build_command"] = "second source snapshot"
		q.manifest = filepath.Join(q.root, "other.json")
		register(impl, true)
		q.manifest = oldPath
		q.call(true, "goal", "artifacts", "register", "--run-id", q.run, "--kind", "qa_candidate", "--path", oldPath, "--by", "maker")
		if r := q.autotest(true); r["candidate_hash"] != oldHash {
			t.Fatal("current candidate did not follow registration event")
		}
		manifest["source_files"] = []any{map[string]string{"path": "source.py", "sha256": strings.Repeat("0", 64)}}
		register(impl, false)
		manifest["source_files"] = []any{map[string]string{"path": "source.py", "sha256": q.sum(filepath.Join(q.root, "source.py"))}}
		manifest["run_id"] = "different-run"
		register(impl, false)
		manifest["run_id"] = q.run
		manifest["artifacts"] = []any{map[string]string{"path": filepath.Join(q.root, "missing"), "sha256": strings.Repeat("0", 64)}}
		register(impl, false)
		// Untracked compiled Go artifacts are bound by hash and VCS metadata.
		q.write(filepath.Join(q.root, "go.mod"), "module fixture\n\ngo 1.22\n")
		q.write(filepath.Join(q.root, "main.go"), "package main\nfunc main() {}\n")
		git("add", "main.go", "go.mod")
		git("commit", "-m", "implement Go fixture")
		impl = git("rev-parse", "HEAD")
		manifest["source_files"] = []any{map[string]string{"path": "main.go", "sha256": q.sum(filepath.Join(q.root, "main.go"))}}
		bin := filepath.Join(t.TempDir(), "product")
		build := func(flags ...string) {
			cmd := exec.Command("go", append([]string{"build", "-o", bin}, flags...)...)
			cmd.Dir = q.root
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("build fixture: %v %s", err, out)
			}
		}
		build(".")
		info, err := buildinfo.ReadFile(bin)
		if err != nil {
			t.Fatal(err)
		}
		modified := false
		for _, s := range info.Settings {
			if s.Key == "vcs.modified" {
				modified = s.Value == "true"
			}
		}
		manifest["vcs_modified"] = modified
		manifest["build_command"] = "go build -o " + bin + " ."
		manifest["artifacts"] = []any{map[string]string{"path": bin, "sha256": q.sum(bin)}}
		register(impl, true)
		manifest["vcs_modified"] = !modified
		register(impl, false)
		manifest["vcs_modified"] = modified
		build("-buildvcs=false", ".")
		manifest["artifacts"] = []any{map[string]string{"path": bin, "sha256": q.sum(bin)}}
		register(impl, false)
		manifest["build_command"] = "go build -buildvcs=false -o " + bin + " . (no VCS metadata)"
		register(impl, true)
	})
	for _, axis := range []string{"artifact", "source", "truth", "tests", "manifest", "replacement", "cross-run"} {
		t.Run(axis, func(t *testing.T) {
			q := newKernelQA(t, "python3 case.py", "actual-case")
			q.verified()
			switch axis {
			case "artifact":
				q.write(filepath.Join(q.root, "artifact.txt"), "tampered")
			case "source":
				q.write(filepath.Join(q.root, "source.py"), "print('changed')\n")
			case "truth":
				b, _ := os.ReadFile(q.truth)
				q.write(q.truth, string(b)+"\nchanged\n")
			case "tests":
				b, _ := os.ReadFile(q.tests)
				q.write(q.tests, string(b)+"\nchanged\n")
			case "manifest":
				b, _ := os.ReadFile(q.manifest)
				q.write(q.manifest, string(b)+" ")
			case "replacement":
				q.write(filepath.Join(q.root, "artifact.txt"), "new candidate")
				q.candidate()
			case "cross-run":
				r := q.call(true, "goal", "run", "second", "--project", q.root, "--feature", q.slug, "--executor", "external")
				q.run = r["run_id"].(string)
				q.call(false, "gate", "evaluate", "qa", "--project", q.root, "--feature", q.slug, "--evaluator", "checker")
			}
			q.gate(false)
		})
	}
}
func TestKernelQAIndependentReview(t *testing.T) {
	for _, axis := range []string{"wrong-run", "wrong-candidate", "wrong-maker", "missing-tc", "unregistered-evidence", "wrong-registrar"} {
		t.Run(axis, func(t *testing.T) {
			q := newKernelQA(t, "python3 case.py", "actual-case")
			q.verified()
			path := filepath.Join(q.root, "review.json")
			var r map[string]any
			b, _ := os.ReadFile(path)
			_ = json.Unmarshal(b, &r)
			by := "checker"
			switch axis {
			case "wrong-run":
				r["run_id"] = "other"
			case "wrong-candidate":
				r["candidate_hash"] = "other"
			case "wrong-maker":
				r["maker"] = "other"
			case "missing-tc":
				r["covered_tcs"] = []string{}
			case "unregistered-evidence":
				p := filepath.Join(q.root, "unregistered.log")
				q.write(p, "observation")
				r["evidence_refs"] = []string{p}
			case "wrong-registrar":
				by = "somebody-else"
			}
			b, _ = json.Marshal(r)
			q.write(path, string(b))
			q.call(true, "goal", "artifacts", "register", "--run-id", q.run, "--kind", "qa_report", "--path", path, "--by", by)
			q.gate(false)
		})
	}
	q := newKernelQA(t, "python3 case.py", "actual-case")
	q.autotest(true)
	q.gate(false)
	p := q.review("product", "pass", "maker")
	q.call(false, "goal", "attach-verifier", "--run-id", q.run, "--source", "maker", "--executor-id", "someone-else", "--ref", p)
	q.verified()
	q.review("contract", "pass", "checker")
	q.gate(false)
	q.review("product", "pass", "checker")
	q.gate(true)
	var review map[string]any
	b, _ := os.ReadFile(p)
	_ = json.Unmarshal(b, &review)
	review["covered_tcs"] = []string{}
	b, _ = json.Marshal(review)
	q.write(p, string(b))
	q.call(true, "goal", "artifacts", "register", "--run-id", q.run, "--kind", "qa_report", "--path", p, "--by", "checker")
	q.gate(false)
	q.review("product", "pass", "checker")
	q.gate(true)
	q.review("product", "fail", "checker")
	q.gate(false)
	q.review("product", "pass", "checker")
	q.gate(true)
	q.write(filepath.Join(q.root, "review.json"), "PASS")
	q.call(true, "goal", "artifacts", "register", "--run-id", q.run, "--kind", "qa_report", "--path", filepath.Join(q.root, "review.json"), "--by", "checker")
	q.gate(false)
	q.review("product", "pass", "checker")
	q.gate(true)
	q.write(filepath.Join(q.root, "case.py"), "pass\n")
	q.autotest(false)
	q.gate(false)
}
func TestKernelQAUnifiedGate(t *testing.T) {
	t.Run("manual-no-formal", func(t *testing.T) {
		q := newKernelQA(t, "python3 case.py", "actual-case")
		q.autotest(true)
		p := q.review("product", "pass", "checker")
		q.call(true, "goal", "attach-verifier", "--run-id", q.run, "--source", "checker", "--ref", p)
		for i := 0; i < 3; i++ {
			q.call(true, "goal", "tick", "--run-id", q.run, "--by", "maker")
		}
		q.call(false, "goal", "tick", "--run-id", q.run, "--by", "checker")
		q.gate(true)
		q.call(true, "goal", "recover", "--run-id", q.run, "--by", "human", "--note", "current formal evidence now exists")
		q.call(true, "goal", "tick", "--run-id", q.run, "--by", "checker")
	})
	q := newKernelQA(t, "python3 case.py", "actual-case")
	q.verified()
	for i := 0; i < 3; i++ {
		q.call(true, "goal", "tick", "--run-id", q.run, "--by", "maker")
	}
	q.call(true, "goal", "tick", "--run-id", q.run, "--by", "checker")
	q.call(false, "goal", "complete", "--run-id", q.run)
	q.write(filepath.Join(q.root, "source.py"), "print('tampered')\n")
	q.call(false, "goal", "complete", "--run-id", q.run, "--human-confirmed", "--confirmed-by", "human")
	q.write(filepath.Join(q.root, "source.py"), "print('implementation')\n")
	q.call(true, "goal", "complete", "--run-id", q.run, "--human-confirmed", "--confirmed-by", "human")
	q.write(filepath.Join(q.root, "source.py"), "print('tampered')\n")
	// Exercise a compiled CLI too; this regression is in production wiring.
	bin := filepath.Join(t.TempDir(), "beacon")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/beacon")
	cmd.Dir = "../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	cmd = exec.Command(bin, "gate", "evaluate", "qa", "--project", q.root, "--feature", "qa-probe", "--run", q.run, "--evaluator", "checker", "--json")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("compiled CLI accepted stale candidate: %s", out)
	}
	t.Run("external-whole-lake", func(t *testing.T) {
		q := newKernelQA(t, "python3 case.py", "actual-case")
		q.call(false, "goal", "receive-result", "--external", "--run-id", q.run, "--by", "room", "--summary", "whole lake", "--artifact", filepath.Join(q.root, "artifact.txt"))
		q.verified()
		q.call(true, "goal", "receive-result", "--external", "--run-id", q.run, "--by", "room", "--summary", "whole lake", "--artifact", filepath.Join(q.root, "artifact.txt"))
	})
	for _, axis := range []string{"artifact.txt", "source.py", "review.json", "review.log"} {
		t.Run("exit-tamper-"+axis, func(t *testing.T) {
			q := newKernelQA(t, "python3 case.py", "actual-case")
			q.verified()
			for i := 0; i < 3; i++ {
				q.call(true, "goal", "tick", "--run-id", q.run, "--by", "maker")
			}
			q.write(filepath.Join(q.root, axis), "tampered")
			q.call(false, "goal", "tick", "--run-id", q.run, "--by", "checker")
		})
	}
	for _, axis := range []string{"artifact", "source", "truth", "tests", "manifest", "review", "raw", "aggregate"} {
		t.Run("complete-tamper-"+axis, func(t *testing.T) {
			q := newKernelQA(t, "python3 case.py", "actual-case")
			q.verified()
			for i := 0; i < 4; i++ {
				q.call(true, "goal", "tick", "--run-id", q.run, "--by", "maker")
			}
			path := map[string]string{"artifact": filepath.Join(q.root, "artifact.txt"), "source": filepath.Join(q.root, "source.py"), "truth": q.truth, "tests": q.tests, "manifest": q.manifest, "review": filepath.Join(q.root, "review.json")}[axis]
			if axis == "raw" || axis == "aggregate" {
				paths, err := filepath.Glob(filepath.Join(q.root, ".beacon", "autotest", q.run, "invocation-*", "report.json"))
				if err != nil || len(paths) != 1 {
					t.Fatal(paths, err)
				}
				path = paths[0]
				if axis == "raw" {
					path = filepath.Join(filepath.Dir(path), "TC-001", "stdout.log")
				}
			}
			q.write(path, "tampered")
			q.call(false, "goal", "complete", "--run-id", q.run, "--human-confirmed", "--confirmed-by", "human")
		})
	}
	t.Run("raw-tamper", func(t *testing.T) {
		q := newKernelQA(t, "python3 case.py", "actual-case")
		q.verified()
		r := q.autotest(true)
		q.gate(true)
		for i := 0; i < 3; i++ {
			q.call(true, "goal", "tick", "--run-id", q.run, "--by", "maker")
		}
		path := filepath.Join(q.root, filepath.Dir(r["report_path"].(string)), "TC-001", "stdout.log")
		q.write(path, "tampered")
		q.call(false, "goal", "tick", "--run-id", q.run, "--by", "checker")
	})
	t.Run("driver-refuses-stale", func(t *testing.T) {
		q := newKernelQA(t, "python3 case.py", "actual-case")
		q.verified()
		for i := 0; i < 3; i++ {
			q.call(true, "goal", "tick", "--run-id", q.run, "--by", "maker")
		}
		q.write(filepath.Join(q.root, "artifact.txt"), "tampered")
		q.call(true, "goal", "driver", "start", "--run-id", q.run, "--max-turns", "3")
		out, _ := runCLI(t, "goal", "driver", "run-loop", "--run-id", q.run, "--executor", "recording", "--max-rounds", "2", "--json")
		d := parseDoc(t, out)
		run := d["run"].(map[string]any)
		if run["current_stage"] != "qa" || run["blocked"] != true {
			t.Fatalf("driver bypassed stale QA: %s", out)
		}
	})
}
