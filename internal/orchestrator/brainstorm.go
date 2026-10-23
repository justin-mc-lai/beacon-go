// Brainstorm generation + convergence (R2d, parity audit M4):
// the kernel side of the ask ledger. Before this slice the
// ledger only RECORDED questions a human typed; Python's
// brainstorm (utils/brainstorm.py run_beacon_brainstorm)
// actively produces the Socratic question set and a convergence
// contract that turns package gaps into questions.
//
// The Go landing keeps the ledger as the single home of
// questions (no sidecar files): `truth brainstorm` generates
// kernel questions INTO the ledger — the four Socratic
// questions verbatim, plus gap questions derived from the
// package's own facts in the convergence contract's question
// shape — each marked with its source, deduped against what is
// already there. Convergence is then a report over the same
// ledger the confirm gate reads: open == 0 means converged.
// No new gate is created — confirm/freeze keep judging the
// ledger exactly as before (R2a alignment).
package orchestrator

import (
	"fmt"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// EventTruthBrainstormGenerated trails one generation pass
// that actually added questions.
const EventTruthBrainstormGenerated = "truth_brainstorm_generated"

// Brainstorm question sources (the ledger's Source column).
const (
	QuestionSourceHuman    = "human"
	QuestionSourceSocratic = "kernel:brainstorm-socratic"
	QuestionSourceGap      = "kernel:brainstorm-gap"
)

// BrainstormView is the brainstorm surface's result: what was
// generated, and whether the package's question ledger has
// converged.
type BrainstormView struct {
	Slug            string   `json:"slug"`
	Mode            string   `json:"mode"`
	CanonicalMode   string   `json:"canonical_mode"`
	GeneratedIDs    []string `json:"generated_ids,omitempty"`
	GeneratedCount  int      `json:"generated_count"`
	OpenQuestions   int      `json:"open_questions"`
	TotalQuestions  int      `json:"total_questions"`
	Converged       bool     `json:"converged"`
	ConfirmBlocked  []string `json:"confirm_blockers,omitempty"`
	Recommendation  string   `json:"recommendation"`
	NextRoute       string   `json:"next_route"`
	FirstPrinciples []string `json:"first_principles"`
	RedundancyCuts  []string `json:"redundancy_cuts"`
}

// gapFocusAreas derives the convergence contract's focus areas
// from the package's own facts (the Go counterpart of the
// Python help payload's completeness focus areas): each area is
// a real, checkable gap in THIS package right now.
func (e *Engine) gapFocusAreas(ctx store.FeatureContext, truthText string) []string {
	var areas []string
	snap := gate.ParseIntentSnapshot(truthText)
	for _, f := range []struct{ name, val string }{
		{"utterance", snap.Utterance}, {"goal", snap.Goal},
		{"non_goals", snap.NonGoals}, {"constraints", snap.Constraints},
	} {
		if isPlaceholderCell(f.val) {
			areas = append(areas, "意图快照 "+f.name)
		}
	}
	// Journey/FSM gaps only count as focus areas when the
	// package's own frontmatter declares a domain (the gate's
	// business-domain condition); a general package is not
	// "missing" an FSM it never needed.
	scalars, _, _ := gate.ParseFrontmatter(truthText)
	domainDeclared := strings.EqualFold(strings.TrimSpace(scalars["domain_required"]), "true") ||
		strings.TrimSpace(scalars["domain_kind"]) != "" || strings.TrimSpace(scalars["package_kind"]) != ""
	if domainDeclared {
		if !strings.Contains(truthText, "## 用户旅程") && !strings.Contains(truthText, "## User Journey") {
			areas = append(areas, "用户旅程")
		}
		if proj := gate.ParseTruthFSM(truthText); !proj.FiveTuplePresent {
			areas = append(areas, "领域状态机 FSM")
		}
	}
	pkg := e.packageFiles(ctx)
	if pkg.Tests.Present {
		covered := map[string]bool{}
		for _, row := range gate.ParseCoverageRows(pkg.Tests.Text) {
			covered[row.AC] = true
		}
		for id := range gate.DefinedACs(truthText) {
			if !covered[id] {
				areas = append(areas, "验收覆盖 AC↔TC")
				break
			}
		}
	}
	if len(e.staleMaterialSurfaces(ctx)) > 0 {
		areas = append(areas, "材料新鲜度")
	}
	return areas
}

// GenerateBrainstorm runs one brainstorm pass over a package:
// generate kernel questions into the ledger (deduped), then
// report convergence. prompt is accepted for parity with the
// Python surface (it seeds the feature inference there); the Go
// generator works from package facts and does not need it — it
// is recorded on the event when given.
func (e *Engine) GenerateBrainstorm(ctx store.FeatureContext, mode, prompt, by string) (BrainstormView, error) {
	if strings.TrimSpace(mode) == "" {
		mode = "requirement-convergence"
	}
	canon, ok := core.CanonicalBrainstormMode(mode)
	if !ok {
		return BrainstormView{}, core.NewError(core.ReasonBrainstormModeInvalid, "truth.Brainstorm",
			"unsupported brainstorm mode: "+mode+" (want one of "+strings.Join(core.BrainstormModes, " | ")+")")
	}
	text, err := readTruthText(ctx)
	if err != nil {
		return BrainstormView{}, err
	}
	existing := gate.ParseOpenQuestions(text)
	known := map[string]bool{}
	for _, q := range existing {
		known[strings.TrimSpace(q.Question)] = true
	}
	type candidate struct{ text, recommendation, source string }
	var candidates []candidate
	for _, q := range core.BrainstormSocraticQuestions {
		candidates = append(candidates, candidate{q, "", QuestionSourceSocratic})
	}
	// Convergence fallback (brainstorm._load_convergence_contract):
	// with focus areas but no questions, each area becomes the
	// question "补齐 `X` 是否会影响冻结后的验收闭环？" (capped at 4).
	areas := e.gapFocusAreas(ctx, text)
	for i, area := range areas {
		if i >= 4 {
			break
		}
		candidates = append(candidates, candidate{
			fmt.Sprintf("补齐 `%s` 是否会影响冻结后的验收闭环？", area), "", QuestionSourceGap})
	}
	rows := existing
	var generated []string
	for _, c := range candidates {
		if known[c.text] {
			continue
		}
		known[c.text] = true
		id := nextQuestionID(rows)
		rows = append(rows, gate.OpenQuestion{
			ID: id, Question: c.text, Recommendation: c.recommendation,
			Status: "open", Source: c.source,
		})
		generated = append(generated, id)
	}
	if len(generated) > 0 {
		text = replaceSection(text, "## Open Questions", rebuildOpenQuestions(rows))
		text, _ = setFrontmatterScalar(text, "intent_confirmed", "false")
		if err := writeTruthText(ctx, text); err != nil {
			return BrainstormView{}, err
		}
		_ = e.appendFeatureEvent(ctx.Feature.ID, EventTruthBrainstormGenerated, map[string]any{
			"by": by, "mode": canon, "generated": generated, "prompt": prompt,
		})
	}
	open := 0
	for _, q := range rows {
		if !isClosedStatusLocal(q.Status) {
			open++
		}
	}
	state, err := e.IntentState(ctx)
	if err != nil {
		return BrainstormView{}, err
	}
	recommendation, nextRoute := core.BrainstormModeRecommendation(canon)
	return BrainstormView{
		Slug:            ctx.Feature.Slug,
		Mode:            mode,
		CanonicalMode:   canon,
		GeneratedIDs:    generated,
		GeneratedCount:  len(generated),
		OpenQuestions:   open,
		TotalQuestions:  len(rows),
		Converged:       open == 0,
		ConfirmBlocked:  state.BlockReasons,
		Recommendation:  recommendation,
		NextRoute:       nextRoute,
		FirstPrinciples: core.BrainstormFirstPrinciples,
		RedundancyCuts:  core.BrainstormRedundancyCuts,
	}, nil
}

func isClosedStatusLocal(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "answered", "dropped", "closed", "resolved", "done", "n/a":
		return true
	}
	return false
}
