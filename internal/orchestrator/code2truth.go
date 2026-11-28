// Code2Truth intake (R2d, parity audit M13): the Go port of
// Python utils/code2truth.py's core subset — scan an existing
// codebase, build a source inventory, derive candidate-truth
// claims, and (the task's landing, one step beyond Python's
// evidence-only output) materialize a DRAFT package whose
// truth.md carries the claims under an explicit
// "反推草稿待人确认" (reverse-drafted, awaiting human
// confirmation) banner. Candidate truth has NO authority:
// authority = candidate_only, and the package's freeze still
// runs the full intent gate — nothing about being
// reverse-drafted exempts it.
//
// Ported core: the skip set, source-extension set, doc names,
// file-kind rules, entrypoint rules, the 500-file cap and the
// claim/ambiguity builders (_candidate_truth), plus the
// understand-anything provider detection (evidence only).
// Go extension (the task's ask): a light static extraction of
// exported symbols / routes / models per language, summarized
// into the inventory. NOT ported: the lighthouse
// architecture-map artifact and the truth-map/source-
// architecture status blocks (noted in the R2d report).
package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// EventTruthCode2Truth trails one intake that materialized a
// draft package.
const EventTruthCode2Truth = "truth_code2truth"

// code2TruthTreeMapping is the one policy value for the claim-to-tree
// boundary: the kernel emits evidence; the host owns reasoning/proposals.
const code2TruthTreeMapping = "host_owned"

// code2truthSkipDirs is code2truth._skip's ignored part set.
var code2truthSkipDirs = map[string]bool{
	".git": true, ".venv": true, "node_modules": true, "__pycache__": true,
	".pytest_cache": true, ".mypy_cache": true, ".ruff_cache": true,
	".beacon": true, ".omc": true, ".understand-anything": true,
}

// code2truthSourceExts is SOURCE_EXTENSIONS.
var code2truthSourceExts = map[string]bool{
	".py": true, ".js": true, ".jsx": true, ".ts": true, ".tsx": true,
	".vue": true, ".go": true, ".rs": true, ".java": true, ".kt": true,
	".swift": true, ".rb": true, ".php": true, ".sh": true, ".zsh": true,
	".bash": true, ".sql": true, ".yml": true, ".yaml": true, ".toml": true,
	".json": true,
}

// code2truthDocNames is DOC_NAMES.
var code2truthDocNames = map[string]bool{
	"README": true, "AGENTS": true, "SKILL": true, "PRODUCT": true, "DESIGN": true,
}

func code2truthFileKind(rel string) string {
	name := filepath.Base(rel)
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	if name == "SKILL.md" {
		return "skill"
	}
	if code2truthDocNames[strings.ToUpper(stem)] {
		if ext := strings.ToLower(filepath.Ext(name)); ext == "" || ext == ".md" {
			return "doc"
		}
	}
	if strings.HasPrefix(name, "test_") || strings.Contains(rel, "/tests/") || strings.HasSuffix(name, ".test.ts") {
		return "test"
	}
	if code2truthSourceExts[strings.ToLower(filepath.Ext(name))] {
		return "source"
	}
	return "other"
}

func code2truthEntrypoint(rel string) bool {
	low := strings.ToLower(rel)
	switch filepath.Base(low) {
	case "main.go", "main.py", "app.py", "cli.py", "__main__.py", "index.ts", "index.js",
		"server.ts", "server.js", "skill.md":
		return true
	}
	return strings.Contains(low, "commands/") || strings.Contains(low, "bin/") ||
		strings.Contains(low, "scripts/")
}

