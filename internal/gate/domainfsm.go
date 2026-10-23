// Domain FSM authority (R2a, parity audit H3): the Go port of
// beacon/utils/domain_fsm.py — parse_domain_fsm_from_truth,
// domain_fsm_is_required, validate_domain_fsm_projection and the
// tests-illegal cross-check of evaluate_domain_fsm_authority.
//
// This is a DIFFERENT system from truth_fsm.py (gate/fsm.go): that
// one validates the five-tuple graph; this one checks that a
// domain package carries a Domain Model / entity-precedence
// surface, Domain FSM aggregates, illegal-transition slots, and
// ACs that bind to FSM paths. Python's truth_integrity.py folds
// this gate's findings into the integrity verdict via the clarity
// gate; the Go integrity gate calls EvaluateDomainFSM directly.
//
// One Python input has no Go counterpart: diagram_truth
// (complexity signals from the diagram-truth projection). The Go
// port treats complexity as absent, so the complexity-driven
// requirement path never fires; every other requirement trigger
// (explicit frontmatter, section presence, fixture expectation)
// is ported. See the R2a report's difference list.
package gate

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// DomainEntity is one entity-precedence row.
type DomainEntity struct {
	Entity     string `json:"entity"`
	Key        string `json:"key"`
	Precedence string `json:"precedence"`
	Requires   string `json:"requires"`
}

// DomainTransition is one aggregate transition (To|From|Guard rows).
type DomainTransition struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Guard   string `json:"guard"`
	Initial string `json:"initial,omitempty"`
}

// DomainAggregate is one parsed Domain FSM section.
type DomainAggregate struct {
	Name        string             `json:"name"`
	States      []string           `json:"states"`
	Transitions []DomainTransition `json:"transitions"`
}

// DomainIllegalSlot is one illegal-transition slot of the domain parse.
type DomainIllegalSlot struct {
	Description string `json:"description"`
	Aggregate   string `json:"aggregate"`
	Source      string `json:"source"`
}

// DomainAC is one acceptance row of the domain parse.
type DomainAC struct {
	ID          string `json:"ac_id"`
	Description string `json:"description"`
}

// DomainFSMProjection mirrors parse_domain_fsm_from_truth's dict.
type DomainFSMProjection struct {
	HasDomainModelSection    bool                `json:"has_domain_model_section"`
	HasDomainFSMSection      bool                `json:"has_domain_fsm_section"`
	HasIllegalSection        bool                `json:"has_illegal_transition_section"`
	Entities                 []DomainEntity      `json:"entities"`
	Aggregates               []DomainAggregate   `json:"aggregates"`
	IllegalSlots             []DomainIllegalSlot `json:"illegal_transition_slots"`
	Acceptance               []DomainAC          `json:"acceptance_criteria"`
	BoundACIDs               []string            `json:"bound_ac_ids"`
	OrphanACIDs              []string            `json:"orphan_ac_ids"`
	AggregatesMissingIllegal []string            `json:"aggregates_missing_illegal_slots"`
}

// DomainFSMFinding is one domain-authority finding; ReasonCode is
// the Python REASON_* spelling.
type DomainFSMFinding struct {
	Kind       string `json:"kind"`
	ReasonCode string `json:"reason_code"`
	Text       string `json:"text"`
	Aggregate  string `json:"aggregate,omitempty"`
	ACID       string `json:"ac_id,omitempty"`
}

var (
	domainModelTitles = []string{
		"domain model", "domain-model", "实体模型", "领域模型",
		"entity precedence", "entity-precedence", "实体前置", "实体前置序",
	}
	domainFSMTitles = []string{
		"domain fsm", "domain-fsm", "业务状态机", "业务域状态机",
		"领域状态机", "domain state machine",
	}
	domainIllegalTitles = []string{
		"illegal transition", "illegal transitions", "非法转移",
		"非法状态转移", "illegal path",
	}
	domainACTitles = []string{"acceptance criteria", "验收标准", "接受标准"}
	// domain_fsm._BUSINESS_DOMAIN_SIGNALS (only used by the
	// complexity path; kept for the requiredness record).
	domainBusinessSignals = []string{
		"order", "product", "cart", "checkout", "payment", "inventory",
		"shipment", "订单", "商品", "购物车", "支付", "库存", "结算", "下单", "聚合根",
	}
)

