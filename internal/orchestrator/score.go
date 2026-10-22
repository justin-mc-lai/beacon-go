// Unified skill score (P4b): the ONE scoring engine of the Go
// kernel, replacing v1's three kits (skill_eval_kit /
// agent_eval_kit / loom_skill_score) under the frozen contract
// AC-151..156 + AC-169/170. It folds into the qa ability face
// (`beacon qa score`) — no new top-level command (charter
// anti-bloat rule).
//
// Schema skill-score.v1 (stable fields, AC-156 — weights and
// fields change only via change, never silently):
//
//   - structural （结构质量， AC-152 first half): deterministic
//     static checks over the SKILL.md package — frontmatter,
//     identity, HARD GATE declaration, progressive disclosure,
//     reference validity. Aligned with skill-creator / the Beacon
//     skill-package-standard; item thresholds distilled from v1
//     loom_skill_score._content_structure_points where v1 had
//     them (body length 200/80), the rest defined here and
//     recorded in the p4b report's provenance table.
//   - effectiveness （实效质量， AC-152 second half): routing /
//     boundary / 越权-negative behaviour on the corpus's fixed
//     test-prompts, with_skill vs no_skill. This half is NOT
//     mechanically decidable — it needs a model run — so the
//     engine never invents it: a human grader supplies it via
//     --human-score (the dimension itself, or per-rubric values
//     over the v1 SCORING_DIMENSIONS rubric), and without input
//     the dimension stays visibly not_scored.
//
// Iron rules, structural (not policy prose):
//
//  1. Score-only, human-edits (AC-154/170): this file contains no
//     write call against the target — the target is read once,
//     hashed, and judged in memory. Suggestions are advisory
//     data on the card; there is no apply path anywhere (the CLI
//     refuses --apply with skill_auto_edit_forbidden).
//  2. runtime_evidence_only (AC-153): the card declares its
//     boundary on its face — a score is a health check, never a
//     gate verdict. The record lands in gate_results under its
//     own gate_type (skill_score) + a skill_scored trail event,
//     listed apart from the four gates + planner_review.
//  3. No gate path reads scores: admission, exit gates, the five
//     gate evaluators and Complete never consult gate_type
//     skill_score (their lookups are all type-scoped), so a high
//     score cannot mint a pass and a low score cannot revoke
//     one. Behavioural tests pin both directions.
//  4. Re-scoring an unchanged package reproduces the identical
//     structural score (AC-170): the checks are pure functions of
//     the bytes, and a suggestion's existence is not an input.
package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// Score surface constants.
const (
	ScoreSchema       = "skill-score.v1"
	ScoreGateType     = "skill_score"
	EventSkillScored  = "skill_scored"
	ScoreAuthority    = "runtime_evidence_only"
	ScoreBoundaryText = "score_is_not_a_gate: this scorecard is runtime evidence only (AC-153); it does not substitute formal QA (qa gate) or the human release decision, and no gate, admission or completion reads it"
	// Dimension weights of the unified schema (AC-156: stable —
	// changing them is a change, not an edit).
	ScoreWeightStructural    = 60.0
	ScoreWeightEffectiveness = 40.0
)

// ScoreItem is one deterministic check inside a dimension.
type ScoreItem struct {
	Check      string  `json:"check"`
	Points     float64 `json:"points"`
	MaxPoints  float64 `json:"max_points"`
	Applicable bool    `json:"applicable"`
	Detail     string  `json:"detail,omitempty"`
}

// ScoreDimension is one axis of the unified schema.
type ScoreDimension struct {
	Dimension string      `json:"dimension"` // structural | effectiveness
	Scored    bool        `json:"scored"`
	Score     *float64    `json:"score,omitempty"` // 0..100 when scored
	Weight    float64     `json:"weight"`
	Source    string      `json:"source"` // mechanical_static | human_input | not_scored
	Items     []ScoreItem `json:"items,omitempty"`
	Rubric    []string    `json:"rubric,omitempty"` // human rubric when not mechanically scored
	Note      string      `json:"note,omitempty"`
}

// ScoreSuggestion is advisory only (AC-154/170): a human may act
// on it by editing the package themselves; nothing applies it.
type ScoreSuggestion struct {
	Dimension string `json:"dimension"`
	Check     string `json:"check"`
	Advice    string `json:"advice"`
	Advisory  bool   `json:"advisory"`
}

