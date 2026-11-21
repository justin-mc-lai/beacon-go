package orchestrator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/gate"
)

const qaFixtureRunner = `import os,json,pathlib
assert 2+2==4
pathlib.Path(os.environ['BEACON_TEST_RESULT_PATH']).write_text(json.dumps({'schema':'beacon-test-execution.v1','run_id':os.environ['BEACON_RUN_ID'],'tc':os.environ['BEACON_TC_ID'],'candidate_hash':os.environ['BEACON_QA_CANDIDATE_HASH'],'cases':[{'name':'actual-case','status':'passed','assertions':1}]}))
`

// These helpers create real runner evidence, never synthetic passing gate rows.
func prepareQACandidate(t *testing.T, env testEnv, runID string) qaCurrent {
	t.Helper()
	run, _, _, err := env.eng.loadRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	fc, err := env.eng.featureContext(run.FeatureID)
	if err != nil {
		t.Fatal(err)
	}
	write := func(p, s string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pkg := env.eng.packageFiles(fc)
	if !pkg.Truth.Present {
		write(resolvePkgPath(env.root, pkg.Truth.Path), frozenTruthDoc)
	}
	if !pkg.Tasks.Present {
		write(resolvePkgPath(env.root, pkg.Tasks.Path), "---\nslug: "+fc.Feature.Slug+"\n---\n- [ ] implement · ac=AC-001\n")
	}
	if !pkg.Evidence.Present {
		write(resolvePkgPath(env.root, pkg.Evidence.Path), "# Evidence\n\n| Surface | Authority | Artifact |\n|---|---|---|\n| truth | requirement_truth | truth.md |\n")
	}
	if !pkg.Tests.Present {
		write(resolvePkgPath(env.root, pkg.Tests.Path), "| AC ID | TC ID | Layer | Command | Assertion |\n|---|---|---|---|---|\n| AC-001 | TC-001 | behavior | python3 qa-case.py | assertion holds |\n")
	}
	write(filepath.Join(env.root, "qa-case.py"), qaFixtureRunner)
	source := filepath.Join(env.root, "qa-source.py")
	write(source, "print('implementation')\n")
	commit := ""
	if _, err := qaGit(env.root, "rev-parse", "--show-toplevel"); err == nil {
		out, err := qaGit(env.root, "log", "-1", "--format=%H", "--", "qa-source.py")
		if err != nil {
			t.Fatal(err)
		}
		commit = strings.TrimSpace(out)
		if commit == "" {
			gitCmd(t, env.root, "add", "qa-source.py")
			gitCmd(t, env.root, "commit", "-m", "fixture implementation source")
			commit = strings.TrimSpace(gitCmd(t, env.root, "rev-parse", "HEAD"))
		}
	}
	hash := func(p string) string {
		t.Helper()
		h, err := qaHash(p)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	c := qaCandidate{Schema: "beacon-qa-candidate.v1", RunID: runID, FeatureSlug: fc.Feature.Slug, TruthSHA256: hash(resolvePkgPath(env.root, pkg.Truth.Path)), TestsSHA256: hash(resolvePkgPath(env.root, pkg.Tests.Path)), SourceCommit: commit, SourceFiles: []qaFile{{"qa-source.py", hash(source)}}, BuildCommand: "fixture source snapshot", Maker: "fixture-maker", Artifacts: []qaFile{{source, hash(source)}}}
	path := filepath.Join(t.TempDir(), "candidate.json")
	raw, _ := json.Marshal(c)
	write(path, string(raw))
	if _, err := env.eng.RegisterArtifact(runID, "qa_candidate", path, c.Maker); err != nil {
		t.Fatal(err)
	}
	return qaCurrent{qaCandidate: c, Hash: hash(path), Path: path}
}

func prepareProductQA(t *testing.T, env testEnv, runID, checker string) string {
	t.Helper()
	run, m, _, err := env.eng.loadRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	fc, err := env.eng.featureContext(run.FeatureID)
	if err != nil {
		t.Fatal(err)
	}
	pkg := env.eng.packageFiles(fc)
	if !pkg.Tests.Present {
		if err := os.WriteFile(resolvePkgPath(env.root, pkg.Tests.Path), []byte("| AC ID | TC ID | Layer | Command | Assertion |\n|---|---|---|---|---|\n| AC-001 | TC-001 | behavior | python3 qa-case.py | assertion holds |\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	} else {
		text := pkg.Tests.Text
		for _, r := range gate.ParseCoverageRows(text) {
			text = strings.ReplaceAll(text, r.Command, "python3 qa-case.py")
		}
		if err := os.WriteFile(resolvePkgPath(env.root, pkg.Tests.Path), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := prepareQACandidate(t, env, runID)
	if _, err := env.eng.RunAutoTest(context.Background(), runID, AutoTestOptions{By: c.Maker}); err != nil {
		t.Fatal(err)
	}
	rawPath := filepath.Join(t.TempDir(), "observation.log")
	if err := os.WriteFile(rawPath, []byte("independent fixture assertion\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.RegisterArtifact(runID, "log", rawPath, checker); err != nil {
		t.Fatal(err)
	}
	var tcs []string
	for _, r := range gate.ParseCoverageRows(env.eng.packageFiles(fc).Tests.Text) {
		tcs = append(tcs, r.TC)
	}
	r := qaProductReview{Schema: "beacon-product-qa.v1", RunID: runID, CandidateHash: c.Hash, Scope: "product", Verdict: "pass", Maker: c.Maker, Checker: checker, CoveredTCs: tcs, EvidenceRefs: []string{rawPath}}
	path := filepath.Join(t.TempDir(), "product-qa.json")
	raw, _ := json.Marshal(r)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.RegisterArtifact(runID, "qa_report", path, checker); err != nil {
		t.Fatal(err)
	}
	if checker == m.ExecutorID {
		t.Fatal("fixture checker equals executor")
	}
	if _, err := env.eng.AttachVerifier(runID, []string{path}, checker, ""); err != nil {
		t.Fatal(err)
	}
	if ev, err := env.eng.EvaluateGate(fc, "qa", runID, checker); err != nil || ev.Verdict != "pass" {
		t.Fatalf("product QA: %+v %v", ev, err)
	}
	return path
}

func bindQAReviewEvidence(t *testing.T, env testEnv, runID, path string) {
	t.Helper()
	_, m, _, _ := env.eng.loadRun(runID)
	p := qaRegistration{Path: m.Verifier.Refs[0]}
	var r qaProductReview
	if err := qaRead(p.Path, &r); err != nil {
		t.Fatal(err)
	}
	r.EvidenceRefs = []string{path}
	raw, _ := json.Marshal(r)
	if err := os.WriteFile(p.Path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.RegisterArtifact(runID, "qa_report", p.Path, r.Checker); err != nil {
		t.Fatal(err)
	}
}