func headingMatchesAny(title string, candidates []string) bool {
	n := normalizeHeading(title)
	for _, tok := range candidates {
		if strings.Contains(n, tok) {
			return true
		}
	}
	return false
}

// domainTableRows ports domain_fsm._table_rows (naive split,
// separator rows dropped).
func domainTableRows(sectionBody string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(sectionBody, "\n") {
		stripped := strings.TrimSpace(line)
		if !strings.HasPrefix(stripped, "|") {
			continue
		}
		cells := splitRowNaive(stripped)
		if len(cells) == 0 {
			continue
		}
		sep := true
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
		rows = append(rows, cells)
	}
	return rows
}

func parseEntityPrecedence(sections []mdSection) []DomainEntity {
	var entities []DomainEntity
	for _, sec := range sections {
		if !headingMatchesAny(sec.Title, domainModelTitles) {
			continue
		}
		rows := domainTableRows(sec.Body)
		if len(rows) == 0 {
			for _, line := range strings.Split(sec.Body, "\n") {
				stripped := strings.TrimSpace(line)
				if !strings.HasPrefix(stripped, "-") && !strings.HasPrefix(stripped, "*") && !strings.HasPrefix(stripped, "•") {
					continue
				}
				text := strings.TrimSpace(strings.TrimLeft(stripped, "-*• "))
				if text == "" {
					continue
				}
				name := text
				for _, sep := range []string{":", "—", "("} {
					if idx := strings.Index(name, sep); idx >= 0 {
						name = name[:idx]
					}
				}
				name = strings.TrimSpace(name)
				if name != "" {
					entities = append(entities, DomainEntity{
						Entity: name, Precedence: text, Requires: text,
					})
				}
			}
			continue
		}
		header := make([]string, len(rows[0]))
		for i, c := range rows[0] {
			header[i] = strings.ToLower(c)
		}
		start := 0
		for _, tok := range []string{"entity", "实体", "key", "precedence", "前置"} {
			if strings.Contains(strings.Join(header, " "), tok) {
				start = 1
				break
			}
		}
		for _, row := range rows[start:] {
			entity := ""
			if len(row) > 0 {
				entity = row[0]
			}
			if entity == "" || strings.ToLower(entity) == "entity" || entity == "实体" || strings.ToLower(entity) == "name" {
				continue
			}
			key, prec := "", ""
			if len(row) > 1 {
				key = row[1]
				prec = row[1]
			}
			if len(row) > 2 {
				prec = row[2]
			}
			entities = append(entities, DomainEntity{
				Entity: entity, Key: key, Precedence: prec, Requires: prec,
			})
		}
	}
	return entities
}

func parseAggregateName(title string) string {
	normalized := strings.TrimSpace(title)
	for _, sep := range []string{"—", "–", "-", ":", "：", "|"} {
		if strings.Contains(normalized, sep) {
			parts := strings.Split(normalized, sep)
			if tail := strings.TrimSpace(parts[len(parts)-1]); tail != "" {
				return tail
			}
		}
	}
	if tokens := strings.Fields(normalized); len(tokens) > 0 {
		return tokens[len(tokens)-1]
	}
	if normalized == "" {
		return "aggregate"
	}
	return normalized
}

var domainArrowFormRe = regexp.MustCompile(`\[([^\]]+)\]\s*--[^>]*-->\s*\[([^\]]+)\]`)