// Scorecard is one persisted scoring record.
type Scorecard struct {
	Schema        string            `json:"schema"`
	GateType      string            `json:"gate_type"`
	TargetType    string            `json:"target_type"` // skill
	TargetPath    string            `json:"target_path"`
	TargetName    string            `json:"target_name"`
	PackageDir    string            `json:"package_dir"`
	TargetSHA256  string            `json:"target_sha256"`
	Evaluator     string            `json:"evaluator"`
	Dimensions    []ScoreDimension  `json:"dimensions"`
	TotalScore    *float64          `json:"total_score,omitempty"`
	Grade         string            `json:"grade,omitempty"` // A | B | C | D
	Incomplete    []string          `json:"incomplete_dimensions,omitempty"`
	Weaknesses    []string          `json:"weaknesses,omitempty"`
	Suggestions   []ScoreSuggestion `json:"suggestions,omitempty"`
	CorpusSchema  string            `json:"corpus_schema"`
	CorpusVersion string            `json:"corpus_version"`
	CorpusSource  string            `json:"corpus_source"`
	CorpusSHA256  string            `json:"corpus_sha256"`
	CorpusCases   int               `json:"corpus_cases"`
	Authority     string            `json:"authority"`
	Boundary      string            `json:"boundary"`
	AutoEdit      bool              `json:"auto_edit_skill_md"` // always false (AC-154)
	FeatureID     string            `json:"feature_id"`
	FeatureSlug   string            `json:"feature_slug,omitempty"`
	GateResultID  string            `json:"gate_result_id,omitempty"`
	ScoredAt      string            `json:"scored_at,omitempty"`
}

// ScoreOptions parameterises one scoring run.
type ScoreOptions struct {
	Target      string             // SKILL.md path, package dir, or skill name under the project
	Evaluator   string             // required (AC-155: attributed, separable grader)
	HumanScores map[string]float64 // "effectiveness" or rubric keys -> 0..100
	CorpusPath  string             // optional external corpus (fail-closed, AC-169)
}

