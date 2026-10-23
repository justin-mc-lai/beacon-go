// Requirement clarity + determinism + extension scans (R2a,
// parity audit H3): three of the five Python TIG groups that had
// no Go counterpart, ported from
//   - requirement_clarity.py:  _scan_file / _line_has_inline_marker
//   - truth_integrity.py:      scan_determinism_lines (TIG-DC)
//   - truth_integrity.py:      scan_integrity_extension_lines
//
// The clarity gate's other inputs (diagram truth, human/research
// closure contracts, support topology) have no Go counterpart and
// are listed as residual differences in the R2a report; what is
// ported here is the gate's package-file scan, which is the part
// that runs over the four-piece set the integrity gate already holds.
package gate

import (
	"regexp"
	"strings"
	"unicode"

	"beacon-go/internal/core"
)

// ---- clarity: inline markers + unresolved sections ----

// requirement_clarity._UNRESOLVED_SECTION_TITLES (lines 37-48).
var unresolvedSectionTitles = map[string]bool{
	"open questions": true, "unresolved": true, "unresolved items": true,
	"pending clarifications": true, "clarifications needed": true,
	"未决项": true, "待澄清": true, "待确认": true,
	"疑问": true, "未澄清": true, "需求疑问": true,
}

// requirement_clarity._INLINE_MARKERS[3:] — the first three
// markers (TBD/TODO/???) have dedicated rules in
// _line_has_inline_marker and are handled separately below.
var inlineMarkerSubstrings = []string{"待补充", "待明确", "待确认", "未澄清", "待澄清", "待定"}

var (
	clarityKVLineRe   = regexp.MustCompile(`^[a-z][A-Za-z0-9_./-]*:\s`)
	clarityTODOExempt = regexp.MustCompile(`(?i)创建的\s*TODO|TODO\s*（|TODO\s+CRUD`)
	clarityTBDPunctRe = regexp.MustCompile(`(?i)\bTBD\b(?:\s*[:（(]|$)`)
	clarityTODOHeadRe = regexp.MustCompile(`(?i)^TODO\s*:`)
	clarityTODOBullet = regexp.MustCompile(`(?i)^[\s*+\-]+\s*TODO\s*:`)
	clarityHeadingRe  = regexp.MustCompile(`^(#{2,6})\s+(.+?)\s*$`)
	clarityHeadPrefix = regexp.MustCompile(`^\d+(?:\.\d+)*[.)]?\s*`)
)

// lineHasInlineMarker ports requirement_clarity._line_has_inline_marker.
func lineHasInlineMarker(stripped string) bool {
	if strings.HasPrefix(stripped, "#") {
		return false
	}
	if clarityKVLineRe.MatchString(stripped) {
		return false
	}
	if strings.Contains(stripped, "docs/beacon/") && strings.HasSuffix(stripped, ".md") {
		return false
	}
	if strings.Contains(stripped, "inline_marker") || strings.Contains(stripped, "_INLINE_MARKERS") {
		return false
	}
	low := strings.ToLower(stripped)
	if strings.Contains(low, "/todos") || strings.Contains(low, "go-todo-") {
		return false
	}
	if clarityTODOExempt.MatchString(stripped) {
		return false
	}
	for _, marker := range inlineMarkerSubstrings {
		if strings.Contains(stripped, marker) {
			return true
		}
	}
	if strings.Contains(stripped, "???") {
		return true
	}
	if clarityTBDPunctRe.MatchString(stripped) {
		return true
	}
	if clarityTODOHeadRe.MatchString(stripped) {
		return true
	}
	if clarityTODOBullet.MatchString(stripped) {
		return true
	}
	return false
}

