// Package migration implements the P3b v1 -> v2 one-way import
// (AC-077..079, AC-016) and the doctor projection rebuild (AC-011).
//
// The scanner in this file is the single source of "what a v1 project
// looks like on disk" for both the importer and doctor rebuild. It was
// probed against the real v1 repository (~/workspace/repos/beacon):
//
//   - Feature packages live at docs/beacon/<version>/features/<slug>/
//     {truth,tests,tasks,evidence}.md; older versions are archived under
//     docs/beacon/archive/<version>/ with the same layout.
//   - Package status is the truth.md frontmatter `status:` field
//     (observed values: frozen, draft).
//   - v1 has NO per-feature lifecycle ledger. The closest per-feature
//     runtime record is docs/beacon/<version>/.machine/execution/
//     <slug>.revision.json {doc_status, delivery_status,
//     lifecycle_status, created_at} (233 files in the sample).
//   - The only per-feature gate verdict file is .machine/execution/
//     <slug>.truth-review-gate.json {feature, version, pass, blocked,
//     reason_code} (40 files in the sample). Project-level gate reports
//     (.beacon/gate-report.json, .beacon/release-gate-report.json) are
//     version-scoped and carry no feature attribution.
//   - evidence.md is an index table whose "Canonical Artifact" column
//     references blob paths relative to the project root.
//   - The AC-077/078 ledger databases (~/.beacon/runtime-ledger/
//     runtime-ledger.sqlite3, <project>/.beacon/loop/ledger/
//     loop-runs.db) do NOT exist in the sample; the scanner only
//     detects their presence and the importer reports them honestly.
package migration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RevisionInfo is the per-feature v1 runtime record
// (.machine/execution/<slug>.revision.json).
type RevisionInfo struct {
	LifecycleStatus string `json:"lifecycle_status"`
	DocStatus       string `json:"doc_status"`
	DeliveryStatus  string `json:"delivery_status"`
	CreatedAt       string `json:"created_at"`
}

// GateInfo is a parsed per-feature v1 gate verdict
// (.machine/execution/<slug>.truth-review-gate.json). EvaluatedAt is the
// file mtime: the file itself carries no timestamp (derived, reported).
type GateInfo struct {
	Pass        bool   `json:"pass"`
	Blocked     bool   `json:"blocked"`
	ReasonCode  string `json:"reason_code"`
	EvaluatedAt string `json:"evaluated_at"`
}

// EvidenceRef is one blob reference from a feature's evidence.md index.
type EvidenceRef struct {
	Surface string `json:"surface"`
	RelPath string `json:"rel_path"`
}

// FeatureScan is one v1 feature package as found on disk.
type FeatureScan struct {
	Slug         string // frontmatter slug; falls back to the directory name
	DirName      string
	SlugMismatch bool // frontmatter slug != directory name
	Dir          string
	TruthPath    string // project-relative, slash-separated
	TestsPath    string
	HasTests     bool
	PkgStatus    string // frontmatter status, else revision doc_status, else ""
	Revision     *RevisionInfo
	Gate         *GateInfo
	EvidenceRefs []EvidenceRef
}

// VersionScan is one docs version root and its feature packages.
type VersionScan struct {
	Version     string
	DocsRoot    string // project-relative, slash-separated
	AbsDocsRoot string
	Features    []FeatureScan
}

// ProjectGateReport is a version-scoped v1 gate report
// (.beacon/gate-report.json / release-gate-report.json). It has no
// feature attribution, so it is never imported into gate_results.
type ProjectGateReport struct {
	File      string `json:"file"`
	Stage     string `json:"stage"`
	Status    string `json:"status"`
	Version   string `json:"version"`
	RunID     string `json:"run_id"`
	UpdatedAt string `json:"updated_at"`
}

// LedgerSource reports the presence of a frozen-AC ledger database
// (AC-077 v1_runtime_ledger / AC-078 v1_loop_ledger).
type LedgerSource struct {
	Kind    string `json:"kind"` // v1_runtime_ledger | v1_loop_ledger
	Path    string `json:"path"`
	Present bool   `json:"present"`
}

// ProjectScan is the full on-disk picture of one v1 project.
type ProjectScan struct {
	Root           string // resolved absolute path
	Name           string
	CurrentVersion string // from beacon/version-truth.json; "" when undeclared
	CurrentSource  string // which version-truth field declared it
	Versions       []VersionScan
	LosingRoots    []VersionScan // duplicate version roots that lost the dedupe
	GateReports    []ProjectGateReport
	Ledgers        []LedgerSource
}