func scoreGrade(total float64) string {
	switch {
	case total >= 85:
		return "A"
	case total >= 70:
		return "B"
	case total >= 55:
		return "C"
	default:
		return "D"
	}
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

// ---- the engine ----

// ScoreSkill scores one skill package and persists the card
// (gate_results row under gate_type skill_score + skill_scored
// trail event). It reads the target once and never writes it.
func (e *Engine) ScoreSkill(ctx store.FeatureContext, opts ScoreOptions) (Scorecard, error) {
	if strings.TrimSpace(opts.Evaluator) == "" {
		return Scorecard{}, core.NewError(core.ReasonSkillScoreEvaluatorReq, "score.ScoreSkill",
			"an evaluator identity is required (--evaluator); unattributed scores are not recorded (AC-155)")
	}
	corpus, corpusSource, err := loadScoreCorpus(opts.CorpusPath)
	if err != nil {
		return Scorecard{}, err
	}
	skillFile, pkgDir, err := resolveScoreTarget(ctx.Project.RootPath, opts.Target)
	if err != nil {
		return Scorecard{}, err
	}
	raw, err := os.ReadFile(skillFile)
	if err != nil {
		return Scorecard{}, core.NewError(core.ReasonSkillTargetInvalid, "score.ScoreSkill",
			"score target unreadable: "+skillFile)
	}
	sum := sha256.Sum256(raw)
	text := string(raw)
	fm, body, fmOK := parseSkillDoc(text)
	targetName := fm["name"]
	if targetName == "" {
		targetName = filepath.Base(pkgDir)
	}

	// Structural dimension: pure over the bytes read above.
	items := structuralItems(fm, fmOK, body, text, pkgDir)
	var earned, maxPoints float64
	var weaknesses []string
	var suggestions []ScoreSuggestion
	for _, it := range items {
		if !it.Applicable {
			continue
		}
		earned += it.Points
		maxPoints += it.MaxPoints
		if it.Points < it.MaxPoints {
			weaknesses = append(weaknesses, "structural:"+it.Check)
			suggestions = append(suggestions, ScoreSuggestion{
				Dimension: "structural", Check: it.Check, Advisory: true,
				Advice: scoreAdvice[it.Check],
			})
		}
	}
	structuralScore := 0.0
	if maxPoints > 0 {
		structuralScore = round2(100 * earned / maxPoints)
	}
	structural := ScoreDimension{
		Dimension: "structural", Scored: true, Score: &structuralScore,
		Weight: ScoreWeightStructural, Source: "mechanical_static", Items: items,
	}

	// Effectiveness dimension: human-graded against the corpus,
	// never invented by the engine.
	effectiveness, err := effectivenessDimension(corpus, opts.HumanScores)
	if err != nil {
		return Scorecard{}, err
	}

	dims := []ScoreDimension{structural, effectiveness}
	var incomplete []string
	var wsum, vsum float64
	for _, d := range dims {
		if d.Scored {
			wsum += d.Weight
			vsum += *d.Score * d.Weight
		} else {
			incomplete = append(incomplete, d.Dimension)
		}
	}
	card := Scorecard{
		Schema: ScoreSchema, GateType: ScoreGateType, TargetType: "skill",
		TargetPath: skillFile, TargetName: targetName, PackageDir: pkgDir,
		TargetSHA256: hex.EncodeToString(sum[:]), Evaluator: opts.Evaluator,
		Dimensions: dims, Incomplete: incomplete, Weaknesses: weaknesses,
		Suggestions:  suggestions,
		CorpusSchema: corpus.Schema, CorpusVersion: corpus.Version,
		CorpusSource: corpusSource, CorpusSHA256: corpus.Hash(), CorpusCases: len(corpus.Cases),
		Authority: ScoreAuthority, Boundary: ScoreBoundaryText, AutoEdit: false,
		FeatureID: ctx.Feature.ID, FeatureSlug: ctx.Feature.Slug,
	}
	if wsum > 0 {
		total := round2(vsum / wsum)
		card.TotalScore = &total
		card.Grade = scoreGrade(total)
	}
	// Score row + scored event commit as one transaction (P6
	// write-path fix), same pattern as persistGateResult.
	err = e.store.WithTx(func(tx *store.Tx) error {
		rec, err := tx.Gates().Record(store.GateResult{
			FeatureID: ctx.Feature.ID, GateType: ScoreGateType,
			Verdict: card.Grade, ReasonCodes: weaknesses,
		})
		if err != nil {
			return err
		}
		card.GateResultID = rec.ID
		card.ScoredAt = rec.EvaluatedAt
		payload, err := json.Marshal(card)
		if err != nil {
			return err
		}
		_, err = tx.AppendEvent(store.LifecycleEvent{
			FeatureID: ctx.Feature.ID, EventType: EventSkillScored, Payload: payload,
		})
		return err
	})
	if err != nil {
		return Scorecard{}, err
	}
	return card, nil
}

// effectivenessDimension builds the effectiveness axis from human
// input only. Keys: "effectiveness" (whole dimension) or rubric
// keys from the corpus (averaged over the keys provided).
func effectivenessDimension(corpus ScoreCorpus, human map[string]float64) (ScoreDimension, error) {
	dim := ScoreDimension{
		Dimension: "effectiveness", Weight: ScoreWeightEffectiveness,
		Source: "not_scored", Rubric: corpus.Rubric,
		Note: "not mechanically decidable: a human grader runs the corpus test-prompts (with_skill vs no_skill) and grades the rubric; supply --human-score effectiveness=<0-100> or per-rubric values",
	}
	if len(human) == 0 {
		return dim, nil
	}
	rubric := corpus.rubricKeys()
	for k, v := range human {
		if k != "effectiveness" && !rubric[k] {
			return ScoreDimension{}, core.NewError(core.ReasonSkillHumanScoreInvalid, "score.ScoreSkill",
				"unknown human-score key "+k+" (want effectiveness or a corpus rubric dimension)")
		}
		if v < 0 || v > 100 || math.IsNaN(v) {
			return ScoreDimension{}, core.NewError(core.ReasonSkillHumanScoreInvalid, "score.ScoreSkill",
				fmt.Sprintf("human score for %s must be within 0..100, got %v", k, v))
		}
	}
	if v, ok := human["effectiveness"]; ok {
		s := round2(v)
		dim.Scored, dim.Score, dim.Source = true, &s, "human_input"
		dim.Note = "whole-dimension human grade against the corpus test-prompts"
		return dim, nil
	}
	var keys []string
	for k := range human {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sum float64
	for _, k := range keys {
		v := human[k]
		sum += v
		dim.Items = append(dim.Items, ScoreItem{Check: k, Points: v, MaxPoints: 100, Applicable: true,
			Detail: "human grade for rubric dimension " + k})
	}
	s := round2(sum / float64(len(keys)))
	dim.Scored, dim.Score, dim.Source = true, &s, "human_input"
	dim.Note = fmt.Sprintf("human grades averaged over %d of %d rubric dimensions (%s); ungraded rubric dimensions are not assumed",
		len(keys), len(corpus.Rubric), strings.Join(keys, ", "))
	return dim, nil
}

// ScoreHistory returns the feature's scorecards oldest first: the
// gate_results rows of gate_type skill_score, enriched with the
// full card carried by their skill_scored trail events.
func (e *Engine) ScoreHistory(ctx store.FeatureContext) ([]Scorecard, error) {
	rows, err := e.store.Gates().ListByFeature(ctx.Feature.ID)
	if err != nil {
		return nil, err
	}
	events, err := e.store.ListEvents(ctx.Feature.ID)
	if err != nil {
		return nil, err
	}
	byResult := map[string]Scorecard{}
	for _, ev := range events {
		if ev.EventType != EventSkillScored || len(ev.Payload) == 0 {
			continue
		}
		var card Scorecard
		if err := json.Unmarshal(ev.Payload, &card); err == nil && card.GateResultID != "" {
			byResult[card.GateResultID] = card
		}
	}
	var out []Scorecard
	for i := len(rows) - 1; i >= 0; i-- { // ListByFeature is newest-first; history reads oldest first
		r := rows[i]
		if r.GateType != ScoreGateType {
			continue
		}
		if card, ok := byResult[r.ID]; ok {
			out = append(out, card)
			continue
		}
		out = append(out, Scorecard{
			Schema: ScoreSchema, GateType: ScoreGateType, TargetType: "skill",
			Grade: r.Verdict, Weaknesses: r.ReasonCodes,
			Authority: ScoreAuthority, Boundary: ScoreBoundaryText,
			FeatureID: ctx.Feature.ID, FeatureSlug: ctx.Feature.Slug,
			GateResultID: r.ID, ScoredAt: r.EvaluatedAt,
		})
	}
	return out, nil
}