// Static extraction patterns (Go extension).
var (
	symGoRe         = regexp.MustCompile(`^(?:func|type)\s+([A-Z]\w*)`)
	symPyRe         = regexp.MustCompile(`^(?:def|class)\s+([A-Za-z_]\w*)`)
	symTSRe         = regexp.MustCompile(`^export\s+(?:default\s+)?(?:function|class|const|interface|type)\s+([A-Za-z_]\w*)`)
	routePyRe       = regexp.MustCompile(`@\w+\.(get|post|put|delete|patch|route)\s*\(\s*["']([^"']+)["']`)
	routeGoRe       = regexp.MustCompile(`\.(GET|POST|PUT|DELETE|PATCH|HandleFunc)\s*\(\s*"([^"]+)"`)
	routeTSRe       = regexp.MustCompile(`\b(?:app|router)\.(get|post|put|delete|patch)\s*\(\s*["']([^"']+)["']`)
	modelDeclRe     = regexp.MustCompile(`(?i)^(?:type|class)\s+(\w*(?:Model|Schema|Entity|Record|Row))\b`)
	stateEvidenceRe = regexp.MustCompile(`(?i)\b(?:state|status|transition|from|to)\b\s*[:=]`)
)

// Code2TruthInventory is _collect_source_inventory's payload,
// plus the Symbols summary (Go extension).
type Code2TruthInventory struct {
	Status      string               `json:"status"`
	Root        string               `json:"root"`
	FileCount   int                  `json:"file_count"`
	Counts      map[string]int       `json:"counts"`
	Files       []Code2TruthFile     `json:"files"`
	EntryPoints []string             `json:"entry_points"`
	SkillFiles  []string             `json:"skill_files"`
	TestFiles   []string             `json:"test_files"`
	DocFiles    []string             `json:"doc_files"`
	Truncated   bool                 `json:"truncated"`
	Symbols     CodeSymbolSummary    `json:"symbols"`
	Evidence    []Code2TruthEvidence `json:"evidence,omitempty"`
}

// Code2TruthFile is one inventory row.
type Code2TruthFile struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Entrypoint bool   `json:"entrypoint"`
}

// Code2TruthEvidence is one static source fact with a stable file:line ref.
type Code2TruthEvidence struct {
	Kind string `json:"kind"` // entry | symbol | route | model | state
	Name string `json:"name"`
	Path string `json:"path"`
	Line int    `json:"line"`
}

// CodeSymbolSummary is the static-extraction summary.
type CodeSymbolSummary struct {
	ExportedSymbols int      `json:"exported_symbols"`
	Routes          []string `json:"routes,omitempty"`
	Models          []string `json:"models,omitempty"`
}

