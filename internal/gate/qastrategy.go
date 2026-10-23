// QA strategy generation (R2d, parity audit M2): the Go port of
// Python utils/qa9.py's build_qa_strategy chain — scan the
// project, infer its type, lay the nine QA types over an
// applicability matrix, and derive per-type command presets.
// qa9.py is consumed in Python by auto_test / gate / qa /
// test_case; in Go the strategy is persisted to
// .beacon/qa-strategy.json and read back by the autotest
// command-discovery chain (orchestrator/autotest.go) — see
// orchestrator/qastrategy.go.
//
// Not ported: build_qa_llm_plan (the LLM plan envelope). The Go
// kernel never calls a model; the strategy itself is the
// rule-based artifact. Noted in the R2d report.
package gate

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// QA9Types is qa9.QA9_TYPES: the nine QA layers.
var QA9Types = []string{
	"unit", "interface", "contract", "integration", "smoke",
	"functional", "regression", "exception", "compatibility",
}

// QAProjectTypes is qa9.PROJECT_TYPES.
var QAProjectTypes = []string{"ai-agent-skill", "web-app", "api-service", "sdk-lib"}

// qaProfileTypes is qa9.QA_PROFILE_TYPES.
var qaProfileTypes = map[string][]string{
	"web-core": {"unit", "interface", "smoke", "functional", "regression", "exception"},
	"web-full": QA9Types,
	"custom":   QA9Types,
}

var qaLegacyToType = map[string]string{
	"ui": "functional", "api-curl": "interface", "docs": "contract",
}

var qaTypeToLegacy = map[string]string{
	"functional": "ui", "interface": "api-curl",
}

// NormalizeQAType ports normalize_qa_type.
func NormalizeQAType(value string) string {
	raw := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(value)), "_", "-")
	if mapped, ok := qaLegacyToType[raw]; ok {
		return mapped
	}
	return raw
}

// QAToLegacyLayer ports to_legacy_layer.
func QAToLegacyLayer(value string) string {
	t := NormalizeQAType(value)
	if mapped, ok := qaTypeToLegacy[t]; ok {
		return mapped
	}
	return t
}

