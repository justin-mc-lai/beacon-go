package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// M2 + M1: the qa strategy and agent-eval faces are file
// surfaces — no store needed.
func TestQAStrategyAndAgentEvalCLI(t *testing.T) {
	root := t.TempDir()
	out, err := runCLI(t, "qa", "strategy", "--project", root, "--json")
	if err != nil {
		t.Fatalf("qa strategy: %v (%s)", err, out)
	}
	doc := parseDoc(t, out)
	if doc["project_type"] != "sdk-lib" || doc["schema_version"] != "1.0.2" {
		t.Fatalf("strategy doc = %v", doc)
	}
	if _, err := os.Stat(filepath.Join(root, ".beacon", "qa-strategy.json")); err != nil {
		t.Fatalf("strategy not persisted: %v", err)
	}

	out, err = runCLI(t, "qa", "agent-eval", "catalog", "--json")
	if err != nil {
		t.Fatalf("catalog: %v (%s)", err, out)
	}
	if n, _ := parseDoc(t, out)["entry_count"].(float64); n == 0 {
		t.Fatalf("catalog doc = %s", out)
	}

	transcript := filepath.Join(root, "transcript.txt")
	if err := os.WriteFile(transcript, []byte("ROUTE: qa\nwork done\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = runCLI(t, "qa", "agent-eval", "grade", "--project", root,
		"--agent", "cli-bot", "--transcript", transcript, "--expect-route", "qa", "--json")
	if err != nil {
		t.Fatalf("grade: %v (%s)", err, out)
	}
	grade, _ := parseDoc(t, out)["grade"].(map[string]any)
	if grade["passed"] != true {
		t.Fatalf("grade doc = %s", out)
	}

	out, err = runCLI(t, "qa", "agent-score", "--project", root, "--agent", "cli-bot", "--json")
	if err != nil {
		t.Fatalf("agent-score: %v (%s)", err, out)
	}
	if parseDoc(t, out)["agent"] != "cli-bot" {
		t.Fatalf("score doc = %s", out)
	}

	out, err = runCLI(t, "qa", "agent-eval", "admission", "--project", root, "--agent", "cli-bot", "--json")
	if err != nil {
		t.Fatalf("admission: %v (%s)", err, out)
	}
	if s := parseDoc(t, out)["state"]; s != "admitted" && s != "warn" {
		t.Fatalf("admission doc = %s", out)
	}

	out, err = runCLI(t, "qa", "agent-eval", "history", "--project", root, "--json")
	if err != nil {
		t.Fatalf("history: %v (%s)", err, out)
	}
	if cards, _ := parseDoc(t, out)["scorecards"].([]any); len(cards) != 1 {
		t.Fatalf("history doc = %s", out)
	}
}

// M3 + M4 + M13 + M14 over the truth face.
func TestTruthR2dFacesCLI(t *testing.T) {
	root := seedP4AProject(t)
	slug := "r2d-cli"
	if _, err := runCLI(t, "truth", "create", "--project", root, "--slug", slug, "--json"); err != nil {
		t.Fatalf("create: %v", err)
	}
	out, err := runCLI(t, "truth", "decompose", "--project", root, "--feature", slug,
		"--utterance", "用户能注册登录，支付订阅后开通权益，还能导出报表。", "--json")
	if err != nil {
		t.Fatalf("decompose: %v (%s)", err, out)
	}
	doc := parseDoc(t, out)
	if acs, _ := doc["materialized_acs"].([]any); len(acs) < 3 {
		t.Fatalf("decompose doc = %s", out)
	}
	decomp, _ := doc["decomposition"].(map[string]any)
	if journeys, _ := decomp["journeys"].([]any); len(journeys) < 2 {
		t.Fatalf("decomposition journeys = %s", out)
	}

	out, err = runCLI(t, "truth", "brainstorm", "--project", root, "--feature", slug, "--json")
	if err != nil {
		t.Fatalf("brainstorm: %v (%s)", err, out)
	}
	bdoc := parseDoc(t, out)
	if n, _ := bdoc["generated_count"].(float64); n < 4 {
		t.Fatalf("brainstorm doc = %s", out)
	}
	if bdoc["converged"] != false {
		t.Fatalf("fresh brainstorm cannot be converged: %s", out)
	}

	// M13: scan a fixture codebase into a second package.
	target := t.TempDir()
	for rel, body := range map[string]string{
		"main.go":      "package main\n\nfunc ExportedRun() {}\n",
		"test_main.py": "def test_boot():\n    pass\n",
		"README.md":    "# legacy\n",
	} {
		if err := os.WriteFile(filepath.Join(target, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err = runCLI(t, "truth", "code2truth", "--project", root,
		"--feature", "legacy-cli", "--target", target, "--json")
	if err != nil {
		t.Fatalf("code2truth: %v (%s)", err, out)
	}
	cdoc := parseDoc(t, out)
	if cdoc["package_created"] != true || cdoc["truth_authority"] != false {
		t.Fatalf("code2truth doc = %s", out)
	}

	// M14: freeze the decomposed package through the normal
	// intent route, then reopen it.
	truthPath := filepath.Join(root, "docs", "beacon", "v2.0.0", "features", slug, "truth.md")
	raw, err := os.ReadFile(truthPath)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw),
		"（一句话说清这个需求解决什么问题、给谁用、做到什么算完。冻结前由作者补齐。）",
		"测试需求：目标、用户与完成标准。", 1))
	if err := os.WriteFile(truthPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "truth", "intent", "--project", root, "--feature", slug,
		"--utterance", "用户能注册登录，支付订阅后开通权益，还能导出报表。",
		"--goal", "deliver it", "--non-goals", "not that", "--constraints", "local", "--json"); err != nil {
		t.Fatalf("intent: %v", err)
	}
	out, err = runCLI(t, "truth", "questions", "--project", root, "--feature", slug, "--json")
	if err != nil {
		t.Fatalf("questions: %v", err)
	}
	qs, _ := parseDoc(t, out)["open_questions"].([]any)
	for _, q := range qs {
		id, _ := q.(map[string]any)["id"].(string)
		if _, err := runCLI(t, "truth", "answer", "--project", root, "--feature", slug,
			"--id", id, "--answer", "answered", "--json"); err != nil {
			t.Fatalf("answer %s: %v", id, err)
		}
	}
	if _, err := runCLI(t, "truth", "confirm", "--project", root, "--feature", slug, "--json"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if _, err := runCLI(t, "truth", "freeze", "--project", root, "--feature", slug, "--json"); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	out, err = runCLI(t, "truth", "reopen", "--project", root, "--feature", slug,
		"--reason", "需求变了", "--json")
	if err != nil {
		t.Fatalf("reopen: %v (%s)", err, out)
	}
	rdoc := parseDoc(t, out)
	if rdoc["status"] != "draft" || rdoc["new_revision_id"] != "R2" || rdoc["previous_revision_id"] != "R1" {
		t.Fatalf("reopen doc = %s", out)
	}
}
