// Structural half of the unified skill score (P4b): the skill
// document reader, target resolution and the deterministic
// static checks. Everything here is a pure function of the
// target's bytes plus existence probes of the paths it
// references — no writes, anywhere (AC-154).
package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"beacon-go/internal/core"
)

// ---- skill document parsing ----

// parseSkillDoc splits a SKILL.md into frontmatter scalars and
// body. Unlike the truth-package frontmatter reader this one
// understands YAML block scalars (`>` / `|`), which skill
// descriptions use heavily. It is a reader for a different
// document family, not a second frozen judgement.
func parseSkillDoc(text string) (fm map[string]string, body string, ok bool) {
	fm = map[string]string{}
	if !strings.HasPrefix(text, "---\n") {
		return fm, text, false
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return fm, text, false
	}
	raw := text[4 : 4+end]
	rest := text[4+end+len("\n---"):]
	body = strings.TrimPrefix(rest, "\n")
	lines := strings.Split(raw, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue // nested content is not a top-level scalar
		}
		key, val, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		if val == ">" || val == "|" || val == ">-" || val == "|-" {
			fold := val == ">" || val == ">-"
			var parts []string
			for i+1 < len(lines) && (strings.HasPrefix(lines[i+1], " ") || strings.HasPrefix(lines[i+1], "\t") || strings.TrimSpace(lines[i+1]) == "") {
				i++
				if t := strings.TrimSpace(lines[i]); t != "" {
					parts = append(parts, t)
				}
			}
			if fold {
				fm[key] = strings.Join(parts, " ")
			} else {
				fm[key] = strings.Join(parts, "\n")
			}
			continue
		}
		fm[key] = strings.Trim(val, `"'`)
	}
	return fm, body, true
}

// ---- target resolution ----

// resolveScoreTarget locates the SKILL.md to score: an explicit
// file, a package directory, or a skill name resolved under the
// project (skills/<name>, skills/*/<name>, <name>).
func resolveScoreTarget(projectRoot, arg string) (skillFile, pkgDir string, err error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", "", core.NewError(core.ReasonSkillTargetNotFound, "score.resolveTarget", "no score target given")
	}
	candidates := []string{arg}
	if !filepath.IsAbs(arg) {
		candidates = append(candidates,
			filepath.Join(projectRoot, "skills", arg, "SKILL.md"),
			filepath.Join(projectRoot, arg, "SKILL.md"),
		)
		if hits, gerr := filepath.Glob(filepath.Join(projectRoot, "skills", "*", arg, "SKILL.md")); gerr == nil {
			candidates = append(candidates, hits...)
		}
	}
	for _, cand := range candidates {
		info, serr := os.Stat(cand)
		if serr != nil {
			continue
		}
		if info.IsDir() {
			f := filepath.Join(cand, "SKILL.md")
			if _, ferr := os.Stat(f); ferr == nil {
				return f, cand, nil
			}
			continue
		}
		if filepath.Base(cand) != "SKILL.md" {
			return "", "", core.NewError(core.ReasonSkillTargetInvalid, "score.resolveTarget",
				"score target must be a SKILL.md file or a skill package directory: "+cand)
		}
		return cand, filepath.Dir(cand), nil
	}
	return "", "", core.NewError(core.ReasonSkillTargetNotFound, "score.resolveTarget",
		"no SKILL.md found for target "+arg+" (looked at the path itself and under "+filepath.Join(projectRoot, "skills")+")")
}

// ---- structural checks (deterministic, pure over the bytes) ----

var (
	scoreH1Re       = regexp.MustCompile(`(?m)^#\s+\S`)
	scoreHardGateRe = regexp.MustCompile(`(?im)^#{1,6}\s+.*hard gate`)
	scoreGateHeadRe = regexp.MustCompile(`(?im)^#{1,6}\s+.*gate`)
	scoreLinkRe     = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	scoreRefPathRe  = regexp.MustCompile(`references/[A-Za-z0-9._\-/]+`)
)