func collectCode2TruthInventory(root string) Code2TruthInventory {
	inv := Code2TruthInventory{
		Status: "collected", Root: root,
		Counts: map[string]int{"source": 0, "doc": 0, "skill": 0, "test": 0, "other": 0},
	}
	var rels []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if path != root && code2truthSkipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		rels = append(rels, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(rels)
	symbolSet := map[string]bool{}
	var routes, models []string
	routeSeen, modelSeen := map[string]bool{}, map[string]bool{}
	for _, rel := range rels {
		if len(inv.Files) >= 500 {
			break
		}
		kind := code2truthFileKind(rel)
		if kind == "other" && !code2truthSourceExts[strings.ToLower(filepath.Ext(rel))] {
			continue
		}
		inv.Counts[kind]++
		row := Code2TruthFile{Path: rel, Kind: kind, Entrypoint: code2truthEntrypoint(rel)}
		inv.Files = append(inv.Files, row)
		if row.Entrypoint {
			inv.EntryPoints = append(inv.EntryPoints, rel)
			if len(inv.Evidence) < 200 {
				inv.Evidence = append(inv.Evidence, Code2TruthEvidence{Kind: "entry", Name: rel, Path: rel, Line: 1})
			}
		}
		switch kind {
		case "skill":
			inv.SkillFiles = append(inv.SkillFiles, rel)
		case "test":
			inv.TestFiles = append(inv.TestFiles, rel)
		case "doc":
			inv.DocFiles = append(inv.DocFiles, rel)
		}
		// Static extraction over source files (bounded reads).
		if kind == "source" {
			raw, rerr := os.ReadFile(filepath.Join(root, rel))
			if rerr != nil || len(raw) > 512*1024 {
				continue
			}
			ext := strings.ToLower(filepath.Ext(rel))
			for lineNo, line := range strings.Split(string(raw), "\n") {
				n := lineNo + 1
				addEvidence := func(kind, name string) {
					if len(inv.Evidence) < 200 {
						inv.Evidence = append(inv.Evidence, Code2TruthEvidence{Kind: kind, Name: name, Path: rel, Line: n})
					}
				}
				switch ext {
				case ".go":
					if m := symGoRe.FindStringSubmatch(line); m != nil {
						symbolSet[m[1]] = true
						addEvidence("symbol", m[1])
					}
					if m := routeGoRe.FindStringSubmatch(line); m != nil && !routeSeen[m[2]] {
						routeSeen[m[2]] = true
						routes = append(routes, m[1]+" "+m[2])
						addEvidence("route", m[1]+" "+m[2])
					}
				case ".py":
					if m := symPyRe.FindStringSubmatch(line); m != nil {
						symbolSet[m[1]] = true
						addEvidence("symbol", m[1])
					}
					if m := routePyRe.FindStringSubmatch(line); m != nil && !routeSeen[m[2]] {
						routeSeen[m[2]] = true
						routes = append(routes, strings.ToUpper(m[1])+" "+m[2])
						addEvidence("route", strings.ToUpper(m[1])+" "+m[2])
					}
				case ".ts", ".tsx", ".js", ".jsx":
					if m := symTSRe.FindStringSubmatch(line); m != nil {
						symbolSet[m[1]] = true
						addEvidence("symbol", m[1])
					}
					if m := routeTSRe.FindStringSubmatch(line); m != nil && !routeSeen[m[2]] {
						routeSeen[m[2]] = true
						routes = append(routes, strings.ToUpper(m[1])+" "+m[2])
						addEvidence("route", strings.ToUpper(m[1])+" "+m[2])
					}
				}
				if m := modelDeclRe.FindStringSubmatch(line); m != nil && !modelSeen[m[1]] {
					modelSeen[m[1]] = true
					models = append(models, m[1])
					addEvidence("model", m[1])
				}
				if stateEvidenceRe.MatchString(line) {
					addEvidence("state", strings.TrimSpace(line))
				}
			}
		}
	}
	inv.FileCount = len(inv.Files)
	inv.Truncated = len(inv.Files) >= 500
	inv.EntryPoints = capStrings(inv.EntryPoints, 50)
	inv.SkillFiles = capStrings(inv.SkillFiles, 100)
	inv.TestFiles = capStrings(inv.TestFiles, 100)
	inv.DocFiles = capStrings(inv.DocFiles, 100)
	inv.Symbols = CodeSymbolSummary{
		ExportedSymbols: len(symbolSet),
		Routes:          capStrings(routes, 50),
		Models:          capStrings(models, 50),
	}
	return inv
}

func capStrings(in []string, n int) []string {
	if len(in) > n {
		return in[:n]
	}
	if in == nil {
		return []string{}
	}
	return in
}

// Code2TruthClaim is one candidate-truth claim (_candidate_truth row).
type Code2TruthClaim struct {
	ID         string   `json:"id"`
	Claim      string   `json:"claim"`
	Confidence string   `json:"confidence"`
	SourceRefs []string `json:"source_refs"`
	Authority  string   `json:"authority"`
}

// Code2TruthAmbiguity is one ambiguity row.
type Code2TruthAmbiguity struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
	Route  string `json:"route"`
}

