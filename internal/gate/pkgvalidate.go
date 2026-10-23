// Feature package validation (R2a, parity audit H3): the Go port
// of feature_package.validate_feature_package's structural
// diagnostics, folded into integrity findings the way
// truth_integrity._validation_findings does (kind
// feature_package_validation, reason
// feature_package_validation_failed, text = the Python diagnostic
// code).
//
// Ported: required frontmatter keys, parser contract, slug match,
// AC row presence, test-row command/soft checks, checkbox ledger
// shape, evidence authority table (required columns, forbidden
// gate-truth fields, authority vocabulary).
// NOT ported (residual differences, see the R2a report): the
// v1.6.7 human-readable L0 sub-gate (truth_human_readable), the
// task ac=/evidence= binding enforcement (a package-dialect rule
// the Go task ledger does not carry), and the canonical-ready
// machine-JSON checks. Missing package files are also not
// re-reported here — CheckIntegrity step 1 owns them.
package gate

import (
	"regexp"
	"strconv"
	"strings"

	"beacon-go/internal/core"
)

// feature_package.REQUIRED_TRUTH_FRONTMATTER / ALLOWED_PARSER_CONTRACTS.
var (
	requiredTruthFrontmatter = []string{"slug", "version", "status", "language", "canonical_refs", "parser_contract"}
	allowedParserContracts   = map[string]bool{"beacon-feature-package-v1": true, "beacon-feature-package-v2": true}
	// feature_package authority vocabulary.
	allowedEvidenceAuthorities = map[string]bool{
		"support_advisory": true, "qa_verdict": true, "release_verdict": true,
		"requirement_truth": true, "acceptance_truth": true, "test_truth": true,
	}
	forbiddenEvidenceFields = []string{"final_verdict", "gate_truth", "release_ready", "qa_passed", "source_of_truth"}
)

var forbiddenEvidenceFieldSet = map[string]bool{
	"final_verdict": true, "gate_truth": true, "release_ready": true,
	"qa_passed": true, "source_of_truth": true,
}

// frontmatterTopKeys returns the set of top-level frontmatter keys.
func frontmatterTopKeys(text string) (map[string]bool, bool) {
	keys := map[string]bool{}
	if !strings.HasPrefix(text, "---\n") {
		return keys, false
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return keys, false
	}
	for _, line := range strings.Split(text[4:4+end], "\n") {
		if line == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		if k, _, ok := strings.Cut(line, ":"); ok {
			keys[strings.TrimSpace(k)] = true
		}
	}
	return keys, true
}

// ---- feature_package._extract_ac_rows ----

var (
	fpACRowStartRe = regexp.MustCompile(`^AC-[A-Z0-9]+(?:-[A-Z0-9]+)*`)
	fpACBulletRe   = regexp.MustCompile(`^\s*[-*]\s+(AC-[A-Z0-9]+(?:-[A-Z0-9]+)*)[：:\s]+(.+)$`)
	fpACBoldRe     = regexp.MustCompile(`\*\*(AC-[A-Z0-9]+(?:-[A-Z0-9]+)*)\*\*`)
)

