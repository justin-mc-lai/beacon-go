// Truth Integrity Gate (P3a, frozen AC-101): the independent
// cross-reference check of one feature package. Semantic alignment
// target is v1.6's build_truth_integrity_gate (beacon/utils/
// truth_integrity.py): missing ACs, soft TCs, undecided deferrals,
// fake delivery and package validation failures are hard blocks in
// block mode; an already-frozen historical package defaults to
// audit_only and does not block (AC-101).
//
// Everything here is pure: file contents and artifact facts are
// injected, so the verdict is a deterministic function of its input.
package gate

import (
	"regexp"
	"sort"
	"strings"

	"beacon-go/internal/core"
)

// Integrity modes.
const (
	IntegrityModeBlock     = "block"
	IntegrityModeAuditOnly = "audit_only"
)

// DefaultRepairRoute is where a blocked integrity verdict sends the
// author (v1 TIG's _DEFAULT_REPAIR_ROUTE).
const DefaultRepairRoute = "beacon-gen-truth"

// PackageFile is one package member's content (Present=false when
// the file does not exist on disk).
type PackageFile struct {
	Path    string // path as recorded (project-relative or absolute)
	Present bool
	Text    string
}

// PackageFiles is the frozen four-piece set (canonical_refs of the
// v1.5.2 package-authoritative model): truth / tests / tasks /
// evidence.
type PackageFiles struct {
	Slug     string
	Truth    PackageFile
	Tests    PackageFile
	Tasks    PackageFile
	Evidence PackageFile
}

// ArtifactFact is one registered artifact pointer (artifact_refs).
type ArtifactFact struct {
	AbsPath string
}

// Finding is one integrity fracture, itemised (AC-101: hard block +
// repair_route; the Go surface adds one stable reason code per kind).
// Severity is "" (blocking) or "warn" (reported, never blocks) —
// the Python TIG's one warn class is the determinism hollow-template
// scan (truth_integrity.py: blocking_findings excludes
// TRUTH_DETERMINISM_HOLLOW_TEMPLATE findings).
type Finding struct {
	Kind        string `json:"kind"`
	ReasonCode  string `json:"reason_code"`
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
	Detail      string `json:"detail"`
	RepairRoute string `json:"repair_route"`
	Severity    string `json:"severity,omitempty"`
}

// IntegrityReport is the integrity evaluation outcome. Verdict is
// pass (no findings), blocked (block mode with findings) or audit
// (audit_only mode with findings — reported, not blocking).
type IntegrityReport struct {
	Verdict     Verdict   `json:"verdict"`
	Mode        string    `json:"mode"`
	ReasonCodes []string  `json:"reason_codes,omitempty"`
	Findings    []Finding `json:"findings,omitempty"`
}

// IntegrityInput is everything the check may consult.
type IntegrityInput struct {
	Pkg       PackageFiles
	Artifacts []ArtifactFact
	// ResolveRef maps an evidence.md reference to the absolute path
	// it denotes (project-root relative / ~/ / absolute).
	ResolveRef func(ref string) string
	// FileExists reports whether an absolute path exists on disk.
	FileExists func(absPath string) bool
}

var (
	acDefBoldRe  = regexp.MustCompile(`\*\*AC-(\d{3})\*\*`)
	acDefTableRe = regexp.MustCompile(`(?m)^\|\s*AC-(\d{3})\s*\|`)
	acRefRe      = regexp.MustCompile(`AC-\d{3}`)
	// truth_integrity._DEFERRAL_PENDING_RE (the 状态 pending
	// alternative joined in R2a; the other two predate it).
	deferralRe = regexp.MustCompile(`(?i)\|\s*pending\s*\||user_decision:\s*pending|状态\s*pending`)
)

