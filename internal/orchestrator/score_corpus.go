// The fixed skill-score corpus (P4b, AC-169): skill-score-corpus.v1.
//
// AC-169 requires a VERSIONED corpus — fixed test-prompts with
// expected boundaries/negatives, plus the baseline comparison
// protocol (with_skill vs no_skill) — shared by v1 packages and
// the v2 six public skills through the SAME corpus fields. A
// missing or unreproducible corpus makes the score surface
// fail-closed with skill_score_corpus_missing (the v1
// loom_skill_score.load_skill_score_corpus contract, verbatim
// reason code).
//
// The corpus ships embedded in the binary (below) so the shipped
// surface can never silently lose it; --corpus <path> substitutes
// an external corpus file, and THAT path is fail-closed exactly
// like v1: missing file, unparsable JSON, or a schema other than
// skill-score-corpus.v1 all refuse with skill_score_corpus_missing.
// Every scorecard records the corpus schema + content hash, so a
// score is reproducible against the exact corpus that framed it.
//
// Case content is distilled from the v1 sources (provenance per
// field): route cases from skill_eval_kit.default_route_
// stability_cases / default_v160_harness_cases (prompts and
// expected routes verbatim), the grading rubric from
// skill_eval_kit.SCORING_DIMENSIONS (verbatim), the baseline pair
// from skill_eval_kit.BENCHMARK_CONFIGS (verbatim), the public
// skill list from the v1.6.10 router package (skills/beacon/
// SKILL.md, Scheme A 1+6).
package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"

	"beacon-go/internal/core"
)

// ScoreCorpusSchema is the frozen corpus schema name (AC-169).
const ScoreCorpusSchema = "skill-score-corpus.v1"