// buildCandidateClaims ports _candidate_truth's claim set
// (the provider-artifact CLAIM-004 needs the understand-
// anything detection, included below).
func buildCandidateClaims(inv Code2TruthInventory, providerAvailable bool, providerRefs []string) (claims []Code2TruthClaim, ambiguity []Code2TruthAmbiguity) {
	kindCount := map[string]int{}
	add := func(kind, claim, confidence, ref string) {
		kindCount[kind]++
		claims = append(claims, Code2TruthClaim{
			ID:    "CLAIM-" + strings.ToUpper(kind) + "-" + fmt.Sprintf("%03d", kindCount[kind]),
			Claim: claim, Confidence: confidence, Authority: "candidate_only",
			SourceRefs: []string{ref},
		})
	}
	var symbolRefs []string
	for _, ev := range inv.Evidence {
		ref := fmt.Sprintf("%s:%d", ev.Path, ev.Line)
		switch ev.Kind {
		case "symbol":
			if len(symbolRefs) < 10 {
				symbolRefs = append(symbolRefs, ref)
			}
		case "entry":
			add("entry", "Entry point detected: "+ev.Name, "high", ref)
		case "route":
			add("route", "HTTP route candidate: "+ev.Name, "medium", ref)
		case "model":
			add("model", "Data model candidate: "+ev.Name, "medium", ref)
		case "state":
			add("state", "State/transition candidate: "+ev.Name, "low", ref)
		}
	}
	if len(symbolRefs) > 0 {
		claims = append(claims, Code2TruthClaim{
			ID: "CLAIM-SYMBOL-001", Claim: "Exported symbols provide candidate boundaries for behavior reconstruction.",
			Confidence: "low", Authority: "candidate_only", SourceRefs: symbolRefs,
		})
	}
	if len(inv.SkillFiles) > 0 {
		ref := inv.SkillFiles[0] + ":1"
		add("skill", "Skill files provide strong workflow evidence for candidate truth reconstruction.", "high", ref)
	}
	if len(inv.TestFiles) > 0 {
		ref := inv.TestFiles[0] + ":1"
		add("test", "Existing tests can be used as behavior evidence for coverage reconstruction.", "medium", ref)
	}
	if providerAvailable && len(providerRefs) > 0 {
		ref := providerRefs[0]
		if !strings.Contains(ref, ":") {
			ref += ":1"
		}
		add("provider", "Understand-Anything artifacts can enrich source graph navigation.", "medium", ref)
	}
	if len(inv.TestFiles) == 0 {
		ambiguity = append(ambiguity, Code2TruthAmbiguity{
			ID: "AMB-002", Reason: "No tests were detected in the intake scope.",
			Route: "Keep coverage claims low-confidence until QA evidence is added.",
		})
	}
	return claims, ambiguity
}

// code2truthProviderArtifacts is PROVIDER_ARTIFACTS.
var code2truthProviderArtifacts = []string{
	".understand-anything/intermediate/scan-result.json",
	".understand-anything/intermediate/import-map.json",
	".understand-anything/intermediate/batches.json",
	".understand-anything/intermediate/structure-result.json",
	".understand-anything/knowledge-graph.json",
	".understand-anything/graph-build-report.json",
	".understand-anything/meta.json",
}

func detectUnderstandAnything(root string) (available bool, existing []string) {
	for _, rel := range code2truthProviderArtifacts {
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			available = true
			existing = append(existing, rel)
		}
	}
	return available, existing
}

// Code2TruthView is the intake surface's result.
type Code2TruthView struct {
	Slug           string                `json:"slug"`
	Version        string                `json:"version"`
	TargetRoot     string                `json:"target_project_root"`
	Status         string                `json:"status"` // candidate
	TruthAuthority bool                  `json:"truth_authority"`
	Inventory      Code2TruthInventory   `json:"source_inventory"`
	Claims         []Code2TruthClaim     `json:"candidate_claims"`
	Ambiguity      []Code2TruthAmbiguity `json:"ambiguity,omitempty"`
	IntakeJSON     string                `json:"intake_json_path"`
	IntakeMarkdown string                `json:"intake_markdown_path"`
	PackageCreated bool                  `json:"package_created"`
	TreeMapping    string                `json:"tree_mapping"`
}

// Code2TruthRegion is one area in a multi-area intake.
type Code2TruthRegion struct {
	Area   string `json:"area"`
	Target string `json:"target"`
	Slug   string `json:"slug"`
	Files  int    `json:"files"`
	Claims int    `json:"claims"`
}

// Code2TruthBatchView is the result of a multi-area intake.
type Code2TruthBatchView struct {
	BaseFeature string             `json:"base_feature"`
	Areas       []Code2TruthView   `json:"areas"`
	Regions     []Code2TruthRegion `json:"regions"`
}