// ScanProject reads a v1 project directory (read-only) and returns its
// on-disk picture. It never writes to the project.
func ScanProject(root string) (*ProjectScan, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, os.ErrInvalid
	}
	scan := &ProjectScan{Root: abs, Name: filepath.Base(abs)}

	// Docs version roots: docs/beacon/<v> and docs/beacon/archive/<v>,
	// keeping only directories that actually hold a features/ folder.
	var roots []VersionScan
	for _, base := range []string{filepath.Join("docs", "beacon"), filepath.Join("docs", "beacon", "archive")} {
		entries, err := os.ReadDir(filepath.Join(abs, base))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			docsRoot := filepath.Join(base, e.Name())
			if _, err := os.Stat(filepath.Join(abs, docsRoot, "features")); err != nil {
				continue
			}
			roots = append(roots, VersionScan{
				Version:     e.Name(),
				DocsRoot:    filepath.ToSlash(docsRoot),
				AbsDocsRoot: filepath.Join(abs, docsRoot),
			})
		}
	}
	// Dedupe duplicate version names (the sample repo has v1.6.3 both as
	// a near-empty top-level root and as a full archive root): the root
	// with more feature packages wins; ties go to the non-archive root.
	byVersion := map[string][]VersionScan{}
	for _, r := range roots {
		r.Features = scanFeatures(abs, r)
		byVersion[r.Version] = append(byVersion[r.Version], r)
	}
	for version, group := range byVersion {
		winner := group[0]
		for _, cand := range group[1:] {
			if len(cand.Features) > len(winner.Features) ||
				(len(cand.Features) == len(winner.Features) && !isArchiveRoot(cand.DocsRoot) && isArchiveRoot(winner.DocsRoot)) {
				scan.LosingRoots = append(scan.LosingRoots, winner)
				winner = cand
			} else {
				scan.LosingRoots = append(scan.LosingRoots, cand)
			}
		}
		_ = version
		scan.Versions = append(scan.Versions, winner)
	}
	sort.Slice(scan.Versions, func(i, j int) bool {
		return CompareVersions(scan.Versions[i].Version, scan.Versions[j].Version) < 0
	})
	sort.Slice(scan.LosingRoots, func(i, j int) bool {
		return scan.LosingRoots[i].DocsRoot < scan.LosingRoots[j].DocsRoot
	})

	scan.CurrentVersion, scan.CurrentSource = scanCurrentVersion(abs)
	scan.GateReports = scanGateReports(abs)
	scan.Ledgers = scanLedgers(abs)
	return scan, nil
}

func isArchiveRoot(docsRoot string) bool {
	return strings.Contains(docsRoot, "/archive/")
}

// scanCurrentVersion reads beacon/version-truth.json, the v1 version
// authority: docs_default_line first, then the runtime release lines.
func scanCurrentVersion(abs string) (string, string) {
	raw, err := os.ReadFile(filepath.Join(abs, "beacon", "version-truth.json"))
	if err != nil {
		return "", ""
	}
	var vt struct {
		DocsDefaultLine           string `json:"docs_default_line"`
		CurrentRuntimeReleaseLine string `json:"current_runtime_release_line"`
		RuntimeVersion            string `json:"runtime_version"`
	}
	if err := json.Unmarshal(raw, &vt); err != nil {
		return "", ""
	}
	for _, cand := range []struct{ v, src string }{
		{vt.DocsDefaultLine, "version-truth.json:docs_default_line"},
		{vt.CurrentRuntimeReleaseLine, "version-truth.json:current_runtime_release_line"},
		{vt.RuntimeVersion, "version-truth.json:runtime_version"},
	} {
		if cand.v != "" {
			return cand.v, cand.src
		}
	}
	return "", ""
}

func scanGateReports(abs string) []ProjectGateReport {
	var out []ProjectGateReport
	for _, name := range []string{"gate-report.json", "release-gate-report.json"} {
		raw, err := os.ReadFile(filepath.Join(abs, ".beacon", name))
		if err != nil {
			continue
		}
		var rep struct {
			Stage     string `json:"stage"`
			Status    string `json:"status"`
			Version   string `json:"version"`
			RunID     string `json:"run_id"`
			UpdatedAt string `json:"updated_at"`
		}
		if err := json.Unmarshal(raw, &rep); err != nil {
			continue
		}
		out = append(out, ProjectGateReport{
			File: ".beacon/" + name, Stage: rep.Stage, Status: rep.Status,
			Version: rep.Version, RunID: rep.RunID, UpdatedAt: rep.UpdatedAt,
		})
	}
	return out
}

func scanLedgers(abs string) []LedgerSource {
	home, _ := os.UserHomeDir()
	candidates := []LedgerSource{
		{Kind: "v1_runtime_ledger", Path: filepath.Join(abs, ".beacon", "runtime-ledger", "runtime-ledger.sqlite3")},
		{Kind: "v1_loop_ledger", Path: filepath.Join(abs, ".beacon", "loop", "ledger", "loop-runs.db")},
	}
	if home != "" {
		candidates = append(candidates, LedgerSource{
			Kind: "v1_runtime_ledger",
			Path: filepath.Join(home, ".beacon", "runtime-ledger", "runtime-ledger.sqlite3"),
		})
	}
	for i := range candidates {
		if _, err := os.Stat(candidates[i].Path); err == nil {
			candidates[i].Present = true
		}
	}
	return candidates
}