func structuralItems(fm map[string]string, fmOK bool, body, raw, pkgDir string) []ScoreItem {
	var items []ScoreItem
	add := func(check string, points, max float64, applicable bool, detail string) {
		items = append(items, ScoreItem{Check: check, Points: points, MaxPoints: max, Applicable: applicable, Detail: detail})
	}
	// frontmatter_present
	if fmOK {
		add("frontmatter_present", 2, 2, true, "frontmatter block parses")
	} else {
		add("frontmatter_present", 0, 2, true, "no parseable --- frontmatter block")
	}
	// frontmatter_identity: name + description (skill-creator).
	name, desc := fm["name"], fm["description"]
	switch {
	case name != "" && desc != "":
		add("frontmatter_identity", 2, 2, true, "name and description present")
	case name != "" || desc != "":
		add("frontmatter_identity", 1, 2, true, "one of name/description missing")
	default:
		add("frontmatter_identity", 0, 2, true, "name and description both missing")
	}
	// name_matches_package: the package directory, or its installed
	// host name beacon-<dir> (Scheme A install convention).
	base := filepath.Base(pkgDir)
	switch {
	case name == "":
		add("name_matches_package", 0, 1, true, "no frontmatter name to match against the package directory "+base)
	case name == base || name == "beacon-"+base:
		add("name_matches_package", 1, 1, true, "frontmatter name matches package directory")
	default:
		add("name_matches_package", 0, 1, true, "frontmatter name "+name+" != package directory "+base)
	}
	// title_present.
	if scoreH1Re.MatchString(body) {
		add("title_present", 1, 1, true, "body carries an H1 title")
	} else {
		add("title_present", 0, 1, true, "body has no H1 title")
	}
	// body_substantial (v1 thresholds, whole document text).
	switch n := len(strings.TrimSpace(raw)); {
	case n >= 200:
		add("body_substantial", 2, 2, true, fmt.Sprintf("document is substantial (%d chars >= 200)", n))
	case n >= 80:
		add("body_substantial", 1, 2, true, fmt.Sprintf("document is minimal (%d chars >= 80)", n))
	default:
		add("body_substantial", 0, 2, true, fmt.Sprintf("document is thinner than 80 chars (%d)", n))
	}
	// hard_gate_declared (AC-152: HARD GATE compliance).
	switch {
	case scoreHardGateRe.MatchString(body):
		add("hard_gate_declared", 2, 2, true, "a HARD GATE section is declared")
	case scoreGateHeadRe.MatchString(body) || strings.Contains(body, "禁止") || strings.Contains(body, "不得") || strings.Contains(strings.ToLower(body), "must not"):
		add("hard_gate_declared", 1, 2, true, "rules/gate language present but no HARD GATE section")
	default:
		add("hard_gate_declared", 0, 2, true, "no HARD GATE section and no rules language")
	}
	// progressive_disclosure: the SKILL.md itself stays lean, or
	// the overflow demonstrably lives in references/.
	lines := strings.Count(raw, "\n") + 1
	_, refErr := os.Stat(filepath.Join(pkgDir, "references"))
	hasRefs := refErr == nil
	switch {
	case lines <= 500:
		add("progressive_disclosure", 1, 1, true, fmt.Sprintf("SKILL.md is %d lines (<= 500)", lines))
	case hasRefs:
		add("progressive_disclosure", 1, 1, true, fmt.Sprintf("SKILL.md is %d lines but detail is pushed into references/", lines))
	default:
		add("progressive_disclosure", 0, 1, true, fmt.Sprintf("SKILL.md is %d lines (> 500) with no references/ directory", lines))
	}
	// references_resolve: every referenced path exists.
	refs := map[string]bool{}
	for _, m := range scoreLinkRe.FindAllStringSubmatch(raw, -1) {
		target := m[1]
		if strings.Contains(target, "://") || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "mailto:") {
			continue
		}
		target = strings.Split(target, "#")[0]
		if target != "" {
			refs[target] = true
		}
	}
	for _, tok := range scoreRefPathRe.FindAllString(raw, -1) {
		refs[strings.TrimRight(tok, ".")] = true
	}
	if len(refs) == 0 {
		add("references_resolve", 0, 2, false, "the document references no package files")
	} else {
		sorted := make([]string, 0, len(refs))
		for r := range refs {
			sorted = append(sorted, r)
		}
		sort.Strings(sorted)
		var missing []string
		for _, r := range sorted {
			if _, err := os.Stat(filepath.Join(pkgDir, filepath.Clean(r))); err != nil {
				missing = append(missing, r)
			}
		}
		switch {
		case len(missing) == 0:
			add("references_resolve", 2, 2, true, fmt.Sprintf("all %d referenced paths resolve", len(sorted)))
		case len(missing) < len(sorted):
			add("references_resolve", 1, 2, true, "unresolved references: "+strings.Join(missing, ", "))
		default:
			add("references_resolve", 0, 2, true, "no referenced path resolves: "+strings.Join(missing, ", "))
		}
	}
	// evals: the package must carry machine-checkable eval cases.
	// Structural only — it verifies the skill *states* the rules its
	// cases expect. Whether a model obeys them needs a run and stays
	// a human grade (the effectiveness dimension).
	items = append(items, evalsItems(pkgDir, body)...)
	return items
}

// ---- evals (structural half) ----

type evalCheck struct {
	Kind string `json:"kind"` // contains | references_contains | min_examples
	Text string `json:"text,omitempty"`
	Min  int    `json:"min,omitempty"`
}

type evalCase struct {
	ID           string      `json:"id"`
	Prompt       string      `json:"prompt"`
	Expectations []string    `json:"expectations"`
	Checks       []evalCheck `json:"checks"`
}

type evalSet struct {
	Schema string     `json:"schema"`
	Skill  string     `json:"skill"`
	Cases  []evalCase `json:"cases"`
}

