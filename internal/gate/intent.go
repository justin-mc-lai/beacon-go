// Intent gate (R1: ask before sign). The freeze gate's intent half:
// before a package can be signed, the author must have recorded the
// user's intent, closed every open question, traced every AC to an
// intent source, and produced the domain FSM / parity materials the
// package's own frontmatter requires.
//
// The interaction (one question at a time) lives in the truth
// ask/answer CLI; this file is the pure, deterministic judgement over
// the resulting markdown. No prompts, no model calls — the same bytes
// always produce the same findings, so the gate is auditable.
package gate

import (
	"regexp"
	"strings"

	"beacon-go/internal/core"
)

// IntentSnapshot is the recorded user-intent block.
type IntentSnapshot struct {
	Utterance   string `json:"utterance"`
	Goal        string `json:"goal"`
	NonGoals    string `json:"non_goals"`
	Constraints string `json:"constraints"`
	ScopeMode   string `json:"scope_mode"`
}

// OpenQuestion is one row of the open-question ledger.
type OpenQuestion struct {
	ID             string `json:"id"`
	Question       string `json:"question"`
	Recommendation string `json:"recommendation,omitempty"`
	Status         string `json:"status"`
	Answer         string `json:"answer,omitempty"`
	// Source marks who raised the question (R2d, audit M4):
	// "human" for hand-asked rows, "kernel:brainstorm-*" for
	// rows the kernel generated. Empty parses as human.
	Source string `json:"source,omitempty"`
	Line   int    `json:"line,omitempty"`
}

// IntentCoverageRow traces one AC to the intent/evidence it serves.
type IntentCoverageRow struct {
	AC       string `json:"ac"`
	Source   string `json:"source"`
	Evidence string `json:"evidence"`
	Line     int    `json:"line,omitempty"`
}

// FSMRow is one row of the domain state machine.
type FSMRow struct {
	State  string `json:"state"`
	Event  string `json:"event"`
	Guard  string `json:"guard"`
	To     string `json:"to"`
	Action string `json:"action"`
	Line   int    `json:"line,omitempty"`
}

var placeholderPrefixes = []string{"(fill", "（fill", "(todo", "（todo", "待补", "待填", "tbd", "todo"}

