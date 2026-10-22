// CLI tests for the unified skill score folded into the qa face
// (P4b): score writes + JSON shape, history reads back, the
// --apply prohibition, and the evaluator obligation.
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
)

const cliGoodSkill = `---
name: cli-skill
description: A small well-formed skill used by the qa score CLI tests.
---

# cli-skill

Purpose and workflow, written out in the document itself so the
structural checks have something real to read: it routes one kind
of request, keeps its boundaries explicit, and produces evidence.

## HARD GATE

Never self-certify. Never edit a scored skill from the score path.
`

func writeCLISkill(t *testing.T) string {
	t.Helper()
	pkg := filepath.Join(t.TempDir(), "cli-skill")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(pkg, "SKILL.md")
	if err := os.WriteFile(p, []byte(cliGoodSkill), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestQAScoreAndHistoryCLI(t *testing.T) {
	root, _ := seedGateProject(t, true)
	skill := writeCLISkill(t)
	out, err := runCLI(t, "qa", "score", skill,
		"--project", root, "--feature", "feat-gate", "--evaluator", "grader-1",
		"--human-score", "effectiveness=90", "--json")
	if err != nil {
		t.Fatalf("qa score: %v\n%s", err, out)
	}
	doc := parseDoc(t, out)
	assertKeys(t, doc, "schema", "gate_type", "grade", "corpus_schema", "boundary", "gate_result_id")
	if strField(t, doc, "schema") != "skill-score.v1" || strField(t, doc, "gate_type") != "skill_score" {
		t.Fatalf("doc identity wrong: %v", doc)
	}
	if strField(t, doc, "corpus_schema") != "skill-score-corpus.v1" {
		t.Fatalf("corpus schema = %v", doc["corpus_schema"])
	}
	if doc["auto_edit_skill_md"] != false {
		t.Fatalf("auto_edit_skill_md must be false, got %v", doc["auto_edit_skill_md"])
	}
	// Structural 100 (weight 60) + effectiveness 90 (weight 40)
	// -> total 96, grade A.
	if strField(t, doc, "grade") != "A" {
		t.Fatalf("grade = %v, want A (doc %v)", doc["grade"], doc)
	}
	// gate status must NOT list the score among the six gate types.
	out, err = runCLI(t, "gate", "status", "--project", root, "--feature", "feat-gate", "--json")
	if err != nil {
		t.Fatalf("gate status: %v", err)
	}
	gdoc := parseDoc(t, out)
	gates, _ := gdoc["gates"].([]any)
	if len(gates) != 6 {
		t.Fatalf("gate status must list exactly the six gate types, got %d", len(gates))
	}
	for _, g := range gates {
		if gm, _ := g.(map[string]any); gm["gate_type"] == "skill_score" {
			t.Fatalf("skill_score leaked into the gate status list")
		}
	}
	// History reads the record back.
	out, err = runCLI(t, "qa", "score-history", "--project", root, "--feature", "feat-gate", "--json")
	if err != nil {
		t.Fatalf("score-history: %v\n%s", err, out)
	}
	hdoc := parseDoc(t, out)
	scores, _ := hdoc["scores"].([]any)
	if len(scores) != 1 {
		t.Fatalf("history = %v, want exactly one card", hdoc["scores"])
	}
	first, _ := scores[0].(map[string]any)
	if first["evaluator"] != "grader-1" || first["target_name"] != "cli-skill" {
		t.Fatalf("history card lost attribution: %v", first)
	}
	// Human output works too.
	out, err = runCLI(t, "qa", "score-history", "--project", root, "--feature", "feat-gate")
	if err != nil || !strings.Contains(out, "cli-skill") {
		t.Fatalf("human history: err %v out %q", err, out)
	}
}

func TestQAScoreRefusalsCLI(t *testing.T) {
	root, _ := seedGateProject(t, true)
	skill := writeCLISkill(t)
	// --apply is forbidden outright (AC-170).
	_, err := runCLI(t, "qa", "score", skill, "--project", root, "--feature", "feat-gate",
		"--evaluator", "grader-1", "--apply")
	if ce, ok := err.(*core.Error); !ok || ce.ReasonCode != core.ReasonSkillAutoEditForbidden {
		t.Fatalf("--apply: err = %v, want skill_auto_edit_forbidden", err)
	}
	// No evaluator: refused before anything is recorded.
	_, err = runCLI(t, "qa", "score", skill, "--project", root, "--feature", "feat-gate")
	if ce, ok := err.(*core.Error); !ok || ce.ReasonCode != core.ReasonSkillScoreEvaluatorReq {
		t.Fatalf("no evaluator: err = %v, want skill_score_evaluator_required", err)
	}
	// Malformed human score.
	_, err = runCLI(t, "qa", "score", skill, "--project", root, "--feature", "feat-gate",
		"--evaluator", "grader-1", "--human-score", "effectiveness=lots")
	if ce, ok := err.(*core.Error); !ok || ce.ReasonCode != core.ReasonSkillHumanScoreInvalid {
		t.Fatalf("bad human score: err = %v, want skill_human_score_invalid", err)
	}
	// Nothing was recorded by the refused runs.
	out, err := runCLI(t, "qa", "score-history", "--project", root, "--feature", "feat-gate", "--json")
	if err != nil {
		t.Fatalf("score-history: %v", err)
	}
	hdoc := parseDoc(t, out)
	if scores, _ := hdoc["scores"].([]any); len(scores) != 0 {
		t.Fatalf("refused runs recorded scores: %v", hdoc["scores"])
	}
}