func parseDomainFSMTables(sections []mdSection) []DomainAggregate {
	var aggregates []DomainAggregate
	for _, sec := range sections {
		if !headingMatchesAny(sec.Title, domainFSMTitles) {
			continue
		}
		name := parseAggregateName(sec.Title)
		stateSet := map[string]bool{}
		var transitions []DomainTransition
		rows := domainTableRows(sec.Body)
		if len(rows) > 0 {
			header := make([]string, len(rows[0]))
			for i, c := range rows[0] {
				header[i] = strings.ToLower(c)
			}
			start := 0
			for _, tok := range []string{"state", "from", "to", "状态", "guard"} {
				if strings.Contains(strings.Join(header, " "), tok) {
					start = 1
					break
				}
			}
			for _, row := range rows[start:] {
				if len(row) == 0 {
					continue
				}
				toState := strings.TrimSpace(row[0])
				fromRaw, guard := "", ""
				if len(row) > 1 {
					fromRaw = strings.TrimSpace(row[1])
				}
				if len(row) > 2 {
					guard = strings.TrimSpace(row[2])
				}
				if toState == "" || strings.ToLower(toState) == "state" ||
					strings.ToLower(toState) == "to" || toState == "状态" {
					continue
				}
				stateSet[toState] = true
				if fromRaw != "" && fromRaw != "-" && fromRaw != "—" && fromRaw != "–" &&
					!strings.EqualFold(fromRaw, "none") && !strings.EqualFold(fromRaw, "n/a") {
					for _, source := range strings.FieldsFunc(fromRaw, func(r rune) bool {
						return r == ',' || r == '/' || r == '|' || r == '，' || r == '、'
					}) {
						source = strings.TrimSpace(source)
						if source == "" || source == "-" || source == "—" {
							continue
						}
						stateSet[source] = true
						transitions = append(transitions, DomainTransition{From: source, To: toState, Guard: guard})
					}
				} else {
					transitions = append(transitions, DomainTransition{From: "", To: toState, Guard: guard, Initial: "true"})
				}
			}
		} else {
			for _, m := range domainArrowFormRe.FindAllStringSubmatch(sec.Body, -1) {
				source, target := strings.TrimSpace(m[1]), strings.TrimSpace(m[2])
				if source != "" && target != "" {
					stateSet[source] = true
					stateSet[target] = true
					transitions = append(transitions, DomainTransition{From: source, To: target})
				}
			}
		}
		var states []string
		for s := range stateSet {
			states = append(states, s)
		}
		sort.Strings(states)
		aggregates = append(aggregates, DomainAggregate{Name: name, States: states, Transitions: transitions})
	}
	return aggregates
}

