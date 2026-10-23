// Intent decomposition materialization (R2d, parity audit M3):
// turns one utterance into package substance. The decomposition
// itself is pure (internal/core/decompose.go, the
// intent_decomposition.py port); this layer renders it into an
// EXISTING draft package created by `truth create`:
//
//	truth.md  — Acceptance Criteria rows derived per intent
//	            clause and per journey acceptance seed (each
//	            with provenance), the Intent Coverage matrix,
//	            a real ## 用户旅程 section built from the
//	            journey seeds (steps + acceptance seeds — the
//	            R2a journey hard check judges this section, so
//	            it is generated content, never a stub), and the
//	            intent snapshot's utterance/scope filled from
//	            the decomposition.
//	tests.md  — one TC per AC. Commands are REAL execution
//	            commands discovered from the project scan (the
//	            qa9 preset machinery): intent_first.py's
//	            provenance `rg` commands would be judged
//	            docs-only by the Go integrity gate for every
//	            row, so the exec layer uses the project's own
//	            test command; the rg provenance lives in the
//	            coverage Source column instead.
//	tasks.md  — the intent_first task ledger shape (per-AC
//	            rows with ac=/evidence= bindings + the two
//	            closing rows).
//
// The decomposition payload itself is persisted to
// .beacon/state/decompose/<slug>.json (project state, like
// Python's plan artifacts — not requirement truth).
//
// A frozen package is never rewritten by decomposition: it
// refuses (decompose_frozen_package). Signing is NOT done here
// — the human still records intent and confirms; decomposition
// fills the snapshot's utterance and derived scope only.
package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// EventTruthDecomposed trails one decomposition materialization.
const EventTruthDecomposed = "truth_decomposed"

// DecomposeView is the decompose surface's result.
type DecomposeView struct {
	Slug          string             `json:"slug"`
	Version       string             `json:"version"`
	Decomposition core.Decomposition `json:"decomposition"`
	Materialized  []string           `json:"materialized_acs"`
	ArtifactPath  string             `json:"artifact_path"`
	Warnings      []string           `json:"warnings,omitempty"`
}

// acSeedVagueRewrite normalizes ONE built-in journey seed when
// it becomes AC text: the J-qa-release seed "旅程有测试或清单证据"
// contains the branch word 或 and would be flagged by the R2a
// vague-word check the moment it lands in the AC table. The
// seed library itself (core) and the journey section keep the
// verbatim wording; only the AC rendering is normalized, and
// the rewrite is meaning-preserving (test evidence AND list
// evidence). User-supplied clause text is never rewritten —
// vague user words surface as warnings instead.
var acSeedVagueRewrite = map[string]string{
	"旅程有测试或清单证据": "旅程具备测试证据与清单证据",
}

var vagueWarningTokens = []string{"或者", "至少", "不少于", "若干", "视情况", " at least ", " at most "}

func acDescriptionWarnings(id, desc string) []string {
	low := " " + strings.ToLower(desc) + " "
	for _, tok := range vagueWarningTokens {
		if strings.Contains(desc, tok) || strings.Contains(low, tok) {
			return []string{id + " 的描述含模糊词 '" + strings.TrimSpace(tok) + "'，冻结前需改写为确定表述（R2a 模糊词硬检会拦）"}
		}
	}
	return nil
}

// projectTestCommand discovers the project's own test command
// from the qa9 scan (the same fact base command presets use):
// go -> go test ./..., javascript -> npm test, python -> the
// qa9 unit default, anything else -> the qa9 unit default.
func projectTestCommand(root string) (cmd string, discovered bool) {
	scan := gate.InferProjectScan(root)
	switch scan.Language {
	case "go":
		return "go test ./...", true
	case "javascript":
		return "npm test", true
	case "python":
		return gate.DefaultCommandForQAType("unit", "playwright"), true
	}
	return gate.DefaultCommandForQAType("unit", "playwright"), false
}

