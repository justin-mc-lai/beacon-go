package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillsListAndShow(t *testing.T) {
	out, err := runCLI(t, "skills", "list", "--json")
	if err != nil {
		t.Fatalf("skills list: %v", err)
	}
	doc := parseDoc(t, out)
	list, ok := doc["skills"].([]any)
	if !ok || len(list) != 6 {
		t.Fatalf("skills list = %v, want 6", doc)
	}
	out, err = runCLI(t, "skills", "show", "truth")
	if err != nil {
		t.Fatalf("skills show: %v", err)
	}
	if !strings.Contains(out, "ask -> sign -> freeze") {
		t.Fatalf("truth skill body missing the chain:\n%s", out)
	}
	if _, err := runCLI(t, "skills", "show", "nope"); err == nil {
		t.Fatal("unknown skill must fail")
	}
}

func TestTruthAskConfirmDriftCLI(t *testing.T) {
	root := seedP4AProject(t)
	slug := "ask-cli"
	if _, err := runCLI(t, "truth", "create", "--project", root, "--slug", slug, "--json"); err != nil {
		t.Fatalf("create: %v", err)
	}
	truthPath := filepath.Join(root, "docs", "beacon", "v2.0.0", "features", slug, "truth.md")
	if raw, err := os.ReadFile(truthPath); err == nil {
		raw = []byte(strings.Replace(string(raw),
			"（一句话说清这个需求解决什么问题、给谁用、做到什么算完。冻结前由作者补齐。）",
			"测试需求：目标、用户与完成标准。", 1))
		if err := os.WriteFile(truthPath, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := runCLI(t, "truth", "ask", "--project", root, "--feature", slug,
		"--question", "What is explicitly out of scope?", "--json")
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if !strings.Contains(out, "What is explicitly out of scope?") {
		t.Fatalf("ask output = %s", out)
	}
	if _, err := runCLI(t, "truth", "confirm", "--project", root, "--feature", slug, "--json"); err == nil {
		t.Fatal("confirm before answering must fail")
	}
	if _, err := runCLI(t, "truth", "intent", "--project", root, "--feature", slug,
		"--utterance", "user words", "--goal", "deliver it",
		"--non-goals", "not that", "--constraints", "local", "--json"); err != nil {
		t.Fatalf("intent: %v", err)
	}
	out, err = runCLI(t, "truth", "questions", "--project", root, "--feature", slug, "--json")
	if err != nil {
		t.Fatalf("questions: %v", err)
	}
	doc := parseDoc(t, out)
	qs, _ := doc["open_questions"].([]any)
	if len(qs) < 2 {
		t.Fatalf("questions = %v, want seeded + asked", doc)
	}
	for _, q := range qs {
		id, _ := q.(map[string]any)["id"].(string)
		if id == "" {
			t.Fatalf("question row has no ID: %v", q)
		}
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
	raw, err := os.ReadFile(truthPath)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(raw), "作者写下第一条可验证的完成标准", "作者写下第一条可验证的完成标准（改）", 1)
	if err := os.WriteFile(truthPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = runCLI(t, "truth", "drift", "--project", root, "--feature", slug, "--json")
	if err != nil {
		t.Fatalf("drift: %v", err)
	}
	doc = parseDoc(t, out)
	if doc["clean"] != false || doc["body_drift"] != true {
		t.Fatalf("drift = %v", doc)
	}
	changed, _ := doc["changed"].([]any)
	if len(changed) != 1 || changed[0] != "AC-001" {
		t.Fatalf("changed = %v, want [AC-001]", doc["changed"])
	}
}
