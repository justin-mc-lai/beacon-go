// Unified skill score tests (P4b): dimension behaviour, the
// iron rules (never writes the target, never moves a gate,
// re-score determinism), history, and the fail-closed refusals.
package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

const goodSkillDoc = `---
name: good-skill
description: >
  A well-formed demonstration skill: does one thing, says when to
  use it, and keeps its detail in references.
---

# good-skill

Purpose: demonstrate the unified score's structural dimension on a
package that follows the skill-package standard. The body explains
the workflow, the boundaries and the evidence it produces, in the
document itself rather than in chat history alone.

## HARD GATE

Never start without a frozen truth package. Never self-certify QA.
Release stays a human decision; a score is not a passport.

## Workflow

Follow [the mode reference](references/mode.md) step by step.
`

const weakSkillDoc = `# no frontmatter here

tiny

See [the missing guide](missing-guide.md) for details.
`

// writeSkill lays a skill package on disk and returns the SKILL.md path.
func writeSkill(t *testing.T, dir, name, doc string, withRef bool) string {
	t.Helper()
	pkg := filepath.Join(dir, name)
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	if withRef {
		refDir := filepath.Join(pkg, "references")
		if err := os.MkdirAll(refDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(refDir, "mode.md"), []byte("# mode\n\nsteps\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p := filepath.Join(pkg, "SKILL.md")
	if err := os.WriteFile(p, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	// Current package standard: three worked examples plus eval cases
	// with machine checks. A "good skill" fixture must carry them,
	// otherwise the structural score is not comparable.
	exDir := filepath.Join(pkg, "examples")
	if err := os.MkdirAll(exDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"one.md", "two.md", "three.md"} {
		if err := os.WriteFile(filepath.Join(exDir, n), []byte("# example\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	evDir := filepath.Join(pkg, "evals")
	if err := os.MkdirAll(evDir, 0o755); err != nil {
		t.Fatal(err)
	}
	evals := `{"schema":"skill-evals.v1","skill":"` + name + `","cases":[
{"id":"c1","prompt":"p1","expectations":["a","b"],"checks":[{"kind":"contains","text":"## HARD GATE"}]},
{"id":"c2","prompt":"p2","expectations":["a","b"],"checks":[{"kind":"contains","text":"Never self-certify"}]},
{"id":"c3","prompt":"p3","expectations":["a","b"],"checks":[{"kind":"min_examples","min":3}]}]}`
	if err := os.WriteFile(filepath.Join(evDir, "evals.json"), []byte(evals), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func scoreCtx(t *testing.T, env testEnv) store.FeatureContext {
	t.Helper()
	ctx, err := env.st.ResolveFeature(env.root, "feat-a", "")
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

func dimOf(t *testing.T, card Scorecard, name string) ScoreDimension {
	t.Helper()
	for _, d := range card.Dimensions {
		if d.Dimension == name {
			return d
		}
	}
	t.Fatalf("card has no %s dimension: %+v", name, card.Dimensions)
	return ScoreDimension{}
}

func itemOf(t *testing.T, dim ScoreDimension, check string) ScoreItem {
	t.Helper()
	for _, it := range dim.Items {
		if it.Check == check {
			return it
		}
	}
	t.Fatalf("dimension %s has no item %s", dim.Dimension, check)
	return ScoreItem{}
}

func hashFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func TestScoreGoodSkillFullStructural(t *testing.T) {
	env := newTestEnv(t, "active")
	ctx := scoreCtx(t, env)
	path := writeSkill(t, t.TempDir(), "good-skill", goodSkillDoc, true)
	card, err := env.eng.ScoreSkill(ctx, ScoreOptions{Target: path, Evaluator: "grader-1"})
	if err != nil {
		t.Fatal(err)
	}
	structural := dimOf(t, card, "structural")
	if !structural.Scored || structural.Score == nil || *structural.Score != 100 {
		t.Fatalf("structural = %+v, want a full 100", structural)
	}
	for _, it := range structural.Items {
		if it.Applicable && it.Points != it.MaxPoints {
			t.Errorf("item %s = %g/%g, want full points (%s)", it.Check, it.Points, it.MaxPoints, it.Detail)
		}
	}
	eff := dimOf(t, card, "effectiveness")
	if eff.Scored {
		t.Fatalf("effectiveness must stay unscored without human input: %+v", eff)
	}
	if len(eff.Rubric) != 7 {
		t.Errorf("effectiveness rubric = %v, want the 7 v1 SCORING_DIMENSIONS", eff.Rubric)
	}
	if len(card.Incomplete) != 1 || card.Incomplete[0] != "effectiveness" {
		t.Errorf("incomplete = %v", card.Incomplete)
	}
	if card.TotalScore == nil || *card.TotalScore != 100 || card.Grade != "A" {
		t.Fatalf("total/grade = %v/%s, want 100/A over the scored dimensions", card.TotalScore, card.Grade)
	}
	if card.CorpusSchema != ScoreCorpusSchema || card.CorpusCases == 0 || card.CorpusSHA256 == "" {
		t.Errorf("corpus binding incomplete: %+v", card)
	}
	if card.Authority != ScoreAuthority || card.AutoEdit {
		t.Errorf("authority/auto_edit = %s/%v, want runtime_evidence_only/false", card.Authority, card.AutoEdit)
	}
	if card.TargetSHA256 != hashFile(t, path) {
		t.Errorf("card hash does not match the file on disk")
	}
	// Persisted: gate_results row under skill_score, apart from the gates.
	row, err := env.st.Gates().Latest(env.feature.ID, ScoreGateType)
	if err != nil {
		t.Fatal(err)
	}
	if row.Verdict != "A" || row.ID != card.GateResultID {
		t.Fatalf("persisted row = %+v", row)
	}
	latest, err := env.st.Gates().LatestByType(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range latest {
		if g.GateType != ScoreGateType {
			t.Fatalf("no real gate may appear from scoring, got %+v", g)
		}
	}
}

func TestScoreWeakSkillItemsFail(t *testing.T) {
	env := newTestEnv(t, "active")
	ctx := scoreCtx(t, env)
	path := writeSkill(t, t.TempDir(), "weak-skill", weakSkillDoc, false)
	card, err := env.eng.ScoreSkill(ctx, ScoreOptions{Target: path, Evaluator: "grader-1"})
	if err != nil {
		t.Fatal(err)
	}
	structural := dimOf(t, card, "structural")
	for _, check := range []string{
		"frontmatter_present", "frontmatter_identity", "name_matches_package",
		"hard_gate_declared", "references_resolve",
	} {
		if it := itemOf(t, structural, check); it.Points != 0 {
			t.Errorf("item %s = %g points, want 0 (%s)", check, it.Points, it.Detail)
		}
	}
	// The doc is 83 chars: the v1 minimal band (>= 80) gives
	// body_substantial 1 of 2 rather than 0.
	if it := itemOf(t, structural, "body_substantial"); it.Points != 1 {
		t.Errorf("body_substantial = %g, want the 1-point minimal band", it.Points)
	}
	// The H1-looking first line is a real H1, so title_present
	// passes; progressive passes too (short file). Earned 3 of 13
	// -> far below the C band.
	if card.Grade != "D" {
		t.Fatalf("grade = %s (total %v), want D", card.Grade, card.TotalScore)
	}
	for _, want := range []string{"structural:hard_gate_declared", "structural:references_resolve", "structural:frontmatter_identity"} {
		found := false
		for _, w := range card.Weaknesses {
			if w == want {
				found = true
			}
		}
		if !found {
			t.Errorf("weaknesses %v missing %s", card.Weaknesses, want)
		}
	}
	if len(card.Suggestions) == 0 {
		t.Errorf("a weak skill must carry advisory suggestions")
	}
	for _, s := range card.Suggestions {
		if !s.Advisory {
			t.Errorf("suggestion %+v must be advisory", s)
		}
	}
}

func TestScoreHumanEffectiveness(t *testing.T) {
	env := newTestEnv(t, "active")
	ctx := scoreCtx(t, env)
	path := writeSkill(t, t.TempDir(), "good-skill", goodSkillDoc, true)

	card, err := env.eng.ScoreSkill(ctx, ScoreOptions{
		Target: path, Evaluator: "grader-1", HumanScores: map[string]float64{"effectiveness": 50},
	})
	if err != nil {
		t.Fatal(err)
	}
	eff := dimOf(t, card, "effectiveness")
	if !eff.Scored || *eff.Score != 50 || eff.Source != "human_input" {
		t.Fatalf("effectiveness = %+v, want scored 50 by human", eff)
	}
	// (100*60 + 50*40) / 100 = 80 -> B.
	if card.TotalScore == nil || *card.TotalScore != 80 || card.Grade != "B" {
		t.Fatalf("total/grade = %v/%s, want 80/B", card.TotalScore, card.Grade)
	}
	if len(card.Incomplete) != 0 {
		t.Errorf("incomplete = %v, want empty", card.Incomplete)
	}

	// Per-rubric grades average over the keys provided.
	card2, err := env.eng.ScoreSkill(ctx, ScoreOptions{
		Target: path, Evaluator: "grader-2",
		HumanScores: map[string]float64{"route_stability": 100, "boundary_preservation": 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	eff2 := dimOf(t, card2, "effectiveness")
	if !eff2.Scored || *eff2.Score != 50 || len(eff2.Items) != 2 {
		t.Fatalf("rubric effectiveness = %+v, want average 50 over 2 graded keys", eff2)
	}
}

func TestScoreNeverWritesTarget(t *testing.T) {
	env := newTestEnv(t, "active")
	ctx := scoreCtx(t, env)
	path := writeSkill(t, t.TempDir(), "weak-skill", weakSkillDoc, false)
	before := hashFile(t, path)
	if _, err := env.eng.ScoreSkill(ctx, ScoreOptions{Target: path, Evaluator: "grader-1"}); err != nil {
		t.Fatal(err)
	}
	if after := hashFile(t, path); after != before {
		t.Fatalf("SKILL.md changed under scoring: %s -> %s (AC-154 violated)", before, after)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != weakSkillDoc {
		t.Fatalf("SKILL.md bytes were modified")
	}
}

func TestScoreDoesNotInfluenceGates(t *testing.T) {
	env := newTestEnv(t, "active")
	ctx := scoreCtx(t, env)
	good := writeSkill(t, t.TempDir(), "good-skill", goodSkillDoc, true)
	weak := writeSkill(t, t.TempDir(), "weak-skill", weakSkillDoc, false)

	// Direction 1: a grade-A score must not mint a qa pass.
	evBefore, err := env.eng.EvaluateGate(ctx, "qa", "", "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if evBefore.Verdict != "blocked" {
		t.Fatalf("precondition: qa without evidence must be blocked, got %s", evBefore.Verdict)
	}
	if _, err := env.eng.ScoreSkill(ctx, ScoreOptions{Target: good, Evaluator: "grader-1"}); err != nil {
		t.Fatal(err)
	}
	evAfter, err := env.eng.EvaluateGate(ctx, "qa", "", "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if evAfter.Verdict != "blocked" || strings.Join(evAfter.ReasonCodes, ",") != strings.Join(evBefore.ReasonCodes, ",") {
		t.Fatalf("qa verdict moved under a high score: before %v after %v", evBefore.ReasonCodes, evAfter.ReasonCodes)
	}

	// Direction 2: a grade-D score must not revoke an existing pass.
	env.seedQAPass(t)
	if _, err := env.eng.ScoreSkill(ctx, ScoreOptions{Target: weak, Evaluator: "grader-1"}); err != nil {
		t.Fatal(err)
	}
	qa, err := env.st.Gates().Latest(env.feature.ID, "qa")
	if err != nil {
		t.Fatal(err)
	}
	if qa.Verdict != "pass" {
		t.Fatalf("qa pass was revoked by a low score: %+v", qa)
	}
	// Admission is equally indifferent to scores.
	run := env.createRun(t)
	admBefore, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.ScoreSkill(ctx, ScoreOptions{Target: good, Evaluator: "grader-1"}); err != nil {
		t.Fatal(err)
	}
	admAfter, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(admBefore.ReasonCodes, ",") != strings.Join(admAfter.ReasonCodes, ",") {
		t.Fatalf("admission moved under scoring: before %v after %v", admBefore.ReasonCodes, admAfter.ReasonCodes)
	}
}

func TestScoreHistoryAndRescoreDeterminism(t *testing.T) {
	env := newTestEnv(t, "active")
	ctx := scoreCtx(t, env)
	path := writeSkill(t, t.TempDir(), "good-skill", goodSkillDoc, true)
	first, err := env.eng.ScoreSkill(ctx, ScoreOptions{Target: path, Evaluator: "grader-1"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := env.eng.ScoreSkill(ctx, ScoreOptions{Target: path, Evaluator: "grader-2"})
	if err != nil {
		t.Fatal(err)
	}
	// AC-170: an unchanged package re-scores identically — the
	// first card's suggestions existing must not raise the score.
	s1 := dimOf(t, first, "structural")
	s2 := dimOf(t, second, "structural")
	if *s1.Score != *s2.Score || first.Grade != second.Grade {
		t.Fatalf("re-score drifted: %v/%s -> %v/%s", *s1.Score, first.Grade, *s2.Score, second.Grade)
	}
	hist, err := env.eng.ScoreHistory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 2 {
		t.Fatalf("history = %d cards, want 2", len(hist))
	}
	if hist[0].GateResultID != first.GateResultID || hist[1].GateResultID != second.GateResultID {
		t.Fatalf("history order wrong: %s then %s", hist[0].GateResultID, hist[1].GateResultID)
	}
	if hist[0].Evaluator != "grader-1" || hist[1].Evaluator != "grader-2" {
		t.Fatalf("history lost evaluator attribution: %+v", hist)
	}
	if hist[0].TargetName != "good-skill" || hist[0].CorpusSchema != ScoreCorpusSchema {
		t.Fatalf("history card lost detail: %+v", hist[0])
	}
}

func TestScoreRefusals(t *testing.T) {
	env := newTestEnv(t, "active")
	ctx := scoreCtx(t, env)
	path := writeSkill(t, t.TempDir(), "good-skill", goodSkillDoc, true)
	cases := []struct {
		name string
		opts ScoreOptions
		code string
	}{
		{"no evaluator", ScoreOptions{Target: path}, core.ReasonSkillScoreEvaluatorReq},
		{"corpus missing", ScoreOptions{Target: path, Evaluator: "g", CorpusPath: filepath.Join(t.TempDir(), "nope.json")}, core.ReasonSkillScoreCorpusMissing},
		{"human out of range", ScoreOptions{Target: path, Evaluator: "g", HumanScores: map[string]float64{"effectiveness": 120}}, core.ReasonSkillHumanScoreInvalid},
		{"human unknown key", ScoreOptions{Target: path, Evaluator: "g", HumanScores: map[string]float64{"vibes": 10}}, core.ReasonSkillHumanScoreInvalid},
		{"target missing", ScoreOptions{Target: filepath.Join(t.TempDir(), "no-such-skill"), Evaluator: "g"}, core.ReasonSkillTargetNotFound},
	}
	for _, tc := range cases {
		_, err := env.eng.ScoreSkill(ctx, tc.opts)
		if err == nil {
			t.Errorf("%s: expected refusal %s, got success", tc.name, tc.code)
			continue
		}
		if ce, ok := err.(*core.Error); !ok || ce.ReasonCode != tc.code {
			t.Errorf("%s: error = %v, want reason %s", tc.name, err, tc.code)
		}
	}
	// A corpus file with the wrong schema is equally fail-closed.
	bad := filepath.Join(t.TempDir(), "corpus.json")
	if err := os.WriteFile(bad, []byte(`{"schema":"something-else","cases":[{}],"rubric":["x"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := env.eng.ScoreSkill(ctx, ScoreOptions{Target: path, Evaluator: "g", CorpusPath: bad})
	if ce, ok := err.(*core.Error); !ok || ce.ReasonCode != core.ReasonSkillScoreCorpusMissing {
		t.Fatalf("bad-schema corpus: err = %v, want skill_score_corpus_missing", err)
	}
}

func TestScoreTargetByName(t *testing.T) {
	env := newTestEnv(t, "active")
	ctx := scoreCtx(t, env)
	writeSkill(t, filepath.Join(env.root, "skills"), "named-skill",
		strings.Replace(goodSkillDoc, "good-skill", "named-skill", -1), true)
	card, err := env.eng.ScoreSkill(ctx, ScoreOptions{Target: "named-skill", Evaluator: "grader-1"})
	if err != nil {
		t.Fatal(err)
	}
	if card.TargetName != "named-skill" || !strings.HasSuffix(card.TargetPath, filepath.Join("skills", "named-skill", "SKILL.md")) {
		t.Fatalf("name resolution landed wrong: %+v", card)
	}
}