// DecomposeTruthPackage decomposes one utterance and
// materializes it into the named draft package.
func (e *Engine) DecomposeTruthPackage(ctx store.FeatureContext, utterance string, lakes []string, subintent, by string) (DecomposeView, error) {
	view, err := e.TruthPackage(ctx)
	if err != nil {
		return DecomposeView{}, err
	}
	if view.Frozen {
		return DecomposeView{}, core.NewError(core.ReasonDecomposeFrozen, "truth.Decompose",
			"package "+view.Slug+" is frozen; decomposition materializes draft files and never rewrites a frozen body (open a change or reopen a new revision instead)")
	}
	if view.ChangePending {
		return DecomposeView{}, core.NewError(core.ReasonTruthChangePending, "truth.Decompose",
			"package "+view.Slug+" has a pending change; close it before decomposing into the package")
	}
	root := ctx.Project.RootPath
	d := core.DecomposeIntent(utterance, lakes, subintent, nowISO())
	if len(d.Intents) == 0 {
		return DecomposeView{}, core.NewError(core.ReasonDecomposeNoIntents, "truth.Decompose",
			"the utterance produced no substantive clause to decompose; give at least one sentence of intent")
	}

	// Derive the AC set: one per intent clause, one per journey
	// acceptance seed, deduped by description.
	type acSeed struct {
		desc, source, evidence string
	}
	var seeds []acSeed
	seenDesc := map[string]bool{}
	addSeed := func(desc, source, evidence string) {
		desc = strings.TrimSpace(desc)
		if desc == "" || seenDesc[desc] {
			return
		}
		seenDesc[desc] = true
		seeds = append(seeds, acSeed{desc, source, evidence})
	}
	for _, in := range d.Intents {
		addSeed(in.Text, in.SourceRef, "truth.md (surface_ask)")
	}
	for _, j := range d.Journeys {
		for _, seed := range j.AcceptanceSeeds {
			if rw, ok := acSeedVagueRewrite[seed]; ok {
				seed = rw
			}
			addSeed(seed, "journey_seed "+j.ID, "truth.md#用户旅程")
		}
	}

	// ---- truth.md ----
	text, err := readTruthText(ctx)
	if err != nil {
		return DecomposeView{}, err
	}
	var warnings []string
	var acIDs []string
	var acTable strings.Builder
	acTable.WriteString("| AC ID | Criterion |\n|-------|-----------|\n")
	var coverage []gate.IntentCoverageRow
	for i, s := range seeds {
		id := fmt.Sprintf("AC-%03d", i+1)
		acIDs = append(acIDs, id)
		fmt.Fprintf(&acTable, "| %s | %s |\n", id, cellEscape(s.desc))
		coverage = append(coverage, gate.IntentCoverageRow{AC: id, Source: s.source, Evidence: s.evidence})
		warnings = append(warnings, acDescriptionWarnings(id, s.desc)...)
	}
	text = replaceSection(text, "## Acceptance Criteria", strings.TrimRight(acTable.String(), "\n"))
	text = replaceSection(text, "## Intent Coverage", rebuildCoverage(coverage))
	if len(d.Journeys) > 0 {
		var jb strings.Builder
		for _, j := range d.Journeys {
			fmt.Fprintf(&jb, "### %s %s\n\n", j.ID, j.Name)
			fmt.Fprintf(&jb, "- 参与者：%s\n", strings.Join(j.Actors, "、"))
			for _, step := range j.StepsSeed {
				fmt.Fprintf(&jb, "- %s\n", step)
			}
			fmt.Fprintf(&jb, "- 验收种子：%s\n\n", strings.Join(j.AcceptanceSeeds, "；"))
		}
		text = replaceSection(text, "## 用户旅程", strings.TrimRight(jb.String(), "\n"))
	}
	// Snapshot: fill the utterance (and goal from the first
	// clause, scope from the decomposition axis) only where the
	// author has not already written real values.
	snap := gate.ParseIntentSnapshot(text)
	if isPlaceholderCell(snap.Utterance) {
		snap.Utterance = d.SurfaceAsk
	}
	if isPlaceholderCell(snap.Goal) && len(d.Intents) > 0 {
		snap.Goal = d.Intents[0].Text
	}
	if isPlaceholderCell(snap.ScopeMode) || snap.ScopeMode == "focused" {
		snap.ScopeMode = d.ScopeMode // lake | ocean (gate vocabulary)
	}
	snapBody := "```yaml\n" +
		"lake_or_ocean: " + map[string]string{"lake": "湖", "ocean": "海"}[d.ScopeMode] + "\nlanguage: zh\nsource: user_intent\n" +
		"utterance: " + quoteYAML(snap.Utterance) + "\n" +
		"goal: " + quoteYAML(snap.Goal) + "\n" +
		"non_goals: " + quoteYAML(snap.NonGoals) + "\n" +
		"constraints: " + quoteYAML(snap.Constraints) + "\n" +
		"scope_mode: " + quoteYAML(snap.ScopeMode) + "\n```"
	text = replaceSection(text, "## User Intent Snapshot", snapBody)
	// Decomposition summary section (deterministic rendering of
	// the persisted payload's headline facts).
	var db strings.Builder
	fmt.Fprintf(&db, "- schema: `%s`\n", d.Schema)
	fmt.Fprintf(&db, "- scope: `%s`（lakes: %s）\n", d.ScopeMode, strings.Join(d.Lakes, ", "))
	fmt.Fprintf(&db, "- deep_want: %s\n", d.DeepWant)
	fmt.Fprintf(&db, "- readiness: ready=%v missing=%s\n", d.Readiness.Ready, strings.Join(d.Readiness.Missing, ", "))
	fmt.Fprintf(&db, "- 说明：本节由 `beacon truth decompose` 按 intent_decomposition 规则生成；子意图与旅程种子见上文 AC 表与用户旅程，签字仍由人完成。\n")
	text = replaceSection(text, "## Intent Decomposition", strings.TrimRight(db.String(), "\n"))
	// Decomposition changes what the ACs are: any earlier
	// signature no longer describes this package.
	text, _ = setFrontmatterScalar(text, "intent_confirmed", "false")
	if err := writeTruthText(ctx, text); err != nil {
		return DecomposeView{}, err
	}

	// ---- tests.md ----
	execCmd, discovered := projectTestCommand(root)
	if !discovered {
		warnings = append(warnings, "未从项目扫描发现测试命令，TC 命令使用 qa9 unit 默认值；项目有真实测试命令后应替换")
	}
	testsAbs := resolvePkgPath(root, ctx.Feature.TestsPath)
	front := frontmatterBlock(testsAbs, ctx.Feature.Slug, ctx.Version.Version)
	var tb strings.Builder
	tb.WriteString(front)
	fmt.Fprintf(&tb, "\n# Tests: %s\n\n", ctx.Feature.Slug)
	tb.WriteString("| AC ID | TC ID | Layer | Command | Assertion |\n")
	tb.WriteString("|-------|-------|-------|---------|-----------|\n")
	for i, id := range acIDs {
		fmt.Fprintf(&tb, "| %s | TC-%03d | contract | `%s` | acceptance holds for %s |\n",
			id, i+1, execCmd, id)
	}
	if err := os.WriteFile(testsAbs, []byte(tb.String()), 0o644); err != nil {
		return DecomposeView{}, err
	}

	// ---- tasks.md (intent_first.render_intent_first_tasks shape) ----
	tasksAbs := resolvePkgPath(root, e.packageFiles(ctx).Tasks.Path)
	var kb strings.Builder
	kb.WriteString(frontmatterBlock(tasksAbs, ctx.Feature.Slug, ctx.Version.Version))
	fmt.Fprintf(&kb, "\n# Tasks: %s\n\n## Task Ledger\n\n", ctx.Feature.Slug)
	for i, s := range seeds {
		desc := s.desc
		if r := []rune(desc); len(r) > 72 {
			desc = string(r[:69]) + "..."
		}
		evidence := fmt.Sprintf(".beacon/evidence/implement/%s/%s.json", ctx.Feature.Slug, acIDs[i])
		fmt.Fprintf(&kb, "- [ ] TASK-%03d Implement %s: %s · ac=%s · evidence=`%s`\n",
			i+1, acIDs[i], desc, acIDs[i], evidence)
	}
	n := len(seeds)
	fmt.Fprintf(&kb, "- [ ] TASK-%03d Validate package gates (truth review + freeze ack) · ac=GATES · evidence=`.beacon/evidence/implement/%s/GATES.json`\n", n+1, ctx.Feature.Slug)
	fmt.Fprintf(&kb, "- [ ] TASK-%03d Run QA against AC↔TC matrix + exec-layer evidence · ac=QA · evidence=`.beacon/evidence/implement/%s/QA-MATRIX.json`\n", n+2, ctx.Feature.Slug)
	kb.WriteString("\n## Boundary\n\nCheckboxes are AC-bound ledgers. `[x]` is valid only when the bound `evidence=` file exists.\n")
	if err := os.WriteFile(tasksAbs, []byte(kb.String()), 0o644); err != nil {
		return DecomposeView{}, err
	}

	// ---- persisted decomposition artifact ----
	artifactRel := filepath.ToSlash(filepath.Join(".beacon", "state", "decompose", ctx.Feature.Slug+".json"))
	if err := writeJSONFile(filepath.Join(root, artifactRel), d); err != nil {
		return DecomposeView{}, err
	}
	_ = e.appendFeatureEvent(ctx.Feature.ID, EventTruthDecomposed, map[string]any{
		"by": by, "scope_mode": d.ScopeMode, "intents": len(d.Intents),
		"journeys": len(d.Journeys), "acs": acIDs, "artifact": artifactRel,
		"readiness_missing": d.Readiness.Missing,
	})
	return DecomposeView{
		Slug: ctx.Feature.Slug, Version: ctx.Version.Version,
		Decomposition: d, Materialized: acIDs,
		ArtifactPath: artifactRel, Warnings: warnings,
	}, nil
}

// frontmatterBlock returns the existing file's frontmatter
// block (including the closing --- and trailing newline), or a
// minimal synthesized one when the file has none.
func frontmatterBlock(absPath, slug, version string) string {
	raw, err := os.ReadFile(absPath)
	if err == nil {
		text := string(raw)
		if strings.HasPrefix(text, "---\n") {
			if end := strings.Index(text[4:], "\n---"); end >= 0 {
				return text[:4+end+4] + "\n"
			}
		}
	}
	return "---\nslug: " + slug + "\nversion: " + version + "\nmaterials_status: draft\n---\n"
}