func parseDomainIllegalSlots(sections []mdSection) []DomainIllegalSlot {
	var slots []DomainIllegalSlot
	for _, sec := range sections {
		if !headingMatchesAny(sec.Title, domainIllegalTitles) {
			continue
		}
		for _, line := range strings.Split(sec.Body, "\n") {
			stripped := strings.TrimSpace(line)
			if stripped == "" {
				continue
			}
			if !strings.HasPrefix(stripped, "-") && !strings.HasPrefix(stripped, "*") &&
				!strings.HasPrefix(stripped, "•") && !strings.HasPrefix(stripped, "|") {
				continue
			}
			text := strings.TrimSpace(strings.Trim(strings.TrimSpace(strings.TrimLeft(stripped, "-*• ")), "|"))
			if text == "" || onlySepChars(text) {
				continue
			}
			if strings.HasPrefix(strings.ToLower(text), "illegal") && len(strings.Fields(text)) <= 2 {
				continue
			}
			aggregate := ""
			if first := strings.Fields(text); len(first) > 0 && strings.Contains(first[0], ".") {
				aggregate = strings.Trim(strings.SplitN(first[0], ".", 2)[0], "`")
			}
			slots = append(slots, DomainIllegalSlot{Description: text, Aggregate: aggregate, Source: "truth"})
		}
	}
	seen := map[string]bool{}
	var out []DomainIllegalSlot
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

func parseDomainAcceptance(sections []mdSection, body string) []DomainAC {
	var acs []DomainAC
	for _, sec := range sections {
		n := normalizeHeading(sec.Title)
		if !headingMatchesAny(sec.Title, domainACTitles) && !strings.Contains(n, "acceptance criteria") {
			if !strings.Contains(n, "ac") && !strings.Contains(sec.Title, "验收") {
				continue
			}
		}
		rows := domainTableRows(sec.Body)
		start := 0
		if len(rows) > 0 {
			joined := strings.Join(rows[0], " ")
			if strings.Contains(strings.ToLower(joined), "ac") {
				start = 1
			}
		}
		for _, row := range rows[start:] {
			if len(row) == 0 {
				continue
			}
			m := acIDPatternRe.FindStringSubmatch(row[0])
			if m == nil {
				continue
			}
			desc := ""
			if len(row) > 1 {
				desc = row[len(row)-1]
			}
			if len(row) >= 3 && row[2] != "" {
				desc = row[2]
			}
			acs = append(acs, DomainAC{ID: strings.ToUpper(m[1]), Description: desc})
		}
	}
	if len(acs) > 0 {
		return acs
	}
	// Python's fallback is a bare id scan with EMPTY
	// descriptions (domain_fsm.py _parse_acceptance_criteria) —
	// such ACs can never bind to the FSM, which is exactly why
	// the fail-closed orphan finding fires for table-less
	// packages. Ported as-is, verified against a live run.
	seen := map[string]bool{}
	for _, m := range acIDPatternRe.FindAllStringSubmatch(body, -1) {
		id := strings.ToUpper(m[1])
		if seen[id] {
			continue
		}
		seen[id] = true
		acs = append(acs, DomainAC{ID: id})
	}
	return acs
}

func legalStateNames(aggregates []DomainAggregate) map[string]bool {
	names := map[string]bool{}
	for _, agg := range aggregates {
		for _, s := range agg.States {
			names[strings.ToLower(s)] = true
		}
		for _, t := range agg.Transitions {
			if t.From != "" {
				names[strings.ToLower(t.From)] = true
			}
			if t.To != "" {
				names[strings.ToLower(t.To)] = true
			}
		}
		if agg.Name != "" {
			names[strings.ToLower(agg.Name)] = true
		}
	}
	return names
}

var domainPathVerbs = []string{
	"place", "placed", "pay", "paid", "ship", "shipped", "list", "listed",
	"create", "draft", "cancel", "下单", "支付", "发货", "上架", "创建",
}

func acBindsToFSM(ac DomainAC, aggregates []DomainAggregate, entities []DomainEntity) bool {
	desc := strings.ToLower(ac.Description)
	if desc == "" {
		return false
	}
	for _, tok := range []string{"illegal", "reject", "拒绝", "without", "无商品", "不允许"} {
		if strings.Contains(desc, tok) {
			return true
		}
	}
	for name := range legalStateNames(aggregates) {
		if name != "" && strings.Contains(desc, name) {
			return true
		}
	}
	for _, e := range entities {
		if n := strings.ToLower(strings.TrimSpace(e.Entity)); n != "" && strings.Contains(desc, n) {
			return true
		}
	}
	if len(aggregates) > 0 {
		for _, v := range domainPathVerbs {
			if strings.Contains(desc, v) {
				return true
			}
		}
	}
	return false
}

var domainFlavorTokens = []string{
	"ready", "draft", "publish", "schedule", "archive", "ship", "place", "paid",
	"order", "product", "listed", "transition", "状态", "发布", "草稿", "上架",
	"入库", "续写", "pipeline", "prefill", "illegal", "reject",
}

// ParseDomainFSM ports parse_domain_fsm_from_truth (minus the
// frontmatter echo, which the caller already holds).
func ParseDomainFSM(truthMD string) DomainFSMProjection {
	_, body := splitFrontmatterBody(truthMD)
	sections := extractMDSections(body)
	entities := parseEntityPrecedence(sections)
	aggregates := parseDomainFSMTables(sections)
	illegal := parseDomainIllegalSlots(sections)
	acceptance := parseDomainAcceptance(sections, body)

	hasModel, hasFSM, hasIllegal := false, false, false
	for _, sec := range sections {
		if headingMatchesAny(sec.Title, domainModelTitles) {
			hasModel = true
		}
		if headingMatchesAny(sec.Title, domainFSMTitles) {
			hasFSM = true
		}
		if headingMatchesAny(sec.Title, domainIllegalTitles) {
			hasIllegal = true
		}
	}
	hasModel = hasModel || len(entities) > 0
	hasFSM = hasFSM || len(aggregates) > 0
	hasIllegal = hasIllegal || len(illegal) > 0

	var bound, flavoredUnbound []string
	if len(aggregates) > 0 || len(entities) > 0 {
		for _, ac := range acceptance {
			if acBindsToFSM(ac, aggregates, entities) {
				bound = append(bound, ac.ID)
				continue
			}
			desc := strings.ToLower(ac.Description)
			for _, tok := range domainFlavorTokens {
				if strings.Contains(desc, tok) {
					flavoredUnbound = append(flavoredUnbound, ac.ID)
					break
				}
			}
		}
	}

	var missingIllegal []string
	if len(aggregates) > 0 {
		if len(illegal) == 0 {
			for _, agg := range aggregates {
				missingIllegal = append(missingIllegal, agg.Name)
			}
		} else {
			covered := map[string]bool{}
			for _, s := range illegal {
				if s.Aggregate != "" {
					covered[strings.ToLower(s.Aggregate)] = true
				}
			}
			for _, agg := range aggregates {
				name := strings.TrimSpace(agg.Name)
				if name == "" {
					continue
				}
				lname := strings.ToLower(name)
				if lname == "domain" || lname == "fsm" || lname == "domain fsm" || lname == "状态机" {
					continue
				}
				if len(covered) > 0 && !covered[lname] {
					found := false
					for _, s := range illegal {
						if strings.Contains(strings.ToLower(s.Description), lname) {
							found = true
							break
						}
					}
					if !found {
						missingIllegal = append(missingIllegal, name)
					}
				}
			}
			if len(covered) == 0 {
				missingIllegal = nil
			}
		}
	}

	return DomainFSMProjection{
		HasDomainModelSection:    hasModel,
		HasDomainFSMSection:      hasFSM,
		HasIllegalSection:        hasIllegal,
		Entities:                 entities,
		Aggregates:               aggregates,
		IllegalSlots:             illegal,
		Acceptance:               acceptance,
		BoundACIDs:               bound,
		OrphanACIDs:              flavoredUnbound,
		AggregatesMissingIllegal: missingIllegal,
	}
}

// splitFrontmatterBody returns (scalars, body) using the package's
// existing frontmatter reader.
func splitFrontmatterBody(text string) (map[string]string, string) {
	scalars, _, ok := ParseFrontmatter(text)
	if !ok {
		return scalars, text
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return scalars, text
	}
	return scalars, text[4+end+4:]
}

func semverTuple(version string) (int, int, int) {
	v := strings.TrimPrefix(strings.TrimSpace(version), "v")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return 0, 0, 0
	}
	var nums [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return 0, 0, 0
		}
		nums[i] = n
	}
	return nums[0], nums[1], nums[2]
}