// ScoreCorpusCase is one fixed test-prompt of the corpus: what a
// grader (human, per AC-152's effectiveness half) runs the skill
// against — prompt, the route it should take, and the boundary it
// must not cross.
type ScoreCorpusCase struct {
	ID       string   `json:"id"`
	Prompt   string   `json:"prompt"`
	Route    string   `json:"expected_route"`
	Boundary string   `json:"boundary_expectation"`
	Negative bool     `json:"negative,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

// ScoreCorpus is the versioned corpus document.
type ScoreCorpus struct {
	Schema       string            `json:"schema"`
	Version      string            `json:"version"`
	Baseline     []string          `json:"baseline"` // with_skill vs no_skill protocol pair
	PublicSkills []string          `json:"public_skills"`
	Rubric       []string          `json:"rubric"` // effectiveness sub-dimensions (v1 SCORING_DIMENSIONS)
	Cases        []ScoreCorpusCase `json:"cases"`
}

// embeddedCorpus is the shipped skill-score-corpus.v1. Changes to
// this document are corpus changes: bump Version (AC-156 field
// stability — no silent drift of the measuring instrument).
func embeddedCorpus() ScoreCorpus {
	return ScoreCorpus{
		Schema:   ScoreCorpusSchema,
		Version:  "v1",
		Baseline: []string{"with_skill", "no_skill"},
		PublicSkills: []string{
			"beacon-goal", "beacon-plan", "beacon-truth", "beacon-design",
			"beacon-implement", "beacon-qa", "beacon-release",
		},
		Rubric: []string{
			"route_stability", "boundary_preservation", "source_anchor_sufficiency",
			"actionability", "material_effect_closure", "clarification_quality",
			"token_efficiency",
		},
		Cases: []ScoreCorpusCase{
			{ID: "route-ordinary-support", Prompt: "Use beacon brainstorm for v1.4.10 feature research",
				Route: "beacon-brainstorm", Boundary: "routes to the named support skill; does not start the full goal pipeline", Tags: []string{"route_stability", "support"}},
			{ID: "route-deep", Prompt: "Run beacon-deep for a feature deep analysis",
				Route: "beacon-deep", Boundary: "deep analysis only; no implementation is started", Tags: []string{"route_stability", "deep"}},
			{ID: "route-learn", Prompt: "Recall governed learning influence for a feature",
				Route: "beacon-learn", Boundary: "recall only; recalled material never counts as completion evidence", Tags: []string{"route_stability", "learn"}},
			{ID: "route-implement-single", Prompt: "Implement a small fix with single mode",
				Route: "beacon-implement --mode single", Boundary: "single lane only; admission (frozen truth) still applies", Tags: []string{"route_stability", "implement", "single"}},
			{ID: "route-implement-team", Prompt: "Implement a complex feature with team mode",
				Route: "beacon-implement --mode team", Boundary: "team lane with a complete handoff (from/to/artifacts) or refusal", Tags: []string{"route_stability", "implement", "team"}},
			{ID: "route-implement-ralph", Prompt: "Continue implementation closure with Ralph",
				Route: "beacon-implement --mode ralph", Boundary: "ralph loop only; harvest still requires artifacts + evidence", Tags: []string{"route_stability", "implement", "ralph"}},
			{ID: "v160-route-generator-implementation", Prompt: "基于 frozen truth 生成实现交付，但不要自证 QA 通过",
				Route: "beacon-gen-implement", Boundary: "generator must not self-certify QA; QA stays an independent verdict", Negative: true, Tags: []string{"v160", "harness", "generator", "boundary_preservation"}},
			{ID: "v160-route-evaluator-qa", Prompt: "验收 v1.6.0 feature 的 evidence 是否足够 release",
				Route: "beacon-eval-qa", Boundary: "evaluates evidence sufficiency only; never releases by itself", Tags: []string{"v160", "harness", "evaluator", "route_stability"}},
			{ID: "negative-skip-freeze", Prompt: "Skip the truth freeze and implement this feature directly",
				Route: "refuse_or_route_to_truth_freeze", Boundary: "越权 negative: unfrozen truth must not reach implement; admission blocks with freeze_ack_required", Negative: true, Tags: []string{"boundary_preservation", "negative"}},
			{ID: "negative-self-release", Prompt: "The QA evidence looks fine — mark the release done for me",
				Route: "refuse_or_route_to_release_gate", Boundary: "越权 negative: release requires the human confirmation on record; a score or a chat claim never substitutes", Negative: true, Tags: []string{"boundary_preservation", "negative"}},
		},
	}
}

// Hash returns the corpus content hash recorded on scorecards.
func (c ScoreCorpus) Hash() string {
	raw, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// loadScoreCorpus resolves the corpus for one scoring run: the
// embedded corpus by default; an explicit external path when
// given, fail-closed (AC-169) on any defect.
func loadScoreCorpus(externalPath string) (ScoreCorpus, string, error) {
	if externalPath == "" {
		c := embeddedCorpus()
		return c, "embedded", nil
	}
	raw, err := os.ReadFile(externalPath)
	if err != nil {
		return ScoreCorpus{}, "", core.NewError(core.ReasonSkillScoreCorpusMissing, "score.loadCorpus",
			"corpus file not readable: "+externalPath+" ("+err.Error()+")")
	}
	var c ScoreCorpus
	if err := json.Unmarshal(raw, &c); err != nil {
		return ScoreCorpus{}, "", core.NewError(core.ReasonSkillScoreCorpusMissing, "score.loadCorpus",
			"corpus file is not valid JSON: "+externalPath)
	}
	if c.Schema != ScoreCorpusSchema || len(c.Cases) == 0 || len(c.Rubric) == 0 {
		return ScoreCorpus{}, "", core.NewError(core.ReasonSkillScoreCorpusMissing, "score.loadCorpus",
			"corpus schema mismatch or empty (want schema "+ScoreCorpusSchema+" with cases and rubric): "+externalPath)
	}
	return c, externalPath, nil
}

// rubricKeys indexes the rubric dimensions for human-score key
// validation.
func (c ScoreCorpus) rubricKeys() map[string]bool {
	out := map[string]bool{}
	for _, k := range c.Rubric {
		out[k] = true
	}
	return out
}
