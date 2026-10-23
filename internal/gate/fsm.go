// Truth FSM graph validation (R2a, parity audit H2): the Go port of
// beacon/utils/truth_fsm.py's parse_truth_fsm + validate_truth_fsm.
//
// Python's review Check B used to keep a second, lenient terminal
// heuristic that could disagree with freeze; v1.6.14 unified both on
// this one validator (truth_review_gate.check_b_fsm_audit delegates
// here). The Go intent gate's B segment calls this same validator —
// graph-level checks (transition targets, events, illegal slots,
// illegal edges that are also declared legal), not keyword sniffing.
//
// Constants and rules are copied from the Python source; comments
// cite truth_fsm.py sections. Mermaid rendering / digests are NOT
// ported: they are display/change-detection projections, not gate
// judgements (see the R2a report's difference list).
package gate

import (
	"regexp"
	"sort"
	"strings"
)

// FSMTransition is one five-tuple row (State|Event|Guard|To|Action).
type FSMTransition struct {
	State  string `json:"state"`
	Event  string `json:"event"`
	Guard  string `json:"guard"`
	To     string `json:"to"`
	Action string `json:"action"`
}

// IllegalSlot is one illegal-transition entry. From/To are populated
// only for explicit machine syntax (`illegal: a -> b` or a table
// row); free prose stays documentation (truth_fsm.py _parse_illegal_slots).
type IllegalSlot struct {
	Description string `json:"description"`
	From        string `json:"from,omitempty"`
	To          string `json:"to,omitempty"`
}

// FSMProjection is the normalized FSM contract of one truth document.
type FSMProjection struct {
	FiveTuplePresent bool            `json:"five_tuple_present"`
	States           []string        `json:"states"`
	Transitions      []FSMTransition `json:"transitions"`
	Illegal          []IllegalSlot   `json:"illegal"`
	InitialStates    []string        `json:"initial_states"`
	TerminalStates   []string        `json:"terminal_states"`
	IllegalPresent   bool            `json:"illegal_present"`
}

// FSMFinding is one validation finding; Severity is "error"
// (blocks) or "warning" (reported, never blocks) — the Python
// validator's own distinction.
type FSMFinding struct {
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	Text     string `json:"text"`
	State    string `json:"state,omitempty"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
}

// ---- markdown table plumbing (truth_review_gate.py helpers) ----

// splitTableRowTicks splits a markdown table row on | while
// ignoring pipes inside backticks (truth_review_gate._split_table_row).
func splitTableRowTicks(line string) []string {
	stripped := strings.TrimSpace(line)
	stripped = strings.TrimPrefix(stripped, "|")
	stripped = strings.TrimSuffix(stripped, "|")
	var cells []string
	var buf strings.Builder
	inTick := false
	for _, ch := range stripped {
		if ch == '`' {
			inTick = !inTick
			buf.WriteRune(ch)
			continue
		}
		if ch == '|' && !inTick {
			cells = append(cells, strings.TrimSpace(buf.String()))
			buf.Reset()
			continue
		}
		buf.WriteRune(ch)
	}
	cells = append(cells, strings.TrimSpace(buf.String()))
	return cells
}

// mdTableBlocks splits markdown into contiguous pipe-table blocks
// (truth_review_gate._table_blocks): separator rows are dropped.
func mdTableBlocks(markdown string) [][][]string {
	var blocks [][][]string
	var current [][]string
	flush := func() {
		if current != nil {
			blocks = append(blocks, current)
			current = nil
		}
	}
	for _, line := range strings.Split(markdown, "\n") {
		stripped := strings.TrimSpace(line)
		if strings.HasPrefix(stripped, "|") {
			cells := splitTableRowTicks(stripped)
			sep := len(cells) > 0
			for _, c := range cells {
				if c == "" {
					continue
				}
				for _, r := range c {
					if r != '-' && r != ':' && r != ' ' {
						sep = false
					}
				}
			}
			if sep {
				continue
			}
			current = append(current, cells)
			continue
		}
		flush()
	}
	flush()
	return blocks
}

func headerJoin(row []string) string {
	parts := make([]string, len(row))
	for i, c := range row {
		parts[i] = strings.ToLower(c)
	}
	return strings.Join(parts, " ")
}

// ---- five-tuple extraction (truth_review_gate.extract_fsm_five_tuple) ----