// evalsItems scores evals/evals.json: presence, case depth, and
// whether every machine check passes against the package bytes.
func evalsItems(pkgDir, body string) []ScoreItem {
	raw, err := os.ReadFile(filepath.Join(pkgDir, "evals", "evals.json"))
	if err != nil {
		return []ScoreItem{{
			Check: "evals_present", Points: 0, MaxPoints: 3, Applicable: true,
			Detail: "no evals/evals.json (a skill without eval cases cannot be checked for drift)",
		}}
	}
	var set evalSet
	if err := json.Unmarshal(raw, &set); err != nil || set.Schema != "skill-evals.v1" {
		return []ScoreItem{{
			Check: "evals_present", Points: 0, MaxPoints: 3, Applicable: true,
			Detail: "evals/evals.json does not parse as skill-evals.v1",
		}}
	}
	items := []ScoreItem{{
		Check: "evals_present", Points: 3, MaxPoints: 3, Applicable: true,
		Detail: fmt.Sprintf("evals/evals.json parses (%d cases)", len(set.Cases)),
	}}

	// Depth: >= 3 cases, each with id, prompt and >= 2 expectations.
	thin := []string{}
	for _, c := range set.Cases {
		if strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Prompt) == "" || len(c.Expectations) < 2 {
			thin = append(thin, c.ID)
		}
	}
	switch {
	case len(set.Cases) >= 3 && len(thin) == 0:
		items = append(items, ScoreItem{Check: "evals_cases", Points: 3, MaxPoints: 3, Applicable: true,
			Detail: fmt.Sprintf("%d cases, each with id/prompt/2+ expectations", len(set.Cases))})
	case len(set.Cases) > 0:
		items = append(items, ScoreItem{Check: "evals_cases", Points: 1, MaxPoints: 3, Applicable: true,
			Detail: fmt.Sprintf("%d cases; thin or incomplete: %s", len(set.Cases), strings.Join(thin, ", "))})
	default:
		items = append(items, ScoreItem{Check: "evals_cases", Points: 0, MaxPoints: 3, Applicable: true,
			Detail: "no cases"})
	}

	// Machine checks: every assertion must hold against the package.
	var failed []string
	total := 0
	for _, c := range set.Cases {
		for _, chk := range c.Checks {
			total++
			if ok, why := evalCheckPasses(chk, pkgDir, body); !ok {
				failed = append(failed, c.ID+":"+why)
			}
		}
	}
	switch {
	case total == 0:
		items = append(items, ScoreItem{Check: "evals_checks_pass", Points: 0, MaxPoints: 4, Applicable: true,
			Detail: "no machine checks declared (expectations alone cannot be scored without a model run)"})
	case len(failed) == 0:
		items = append(items, ScoreItem{Check: "evals_checks_pass", Points: 4, MaxPoints: 4, Applicable: true,
			Detail: fmt.Sprintf("all %d machine checks pass", total)})
	default:
		items = append(items, ScoreItem{Check: "evals_checks_pass", Points: 0, MaxPoints: 4, Applicable: true,
			Detail: "failing checks: " + strings.Join(failed, "; ")})
	}
	return items
}

// evalCheckPasses runs one machine check. It answers a narrow question:
// does the package state/contain what the case expects?
func evalCheckPasses(chk evalCheck, pkgDir, body string) (bool, string) {
	switch chk.Kind {
	case "contains":
		if strings.Contains(strings.ToLower(body), strings.ToLower(chk.Text)) {
			return true, ""
		}
		return false, "body lacks " + chk.Text
	case "references_contains":
		refs := filepath.Join(pkgDir, "references")
		found := false
		_ = filepath.Walk(refs, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if b, rerr := os.ReadFile(p); rerr == nil &&
				strings.Contains(strings.ToLower(string(b)), strings.ToLower(chk.Text)) {
				found = true
			}
			return nil
		})
		if found {
			return true, ""
		}
		return false, "references lack " + chk.Text
	case "min_examples":
		n := 0
		_ = filepath.Walk(filepath.Join(pkgDir, "examples"), func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				n++
			}
			return nil
		})
		if n >= chk.Min {
			return true, ""
		}
		return false, fmt.Sprintf("%d examples < %d", n, chk.Min)
	default:
		return false, "unknown check kind " + chk.Kind
	}
}

var scoreAdvice = map[string]string{
	"frontmatter_present":    "Add a --- frontmatter block at the very top of SKILL.md.",
	"frontmatter_identity":   "Give the frontmatter a name and a description that says what the skill does and when to use it.",
	"name_matches_package":   "Rename the package directory or the frontmatter name so they agree (skill-creator standard).",
	"title_present":          "Open the body with an H1 title naming the skill.",
	"body_substantial":       "Flesh the body out: purpose, workflow and boundaries in the document itself, not only in chat history.",
	"hard_gate_declared":     "Declare the non-negotiable rules under a HARD GATE section (the Beacon skill-package standard).",
	"progressive_disclosure": "Move detail out of SKILL.md into references/ so the entry document stays lean (progressive disclosure).",
	"references_resolve":     "Fix or remove the references that do not resolve to files inside the package.",
	"evals_present":          "Add evals/evals.json (schema skill-evals.v1): test prompts with expectations and machine checks.",
	"evals_cases":            "Give the package at least three eval cases, each with an id, a prompt and two or more expectations.",
	"evals_checks_pass":      "Fix the machine checks: every case must assert something the package actually states (contains / references_contains / min_examples).",
}