// scanFeatures enumerates docsRoot/features/* directories holding a
// truth.md and parses the per-feature v1 records.
func scanFeatures(projectRoot string, v VersionScan) []FeatureScan {
	entries, err := os.ReadDir(filepath.Join(v.AbsDocsRoot, "features"))
	if err != nil {
		return nil
	}
	var out []FeatureScan
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(v.AbsDocsRoot, "features", e.Name())
		truthAbs := filepath.Join(dir, "truth.md")
		if _, err := os.Stat(truthAbs); err != nil {
			continue
		}
		rel := func(name string) string {
			return filepath.ToSlash(filepath.Join(v.DocsRoot, "features", e.Name(), name))
		}
		f := FeatureScan{
			DirName:   e.Name(),
			Dir:       dir,
			TruthPath: rel("truth.md"),
			TestsPath: rel("tests.md"),
		}
		if _, err := os.Stat(filepath.Join(dir, "tests.md")); err == nil {
			f.HasTests = true
		}
		fm := parseFrontmatter(truthAbs)
		f.Slug = fm["slug"]
		if f.Slug == "" {
			f.Slug = e.Name()
		} else if f.Slug != e.Name() {
			f.SlugMismatch = true
		}
		f.PkgStatus = fm["status"]
		f.Revision = scanRevision(v.AbsDocsRoot, e.Name(), f.Slug)
		if f.PkgStatus == "" && f.Revision != nil {
			f.PkgStatus = f.Revision.DocStatus
		}
		f.Gate = scanGateFile(v.AbsDocsRoot, e.Name(), f.Slug)
		f.EvidenceRefs = scanEvidenceRefs(filepath.Join(dir, "evidence.md"))
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// scanRevision loads .machine/execution/<slug>.revision.json, trying the
// directory name first, then the frontmatter slug.
func scanRevision(docsRoot, dirName, slug string) *RevisionInfo {
	for _, name := range []string{dirName, slug} {
		raw, err := os.ReadFile(filepath.Join(docsRoot, ".machine", "execution", name+".revision.json"))
		if err != nil {
			continue
		}
		var rev RevisionInfo
		if err := json.Unmarshal(raw, &rev); err != nil {
			return nil
		}
		return &rev
	}
	return nil
}

func scanGateFile(docsRoot, dirName, slug string) *GateInfo {
	for _, name := range []string{dirName, slug} {
		path := filepath.Join(docsRoot, ".machine", "execution", name+".truth-review-gate.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var g struct {
			Pass       bool   `json:"pass"`
			Blocked    bool   `json:"blocked"`
			ReasonCode string `json:"reason_code"`
		}
		if err := json.Unmarshal(raw, &g); err != nil {
			return nil
		}
		info := &GateInfo{Pass: g.Pass, Blocked: g.Blocked, ReasonCode: g.ReasonCode}
		if st, err := os.Stat(path); err == nil {
			info.EvaluatedAt = st.ModTime().UTC().Format("2006-01-02T15:04:05.999999999Z07:00")
		}
		return info
	}
	return nil
}

// scanEvidenceRefs parses the evidence.md index table: rows of
// | Surface | Authority | Canonical Artifact | Status | Route |.
func scanEvidenceRefs(path string) []EvidenceRef {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []EvidenceRef
	seen := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 5 {
			continue
		}
		surface := strings.TrimSpace(cells[1])
		artifact := strings.Trim(strings.TrimSpace(cells[3]), "`")
		if surface == "" || surface == "Surface" || strings.HasPrefix(surface, "---") ||
			artifact == "" || artifact == "Canonical Artifact" {
			continue
		}
		if seen[artifact] {
			continue
		}
		seen[artifact] = true
		out = append(out, EvidenceRef{Surface: surface, RelPath: artifact})
	}
	return out
}

// parseFrontmatter extracts top-level `key: value` pairs from a leading
// YAML frontmatter block. Deliberately minimal: v1 truth.md frontmatter
// is flat for the fields the migration needs (slug, status).
func parseFrontmatter(path string) map[string]string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := string(raw)
	if !strings.HasPrefix(text, "---\n") {
		return nil
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return nil
	}
	out := map[string]string{}
	for _, line := range strings.Split(text[4:4+end], "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return out
}

// CompareVersions orders v-prefixed version strings by numeric segments
// (v1.6.10 > v1.6.9). Non-numeric segments compare lexically.
func CompareVersions(a, b string) int {
	pa := strings.Split(strings.TrimPrefix(a, "v"), ".")
	pb := strings.Split(strings.TrimPrefix(b, "v"), ".")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		var na, nb int
		isNumA := parseSegment(pa[i], &na)
		isNumB := parseSegment(pb[i], &nb)
		if isNumA && isNumB {
			if na != nb {
				if na < nb {
					return -1
				}
				return 1
			}
			continue
		}
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(pa) < len(pb):
		return -1
	case len(pa) > len(pb):
		return 1
	}
	return 0
}

func parseSegment(s string, out *int) bool {
	n := 0
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
		n = n*10 + int(c-'0')
	}
	*out = n
	return true
}