// domainFSMSupported ports domain_fsm_supported: docs >= v1.6.7.
func domainFSMSupported(version string) bool {
	maj, min, pat := semverTuple(version)
	return maj > 1 || (maj == 1 && min > 6) || (maj == 1 && min == 6 && pat >= 7)
}

// domainFSMRequired ports domain_fsm_is_required minus the
// diagram-truth complexity input (see file comment).
func domainFSMRequired(scalars map[string]string, truthMD string, p DomainFSMProjection) bool {
	explicit := false
	for _, key := range []string{"domain_required", "domain_fsm_required", "entity_precedence_required", "require_domain_fsm"} {
		if truthyValue(scalars[key]) {
			explicit = true
		}
	}
	lowered := strings.ToLower(truthMD)
	sectionPresence := false
	for _, tok := range []string{
		"## domain model", "## domain fsm", "## 领域模型", "## 业务状态机",
		"## 业务域状态机", "## 实体前置", "## entity precedence",
		"## illegal transition", "## 非法转移",
	} {
		if strings.Contains(lowered, tok) {
			sectionPresence = true
			break
		}
	}
	partial := p.HasDomainModelSection || p.HasDomainFSMSection || p.HasIllegalSection
	fixtureNegative := false
	switch strings.ToLower(strings.TrimSpace(scalars["expected_gate"])) {
	case "freeze_or_qa_fail", "domain_fsm", "entity_precedence":
		fixtureNegative = true
	}
	required := explicit || partial || sectionPresence || fixtureNegative
	if !domainFSMSupported(scalars["version"]) && !(explicit || partial || sectionPresence || fixtureNegative) {
		required = false
	}
	return required
}