// scanClarityFile ports requirement_clarity._scan_file over one
// package file's text.
func scanClarityFile(pf PackageFile) []Finding {
	var findings []Finding
	add := func(kind string, line int, text string) {
		findings = append(findings, Finding{
			Kind: kind, ReasonCode: core.ReasonRequirementClarityOpen,
			File: pf.Path, Line: line, Detail: text, RepairRoute: DefaultRepairRoute,
		})
	}
	lines := strings.Split(pf.Text, "\n")
	inFence, inFM, fmDone := false, false, false
	currentUnresolved := ""
	for i, raw := range lines {
		lineNo := i + 1
		stripped := strings.TrimSpace(raw)
		if !fmDone {
			if lineNo == 1 && stripped == "---" {
				inFM = true
				continue
			}
			if inFM {
				if stripped == "---" {
					inFM = false
					fmDone = true
				}
				continue
			}
			fmDone = true
		}
		if strings.HasPrefix(stripped, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if m := clarityHeadingRe.FindStringSubmatch(stripped); m != nil {
			title := clarityHeadPrefix.ReplaceAllString(strings.ToLower(strings.TrimSpace(m[2])), "")
			title = strings.TrimSpace(title)
			if unresolvedSectionTitles[title] {
				currentUnresolved = title
			} else {
				currentUnresolved = ""
			}
			continue
		}
		if currentUnresolved != "" && stripped != "" {
			first, _ := utf8FirstRune(stripped)
			if strings.HasPrefix(stripped, "-") || strings.HasPrefix(stripped, "*") ||
				strings.HasPrefix(stripped, "+") || unicode.IsDigit(first) {
				add("unresolved_section_item", lineNo, stripped)
				continue
			}
		}
		if lineHasInlineMarker(stripped) {
			add("inline_marker", lineNo, stripped)
			continue
		}
		if (strings.HasPrefix(stripped, "-") || strings.HasPrefix(stripped, "*") ||
			strings.HasPrefix(stripped, "+")) && (strings.Contains(stripped, "?") || strings.Contains(stripped, "？")) {
			add("question_bullet", lineNo, stripped)
		}
	}
	return findings
}

func utf8FirstRune(s string) (rune, int) {
	for _, r := range s {
		return r, 1
	}
	return 0, 0
}

// ---- determinism: placeholders (block) + hollow templates (warn) ----

// truth_integrity._DC_PLACEHOLDER_PATTERNS, in source order.
var dcPlaceholderPatterns = []struct {
	code string
	re   *regexp.Regexp
}{
	{"english_tbd", regexp.MustCompile(`\bTBD\b`)},
	{"english_todo", regexp.MustCompile(`\bTODO\b`)},
	{"english_tbc", regexp.MustCompile(`\bTBC\b`)},
	{"english_pending", regexp.MustCompile(`(?i)\bpending\b`)},
	{"chinese_daiding", regexp.MustCompile(`待定`)},
	{"chinese_daiqueren", regexp.MustCompile(`待确认`)},
	{"chinese_xujinyibuqueren", regexp.MustCompile(`需进一步确认`)},
	{"chinese_shiqingkuangerding", regexp.MustCompile(`视情况而定`)},
	{"chinese_houxubuchong", regexp.MustCompile(`后续补充`)},
	{"chinese_dai_x_queren", regexp.MustCompile(`待.+确认`)},
}

// truth_integrity._DC_HOLLOW_TEMPLATE_PATTERNS, in source order.
var dcHollowPatterns = []struct {
	code string
	re   *regexp.Regexp
}{
	{"hollow_good", regexp.MustCompile(`具有良好的`)},
	{"hollow_all", regexp.MustCompile(`支持所有`)},
	{"hollow_full_compat", regexp.MustCompile(`兼容全部`)},
	{"hollow_actual", regexp.MustCompile(`根据实际情况`)},
	{"hollow_per_spec", regexp.MustCompile(`按照.+规范`)},
}

var dcDomainVocabSkipRe = regexp.MustCompile(`(?i)^(slug|canonical_refs|prd|user_story|test_case|version|language|parser_contract|truth_source_model|truth_map_contract|status|parent_feature)\s*:|docs/beacon/|go-todo-|/todos|TODO\s*(CRUD|api|model|item|handler|service|route|controller|repo|store|list|entity|struct|type|interface)|创建的\s*TODO`)

// scanDeterminismFile ports truth_integrity.scan_determinism_lines
// for one package file: frontmatter, code fences, headings and
// domain-vocabulary lines are skipped; placeholder hits block,
// hollow-template hits are warn-only.
func scanDeterminismFile(pf PackageFile) []Finding {
	var findings []Finding
	lines := strings.Split(pf.Text, "\n")
	inFM, fmDone, inFence := false, false, false
	for i, line := range lines {
		lineNo := i + 1
		stripped := strings.TrimSpace(line)
		if !fmDone {
			if lineNo == 1 && stripped == "---" {
				inFM = true
				continue
			}
			if inFM {
				if stripped == "---" {
					inFM = false
					fmDone = true
				}
				continue
			}
			fmDone = true
		}
		if strings.HasPrefix(stripped, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if dcDomainVocabSkipRe.MatchString(stripped) {
			continue
		}
		if strings.HasPrefix(stripped, "#") {
			continue
		}
		for _, p := range dcPlaceholderPatterns {
			if p.re.MatchString(stripped) {
				findings = append(findings, Finding{
					Kind: "dc_placeholder_" + p.code, ReasonCode: core.ReasonDeterminismPlaceholder,
					File: pf.Path, Line: lineNo, Detail: strings.TrimSpace(line), RepairRoute: DefaultRepairRoute,
				})
			}
		}
		for _, p := range dcHollowPatterns {
			if p.re.MatchString(stripped) {
				findings = append(findings, Finding{
					Kind: "dc_hollow_" + p.code, ReasonCode: core.ReasonDeterminismHollowTemplate,
					File: pf.Path, Line: lineNo, Detail: strings.TrimSpace(line), RepairRoute: DefaultRepairRoute,
					Severity: "warn",
				})
			}
		}
	}
	return findings
}

// ---- extension scan: placeholder AC language, illegal closure
// paths, soft test cells, ocean alignment/intent surfaces ----

var (
	extPlaceholderACRe    = regexp.MustCompile(`(?i)占位|placeholder|stub-only|tbd closure|假装 closure|假装闭环`)
	extACIDRe             = regexp.MustCompile(`(?i)AC-\d{3}`)
	extIllegalClosureRe   = regexp.MustCompile(`(?i)仅 contract|subprocess 替身|零断言即可|contract-only closure`)
	extSoftCellRe         = regexp.MustCompile(`(?i)^(integration|目录存在|fixture|可选)$`)
	extHardMarkersRe      = regexp.MustCompile(`(?i)cargo|beacon\s|pnpm|npm test|pytest|BLOCKED:`)
	extOceanMarkersRe     = regexp.MustCompile("(?i)scope_mode\\s*[:=]\\s*[`\"']?ocean[`\"']?|scope:\\s*ocean")
	extAlignmentSurfaceRe = regexp.MustCompile(`(?i)alignment surface|phased backlog|对齐面|alignment surface（`)
	extUserIntentRe       = regexp.MustCompile(`(?i)user intent|用户意图`)
)

// scanExtensionFiles ports truth_integrity.scan_integrity_extension_lines.
// The Python original's deferral_pending kind is not re-emitted
// here: CheckIntegrity step 4 already owns it (its pattern gains
// the Python 状态\s*pending alternative in this batch).
func scanExtensionFiles(pkg PackageFiles) []Finding {
	var findings []Finding
	if pkg.Truth.Present {
		for i, line := range strings.Split(pkg.Truth.Text, "\n") {
			if extPlaceholderACRe.MatchString(line) && extACIDRe.MatchString(line) {
				findings = append(findings, Finding{
					Kind: "placeholder_ac_language", ReasonCode: core.ReasonIntegrityPlaceholderAC,
					File: pkg.Truth.Path, Line: i + 1, Detail: strings.TrimSpace(line), RepairRoute: DefaultRepairRoute,
				})
			}
			if extIllegalClosureRe.MatchString(line) {
				findings = append(findings, Finding{
					Kind: "illegal_closure_path", ReasonCode: core.ReasonIntegrityIllegalClosure,
					File: pkg.Truth.Path, Line: i + 1, Detail: strings.TrimSpace(line), RepairRoute: DefaultRepairRoute,
				})
			}
		}
		if extOceanMarkersRe.MatchString(pkg.Truth.Text) {
			if !extAlignmentSurfaceRe.MatchString(pkg.Truth.Text) {
				findings = append(findings, Finding{
					Kind: "alignment_surface_missing", ReasonCode: core.ReasonIntegrityAlignmentMissing,
					File: pkg.Truth.Path, Line: 1,
					Detail:      "ocean-scope feature missing Alignment Surface or Phased Backlog section",
					RepairRoute: DefaultRepairRoute,
				})
			}
			if !extUserIntentRe.MatchString(pkg.Truth.Text) {
				findings = append(findings, Finding{
					Kind: "user_intent_missing", ReasonCode: core.ReasonIntegrityUserIntentMiss,
					File: pkg.Truth.Path, Line: 1,
					Detail:      "ocean-scope feature missing User Intent section",
					RepairRoute: DefaultRepairRoute,
				})
			}
		}
	}
	if pkg.Tests.Present {
		for i, line := range strings.Split(pkg.Tests.Text, "\n") {
			if !strings.Contains(line, "|") {
				continue
			}
			cells := splitRowNaive(strings.TrimSpace(line))
			soft := false
			for _, cell := range cells {
				if extSoftCellRe.MatchString(strings.TrimSpace(cell)) {
					soft = true
					break
				}
			}
			if soft && !extHardMarkersRe.MatchString(line) {
				findings = append(findings, Finding{
					Kind: "soft_test_assertion", ReasonCode: core.ReasonIntegritySoftTestAssert,
					File: pkg.Tests.Path, Line: i + 1, Detail: strings.TrimSpace(line), RepairRoute: DefaultRepairRoute,
				})
			}
		}
	}
	return findings
}
