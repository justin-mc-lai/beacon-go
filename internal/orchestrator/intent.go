// Intent flow (R1: ask before sign) and truth self-check (drift +
// change-base divergence). The truth document stays the single source
// of requirement truth: questions, answers and coverage live in its
// markdown, and every mutation is a line-level rewrite that preserves
// the rest of the file byte-for-byte.
package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// IntentView is the read-only intent state of one package.
type IntentView struct {
	Slug          string                   `json:"slug"`
	Confirmed     bool                     `json:"intent_confirmed"`
	Snapshot      gate.IntentSnapshot      `json:"intent_snapshot"`
	OpenQuestions []gate.OpenQuestion      `json:"open_questions"`
	Coverage      []gate.IntentCoverageRow `json:"intent_coverage"`
	Ready         bool                     `json:"ready"`
	BlockReasons  []string                 `json:"block_reasons,omitempty"`
}

// DriftView is the requirement-level self-check result.
type DriftView struct {
	Slug             string            `json:"slug"`
	Version          string            `json:"version"`
	Status           string            `json:"status"`
	Frozen           bool              `json:"frozen"`
	Clean            bool              `json:"clean"`
	BodyDrift        bool              `json:"body_drift"`
	FrozenTruthHash  string            `json:"frozen_truth_sha256,omitempty"`
	CurrentTruthHash string            `json:"current_truth_sha256"`
	RequirementDrift map[string]string `json:"requirement_drift,omitempty"` // added|removed|changed
	Added            []string          `json:"added,omitempty"`
	Removed          []string          `json:"removed,omitempty"`
	Changed          []string          `json:"changed,omitempty"`
	OpenQuestions    int               `json:"open_questions"`
	BaseDiverged     bool              `json:"base_diverged"`
	BaseDetail       string            `json:"base_detail,omitempty"`
	Note             string            `json:"note,omitempty"`
}