// validateDomainFSM ports validate_domain_fsm_projection.
func validateDomainFSM(p DomainFSMProjection) []DomainFSMFinding {
	var findings []DomainFSMFinding
	add := func(kind, code, text, agg, ac string) {
		findings = append(findings, DomainFSMFinding{Kind: kind, ReasonCode: code, Text: text, Aggregate: agg, ACID: ac})
	}
	if !p.HasDomainModelSection && len(p.Entities) == 0 {
		add("domain_model_missing", "domain_model_missing",
			"domain feature requires Domain Model / entity-precedence section", "", "")
	}
	if len(p.Entities) == 0 {
		add("entity_precedence_missing", "entity_precedence_missing",
			"domain feature requires entity-precedence rows (entity order / requires)", "", "")
	}
	if !p.HasDomainFSMSection && len(p.Aggregates) == 0 {
		add("domain_fsm_missing", "domain_fsm_missing",
			"domain feature requires Domain FSM section with states/transitions", "", "")
	}
	if len(p.Aggregates) > 0 {
		validTransitions := 0
		for _, agg := range p.Aggregates {
			if len(agg.States) == 0 {
				add("domain_fsm_incomplete", "domain_fsm_incomplete",
					"aggregate "+agg.Name+" has no states", agg.Name, "")
			}
			for _, t := range agg.Transitions {
				if t.To != "" {
					validTransitions++
				}
			}
		}
		if validTransitions == 0 {
			add("domain_fsm_incomplete", "domain_fsm_incomplete",
				"Domain FSM has aggregates but no parseable transitions/states", "", "")
		}
	}
	if len(p.IllegalSlots) == 0 {
		add("illegal_transition_slot_missing", "illegal_transition_slot_missing",
			"each domain package requires at least one illegal-transition TC slot in truth", "", "")
	}
	for _, name := range p.AggregatesMissingIllegal {
		add("illegal_transition_slot_missing", "illegal_transition_slot_missing",
			"aggregate root missing illegal-transition slot: "+name, name, "")
	}
	if len(p.Acceptance) > 0 && len(p.BoundACIDs) == 0 && (len(p.Aggregates) > 0 || len(p.Entities) > 0) {
		add("orphan_ac_without_fsm_path", "orphan_ac_without_fsm_path",
			"domain package has acceptance criteria but none map to a Domain FSM legal path", "", "")
	} else if len(p.OrphanACIDs) > 0 && len(p.BoundACIDs) == 0 {
		for _, acID := range p.OrphanACIDs {
			add("orphan_ac_without_fsm_path", "orphan_ac_without_fsm_path",
				"domain-flavored acceptance criterion has no mappable Domain FSM legal path: "+acID, "", acID)
		}
	}
	return findings
}

// EvaluateDomainFSM runs the domain authority over one package:
// not-required packages produce no findings.
func EvaluateDomainFSM(truthMD, testsMD string) (required bool, findings []DomainFSMFinding) {
	if strings.TrimSpace(truthMD) == "" {
		return false, nil
	}
	scalars, _ := splitFrontmatterBody(truthMD)
	p := ParseDomainFSM(truthMD)
	if !domainFSMRequired(scalars, truthMD, p) {
		return false, nil
	}
	findings = validateDomainFSM(p)
	// evaluate_domain_fsm_authority: tests.md should mention
	// illegal when the domain authority applies.
	if testsMD != "" {
		low := strings.ToLower(testsMD)
		if !strings.Contains(low, "illegal") && !strings.Contains(testsMD, "非法") {
			findings = append(findings, DomainFSMFinding{
				Kind: "illegal_transition_slot_missing", ReasonCode: "illegal_transition_slot_missing",
				Text: "tests.md missing illegal-transition coverage reference",
			})
		}
	}
	return true, findings
}