// contractTableRows walks markdown the way feature_package's
// extractors do: a header row (matched by headerOK) opens the
// table; a non-blank non-table line ends the scan; blank lines
// only pause it (_ends_contract_scan).
func contractTableRows(markdown string, headerOK func(lowered []string) bool, rowOK func(cells []string) bool) [][]string {
	var rows [][]string
	inTable := false
	for _, line := range strings.Split(markdown, "\n") {
		stripped := strings.TrimSpace(line)
		if !strings.HasPrefix(stripped, "|") {
			if inTable && stripped != "" {
				break
			}
			continue
		}
		cells := splitRowNaive(stripped)
		lowered := make([]string, len(cells))
		for i, c := range cells {
			lowered[i] = strings.ToLower(c)
		}
		if headerOK(lowered) {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		sep := true
		for _, c := range cells {
			if c == "" {
				continue
			}
			for _, r := range c {
				if r != '-' {
					sep = false
				}
			}
		}
		if sep {
			continue
		}
		if len(cells) > 0 && rowOK(cells) {
			rows = append(rows, cells)
		}
	}
	return rows
}

// extractACRowsFP ports feature_package._extract_ac_rows (count
// and presence are all the validation consumes).
func extractACRowsFP(markdown string) int {
	rows := contractTableRows(markdown,
		func(lowered []string) bool { return containsCell(lowered, "ac id") },
		func(cells []string) bool { return fpACRowStartRe.MatchString(cells[0]) })
	if len(rows) > 0 {
		return len(rows)
	}
	count := 0
	for _, line := range strings.Split(markdown, "\n") {
		if fpACBulletRe.MatchString(strings.TrimSpace(line)) {
			count++
			continue
		}
		if fpACBoldRe.MatchString(line) {
			count++
		}
	}
	return count
}

// ---- feature_package._extract_test_rows ----

type fpTestRow struct {
	AC        string
	TC        string
	Command   string
	Assertion string
}

// extractTestRowsFP ports feature_package._extract_test_rows,
// including the command-cell rejoin for pipes inside commands.
func extractTestRowsFP(markdown string) []fpTestRow {
	var out []fpTestRow
	var headers []string
	inTable := false
	for _, line := range strings.Split(markdown, "\n") {
		stripped := strings.TrimSpace(line)
		if !strings.HasPrefix(stripped, "|") {
			if inTable && stripped != "" {
				break
			}
			continue
		}
		cells := splitRowNaive(stripped)
		lowered := make([]string, len(cells))
		for i, c := range cells {
			lowered[i] = strings.ToLower(c)
		}
		if containsCell(lowered, "ac id") && containsCell(lowered, "tc id") {
			inTable = true
			headers = make([]string, len(cells))
			for i, c := range cells {
				headers[i] = strings.ReplaceAll(strings.ToLower(c), " ", "_")
			}
			continue
		}
		if !inTable {
			continue
		}
		sep := true
		for _, c := range cells {
			if c == "" {
				continue
			}
			for _, r := range c {
				if r != '-' {
					sep = false
				}
			}
		}
		if sep || len(cells) == 0 {
			continue
		}
		if !fpACRowStartRe.MatchString(cells[0]) && !fpTCRowStartRe.MatchString(cells[0]) {
			continue
		}
		// Rejoin command cells when pipes appear inside the command.
		cmdIdx := -1
		for i, h := range headers {
			if h == "command" || h == "planned_command" {
				cmdIdx = i
				break
			}
		}
		if cmdIdx >= 0 && len(cells) > len(headers) {
			tail := len(headers) - cmdIdx - 1
			end := len(cells) - tail
			if end < cmdIdx+1 {
				end = cmdIdx + 1
			}
			joined := strings.Join(cells[cmdIdx:end], "|")
			nc := append([]string{}, cells[:cmdIdx]...)
			nc = append(nc, joined)
			nc = append(nc, cells[end:]...)
			cells = nc
		}
		get := func(name string) string {
			for i, h := range headers {
				if h == name && i < len(cells) {
					return cells[i]
				}
			}
			return ""
		}
		row := fpTestRow{
			AC: get("ac_id"), TC: get("tc_id"),
			Command: get("command"), Assertion: get("assertion"),
		}
		if row.Command == "" {
			row.Command = get("planned_command")
		}
		if row.AC == "" {
			for _, c := range cells {
				if m := fpACCellRe.FindStringSubmatch(c); m != nil {
					row.AC = strings.ToUpper(m[1])
					break
				}
			}
		}
		if row.TC == "" {
			for _, c := range cells {
				if m := fpTCCellRe.FindStringSubmatch(c); m != nil {
					row.TC = strings.ToUpper(m[1])
					break
				}
			}
		}
		out = append(out, row)
	}
	return out
}

var (
	fpTCRowStartRe = regexp.MustCompile(`(?i)^TC-[A-Z0-9]+(?:-[A-Z0-9]+)*`)
	fpACCellRe     = regexp.MustCompile(`(?i)^(AC-[A-Z0-9]+(?:-[A-Z0-9]+)*)`)
	fpTCCellRe     = regexp.MustCompile(`(?i)^(TC-[A-Z0-9]+(?:-[A-Z0-9]+)*)`)
	fpHardCmdRe    = regexp.MustCompile(`(?i)cargo|beacon\s|pnpm|npm test|BLOCKED:`)
)

// ---- feature_package._parse_checkboxes ----

var fpCheckboxRe = regexp.MustCompile(`^\s*-\s+\[([ xX-])\]\s+(?:` + "`?" + `([A-Z][A-Z0-9]*(?:-[A-Z0-9]+)*-\d{3,})` + "`?" + `\s+)?(.+?)\s*$`)

// parseCheckboxesFP returns (rowCount, malformedLineNumbers).
func parseCheckboxesFP(markdown string) (int, []int) {
	count := 0
	var bad []int
	for i, line := range strings.Split(markdown, "\n") {
		if !strings.Contains(line, "- [") {
			continue
		}
		if !fpCheckboxRe.MatchString(line) {
			bad = append(bad, i+1)
			continue
		}
		count++
	}
	return count, bad
}

// ---- feature_package._extract_evidence_rows ----

func extractEvidenceRowsFP(markdown string) []map[string]string {
	var headers []string
	var rows []map[string]string
	inTable := false
	for _, line := range strings.Split(markdown, "\n") {
		stripped := strings.TrimSpace(line)
		if !strings.HasPrefix(stripped, "|") {
			if inTable && stripped != "" {
				break
			}
			continue
		}
		cells := splitRowNaive(stripped)
		lowered := make([]string, len(cells))
		for i, c := range cells {
			lowered[i] = strings.ReplaceAll(strings.ToLower(c), " ", "_")
		}
		if containsCell(lowered, "surface") && containsCell(lowered, "authority") {
			inTable = true
			headers = lowered
			continue
		}
		if !inTable {
			continue
		}
		sep := true
		for _, c := range cells {
			if c == "" {
				continue
			}
			for _, r := range c {
				if r != '-' {
					sep = false
				}
			}
		}
		if sep {
			continue
		}
		row := map[string]string{}
		for i, h := range headers {
			if i < len(cells) {
				row[h] = cells[i]
			} else {
				row[h] = ""
			}
		}
		if row["surface"] != "" || row["authority"] != "" {
			rows = append(rows, row)
		}
	}
	return rows
}

// declaredKeyPresent ports the Python (?<![\w/])field\s*: search
// (RE2 has no lookbehind; the preceding rune is checked manually).
func declaredKeyPresent(text, field string) bool {
	re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(field) + `\s*:`)
	for _, loc := range re.FindAllStringIndex(text, -1) {
		if loc[0] == 0 {
			return true
		}
		prev := []rune(text[:loc[0]])
		if len(prev) == 0 {
			return true
		}
		r := prev[len(prev)-1]
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '/' {
			continue
		}
		return true
	}
	return false
}

