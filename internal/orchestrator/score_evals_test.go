package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const demoBody = "---\nname: demo\ndescription: demo skill\n---\n\n# demo\n\n## HARD GATE\n\nNever fake it.\n"

func writeDemoPackage(t *testing.T, mutate func(dir string)) (dir, body string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(demoBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "references", "core.md"), []byte("stops at the release gate"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "examples"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"a.md", "b.md", "c.md"} {
		if err := os.WriteFile(filepath.Join(dir, "examples", n), []byte("# example"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "evals"), 0o755); err != nil {
		t.Fatal(err)
	}
	set := map[string]any{
		"schema": "skill-evals.v1",
		"skill":  "beacon-demo",
		"cases": []map[string]any{
			{"id": "c1", "prompt": "p1", "expectations": []string{"a", "b"},
				"checks": []map[string]any{{"kind": "contains", "text": "HARD GATE"}}},
			{"id": "c2", "prompt": "p2", "expectations": []string{"a", "b"},
				"checks": []map[string]any{{"kind": "references_contains", "text": "release gate"}}},
			{"id": "c3", "prompt": "p3", "expectations": []string{"a", "b"},
				"checks": []map[string]any{{"kind": "min_examples", "min": 3}}},
		},
	}
	raw, _ := json.Marshal(set)
	if err := os.WriteFile(filepath.Join(dir, "evals", "evals.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(dir)
	}
	return dir, demoBody
}

func itemByCheck(items []ScoreItem, name string) (ScoreItem, bool) {
	for _, it := range items {
		if it.Check == name {
			return it, true
		}
	}
	return ScoreItem{}, false
}

func TestEvalsItemsFullPackageScoresMax(t *testing.T) {
	dir, body := writeDemoPackage(t, nil)
	items := evalsItems(dir, body)
	for _, name := range []string{"evals_present", "evals_cases", "evals_checks_pass"} {
		it, ok := itemByCheck(items, name)
		if !ok {
			t.Fatalf("missing item %s", name)
		}
		if it.Points != it.MaxPoints {
			t.Errorf("%s = %v/%v (%s)", name, it.Points, it.MaxPoints, it.Detail)
		}
	}
}

func TestEvalsItemsMissingFileIsZero(t *testing.T) {
	dir, body := writeDemoPackage(t, func(dir string) {
		_ = os.Remove(filepath.Join(dir, "evals", "evals.json"))
	})
	it, ok := itemByCheck(evalsItems(dir, body), "evals_present")
	if !ok || it.Points != 0 {
		t.Fatalf("missing evals must score 0, got %+v", it)
	}
}

// A check that asserts something the package does not say must fail —
// this is the difference between "has evals" and "evals mean anything".
func TestEvalsItemsFailingCheckNamesTheCase(t *testing.T) {
	dir, body := writeDemoPackage(t, func(dir string) {
		p := filepath.Join(dir, "evals", "evals.json")
		raw, _ := os.ReadFile(p)
		var set map[string]any
		_ = json.Unmarshal(raw, &set)
		cases := set["cases"].([]any)
		first := cases[0].(map[string]any)
		first["checks"] = []any{map[string]any{"kind": "contains", "text": "a rule that does not exist"}}
		out, _ := json.Marshal(set)
		_ = os.WriteFile(p, out, 0o644)
	})
	it, ok := itemByCheck(evalsItems(dir, body), "evals_checks_pass")
	if !ok || it.Points != 0 {
		t.Fatalf("failing check must score 0, got %+v", it)
	}
	if !strings.Contains(it.Detail, "c1") {
		t.Fatalf("detail should name the failing case, got %q", it.Detail)
	}
}

func TestEvalsItemsRejectsWrongSchema(t *testing.T) {
	dir, body := writeDemoPackage(t, func(dir string) {
		p := filepath.Join(dir, "evals", "evals.json")
		_ = os.WriteFile(p, []byte(`{"schema":"something-else","cases":[]}`), 0o644)
	})
	it, _ := itemByCheck(evalsItems(dir, body), "evals_present")
	if it.Points != 0 {
		t.Fatalf("wrong schema must not count, got %+v", it)
	}
}