func extractFSMFiveTuple(truthMD string) (transitions []FSMTransition, present bool, illegalPresent bool) {
	for _, block := range mdTableBlocks(truthMD) {
		if len(block) == 0 {
			continue
		}
		header := headerJoin(block[0])
		if !strings.Contains(header, "state") || !strings.Contains(header, "event") {
			continue
		}
		if !strings.Contains(header, "to") && !strings.Contains(header, "guard") {
			continue
		}
		headers := make([]string, len(block[0]))
		for i, c := range block[0] {
			headers[i] = strings.ReplaceAll(strings.ToLower(c), " ", "_")
		}
		get := func(row []string, idx int, names ...string) string {
			for _, n := range names {
				for i, h := range headers {
					if h == n {
						if i < len(row) {
							return row[i]
						}
						return ""
					}
				}
			}
			if idx < len(row) {
				return row[idx]
			}
			return ""
		}
		for _, row := range block[1:] {
			if len(row) == 0 {
				continue
			}
			state := strings.TrimSpace(get(row, 0, "state", "from"))
			event := strings.TrimSpace(get(row, 1, "event"))
			guard := strings.TrimSpace(get(row, 2, "guard"))
			to := strings.TrimSpace(get(row, 3, "to", "to_state"))
			action := strings.TrimSpace(get(row, 4, "action"))
			if state == "" || strings.ToLower(state) == "state" || strings.ToLower(state) == "from" {
				continue
			}
			if event == "" || strings.ToLower(event) == "event" {
				continue
			}
			transitions = append(transitions, FSMTransition{
				State: state, Event: event, Guard: guard, To: to, Action: action,
			})
		}
	}
	illegalPresent = fsmIllegalHeadingRe.MatchString(truthMD) ||
		fsmInvalidColonRe.MatchString(truthMD) || fsmIllegalColonRe.MatchString(truthMD)
	return transitions, len(transitions) > 0, illegalPresent
}

var (
	fsmIllegalHeadingRe = regexp.MustCompile(`(?im)^(#{2,3}\s+)?(invalid|illegal|非法)`)
	fsmInvalidColonRe   = regexp.MustCompile(`(?i)\binvalid\b[:：]`)
	fsmIllegalColonRe   = regexp.MustCompile(`(?i)illegal[：:]`)
)

// ---- sections + illegal slots (truth_fsm.py) ----

type mdSection struct {
	Title string
	Body  string
}

var mdHeadingRe = regexp.MustCompile(`^(#{2,6})\s+(.+?)\s*$`)

// extractMDSections splits markdown at ##..###### headings
// (truth_fsm._extract_sections).
func extractMDSections(markdown string) []mdSection {
	var sections []mdSection
	title := ""
	var buf []string
	flush := func() {
		if title != "" {
			sections = append(sections, mdSection{Title: title, Body: strings.TrimSpace(strings.Join(buf, "\n"))})
		}
	}
	for _, raw := range strings.Split(markdown, "\n") {
		m := mdHeadingRe.FindStringSubmatch(strings.TrimSpace(raw))
		if m != nil {
			flush()
			title = strings.TrimSpace(m[2])
			buf = nil
			continue
		}
		if title != "" {
			buf = append(buf, raw)
		}
	}
	flush()
	return sections
}

func normalizeHeading(title string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(title))), " ")
}

// truth_fsm._ILLEGAL_SECTION_TITLES + _ILLEGAL_SECTION_EXACT_TITLES.
var illegalSectionTitles = []string{
	"illegal transition", "illegal transitions", "非法转移",
	"非法状态转移", "非法转换", "invalid / illegal", "invalid or illegal",
}

var illegalSectionExactTitles = map[string]bool{"invalid": true, "illegal": true}

var illegalArrowRe = regexp.MustCompile(`([A-Za-z][A-Za-z0-9_]*)\s*(?:->|→|-->)\s*([A-Za-z][A-Za-z0-9_]*)`)
var illegalExplicitRe = regexp.MustCompile(`^(?:illegal|非法)\s*[:：]\s*(.+)$`)

var illegalTableHeaderCells = map[string]bool{
	"from": true, "state": true, "源": true, "从": true, "起始": true, "起始状态": true,
}