// ParseFrontmatter reads the minimal YAML frontmatter of a markdown
// document: top-level scalar keys plus the canonical_refs block.
// It is deliberately small (the package-authoritative fields only).
func ParseFrontmatter(text string) (map[string]string, map[string]string, bool) {
	scalars := map[string]string{}
	refs := map[string]string{}
	if !strings.HasPrefix(text, "---\n") {
		return scalars, refs, false
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return scalars, refs, false
	}
	raw := text[4 : 4+end]
	inRefs := false
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") && strings.Contains(line, ":") {
			key, val, _ := strings.Cut(line, ":")
			key, val = strings.TrimSpace(key), strings.Trim(strings.TrimSpace(val), `"'`)
			inRefs = key == "canonical_refs"
			if !inRefs {
				scalars[key] = val
			}
			continue
		}
		if inRefs && strings.HasPrefix(line, "  ") && strings.Contains(line, ":") {
			key, val, _ := strings.Cut(strings.TrimSpace(line), ":")
			refs[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(val), `"'`)
		}
	}
	return scalars, refs, true
}

// DefinedACs extracts the AC ids a truth document DEFINES: the bold
// `**AC-NNN**` definition lines and AC-table first cells of the
// frozen format. When neither shape occurs (non-frozen package
// styles), every AC mention counts as a definition — fail-open here
// is safe because the tests->truth direction still catches dangling
// references against the strict set whenever one exists.
func DefinedACs(truth string) map[string]bool {
	out := map[string]bool{}
	for _, m := range acDefBoldRe.FindAllStringSubmatch(truth, -1) {
		out["AC-"+m[1]] = true
	}
	for _, m := range acDefTableRe.FindAllStringSubmatch(truth, -1) {
		out["AC-"+m[1]] = true
	}
	if len(out) == 0 {
		for _, id := range acRefRe.FindAllString(truth, -1) {
			out[id] = true
		}
	}
	return out
}

// TCRow is one row of the tests coverage matrix (columns:
// AC ID | TC ID | Layer | Command | Assertion).
type TCRow struct {
	AC        string
	TC        string
	Command   string
	Assertion string
	Line      int
}

var softCellValues = map[string]bool{
	"": true, "integration": true, "目录存在": true, "fixture": true, "可选": true,
}

// IsSoft reports whether the row is a soft TC (AC-101 / AC-164): a
// coverage row whose command or assertion cell is missing or one of
// the forbidden standalone markers, without an explicit BLOCKED:
// escape in the command cell.
func (r TCRow) IsSoft() bool {
	if strings.Contains(r.Command, "BLOCKED:") {
		return false
	}
	return softCellValues[strings.ToLower(strings.TrimSpace(r.Command))] ||
		softCellValues[strings.ToLower(strings.TrimSpace(r.Assertion))]
}

// ParseCoverageRows extracts the coverage-matrix rows of a tests
// document: tables whose header names an "AC ID" column. Other
// tables (e.g. the phase TC index, TC-first) are not the coverage
// authority and are ignored.
func ParseCoverageRows(tests string) []TCRow {
	var rows []TCRow
	inMatrix := false
	for i, line := range strings.Split(tests, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			if trimmed != "" {
				inMatrix = false
			}
			continue
		}
		cells := splitCells(trimmed)
		lowered := make([]string, len(cells))
		for j, c := range cells {
			lowered[j] = strings.ToLower(c)
		}
		if containsCell(lowered, "ac id") {
			inMatrix = true
			continue
		}
		if !inMatrix || len(cells) == 0 {
			continue
		}
		if isSeparatorRow(cells) {
			continue
		}
		if !acRefRe.MatchString(cells[0]) || len(cells[0]) > 8 {
			continue
		}
		row := TCRow{AC: cells[0], Line: i + 1}
		if len(cells) > 1 {
			row.TC = cells[1]
		}
		if len(cells) > 3 {
			row.Command = cells[3]
		}
		if len(cells) > 4 {
			row.Assertion = cells[4]
		}
		rows = append(rows, row)
	}
	return rows
}