// ParseQATypes ports parse_qa_types: filter to known types
// (legacy names normalized), dedupe, fall back to the profile
// default when nothing usable remains.
func ParseQATypes(raw []string, profile string) []string {
	profileNorm := strings.ToLower(strings.TrimSpace(profile))
	if profileNorm == "" {
		profileNorm = "web-full"
	}
	defaults, ok := qaProfileTypes[profileNorm]
	if !ok {
		defaults = qaProfileTypes["web-full"]
	}
	if raw == nil {
		return append([]string{}, defaults...)
	}
	var values []string
	for _, item := range raw {
		for _, part := range strings.Split(item, ",") {
			if part = strings.TrimSpace(part); part != "" {
				values = append(values, part)
			}
		}
	}
	known := map[string]bool{}
	for _, t := range QA9Types {
		known[t] = true
	}
	var out []string
	seen := map[string]bool{}
	for _, v := range values {
		t := NormalizeQAType(v)
		if !known[t] || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	if len(out) == 0 {
		return append([]string{}, defaults...)
	}
	return out
}

// playwrightWorkspaceCommand ports _playwright_workspace_command.
func playwrightWorkspaceCommand(args string) string {
	suffix := ""
	if strings.TrimSpace(args) != "" {
		suffix = " " + strings.TrimSpace(args)
	}
	candidates := []string{".", "tournament_miniapp", "frontend", "web", "app", "client"}
	probes := []string{
		`if [ -f "tools/beacon/functional-layer.sh" ]; then bash tools/beacon/functional-layer.sh; exit $?; fi`,
	}
	for _, candidate := range candidates {
		prefix := ""
		cdClause := ":"
		if candidate != "." {
			prefix = candidate + "/"
			cdClause = `cd "` + candidate + `"`
		}
		probes = append(probes,
			`if [ -f "`+prefix+`playwright.config.ts" ] || [ -f "`+prefix+`playwright.config.js" ]; then `+
				cdClause+` && npx playwright test`+suffix+`; exit $?; fi`)
	}
	probes = append(probes, "npx playwright test"+suffix)
	return strings.Join(probes, "; ")
}

// DefaultCommandForQAType ports default_command_for_qa_type.
func DefaultCommandForQAType(qaType, uiRunner string) string {
	runner := strings.ToLower(strings.TrimSpace(uiRunner))
	if runner == "" {
		runner = "playwright"
	}
	junitArg := `--junitxml "${BEACON_JUNIT_PATH:-.beacon/junit.xml}"`
	switch qaType {
	case "unit":
		return "python3 -m pytest -q -k unit " + junitArg
	case "interface":
		return "python3 -m pytest -q -k interface " + junitArg
	case "contract":
		return "python3 -m pytest -q -k contract " + junitArg
	case "integration":
		return "python3 -m pytest -q -k integration " + junitArg
	case "smoke":
		return "python3 -m pytest -q -m smoke " + junitArg
	case "functional":
		switch runner {
		case "cypress":
			return "npx cypress run --headless"
		case "browser-use":
			return `python3 -m beacon.adapters.browser_use_runner --mode functional --deterministic-command "python3 -m pytest -q -k functional --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\""`
		case "agent-browser":
			return `python3 -m beacon.adapters.agent_browser_runner --mode functional --deterministic-command "python3 -m pytest -q -k functional --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\""`
		case "midscene":
			return "node scripts/midscene-functional.mjs"
		}
		return playwrightWorkspaceCommand("")
	case "regression":
		return "python3 -m pytest -q -k regression " + junitArg
	case "exception":
		return "python3 -m pytest -q -k exception " + junitArg
	case "compatibility":
		switch runner {
		case "cypress":
			return "npx cypress run --browser chrome"
		case "browser-use":
			return `python3 -m beacon.adapters.browser_use_runner --mode compatibility --deterministic-command "python3 -m pytest -q -k compatibility --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\""`
		case "agent-browser":
			return `python3 -m beacon.adapters.agent_browser_runner --mode compatibility --deterministic-command "python3 -m pytest -q -k compatibility --junitxml \"${BEACON_JUNIT_PATH:-.beacon/junit.xml}\""`
		case "midscene":
			return "node scripts/midscene-compatibility.mjs"
		}
		return playwrightWorkspaceCommand("--project=chromium --project=firefox --project=webkit")
	}
	return "python3 -m pytest -q " + junitArg
}

// DefaultAssertionForQAType ports default_assertion_for_qa_type.
func DefaultAssertionForQAType(qaType string) string {
	switch qaType {
	case "unit":
		return "All unit assertions pass"
	case "interface":
		return "HTTP status and schema assertions pass"
	case "contract":
		return "Provider/consumer contract assertions pass"
	case "integration":
		return "Integration flow assertions pass"
	case "smoke":
		return "Critical path assertions pass"
	case "functional":
		return "Web UI assertions and key actions pass"
	case "regression":
		return "No known behavior regression found"
	case "exception":
		return "Negative-path assertions pass"
	case "compatibility":
		return "Cross-browser/device assertions pass"
	}
	return "assertion defined"
}

// ProjectScan is infer_project_scan's result.
type ProjectScan struct {
	Language    string   `json:"language"`
	Framework   string   `json:"framework"`
	Frontend    string   `json:"frontend"`
	APISpecRefs []string `json:"api_spec_refs"`
	RiskPoints  []string `json:"risk_points"`
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// walkRelFiles lists project-relative file paths (sorted),
// skipping VCS/vendor dirs, capped for safety.
func walkRelFiles(root string, cap int) []string {
	var out []string
	skipDirs := map[string]bool{".git": true, "node_modules": true, ".venv": true, "__pycache__": true}
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if path != root && skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return nil
		}
		out = append(out, filepath.ToSlash(rel))
		if len(out) >= cap {
			return filepath.SkipAll
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// InferProjectScan ports infer_project_scan.
func InferProjectScan(root string) ProjectScan {
	language, framework, frontend := "unknown", "unknown", "unknown"
	switch {
	case pathExists(filepath.Join(root, "pom.xml")) || pathExists(filepath.Join(root, "build.gradle")):
		language = "java"
		framework = "java-app"
		if pathExists(filepath.Join(root, "src", "main")) {
			framework = "spring"
		}
	case pathExists(filepath.Join(root, "package.json")):
		language = "javascript"
		switch {
		case pathExists(filepath.Join(root, "next.config.js")) || pathExists(filepath.Join(root, "next.config.mjs")):
			framework = "nextjs"
		case pathExists(filepath.Join(root, "nuxt.config.ts")) || pathExists(filepath.Join(root, "nuxt.config.js")):
			framework = "nuxt"
		default:
			framework = "node-web"
		}
		frontend = "web"
	case pathExists(filepath.Join(root, "pyproject.toml")) || pathExists(filepath.Join(root, "requirements.txt")):
		language = "python"
		framework = "python-app"
		if pathExists(filepath.Join(root, "manage.py")) {
			framework = "django"
		}
	case pathExists(filepath.Join(root, "go.mod")):
		language = "go"
		framework = "go-app"
	}

	var apiRefs []string
	seen := map[string]bool{}
	addRef := func(rel string) {
		if !seen[rel] {
			seen[rel] = true
			apiRefs = append(apiRefs, rel)
		}
	}
	for _, pattern := range []string{"openapi*.yml", "openapi*.yaml", "swagger*.json"} {
		matches, _ := filepath.Glob(filepath.Join(root, pattern))
		for _, m := range matches {
			if rel, err := filepath.Rel(root, m); err == nil {
				addRef(filepath.ToSlash(rel))
			}
		}
	}
	for _, rel := range walkRelFiles(root, 5000) {
		if strings.HasPrefix(rel, "contracts/") && strings.HasSuffix(rel, ".json") {
			addRef(rel)
		}
	}
	sort.Strings(apiRefs)
	if len(apiRefs) > 20 {
		apiRefs = apiRefs[:20]
	}
	if apiRefs == nil {
		apiRefs = []string{}
	}
	return ProjectScan{
		Language: language, Framework: framework, Frontend: frontend,
		APISpecRefs: apiRefs,
		RiskPoints: []string{
			"authz/authn boundary",
			"cross-browser ui behavior",
			"api contract drift",
			"exception handling and replay safety",
		},
	}
}

// InferProjectType ports infer_project_type.
func InferProjectType(root string, scan ProjectScan) string {
	framework := strings.ToLower(scan.Framework)
	hasSkillMarkers := pathExists(filepath.Join(root, "skills")) ||
		pathExists(filepath.Join(root, "agent-skills")) ||
		strings.Contains(framework, "skill")
	if !hasSkillMarkers {
		for _, rel := range walkRelFiles(root, 5000) {
			if filepath.Base(rel) == "SKILL.md" {
				hasSkillMarkers = true
				break
			}
		}
	}
	if hasSkillMarkers {
		return "ai-agent-skill"
	}
	if strings.ToLower(scan.Frontend) == "web" {
		return "web-app"
	}
	if len(scan.APISpecRefs) > 0 {
		return "api-service"
	}
	return "sdk-lib"
}

// QAApplicability ports _build_qa_applicability: per project
// type, which of the nine layers are required / conditional / na.
func QAApplicability(projectType string, hasWebUI bool) map[string]string {
	switch projectType {
	case "ai-agent-skill":
		out := map[string]string{
			"unit": "required", "interface": "required", "contract": "required",
			"integration": "required", "smoke": "required", "functional": "conditional",
			"regression": "required", "exception": "required", "compatibility": "conditional",
		}
		if hasWebUI {
			out["functional"] = "required"
			out["compatibility"] = "required"
		}
		return out
	case "web-app":
		return map[string]string{
			"unit": "required", "interface": "required", "contract": "conditional",
			"integration": "required", "smoke": "required", "functional": "required",
			"regression": "required", "exception": "required", "compatibility": "required",
		}
	case "api-service":
		return map[string]string{
			"unit": "required", "interface": "required", "contract": "required",
			"integration": "required", "smoke": "required", "functional": "na",
			"regression": "required", "exception": "required", "compatibility": "na",
		}
	default: // sdk-lib
		return map[string]string{
			"unit": "required", "interface": "required", "contract": "conditional",
			"integration": "conditional", "smoke": "required", "functional": "na",
			"regression": "required", "exception": "required", "compatibility": "na",
		}
	}
}

func firstExistingRelative(root string, matches []string) string {
	for _, m := range matches {
		if pathExists(filepath.Join(root, m)) {
			return m
		}
	}
	return ""
}

// CommandPresetsFromScan ports command_presets_from_project_scan:
// project facts override the per-type defaults (a postman
// collection, an OpenAPI spec, a compose test file), then every
// remaining type falls back to its default command.
func CommandPresetsFromScan(root string, scan ProjectScan, uiRunner string) map[string]string {
	presets := map[string]string{}
	firstAPISpec := ""
	for _, ref := range scan.APISpecRefs {
		if strings.TrimSpace(ref) != "" {
			firstAPISpec = ref
			break
		}
	}
	// Postman collection: first match across the source's pattern
	// order, paths sorted within a pattern.
	var postman string
	{
		var anyCollection, inPostman, inCollections []string
		for _, rel := range walkRelFiles(root, 5000) {
			switch {
			case strings.HasSuffix(rel, ".postman_collection.json"):
				anyCollection = append(anyCollection, rel)
			case strings.HasPrefix(rel, "postman/") && strings.HasSuffix(rel, ".json"):
				inPostman = append(inPostman, rel)
			case strings.HasPrefix(rel, "collections/") && strings.HasSuffix(rel, ".json"):
				inCollections = append(inCollections, rel)
			}
		}
		for _, group := range [][]string{anyCollection, inPostman, inCollections} {
			if len(group) > 0 {
				postman = group[0]
				break
			}
		}
	}
	if postman != "" {
		presets["interface"] = "npx newman run " + postman
	}
	if firstAPISpec != "" {
		presets["contract"] = "python3 -m schemathesis run " + firstAPISpec
		if _, ok := presets["interface"]; !ok {
			presets["interface"] = "python3 -m schemathesis run " + firstAPISpec + " --checks all"
		}
	}
	if pathExists(filepath.Join(root, "docker-compose.test.yml")) {
		presets["integration"] = "docker compose -f docker-compose.test.yml up --abort-on-container-exit"
	} else if pathExists(filepath.Join(root, "testcontainers")) {
		presets["integration"] = "python3 -m pytest -q -k integration --maxfail=1"
	}
	for _, t := range QA9Types {
		if _, ok := presets[t]; !ok {
			presets[t] = DefaultCommandForQAType(t, uiRunner)
		}
	}
	return presets
}

// QAStrategy is build_qa_strategy's payload (schema 1.0.2).
type QAStrategy struct {
	SchemaVersion   string            `json:"schema_version"`
	GeneratedAt     string            `json:"generated_at"`
	ProjectType     string            `json:"project_type"`
	ProjectScan     ProjectScan       `json:"project_scan"`
	Applicability   map[string]string `json:"qa_applicability"`
	RequiredTypes   []string          `json:"qa_required_types"`
	RunnerPolicy    map[string]any    `json:"runner_policy"`
	CommandBaseline map[string]string `json:"command_baseline"`
	UIRunner        string            `json:"ui_runner"`
	// DefaultCommand is a Go-side addition (not in qa9.py): the
	// single project-level fallback the autotest discovery chain
	// consumes — the smoke baseline, else the unit baseline.
	DefaultCommand string `json:"default_command,omitempty"`
	// Assertions carries default_assertion_for_qa_type per type
	// (qa9.py exposes it to test-case generation; the Go face
	// surfaces it alongside the strategy).
	Assertions map[string]string `json:"assertions,omitempty"`
}

// BuildQAStrategy ports build_qa_strategy. generatedAt is
// injected for determinism in tests.
func BuildQAStrategy(root string, qaTypes []string, uiRunner, generatedAt string) QAStrategy {
	scan := InferProjectScan(root)
	projectType := InferProjectType(root, scan)
	hasWebUI := strings.ToLower(scan.Frontend) == "web"
	applicability := QAApplicability(projectType, hasWebUI)
	selected := ParseQATypes(qaTypes, "web-full")
	var required []string
	for _, t := range selected {
		if applicability[t] != "na" {
			required = append(required, t)
		}
	}
	if required == nil {
		required = []string{}
	}
	presets := CommandPresetsFromScan(root, scan, uiRunner)
	runner := strings.ToLower(strings.TrimSpace(uiRunner))
	if runner == "" {
		runner = "playwright"
	}
	functionalPrimary := "playwright"
	if projectType == "ai-agent-skill" || projectType == "web-app" {
		functionalPrimary = "browser-use"
	}
	baseline := map[string]string{}
	assertions := map[string]string{}
	for _, t := range QA9Types {
		baseline[t] = presets[t]
		assertions[t] = DefaultAssertionForQAType(t)
	}
	defaultCmd := baseline["smoke"]
	if defaultCmd == "" {
		defaultCmd = baseline["unit"]
	}
	return QAStrategy{
		SchemaVersion: "1.0.2",
		GeneratedAt:   generatedAt,
		ProjectType:   projectType,
		ProjectScan:   scan,
		Applicability: applicability,
		RequiredTypes: required,
		RunnerPolicy: map[string]any{
			"functional": map[string]any{
				"primary":  functionalPrimary,
				"fallback": []string{"playwright", "cypress", "agent-browser"},
			},
			"compatibility": map[string]any{
				"primary":  "playwright",
				"fallback": []string{"cypress"},
			},
			"api_layers": map[string]string{
				"interface":   "schemathesis/newman/pytest",
				"contract":    "schemathesis/pact",
				"integration": "pytest+testcontainers",
			},
		},
		CommandBaseline: baseline,
		UIRunner:        runner,
		DefaultCommand:  defaultCmd,
		Assertions:      assertions,
	}
}