// parseIllegalSlots ports truth_fsm._parse_illegal_slots: entries
// from illegal-titled sections only; from/to only for explicit
// machine syntax, so prose describing a bypass never becomes a
// phantom illegal edge.
func parseIllegalSlots(truthMD string) []IllegalSlot {
	var slots []IllegalSlot
	for _, sec := range extractMDSections(truthMD) {
		title := normalizeHeading(sec.Title)
		matched := illegalSectionExactTitles[title]
		if !matched {
			for _, tok := range illegalSectionTitles {
				if strings.Contains(title, tok) {
					matched = true
					break
				}
			}
		}
		if !matched {
			continue
		}
		for _, line := range strings.Split(sec.Body, "\n") {
			stripped := strings.TrimSpace(line)
			if strings.HasPrefix(stripped, "|") {
				cells := splitRowNaive(stripped)
				if len(cells) < 2 || cells[0] == "" {
					continue
				}
				if isSepCell(cells[0]) || illegalTableHeaderCells[strings.ToLower(cells[0])] {
					continue
				}
				slots = append(slots, IllegalSlot{
					Description: strings.Join(cells, " | "), From: cells[0], To: cells[1],
				})
				continue
			}
			if !strings.HasPrefix(stripped, "-") && !strings.HasPrefix(stripped, "*") && !strings.HasPrefix(stripped, "•") {
				continue
			}
			text := strings.TrimSpace(strings.TrimLeft(stripped, "-*• "))
			if text == "" || onlySepChars(text) {
				continue
			}
			from, to := "", ""
			if m := illegalExplicitRe.FindStringSubmatch(text); m != nil {
				if a := illegalArrowRe.FindStringSubmatch(m[1]); a != nil {
					from, to = a[1], a[2]
				}
			}
			slots = append(slots, IllegalSlot{Description: text, From: from, To: to})
		}
	}
	seen := map[string]bool{}
	var out []IllegalSlot
	for _, s := range slots {
		key := strings.ToLower(s.Description)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, s)
	}
	return out
}