func isPlaceholder(s string) bool {
	t := strings.ToLower(strings.TrimSpace(s))
	if t == "" || t == "-" || t == "n/a" {
		return true
	}
	for _, p := range placeholderPrefixes {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

// sectionText returns the body of the first `## ` section whose
// heading contains name (case-insensitive), without the heading line.
func sectionText(text, name string) (string, bool) {
	name = strings.ToLower(name)
	lines := strings.Split(text, "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") && strings.Contains(strings.ToLower(line), name) {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return "", false
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n"), true
}

func splitRow(line string) []string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// tableOf returns the first markdown table in a section: lowercased
// headers plus rows (separator rows skipped).
func tableOf(section string) (headers []string, rows [][]string) {
	lines := strings.Split(section, "\n")
	in := false
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "|") {
			if in && t != "" {
				break
			}
			continue
		}
		cells := splitRow(t)
		if !in {
			in = true
			for _, c := range cells {
				headers = append(headers, strings.ToLower(strings.TrimSpace(c)))
			}
			continue
		}
		if isSeparatorRow(cells) {
			continue
		}
		rows = append(rows, cells)
	}
	return headers, rows
}

func col(headers []string, row []string, names ...string) string {
	for i, h := range headers {
		for _, n := range names {
			if h == n {
				if i < len(row) {
					return strings.TrimSpace(row[i])
				}
				return ""
			}
		}
	}
	return ""
}

func yamlBlock(section string) map[string]string {
	out := map[string]string{}
	lines := strings.Split(section, "\n")
	in := false
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") {
			in = !in
			continue
		}
		if !in || t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		k, v, ok := strings.Cut(t, ":")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	// Tolerate key: value lines without a fenced block.
	if len(out) == 0 {
		for _, line := range lines {
			t := strings.TrimSpace(line)
			k, v, ok := strings.Cut(t, ":")
			if !ok || strings.Contains(k, "|") {
				continue
			}
			out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return out
}

// ParseIntentSnapshot reads the `## User Intent Snapshot` block.
func ParseIntentSnapshot(truth string) IntentSnapshot {
	sec, _ := sectionText(truth, "user intent snapshot")
	m := yamlBlock(sec)
	return IntentSnapshot{
		Utterance: m["utterance"], Goal: m["goal"],
		NonGoals: m["non_goals"], Constraints: m["constraints"],
		ScopeMode: m["scope_mode"],
	}
}

func questionStatus(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "open"
	}
	return s
}

func isClosedStatus(s string) bool {
	switch questionStatus(s) {
	case "answered", "dropped", "closed", "resolved", "done", "n/a":
		return true
	}
	return false
}

// ParseOpenQuestions reads the `## Open Questions` ledger.
func ParseOpenQuestions(truth string) []OpenQuestion {
	sec, _ := sectionText(truth, "open questions")
	headers, rows := tableOf(sec)
	out := []OpenQuestion{}
	for i, r := range rows {
		q := OpenQuestion{
			ID:             col(headers, r, "q id", "id", "question id"),
			Question:       col(headers, r, "question"),
			Recommendation: col(headers, r, "recommendation"),
			Status:         questionStatus(col(headers, r, "status")),
			Answer:         col(headers, r, "answer"),
			Source:         col(headers, r, "source"),
			Line:           i + 1,
		}
		if q.ID == "" && q.Question == "" {
			continue
		}
		out = append(out, q)
	}
	return out
}

// ParseIntentCoverage reads the `## Intent Coverage` matrix.
func ParseIntentCoverage(truth string) []IntentCoverageRow {
	sec, _ := sectionText(truth, "intent coverage")
	headers, rows := tableOf(sec)
	out := []IntentCoverageRow{}
	for i, r := range rows {
		ac := strings.ToUpper(col(headers, r, "ac id", "ac", "requirement"))
		if !acRefRe.MatchString(ac) {
			continue
		}
		out = append(out, IntentCoverageRow{
			AC: ac, Source: col(headers, r, "intent source", "source"),
			Evidence: col(headers, r, "evidence", "evidence ref"), Line: i + 1,
		})
	}
	return out
}

// fsmSectionText returns the body of the package's FSM section
// under any of its dialect headings: the Go skeleton's original
// `## Domain FSM`, the v1.6.19 gold dialect's `## FSM 五元组`,
// or a domain state-machine heading.
func fsmSectionText(truth string) (string, bool) {
	for _, name := range []string{"domain fsm", "fsm 五元组", "业务状态机", "领域状态机", "domain state machine", "状态机"} {
		if sec, ok := sectionText(truth, name); ok {
			return sec, true
		}
	}
	return "", false
}

// ParseFSM reads the FSM five-tuple table.
func ParseFSM(truth string) []FSMRow {
	sec, _ := fsmSectionText(truth)
	headers, rows := tableOf(sec)
	out := []FSMRow{}
	for i, r := range rows {
		out = append(out, FSMRow{
			State: col(headers, r, "state"), Event: col(headers, r, "event"),
			Guard: col(headers, r, "guard"), To: col(headers, r, "to"),
			Action: col(headers, r, "action"), Line: i + 1,
		})
	}
	return out
}

// ---- Check E: vague disjunction (truth_review_gate.check_e_vague) ----

// vaguePatterns are check_e_vague's pattern list, copied verbatim
// (truth_review_gate.py lines 660-675): a deterministic AC cannot
// carry a branch word — the implementer would have to guess.
var vaguePatterns = []struct {
	re    *regexp.Regexp
	label string
}{
	{regexp.MustCompile(`或者`), "或者"},
	{regexp.MustCompile(`至少`), "至少"},
	{regexp.MustCompile(`不少于`), "不少于"},
	{regexp.MustCompile(`等[，。、]|等等`), "等"},
	{regexp.MustCompile(`(?:^|[^a-zA-Z0-9_])或(?:[^a-zA-Z0-9_]|$)`), "或"},
	{regexp.MustCompile(`若干`), "若干"},
	{regexp.MustCompile(`视情况`), "视情况"},
	{regexp.MustCompile(`(?i)\bat\s+least\b`), "at least"},
	{regexp.MustCompile(`(?i)\bat\s+most\b`), "at most"},
	{regexp.MustCompile(`(?i)\betc\.?\b`), "etc"},
	{regexp.MustCompile(`(?i)\band\s+so\s+on\b`), "and so on"},
	{regexp.MustCompile(`(?i)\bor\b`), "or"},
}

var backtickSpanRe = regexp.MustCompile("`[^`]*`")

// checkEVague ports check_e_vague: strip code ticks first so code
// identifiers never false-fire, then match the branch words.
func checkEVague(truth string, add func(kind, code, detail string)) {
	for _, ac := range ExtractACDescriptions(truth) {
		plain := backtickSpanRe.ReplaceAllString(ac.Description, "")
		for _, p := range vaguePatterns {
			if p.re.MatchString(plain) {
				text := strings.TrimSpace(plain)
				if r := []rune(text); len(r) > 80 {
					text = string(r[:80])
				}
				add("ac_vague_disjunction", core.ReasonACVagueDisjunction,
					ac.ID+" 含模糊词 '"+p.label+"': "+text)
				break
			}
		}
	}
}

// ---- Check D additions (truth_review_gate.check_d_fill_depth) ----

var journeyHeadingRe = regexp.MustCompile(`(?im)^##\s+(用户旅程|user journey|journey)(?:\s|$)`)

// metaFSMStates is check_d_fill_depth's _META_FSM_STATES: a
// business package whose FSM models only the package lifecycle
// (draft/frozen/...) is impersonating a domain FSM.
var metaFSMStates = map[string]bool{
	"draft": true, "in_review": true, "frozen": true,
	"humanize_pending": true, "refreeze_review": true, "in-review": true,
}

func checkDFillDepth(pkg PackageFiles, scalars map[string]string, add func(kind, code, detail string)) {
	body := pkg.Truth.Text
	// Scaffold snapshot markers (_SCAFFOLD_MARKERS handling).
	if strings.Contains(body, "scaffold: v1.6.10-gold-aligned") ||
		(strings.Contains(body, "v1.6.10-gold-aligned") && strings.Contains(body, "scaffold:")) {
		add("scaffold_snapshot", core.ReasonTruthScaffoldNotFilled,
			"User Intent Snapshot still marks scaffold: v1.6.10-gold-aligned — replace with filled lake content")
	}
	// L0 generic placeholder wording.
	if l0PlaceholderRe.MatchString(body) {
		add("scaffold_l0_placeholder", core.ReasonTruthScaffoldNotFilled,
			"L0 still uses generic scaffold wording — rewrite 人话 for this lake")
	}
	if !businessDomain(scalars, body) {
		return
	}
	if !journeyHeadingRe.MatchString(body) {
		add("journey_missing", core.ReasonTruthScaffoldNotFilled,
			"business domain lake missing ## 用户旅程 / ## User Journey section")
	}
	proj := ParseTruthFSM(body)
	if len(proj.States) > 0 {
		onlyMeta := true
		for _, st := range proj.States {
			if !metaFSMStates[strings.ToLower(strings.TrimSpace(st))] {
				onlyMeta = false
				break
			}
		}
		if onlyMeta {
			add("scaffold_meta_fsm", core.ReasonTruthScaffoldNotFilled,
				"FSM only models package lifecycle (draft/in_review/frozen) — fill real domain states before freeze")
		}
	}
}

var l0PlaceholderRe = regexp.MustCompile("本湖让用户能完成\\s*`[^`]+`\\s*的核心结果")

// CheckIntentGate is the pure intent judgement over one package.
func CheckIntentGate(pkg PackageFiles) []Finding {
	if !pkg.Truth.Present || strings.TrimSpace(pkg.Truth.Text) == "" {
		return nil // completeness owns the missing-file finding
	}
	file := pkg.Truth.Path
	scalars, _, _ := ParseFrontmatter(pkg.Truth.Text)
	var out []Finding
	add := func(kind, code, detail string) {
		out = append(out, Finding{Kind: kind, ReasonCode: code, File: file, Detail: detail, RepairRoute: DefaultRepairRoute})
	}

	if strings.ToLower(strings.TrimSpace(scalars["intent_confirmed"])) != "true" {
		add("intent_not_confirmed", core.ReasonIntentNotConfirmed,
			"intent_confirmed is not true; run `beacon truth confirm` after recording intent and closing open questions")
	}

	snap := ParseIntentSnapshot(pkg.Truth.Text)
	missing := []string{}
	for _, f := range []struct{ name, val string }{
		{"utterance", snap.Utterance}, {"goal", snap.Goal},
		{"non_goals", snap.NonGoals}, {"constraints", snap.Constraints},
		{"scope_mode", snap.ScopeMode},
	} {
		if isPlaceholder(f.val) {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 {
		add("intent_snapshot_incomplete", core.ReasonIntentSnapshotIncomple,
			"User Intent Snapshot is missing: "+strings.Join(missing, ", ")+
				" (one answer per unknown; `beacon truth intent` records the fields)")
	}

	for _, q := range ParseOpenQuestions(pkg.Truth.Text) {
		if !isClosedStatus(q.Status) {
			add("intent_question_open", core.ReasonIntentQuestionOpen,
				q.ID+" is still open: "+q.Question)
		}
	}

	defined := DefinedACs(pkg.Truth.Text)
	covered := map[string]bool{}
	for _, row := range ParseIntentCoverage(pkg.Truth.Text) {
		covered[row.AC] = true
		if !defined[row.AC] {
			add("intent_coverage_unknown_ac", core.ReasonIntentCoverageUnknown,
				"Intent Coverage names "+row.AC+" which truth does not define")
			continue
		}
		if isPlaceholder(row.Source) || isPlaceholder(row.Evidence) {
			add("intent_coverage_untraced", core.ReasonIntentCoverageUntraced,
				row.AC+" has no traced intent source/evidence")
		}
	}
	var acs []string
	for id := range defined {
		acs = append(acs, id)
	}
	sortStrings(acs)
	for _, id := range acs {
		if !covered[id] {
			add("intent_coverage_missing", core.ReasonIntentCoverageMissing,
				id+" is defined in truth but absent from the Intent Coverage matrix")
		}
	}

	// Check B (R2a, audit H2): the domain FSM is validated as a
	// GRAPH by the one validator review and freeze share
	// (truth_fsm.validate_truth_fsm, gate/fsm.go) — the pre-R2a
	// keyword sniffing (a row exists, the words hang/illegal
	// appear) let a wrong graph freeze.
	if truthyValue(scalars["domain_required"]) {
		proj := ParseTruthFSM(pkg.Truth.Text)
		if !proj.FiveTuplePresent {
			add("intent_fsm_missing", core.ReasonIntentFSMMissing,
				"domain_required is true but no FSM five-tuple (State|Event|Guard|To|Action) table is present")
		} else {
			for _, t := range proj.Transitions {
				if isPlaceholder(t.State) || isPlaceholder(t.Event) || isPlaceholder(t.Guard) ||
					isPlaceholder(t.To) || isPlaceholder(t.Action) {
					add("intent_fsm_incomplete", core.ReasonIntentFSMIncomplete,
						"Domain FSM row is missing State/Event/Guard/To/Action")
					break
				}
			}
			ffindings, _ := ValidateTruthFSM(proj)
			for _, ff := range ffindings {
				if ff.Severity == "error" {
					add(ff.Kind, core.ReasonTruthFSMInvalid, ff.Text)
				}
			}
		}
	}

	maturity := strings.ToLower(strings.TrimSpace(scalars["package_maturity"]))
	if maturity == "" || maturity == "scaffold" {
		add("intent_package_scaffold", core.ReasonIntentPackageScaffold,
			"package_maturity is scaffold/empty; fill the package before signing")
	}
	if human, ok := sectionText(pkg.Truth.Text, "人话"); !ok || isPlaceholder(human) ||
		strings.Contains(human, "一句话说清这个需求") || strings.Contains(human, "冻结前由作者补齐") {
		add("intent_package_scaffold", core.ReasonIntentPackageScaffold,
			"## 人话 still carries the skeleton placeholder; write the one-sentence human summary")
	}

	// Check D fill depth (R2a, audit H4): scaffold snapshot
	// markers, the L0 generic placeholder, and — for business
	// domains — the two hard checks (a user journey must exist;
	// a lifecycle-only FSM is an impersonation, not a domain FSM).
	checkDFillDepth(pkg, scalars, add)

	// Check E (R2a, audit H1): vague branch words in AC text.
	checkEVague(pkg.Truth.Text, add)

	// Scope mode is a controlled vocabulary (R2a, audit M6): a
	// hand-written value outside it silently disables the
	// full_parity machinery in Python; here it fails closed.
	if !isPlaceholder(snap.ScopeMode) && !ValidScopeMode(snap.ScopeMode) {
		add("scope_mode_invalid", core.ReasonScopeModeInvalid,
			"scope_mode '"+snap.ScopeMode+"' is not in the controlled vocabulary ("+
				strings.Join(ScopeModeVocabulary, " | ")+"); record intent again with a valid or derived mode")
	}

	if strings.EqualFold(strings.TrimSpace(snap.ScopeMode), "full_parity") {
		sec, ok := sectionText(pkg.Truth.Text, "parity matrix")
		if !ok || len(strings.TrimSpace(sec)) == 0 {
			add("intent_parity_missing", core.ReasonIntentParityMissing,
				"scope_mode is full_parity but no Parity Matrix is present")
		}
		// Deferral ledger (R2a, audit M5): under a full-parity
		// promise, deferral wording in the utterance or the ACs
		// must be dispositioned in the package's Deferral Ledger
		// with a named user approval — a silent scope cut has no
		// record and no intercept point otherwise
		// (planner_review_runtime._deferral_ledger).
		blob := strings.ToLower(snap.Utterance)
		for _, ac := range ExtractACDescriptions(pkg.Truth.Text) {
			blob += "\n" + strings.ToLower(ac.Description)
		}
		if containsAnySignal(blob, deferralSignals) {
			approved := false
			for _, e := range ParseDeferralLedger(pkg.Truth.Text) {
				if e.Approved() {
					approved = true
					break
				}
			}
			if !approved {
				add("unapproved_scope_reduction", core.ReasonDeferralUnapproved,
					"full_parity scope carries deferral wording (mvp/defer/later/延期/裁剪/partial) but no user-approved entry exists in the ## Deferral Ledger; record the deferred item, its reason and the approver, or remove the reduction")
			}
		}
	}
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