// ValidateFeaturePackage runs the ported structural validation
// over the package and returns integrity findings.
func ValidateFeaturePackage(pkg PackageFiles) []Finding {
	var findings []Finding
	add := func(file, code string) {
		findings = append(findings, Finding{
			Kind: "feature_package_validation", ReasonCode: core.ReasonIntegrityPackageValid,
			File: file, Line: 1, Detail: code, RepairRoute: DefaultRepairRoute,
		})
	}

	if pkg.Truth.Present {
		scalars, _, parsed := ParseFrontmatter(pkg.Truth.Text)
		if !parsed {
			if strings.HasPrefix(pkg.Truth.Text, "---\n") {
				add(pkg.Truth.Path, "frontmatter_unclosed")
			} else {
				add(pkg.Truth.Path, "frontmatter_missing")
			}
		} else {
			keys, _ := frontmatterTopKeys(pkg.Truth.Text)
			for _, key := range requiredTruthFrontmatter {
				if !keys[key] {
					add(pkg.Truth.Path, "frontmatter_missing_"+key)
				}
			}
			if !allowedParserContracts[strings.TrimSpace(scalars["parser_contract"])] {
				add(pkg.Truth.Path, "parser_contract_mismatch")
			}
			if slug := strings.TrimSpace(scalars["slug"]); slug != "" && slug != pkg.Slug {
				add(pkg.Truth.Path, "frontmatter_slug_mismatch")
			}
		}
		if extractACRowsFP(pkg.Truth.Text) == 0 {
			add(pkg.Truth.Path, "acceptance_criteria_table_missing")
		}
	}

	if pkg.Tests.Present {
		rows := extractTestRowsFP(pkg.Tests.Text)
		if len(rows) == 0 {
			add(pkg.Tests.Path, "tests_coverage_table_missing")
		}
		for _, row := range rows {
			combined := row.Command + " " + row.Assertion
			if strings.TrimSpace(row.Command) == "" && !strings.Contains(combined, "BLOCKED:") {
				add(pkg.Tests.Path, "test_command_missing_or_blocked")
			} else if extSoftCellRe.MatchString(strings.TrimSpace(row.Command)) && !fpHardCmdRe.MatchString(combined) {
				add(pkg.Tests.Path, "soft_test_command_only")
			}
		}
	}

	if pkg.Tasks.Present {
		count, bad := parseCheckboxesFP(pkg.Tasks.Text)
		for _, line := range bad {
			findings = append(findings, Finding{
				Kind: "feature_package_validation", ReasonCode: core.ReasonIntegrityPackageValid,
				File: pkg.Tasks.Path, Line: line,
				Detail: "malformed_checkbox: line " + strconv.Itoa(line), RepairRoute: DefaultRepairRoute,
			})
		}
		if count == 0 && len(bad) == 0 {
			add(pkg.Tasks.Path, "checkbox_ledger_missing")
		}
	}

	if pkg.Evidence.Present {
		rows := extractEvidenceRowsFP(pkg.Evidence.Text)
		if len(rows) == 0 {
			add(pkg.Evidence.Path, "evidence_authority_table_missing")
		}
		for _, field := range forbiddenEvidenceFields {
			if declaredKeyPresent(pkg.Evidence.Text, field) {
				add(pkg.Evidence.Path, "evidence_forbidden_gate_truth_field:"+field)
			}
		}
		for _, row := range rows {
			authority := strings.TrimSpace(row["authority"])
			if forbiddenEvidenceFieldSet[authority] {
				add(pkg.Evidence.Path, "evidence_forbidden_gate_truth_field:"+authority)
			} else if !allowedEvidenceAuthorities[authority] {
				name := authority
				if name == "" {
					name = "<empty>"
				}
				add(pkg.Evidence.Path, "evidence_invalid_authority:"+name)
			}
		}
	}
	return findings
}