// splitRowNaive mirrors Python's stripped.strip("|").split("|")
// used by truth_fsm's illegal-table parsing (no backtick care).
func splitRowNaive(line string) []string {
	parts := strings.Split(strings.Trim(line, "|"), "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func isSepCell(cell string) bool {
	if cell == "" {
		return false
	}
	for _, r := range cell {
		if r != '-' && r != ':' {
			return false
		}
	}
	return true
}

func onlySepChars(s string) bool {
	for _, r := range s {
		if r != '-' && r != ':' && r != ' ' && r != '>' {
			return false
		}
	}
	return true
}

// ---- terminal states (truth_fsm._parse_terminal_states) ----

var (
	terminalColonRe  = regexp.MustCompile(`(?i)(终态|terminal(?:\s+states?)?)\s*[:：]\s*([^\n]+)`)
	terminalSuffixRe = regexp.MustCompile(`(?i)([A-Za-z][A-Za-z0-9_]*(?:\s*[/、,，]\s*[A-Za-z][A-Za-z0-9_]*)*)\s*为(?:最终状态|终态)`)
	mermaidTermRe    = regexp.MustCompile(`(?m)^\s*([A-Za-z0-9_]+)\s*-->\s*\[\*\]\s*$`)
	terminalSplitRe  = regexp.MustCompile(`[,/|，、\s]+`)
)

func collectTerminalTokens(chunk string, out map[string]bool) {
	for _, tok := range terminalSplitRe.Split(chunk, -1) {
		tok = strings.Trim(tok, "`*。；;，,、")
		if tok != "" && len([]rune(tok)) < 40 {
			out[tok] = true
		}
	}
}

func parseTerminalStates(truthMD string) map[string]bool {
	out := map[string]bool{}
	for _, m := range terminalColonRe.FindAllStringSubmatchIndex(truthMD, -1) {
		// Python's (?<!非）终态 lookbehind: a 终态 immediately
		// preceded by 非 is a non-terminal mention, not a declaration.
		kw := truthMD[m[2]:m[3]]
		if strings.HasPrefix(kw, "终") && m[2] > 0 {
			prev := []rune(truthMD[:m[2]])
			if len(prev) > 0 && prev[len(prev)-1] == '非' {
				continue
			}
		}
		collectTerminalTokens(truthMD[m[4]:m[5]], out)
	}
	for _, m := range terminalSuffixRe.FindAllStringSubmatch(truthMD, -1) {
		collectTerminalTokens(m[1], out)
	}
	for _, m := range mermaidTermRe.FindAllStringSubmatch(truthMD, -1) {
		if tok := strings.TrimSpace(m[1]); tok != "" && len([]rune(tok)) < 40 {
			out[tok] = true
		}
	}
	return out
}

// ---- parse + validate (truth_fsm.parse_truth_fsm / validate_truth_fsm) ----

// ParseTruthFSM parses the five-tuple + illegal transitions of one
// truth document into the normalized projection.
func ParseTruthFSM(truthMD string) FSMProjection {
	transitions, present, illegalRe := extractFSMFiveTuple(truthMD)
	stateSet := map[string]bool{}
	fromSet := map[string]bool{}
	toSet := map[string]bool{}
	for _, t := range transitions {
		if t.State != "" {
			stateSet[t.State] = true
			fromSet[t.State] = true
		}
		if t.To != "" {
			stateSet[t.To] = true
			toSet[t.To] = true
		}
	}
	var states, initial []string
	for s := range stateSet {
		states = append(states, s)
	}
	sort.Strings(states)
	for s := range fromSet {
		if !toSet[s] {
			initial = append(initial, s)
		}
	}
	sort.Strings(initial)
	terminals := parseTerminalStates(truthMD)
	var termList []string
	for s := range terminals {
		termList = append(termList, s)
	}
	sort.Strings(termList)
	illegal := parseIllegalSlots(truthMD)
	return FSMProjection{
		FiveTuplePresent: present,
		States:           states,
		Transitions:      transitions,
		Illegal:          illegal,
		InitialStates:    initial,
		TerminalStates:   termList,
		IllegalPresent:   len(illegal) > 0 || illegalRe,
	}
}

// ValidateTruthFSM ports truth_fsm.validate_truth_fsm. A state
// with no outgoing transition IS terminal by definition; an
// undeclared terminal is a warning, not a block (Python #30/#31).
// blocked reports whether any error-severity finding exists.
func ValidateTruthFSM(p FSMProjection) (findings []FSMFinding, blocked bool) {
	if !p.FiveTuplePresent {
		return nil, false
	}
	states := map[string]bool{}
	for _, s := range p.States {
		states[strings.TrimSpace(s)] = true
	}
	sourceStates := map[string]bool{}
	for _, t := range p.Transitions {
		sourceStates[strings.TrimSpace(t.State)] = true
	}
	declaredTerminals := map[string]bool{}
	for _, s := range p.TerminalStates {
		declaredTerminals[s] = true
	}
	structuralTerminals := map[string]bool{}
	for s := range states {
		if s != "" && !sourceStates[s] {
			structuralTerminals[s] = true
		}
	}
	for _, t := range p.Transitions {
		src := strings.TrimSpace(t.State)
		dst := strings.TrimSpace(t.To)
		if dst == "" {
			name := src
			if name == "" {
				name = "?"
			}
			findings = append(findings, FSMFinding{
				Kind: "transition_target_missing", Severity: "error",
				Text: "transition from '" + name + "' has no destination state", State: src,
			})
			continue
		}
		if src != "" && !states[src] {
			findings = append(findings, FSMFinding{
				Kind: "state_undefined", Severity: "error",
				Text: "transition source state '" + src + "' is not declared in FSM states", State: src,
			})
		}
		if dst != "" && !states[dst] {
			findings = append(findings, FSMFinding{
				Kind: "state_undefined", Severity: "error",
				Text: "transition destination state '" + dst + "' is not declared in FSM states", State: dst,
			})
		}
		if dst != "" && !sourceStates[dst] && !declaredTerminals[dst] {
			findings = append(findings, FSMFinding{
				Kind: "terminal_undeclared", Severity: "warning",
				Text: "state '" + dst + "' ends the flow but is not declared terminal; " +
					"add an illegal-transition entry or declare it （终态: " + dst + " / " + dst + " --> [*])",
				State: dst,
			})
		}
		if t.Event == "" {
			findings = append(findings, FSMFinding{
				Kind: "transition_event_missing", Severity: "error",
				Text: "transition " + orQ(src) + " -> " + orQ(dst) + " has no event",
			})
		}
	}
	var declared []string
	for s := range declaredTerminals {
		declared = append(declared, s)
	}
	sort.Strings(declared)
	for _, d := range declared {
		if d != "" && !states[d] {
			findings = append(findings, FSMFinding{
				Kind: "terminal_undefined", Severity: "warning",
				Text: "declared terminal '" + d + "' is not a state in the FSM five-tuple (typo?)", State: d,
			})
		}
	}
	if len(states) > 0 && len(structuralTerminals) == 0 && len(declaredTerminals) == 0 {
		findings = append(findings, FSMFinding{
			Kind: "fsm_terminal_missing", Severity: "warning",
			Text: "no state can end the flow and no terminal state is declared (event-driven workflows may cycle on purpose)",
		})
	}
	if len(p.Illegal) == 0 {
		findings = append(findings, FSMFinding{
			Kind: "illegal_slot_missing", Severity: "error",
			Text: "FSM five-tuple requires an Illegal transitions section (fail-closed on missing illegal)",
		})
	} else {
		legal := map[[2]string]bool{}
		for _, t := range p.Transitions {
			legal[[2]string{strings.TrimSpace(t.State), strings.TrimSpace(t.To)}] = true
		}
		for _, item := range p.Illegal {
			src, dst := strings.TrimSpace(item.From), strings.TrimSpace(item.To)
			if src != "" && dst != "" && legal[[2]string{src, dst}] {
				findings = append(findings, FSMFinding{
					Kind: "illegal_declared_as_legal", Severity: "error",
					Text: "explicit illegal transition " + src + " -> " + dst + " is also declared as a legal transition",
					From: src, To: dst,
				})
			}
		}
	}
	for _, f := range findings {
		if f.Severity == "error" {
			blocked = true
		}
	}
	return findings, blocked
}

func orQ(s string) string {
	if s == "" {
		return "?"
	}
	return s
}