// Code2TruthIntake runs one intake: scan the target, persist
// the Python-shaped intake artifacts under
// .beacon/code2truth/<slug>/, create the draft package via the
// normal create path, and stamp the claims into its truth.md
// under the reverse-draft banner.
func (e *Engine) Code2TruthIntake(projectRoot, version, slug, targetRoot, by string) (Code2TruthView, error) {
	projectRoot = store.ResolveRootPath(projectRoot)
	targetAbs, err := filepath.Abs(targetRoot)
	if err != nil {
		return Code2TruthView{}, err
	}
	if info, serr := os.Stat(targetAbs); serr != nil || !info.IsDir() {
		return Code2TruthView{}, core.NewError(core.ReasonCode2TruthTargetMissing, "truth.Code2Truth",
			"target project root does not exist or is not a directory: "+targetRoot)
	}
	inv := collectCode2TruthInventory(targetAbs)
	providerAvailable, providerRefs := detectUnderstandAnything(targetAbs)
	claims, ambiguity := buildCandidateClaims(inv, providerAvailable, providerRefs)
	if len(claims) == 0 {
		return Code2TruthView{}, core.NewError(core.ReasonCode2TruthClaimsEmpty, "truth.Code2Truth",
			"intake produced no reviewable claim; empty claim tables are an intake failure")
	}
	if ambiguity == nil {
		ambiguity = []Code2TruthAmbiguity{}
	}

	// Intake artifacts (Python's output shape, minus lighthouse).
	outDir := filepath.Join(projectRoot, ".beacon", "code2truth", slug)
	payload := map[string]any{
		"schema_version": "1.0", "artifact": "code2truth-intake",
		"status": "candidate", "feature": slug,
		"target_project_root": targetAbs, "generated_at": nowISO(),
		"authority": "support_evidence_only", "truth_authority": false,
		"tree_mapping":      code2TruthTreeMapping,
		"formal_truth_rule": "code can propose candidate truth; user-confirmed Beacon package Markdown is formal truth",
		"source_inventory":  inv,
		"candidate_truth": map[string]any{
			"status": "candidate", "truth_authority": false,
			"claims": claims, "ambiguity": ambiguity,
		},
		"promotion_gate": map[string]any{
			"user_confirmation_required":   true,
			"direct_package_write_allowed": false,
			"route":                        "beacon-truth / beacon-change-refreeze",
		},
	}
	if err := writeJSONFile(filepath.Join(outDir, "intake.json"), payload); err != nil {
		return Code2TruthView{}, err
	}
	md := renderCode2TruthMarkdown(slug, targetAbs, inv, claims, ambiguity, providerAvailable)
	if err := os.WriteFile(filepath.Join(outDir, "intake.md"), []byte(md), 0o644); err != nil {
		return Code2TruthView{}, err
	}

	// Draft package via the normal create path.
	pkgView, err := e.CreateTruthPackage(projectRoot, version, slug)
	if err != nil {
		return Code2TruthView{}, err
	}
	ctx, err := e.featureContext(pkgView.FeatureID)
	if err != nil {
		return Code2TruthView{}, err
	}
	text, err := readTruthText(ctx)
	if err != nil {
		return Code2TruthView{}, err
	}
	text, _ = setFrontmatterScalar(text, "code2truth_candidate", "true")
	text = replaceSection(text, "## Code2Truth 反推草稿", renderCode2TruthSection(targetAbs, inv, claims, ambiguity))
	if err := writeTruthText(ctx, text); err != nil {
		return Code2TruthView{}, err
	}
	_ = e.appendFeatureEvent(ctx.Feature.ID, EventTruthCode2Truth, map[string]any{
		"by": by, "target": targetAbs, "claims": len(claims),
		"files": inv.FileCount, "intake": ".beacon/code2truth/" + slug + "/intake.json",
	})
	return Code2TruthView{
		Slug: slug, Version: pkgView.Version, TargetRoot: targetAbs,
		Status: "candidate", TruthAuthority: false,
		Inventory: inv, Claims: claims, Ambiguity: ambiguity,
		IntakeJSON:     ".beacon/code2truth/" + slug + "/intake.json",
		IntakeMarkdown: ".beacon/code2truth/" + slug + "/intake.md",
		PackageCreated: true, TreeMapping: code2TruthTreeMapping,
	}, nil
}