func splitCells(line string) []string {
	parts := strings.Split(strings.Trim(line, "|"), "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func containsCell(cells []string, want string) bool {
	for _, c := range cells {
		if c == want {
			return true
		}
	}
	return false
}

func isSeparatorRow(cells []string) bool {
	for _, c := range cells {
		if c == "" {
			continue
		}
		for _, r := range c {
			if r != '-' && r != ':' {
				return false
			}
		}
	}
	return true
}

// fakeDeliveryPatterns ports the v1 TIG fake-delivery line patterns
// (kind -> regexp). The docs_only kind is handled separately because
// the v1 pattern uses a negative lookbehind for 非/不 negations,
// which Go's RE2 does not support; the Go port checks the preceding
// rune manually (see scanFakeDelivery).
var fakeDeliveryPatterns = []struct {
	kind string
	re   *regexp.Regexp
}{
	{"placeholder_implementation", regexp.MustCompile(`(?i)placeholder[- ]implementation|占位实现|假实现`)},
	{"placeholder_acceptance", regexp.MustCompile(`(?i)placeholder[- ]acceptance|占位验收|placeholder ac|stub-only closure|contract test 占位`)},
	{"fake_runner_evidence", regexp.MustCompile(`(?i)fake[- ]runner[- ]evidence|fake[- ]runner|假 runner|假运行器`)},
	{"zero_assertions", regexp.MustCompile(`(?i)zero[- ]assertions|0 assertions|0 个断言|零断言`)},
	{"placeholder_evidence", regexp.MustCompile(`(?i)placeholder[- ]evidence|占位证据|假证据`)},
	{"soft_test_only", regexp.MustCompile(`(?i)soft[- ]test[- ]only|软占位|仅 integration 验证`)},
	{"pending_deferral", regexp.MustCompile(`(?i)user_decision:\s*pending|\|\s*pending\s*\|`)},
	{"implementation_claim_without_evidence", regexp.MustCompile(`(?i)implementation claim without evidence|无证据声称完成`)},
}

var docsOnlyRe = regexp.MustCompile(`(?i)docs[- ]only([- ]completion)?|只写文档|文档完成`)

// docsOnlyCommandRe matches commands that only read/print files. The
// v1 truth-review check C_exec refused these as the sole "test" for a
// business AC: proving a string exists is not proving the behavior.
var docsOnlyCommandRe = regexp.MustCompile(`(?i)^\s*(rg|grep|cat|head|tail|ls|find|echo|test\s+-[fd])\b`)

func scanFakeDelivery(file string, text string) []Finding {
	var out []Finding
	for i, line := range strings.Split(text, "\n") {
		for _, p := range fakeDeliveryPatterns {
			if p.re.MatchString(line) {
				out = append(out, Finding{
					Kind: "fake_delivery_" + p.kind, ReasonCode: core.ReasonIntegrityFakeDelivery,
					File: file, Line: i + 1, Detail: strings.TrimSpace(line), RepairRoute: DefaultRepairRoute,
				})
			}
		}
		for _, loc := range docsOnlyRe.FindAllStringIndex(line, -1) {
			// v1 excludes negated mentions （非docs-only / 不docs-only).
			if loc[0] > 0 {
				r := []rune(line[:loc[0]])
				if len(r) > 0 && (r[len(r)-1] == '非' || r[len(r)-1] == '不') {
					continue
				}
			}
			out = append(out, Finding{
				Kind: "fake_delivery_docs_only_closure", ReasonCode: core.ReasonIntegrityFakeDelivery,
				File: file, Line: i + 1, Detail: strings.TrimSpace(line), RepairRoute: DefaultRepairRoute,
			})
		}
	}
	return out
}

var evidenceRefRe = regexp.MustCompile(`[^\s|]+/[^\s|]*`)

// EvidenceRefs extracts the artifact references of an evidence
// document: path-like tokens (containing a slash, ending in a file
// extension or slash) from code spans and table cells. conversation:
// pseudo-refs, URLs, template placeholders (<...>) and bare words
// are not artifact references.
func EvidenceRefs(evidence string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(tok string) {
		tok = strings.Trim(tok, "`*,;，。)）")
		if tok == "" || seen[tok] {
			return
		}
		if strings.HasPrefix(tok, "conversation:") || strings.Contains(tok, "://") ||
			strings.ContainsAny(tok, "<>") || !strings.Contains(tok, "/") {
			return
		}
		base := tok[strings.LastIndex(tok, "/")+1:]
		if !strings.Contains(base, ".") && !strings.HasSuffix(tok, "/") {
			return
		}
		seen[tok] = true
		out = append(out, tok)
	}
	for _, line := range strings.Split(evidence, "\n") {
		for _, tok := range evidenceRefRe.FindAllString(line, -1) {
			add(tok)
		}
	}
	return out
}

// CheckIntegrity runs the full cross-reference check and returns the
// itemised fracture list. Checks, in order: package completeness,
// tests<->truth AC cross-reference, soft TCs, undecided deferrals,
// fake delivery, evidence artifact registration + existence, tasks
// slug consistency.
func CheckIntegrity(in IntegrityInput, mode string) IntegrityReport {
	var findings []Finding
	add := func(f Finding) {
		if f.RepairRoute == "" {
			f.RepairRoute = DefaultRepairRoute
		}
		findings = append(findings, f)
	}
	files := []struct {
		key string
		pf  PackageFile
	}{
		{"truth", in.Pkg.Truth}, {"tests", in.Pkg.Tests},
		{"tasks", in.Pkg.Tasks}, {"evidence", in.Pkg.Evidence},
	}
	// 1. Package validation: the four-piece set must be complete and
	// the truth frontmatter must parse.
	for _, f := range files {
		if !f.pf.Present {
			add(Finding{Kind: "missing_" + f.key, ReasonCode: core.ReasonIntegrityPackageIncomplete,
				File: f.pf.Path, Detail: "package member " + f.key + " is missing"})
		}
	}
	var tasksSlug string
	if in.Pkg.Truth.Present {
		if _, _, ok := ParseFrontmatter(in.Pkg.Truth.Text); !ok {
			add(Finding{Kind: "frontmatter_invalid", ReasonCode: core.ReasonIntegrityPackageIncomplete,
				File: in.Pkg.Truth.Path, Detail: "truth.md frontmatter is missing or unclosed"})
		}
	}
	if in.Pkg.Tasks.Present {
		scalars, _, ok := ParseFrontmatter(in.Pkg.Tasks.Text)
		if ok {
			tasksSlug = scalars["slug"]
		}
	}

	// 2+3. tests <-> truth AC cross-reference + soft TCs.
	if in.Pkg.Truth.Present && in.Pkg.Tests.Present {
		defined := DefinedACs(in.Pkg.Truth.Text)
		rows := ParseCoverageRows(in.Pkg.Tests.Text)
		covered := map[string]bool{}
		for _, row := range rows {
			covered[row.AC] = true
			if !defined[row.AC] {
				add(Finding{Kind: "tests_unknown_ac", ReasonCode: core.ReasonIntegrityTestsUnknownAC,
					File: in.Pkg.Tests.Path, Line: row.Line,
					Detail: "tests references " + row.AC + " which truth does not define"})
			}
			if row.IsSoft() {
				add(Finding{Kind: "soft_tc", ReasonCode: core.ReasonIntegritySoftTC,
					File: in.Pkg.Tests.Path, Line: row.Line,
					Detail: row.TC + " for " + row.AC + " is a soft TC (no Command + Assertion, no BLOCKED: escape)"})
			} else if docsOnlyCommandRe.MatchString(strings.Trim(strings.TrimSpace(row.Command), "`")) {
				add(Finding{Kind: "docs_only_tc", ReasonCode: core.ReasonIntegrityDocsOnlyTC,
					File: in.Pkg.Tests.Path, Line: row.Line,
					Detail: row.TC + " for " + row.AC + " only searches/prints files; a business AC needs an execution-layer TC"})
			}
		}
		ids := make([]string, 0, len(defined))
		for id := range defined {
			ids = append(ids, id)
		}
		sort.Strings(ids) // deterministic finding order
		for _, id := range ids {
			if !covered[id] {
				add(Finding{Kind: "ac_without_tc", ReasonCode: core.ReasonIntegrityACWithoutTC,
					File:   in.Pkg.Truth.Path,
					Detail: id + " is defined in truth but has no coverage row in tests"})
			}
		}
	}

	// 4. Undecided deferrals in truth.
	if in.Pkg.Truth.Present {
		for i, line := range strings.Split(in.Pkg.Truth.Text, "\n") {
			if deferralRe.MatchString(line) {
				add(Finding{Kind: "deferral_pending", ReasonCode: core.ReasonIntegrityDeferralPending,
					File: in.Pkg.Truth.Path, Line: i + 1, Detail: strings.TrimSpace(line),
					RepairRoute: "beacon-gen-change"})
			}
		}
	}

	// 5. Fake delivery across the package.
	for _, f := range files {
		if f.pf.Present {
			findings = append(findings, scanFakeDelivery(f.pf.Path, f.pf.Text)...)
		}
	}

	// 6. Evidence artifacts: registered in artifact_refs AND the
	// file exists on disk.
	if in.Pkg.Evidence.Present && in.ResolveRef != nil && in.FileExists != nil {
		registered := map[string]bool{}
		for _, a := range in.Artifacts {
			registered[a.AbsPath] = true
		}
		for _, ref := range EvidenceRefs(in.Pkg.Evidence.Text) {
			abs := in.ResolveRef(ref)
			if !registered[abs] {
				add(Finding{Kind: "evidence_artifact_unregistered", ReasonCode: core.ReasonIntegrityEvidenceUnreg,
					File:   in.Pkg.Evidence.Path,
					Detail: "evidence references " + ref + " (" + abs + ") which no artifact_refs row registers"})
			}
			if !in.FileExists(abs) {
				add(Finding{Kind: "evidence_artifact_missing", ReasonCode: core.ReasonIntegrityEvidenceMissing,
					File:   in.Pkg.Evidence.Path,
					Detail: "evidence references " + ref + " (" + abs + ") which does not exist on disk"})
			}
		}
	}

	// 7. tasks slug consistency.
	if in.Pkg.Tasks.Present && tasksSlug != in.Pkg.Slug {
		add(Finding{Kind: "slug_mismatch", ReasonCode: core.ReasonIntegritySlugMismatch,
			File:   in.Pkg.Tasks.Path,
			Detail: "tasks.md frontmatter slug " + orMissing(tasksSlug) + " != package slug " + in.Pkg.Slug})
	}

	// 8-12. The R2a Python-parity groups (audit H3), appended in
	// the Python TIG's aggregation order: clarity scan of the four
	// files, the domain FSM authority, the extension scan, the
	// determinism scan, and feature-package validation.
	for _, f := range files {
		if f.pf.Present {
			findings = append(findings, scanClarityFile(f.pf)...)
		}
	}
	if in.Pkg.Truth.Present {
		if required, df := EvaluateDomainFSM(in.Pkg.Truth.Text, in.Pkg.Tests.Text); required {
			for _, d := range df {
				findings = append(findings, Finding{
					Kind: d.Kind, ReasonCode: d.ReasonCode,
					File: in.Pkg.Truth.Path, Detail: d.Text, RepairRoute: DefaultRepairRoute,
				})
			}
		}
	}
	findings = append(findings, scanExtensionFiles(in.Pkg)...)
	for _, f := range files {
		if f.pf.Present {
			findings = append(findings, scanDeterminismFile(f.pf)...)
		}
	}
	findings = append(findings, ValidateFeaturePackage(in.Pkg)...)

	rep := IntegrityReport{Mode: mode, Findings: findings, Verdict: VerdictPass}
	// Warn-severity findings (the determinism hollow templates)
	// are reported but never block — the Python TIG excludes them
	// from its blocking set by name.
	blocking := false
	for _, f := range findings {
		if f.Severity != "warn" {
			blocking = true
			break
		}
	}
	if len(findings) > 0 {
		seen := map[string]bool{}
		push := func(code string) {
			if code != "" && !seen[code] {
				seen[code] = true
				rep.ReasonCodes = append(rep.ReasonCodes, code)
			}
		}
		if blocking {
			if mode == IntegrityModeAuditOnly {
				rep.Verdict = VerdictAudit
				push(core.ReasonIntegrityAuditOnly)
			} else {
				rep.Verdict = VerdictBlocked
			}
		}
		for _, f := range findings {
			push(f.ReasonCode)
		}
	}
	return rep
}

func orMissing(s string) string {
	if s == "" {
		return "(missing)"
	}
	return s
}