func readTruthText(ctx store.FeatureContext) (string, error) {
	raw, err := os.ReadFile(resolvePkgPath(ctx.Project.RootPath, ctx.Feature.TruthPath))
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func writeTruthText(ctx store.FeatureContext, text string) error {
	return os.WriteFile(resolvePkgPath(ctx.Project.RootPath, ctx.Feature.TruthPath), []byte(text), 0o644)
}

// setFrontmatterScalar rewrites ONE top-level frontmatter scalar and
// preserves every other byte. It inserts the key after slug when absent.
func setFrontmatterScalar(text, key, value string) (string, bool) {
	if !strings.HasPrefix(text, "---\n") {
		return text, false
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return text, false
	}
	fmEnd := 4 + end
	lines := strings.Split(text[4:fmEnd], "\n")
	want := key + ": " + value
	found := false
	for i, ln := range lines {
		if strings.HasPrefix(ln, key+":") {
			lines[i] = want
			found = true
		}
	}
	if !found {
		idx := len(lines)
		for i, ln := range lines {
			if strings.HasPrefix(ln, "slug:") {
				idx = i + 1
				break
			}
		}
		lines = append(lines[:idx], append([]string{want}, lines[idx:]...)...)
	}
	out := "---\n" + strings.Join(lines, "\n") + text[fmEnd:]
	return out, out != text
}

// replaceSection replaces the body of an exact `## heading` section,
// keeping the heading and everything outside it. When the section is
// absent it is appended.
func replaceSection(text, heading, body string) string {
	lines := strings.Split(text, "\n")
	start := -1
	for i, ln := range lines {
		if strings.EqualFold(strings.TrimSpace(ln), strings.TrimSpace(heading)) {
			start = i
			break
		}
	}
	bodyLines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	if start < 0 {
		out := strings.TrimRight(text, "\n")
		return out + "\n\n" + heading + "\n\n" + strings.Join(bodyLines, "\n") + "\n"
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}
	var out []string
	out = append(out, lines[:start+1]...)
	out = append(out, "")
	out = append(out, bodyLines...)
	out = append(out, "")
	out = append(out, lines[end:]...)
	return strings.Join(out, "\n")
}

func cellEscape(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

func rebuildOpenQuestions(rows []gate.OpenQuestion) string {
	var b strings.Builder
	b.WriteString("One question at a time; a package cannot be signed while a row is open.\n")
	b.WriteString("Use beacon truth ask / beacon truth answer to drive this ledger.\n\n")
	b.WriteString("| Q ID | Question | Recommendation | Status | Answer | Source |\n")
	b.WriteString("|------|----------|----------------|--------|--------|--------|\n")
	for _, r := range rows {
		src := r.Source
		if src == "" {
			src = "human"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n",
			cellEscape(r.ID), cellEscape(r.Question), cellEscape(r.Recommendation),
			cellEscape(r.Status), cellEscape(r.Answer), cellEscape(src))
	}
	return b.String()
}

func rebuildCoverage(rows []gate.IntentCoverageRow) string {
	var b strings.Builder
	b.WriteString("Every AC must trace to the intent or evidence it serves.\n\n")
	b.WriteString("| AC ID | Intent Source | Evidence |\n")
	b.WriteString("|-------|---------------|----------|\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", cellEscape(r.AC), cellEscape(r.Source), cellEscape(r.Evidence))
	}
	return b.String()
}

func nextQuestionID(rows []gate.OpenQuestion) string {
	max := 0
	for _, r := range rows {
		if strings.HasPrefix(strings.ToUpper(r.ID), "Q") {
			if n, err := strconv.Atoi(strings.TrimPrefix(strings.ToUpper(r.ID), "Q")); err == nil && n > max {
				max = n
			}
		}
	}
	return fmt.Sprintf("Q%d", max+1)
}

// UpdateIntent records the user-intent snapshot fields.
func (e *Engine) UpdateIntent(ctx store.FeatureContext, snap gate.IntentSnapshot, by string) (IntentView, error) {
	// Scope mode (R2a, audit M6) is a controlled, derived value in
	// Python — never free text. An explicit value outside the
	// vocabulary fails closed; an empty one is derived from the
	// utterance (planner + decomposition signal axes).
	snap.ScopeMode = gate.NormalizeScopeMode(snap.ScopeMode)
	if snap.ScopeMode == "" {
		snap.ScopeMode = gate.DeriveScopeMode(snap.Utterance)
	} else if !gate.ValidScopeMode(snap.ScopeMode) {
		return IntentView{}, core.NewError(core.ReasonScopeModeInvalid, "truth.Intent",
			"scope_mode '"+snap.ScopeMode+"' is not in the controlled vocabulary ("+
				strings.Join(gate.ScopeModeVocabulary, " | ")+"); leave it empty to derive from the utterance")
	}
	text, err := readTruthText(ctx)
	if err != nil {
		return IntentView{}, err
	}
	body := "```yaml\n" +
		"lake_or_ocean: 湖\nlanguage: zh\nsource: user_intent\n" +
		"utterance: " + quoteYAML(snap.Utterance) + "\n" +
		"goal: " + quoteYAML(snap.Goal) + "\n" +
		"non_goals: " + quoteYAML(snap.NonGoals) + "\n" +
		"constraints: " + quoteYAML(snap.Constraints) + "\n" +
		"scope_mode: " + quoteYAML(snap.ScopeMode) + "\n```"
	text = replaceSection(text, "## User Intent Snapshot", body)
	// Trace every AC to the recorded intent by default; an author
	// can refine a row later, but no AC stays untraced just because
	// the skeleton row was a placeholder.
	existing := map[string]gate.IntentCoverageRow{}
	for _, r := range gate.ParseIntentCoverage(text) {
		existing[r.AC] = r
	}
	defined := gate.DefinedACs(text)
	ids := make([]string, 0, len(defined))
	for id := range defined {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	rows := make([]gate.IntentCoverageRow, 0, len(ids))
	for _, id := range ids {
		r := gate.IntentCoverageRow{AC: id, Source: "user utterance", Evidence: "truth.md#User-Intent-Snapshot"}
		if old, ok := existing[id]; ok && !isPlaceholderCell(old.Source) && !isPlaceholderCell(old.Evidence) {
			r = old
		}
		rows = append(rows, r)
	}
	text = replaceSection(text, "## Intent Coverage", rebuildCoverage(rows))
	// Any intent change invalidates the previous sign-off.
	text, _ = setFrontmatterScalar(text, "intent_confirmed", "false")
	if err := writeTruthText(ctx, text); err != nil {
		return IntentView{}, err
	}
	_ = e.appendFeatureEvent(ctx.Feature.ID, EventTruthIntentUpdated, map[string]any{"by": by})
	return e.IntentState(ctx)
}

func isPlaceholderCell(s string) bool {
	t := strings.ToLower(strings.TrimSpace(s))
	return t == "" || strings.HasPrefix(t, "(fill") || strings.HasPrefix(t, "（fill") || t == "todo" || t == "tbd"
}

func quoteYAML(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// IntentState reads the intent surface plus the same deterministic
// gate the freeze uses (minus the not-confirmed flag itself).
func (e *Engine) IntentState(ctx store.FeatureContext) (IntentView, error) {
	text, err := readTruthText(ctx)
	if err != nil {
		return IntentView{}, err
	}
	scalars, _, _ := gate.ParseFrontmatter(text)
	v := IntentView{
		Slug:          ctx.Feature.Slug,
		Confirmed:     strings.EqualFold(scalars["intent_confirmed"], "true"),
		Snapshot:      gate.ParseIntentSnapshot(text),
		OpenQuestions: gate.ParseOpenQuestions(text),
		Coverage:      gate.ParseIntentCoverage(text),
	}
	pkg := e.packageFiles(ctx)
	for _, f := range gate.CheckIntentGate(pkg) {
		if f.ReasonCode == core.ReasonIntentNotConfirmed {
			continue
		}
		v.BlockReasons = append(v.BlockReasons, f.ReasonCode+": "+f.Detail)
	}
	v.Ready = v.Confirmed && len(v.BlockReasons) == 0
	return v, nil
}

// AskQuestion appends one open question (one at a time).
func (e *Engine) AskQuestion(ctx store.FeatureContext, question, recommendation, by string) (IntentView, error) {
	if strings.TrimSpace(question) == "" {
		return IntentView{}, core.NewError(ReasonTruthFreezeBlocked, "truth.Ask", "a question is required")
	}
	text, err := readTruthText(ctx)
	if err != nil {
		return IntentView{}, err
	}
	rows := gate.ParseOpenQuestions(text)
	rows = append(rows, gate.OpenQuestion{
		ID: nextQuestionID(rows), Question: question, Recommendation: recommendation, Status: "open",
	})
	text = replaceSection(text, "## Open Questions", rebuildOpenQuestions(rows))
	text, _ = setFrontmatterScalar(text, "intent_confirmed", "false")
	if err := writeTruthText(ctx, text); err != nil {
		return IntentView{}, err
	}
	_ = e.appendFeatureEvent(ctx.Feature.ID, EventTruthQuestionAsked, map[string]any{
		"question": question, "recommendation": recommendation, "by": by,
	})
	return e.IntentState(ctx)
}

// AnswerQuestion closes one open question with its answer.
func (e *Engine) AnswerQuestion(ctx store.FeatureContext, id, answer, status, by string) (IntentView, error) {
	text, err := readTruthText(ctx)
	if err != nil {
		return IntentView{}, err
	}
	rows := gate.ParseOpenQuestions(text)
	found := false
	for i := range rows {
		if strings.EqualFold(rows[i].ID, id) {
			rows[i].Answer = answer
			if status == "" {
				status = "answered"
			}
			rows[i].Status = status
			found = true
		}
	}
	if !found {
		return IntentView{}, core.NewError(ReasonTruthFreezeBlocked, "truth.Answer", "question "+id+" not found")
	}
	text = replaceSection(text, "## Open Questions", rebuildOpenQuestions(rows))
	text, _ = setFrontmatterScalar(text, "intent_confirmed", "false")
	if err := writeTruthText(ctx, text); err != nil {
		return IntentView{}, err
	}
	_ = e.appendFeatureEvent(ctx.Feature.ID, EventTruthQuestionAnswered, map[string]any{
		"question_id": id, "status": status, "by": by,
	})
	return e.IntentState(ctx)
}

// ConfirmIntent signs the package's intent: it refuses while any
// structural intent finding remains, then sets intent_confirmed=true.
func (e *Engine) ConfirmIntent(ctx store.FeatureContext, by string) (IntentView, error) {
	text, err := readTruthText(ctx)
	if err != nil {
		return IntentView{}, err
	}
	_ = text
	pkg := e.packageFiles(ctx)
	var reasons []string
	for _, f := range gate.CheckIntentGate(pkg) {
		if f.ReasonCode == core.ReasonIntentNotConfirmed {
			continue
		}
		reasons = append(reasons, f.Kind+": "+f.Detail)
	}
	if len(reasons) > 0 {
		return IntentView{}, core.NewError(core.ReasonIntentNotConfirmed, "truth.Confirm",
			"intent is not ready to sign: "+strings.Join(reasons, "; "))
	}
	out, _ := setFrontmatterScalar(text, "intent_confirmed", "true")
	if err := writeTruthText(ctx, out); err != nil {
		return IntentView{}, err
	}
	_ = e.appendFeatureEvent(ctx.Feature.ID, EventTruthIntentConfirmed, map[string]any{"by": by})
	return e.IntentState(ctx)
}

// TruthDrift compares the live package with the last freeze record:
// requirement-level changes, body drift and change-base divergence.
func (e *Engine) TruthDrift(ctx store.FeatureContext) (DriftView, error) {
	text, err := readTruthText(ctx)
	if err != nil {
		return DriftView{}, err
	}
	recs, err := e.trailRecords(ctx.Feature.ID)
	if err != nil {
		return DriftView{}, err
	}
	vp, err := e.TruthPackage(ctx)
	if err != nil {
		return DriftView{}, err
	}
	v := DriftView{
		Slug: ctx.Feature.Slug, Version: ctx.Version.Version,
		Status: vp.Status, Frozen: vp.Frozen,
		CurrentTruthHash: sha256HexOf([]byte(text)),
		RequirementDrift: map[string]string{},
	}
	for _, q := range gate.ParseOpenQuestions(text) {
		if !strings.EqualFold(q.Status, "answered") && !strings.EqualFold(q.Status, "dropped") &&
			!strings.EqualFold(q.Status, "closed") && !strings.EqualFold(q.Status, "resolved") {
			v.OpenQuestions++
		}
	}
	if recs.freeze == nil {
		v.Note = "never_frozen"
		v.Clean = false
		return v, nil
	}
	v.FrozenTruthHash = recs.freeze.TruthSHA256After
	v.BodyDrift = v.FrozenTruthHash != "" && v.FrozenTruthHash != v.CurrentTruthHash
	cur := core.RequirementFingerprints([]byte(text))
	prev := recs.freeze.RequirementFingerprints
	for id, h := range cur {
		old, ok := prev[id]
		if !ok {
			v.Added = append(v.Added, id)
			v.RequirementDrift[id] = "added"
		} else if old != h {
			v.Changed = append(v.Changed, id)
			v.RequirementDrift[id] = "changed"
		}
	}
	for id := range prev {
		if _, ok := cur[id]; !ok {
			v.Removed = append(v.Removed, id)
			v.RequirementDrift[id] = "removed"
		}
	}
	// Pending change opened on a different frozen body = parallel edit.
	if recs.freeze.TruthSHA256After != "" {
		for _, ch := range recs.pendingChanges {
			if ch.BaseTruthSHA256 != "" && ch.BaseTruthSHA256 != recs.freeze.TruthSHA256After {
				v.BaseDiverged = true
				v.BaseDetail = "change " + ch.ChangeID + " was opened on " + short(ch.BaseTruthSHA256) +
					" but the latest freeze is " + short(recs.freeze.TruthSHA256After) +
					"; merge the landed change and run `beacon truth rebase --change " + ch.ChangeID + " --accept-current`"
				break
			}
		}
	}
	v.Clean = !v.BodyDrift && len(v.RequirementDrift) == 0 && v.OpenQuestions == 0 && !v.BaseDiverged
	return v, nil
}

func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

// RebaseChange explicitly accepts the current frozen body as the base
// for a pending change after a parallel change landed. Without this,
// refreeze refuses (truth_base_diverged) — the guard against silent
// overwrite.
func (e *Engine) RebaseChange(ctx store.FeatureContext, changeID string, acceptCurrent bool, note, by string) (TruthPackageView, error) {
	if !acceptCurrent {
		return TruthPackageView{}, core.NewError(core.ReasonTruthRebaseNotAllowed, "truth.Rebase",
			"rebase rewrites the change base; pass --accept-current after merging the landed change")
	}
	ctxs, err := e.pendingChangeIDs(ctx.Feature.ID)
	if err != nil {
		return TruthPackageView{}, err
	}
	found := false
	for _, id := range ctxs {
		if id == changeID {
			found = true
		}
	}
	if !found {
		return TruthPackageView{}, core.NewError(ReasonTruthNoChangePending, "truth.Rebase",
			"change "+changeID+" is not pending on "+ctx.Feature.Slug)
	}
	recs, err := e.trailRecords(ctx.Feature.ID)
	if err != nil {
		return TruthPackageView{}, err
	}
	if recs.freeze == nil || recs.freeze.TruthSHA256After == "" {
		return TruthPackageView{}, core.NewError(ReasonTruthNotFrozen, "truth.Rebase",
			"no frozen body to rebase onto")
	}
	docPath := filepath.Join(filepath.Dir(ctx.Feature.TruthPath), "changes", changeID+".md")
	abs := resolvePkgPath(ctx.Project.RootPath, docPath)
	raw, err := os.ReadFile(abs)
	if err != nil {
		return TruthPackageView{}, err
	}
	doc, _ := setFrontmatterScalar(string(raw), "base_truth_sha256", recs.freeze.TruthSHA256After)
	doc, _ = setFrontmatterScalar(doc, "rebased_at", nowISO())
	doc, _ = setFrontmatterScalar(doc, "rebased_by", by)
	if strings.TrimSpace(note) != "" {
		doc += "\n## Rebase Note\n\n" + note + "\n"
	}
	if err := os.WriteFile(abs, []byte(doc), 0o644); err != nil {
		return TruthPackageView{}, err
	}
	if err := e.appendFeatureEvent(ctx.Feature.ID, EventTruthChangeRebased, map[string]any{
		"change_id": changeID, "base_truth_sha256": recs.freeze.TruthSHA256After,
		"package_fingerprint": recs.freeze.Fingerprint,
		"surface_fingerprint": recs.freeze.SurfaceFingerprint,
		"by":                  by, "note": note,
	}); err != nil {
		return TruthPackageView{}, err
	}
	return e.TruthPackage(ctx)
}
