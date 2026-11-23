package gate

import "strings"

// fsmSection retains the enclosing FSM scope across nested headings, without
// interpreting lookalike tables in unrelated sections or fenced examples.
type fsmSection struct {
	Body      string
	Model     bool
	Illegal   bool
	Aggregate string
}

func fsmSections(markdown string) []fsmSection {
	type scope struct {
		level          int
		model, illegal bool
		aggregate      string
	}
	var sections []fsmSection
	var stack []scope
	var lines []string
	flush := func() {
		if len(stack) > 0 && len(lines) > 0 {
			s := stack[len(stack)-1]
			sections = append(sections, fsmSection{Body: strings.Join(lines, "\n"), Model: s.model, Illegal: s.illegal, Aggregate: s.aggregate})
		}
		lines = nil
	}
	fence := ""
	for _, raw := range strings.Split(markdown, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			marker := line[:3]
			if fence == "" {
				fence = marker
			} else if fence == marker {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		if m := mdHeadingRe.FindStringSubmatch(line); m != nil {
			flush()
			level := len(m[1])
			for len(stack) > 0 && stack[len(stack)-1].level >= level {
				stack = stack[:len(stack)-1]
			}
			s := scope{level: level}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				s.model, s.illegal, s.aggregate = parent.model, parent.illegal, parent.aggregate
			}
			title := normalizeHeading(m[2])
			if illegalSectionExactTitles[title] || headingMatchesAny(title, illegalSectionTitles) || headingMatchesAny(title, domainIllegalTitles) {
				s.illegal = true
			} else if strings.Contains(title, "coverage") || strings.Contains(title, "覆盖") {
				s.model, s.illegal = false, false
			} else if strings.Contains(title, "fsm") || strings.Contains(title, "state machine") || strings.Contains(title, "状态机") || strings.Contains(title, "五元组") {
				s.model, s.illegal = true, false
				if headingMatchesAny(title, domainFSMTitles) {
					s.aggregate = parseAggregateName(m[2])
				}
			}
			stack = append(stack, s)
			continue
		}
		lines = append(lines, raw)
	}
	flush()
	return sections
}

func fsmColumns(header []string) map[string]int {
	columns := make(map[string]int, len(header))
	for i, cell := range header {
		name := strings.Join(strings.Fields(strings.ToLower(strings.Trim(cell, "`* "))), "_")
		columns[name] = i
	}
	return columns
}

func fsmCell(row []string, columns map[string]int, names ...string) string {
	for _, name := range names {
		if i, ok := columns[name]; ok {
			if i < len(row) {
				return strings.TrimSpace(row[i])
			}
			return ""
		}
	}
	return ""
}

// legacyFSMTransitions projects the old To/State | From | Guard aggregate
// dialect only when no event-based model exists. Legacy rows have no event;
// the fixed label preserves their unspecified-event meaning without evaluation.
func legacyFSMTransitions(truthMD string) (transitions []FSMTransition, present bool) {
	for _, sec := range fsmSections(truthMD) {
		if !sec.Model || sec.Illegal {
			continue
		}
		blocks := mdTableBlocks(sec.Body)
		for _, block := range blocks {
			if len(block) == 0 {
				continue
			}
			columns := fsmColumns(block[0])
			_, from := columns["from"]
			_, state := columns["state"]
			_, to := columns["to"]
			_, event := columns["event"]
			_, trigger := columns["trigger"]
			_, tc := columns["tc_id"]
			if event || trigger || tc {
				continue
			}
			if !from || (!state && !to) {
				continue
			}
			present = true
			for _, row := range block[1:] {
				source := fsmCell(row, columns, "from")
				if source == "" || source == "-" || source == "—" || source == "–" || strings.EqualFold(source, "none") || strings.EqualFold(source, "n/a") {
					continue // Legacy initial-state declarations are not edges.
				}
				for _, from := range strings.FieldsFunc(source, func(r rune) bool { return r == ',' || r == '/' || r == '|' || r == '，' || r == '、' }) {
					transitions = append(transitions, FSMTransition{
						State: strings.TrimSpace(from), To: fsmCell(row, columns, "to", "state"),
						Event: "transition", Guard: fsmCell(row, columns, "guard"), Aggregate: sec.Aggregate,
					})
				}
			}
		}
		if len(blocks) == 0 {
			for _, m := range domainArrowFormRe.FindAllStringSubmatch(sec.Body, -1) {
				present = true
				transitions = append(transitions, FSMTransition{State: strings.TrimSpace(m[1]), To: strings.TrimSpace(m[2]), Event: "transition", Aggregate: sec.Aggregate})
			}
		}
	}
	return transitions, present
}

// CheckTruthFSM checks a present model even when FSM is optional. A missing
// model is not applicable only with an explicit fsm_required:false declaration.
// Freeze callers retain their own legacy applicability policy; graph rules are
// always shared through ParseTruthFSM and ValidateTruthFSM.
func CheckTruthFSM(truthMD string) (projection FSMProjection, findings []FSMFinding, applicable bool, blocked bool) {
	projection = ParseTruthFSM(truthMD)
	scalars, _, _ := ParseFrontmatter(truthMD)
	for _, section := range fsmSections(truthMD) {
		if !section.Model || section.Illegal {
			continue
		}
		for _, block := range mdTableBlocks(section.Body) {
			if len(block) == 0 {
				continue
			}
			columns := fsmColumns(block[0])
			has := func(key string) bool { _, ok := columns[key]; return ok }
			modern := (has("from") || has("state")) && (has("to") || has("to_state")) && (has("event") || has("trigger")) && (has("guard") || has("action") || has("side_effect"))
			legacy := has("from") && (has("state") || has("to")) && !has("event") && !has("trigger") && !has("tc_id")
			if !modern && !legacy {
				return projection, []FSMFinding{{Kind: "fsm_model_invalid", Severity: "error", Text: "FSM table columns do not define a supported transition model"}}, true, true
			}
		}
	}
	if !projection.FiveTuplePresent && strings.EqualFold(strings.TrimSpace(scalars["fsm_required"]), "false") {
		return projection, nil, false, false
	}
	if !projection.FiveTuplePresent {
		for _, section := range fsmSections(truthMD) {
			if section.Model && !section.Illegal {
				return projection, []FSMFinding{{Kind: "fsm_model_missing", Severity: "error", Text: "FSM section has no parseable transition model"}}, true, true
			}
		}
		return projection, []FSMFinding{{Kind: "fsm_model_missing", Severity: "error", Text: "no applicable FSM model found; stateless features must declare fsm_required:false"}}, true, true
	}
	findings, blocked = ValidateTruthFSM(projection)
	return projection, findings, true, blocked
}
