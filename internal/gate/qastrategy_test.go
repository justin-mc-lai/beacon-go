package gate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// M2: project-type inference from facts (qa9's matrix inputs).
func TestInferProjectTypeFromFacts(t *testing.T) {
	goRoot := t.TempDir()
	writeFixture(t, goRoot, "go.mod", "module example\n\ngo 1.22\n")
	writeFixture(t, goRoot, "main.go", "package main\n")
	if got := InferProjectType(goRoot, InferProjectScan(goRoot)); got != "sdk-lib" {
		t.Fatalf("bare go project = %q, want sdk-lib", got)
	}

	apiRoot := t.TempDir()
	writeFixture(t, apiRoot, "app.py", "print('x')\n")
	writeFixture(t, apiRoot, "openapi.yaml", "openapi: 3.0.0\n")
	scan := InferProjectScan(apiRoot)
	if len(scan.APISpecRefs) == 0 {
		t.Fatalf("openapi spec not detected: %+v", scan)
	}
	if got := InferProjectType(apiRoot, scan); got != "api-service" {
		t.Fatalf("python+openapi = %q, want api-service", got)
	}

	skillRoot := t.TempDir()
	writeFixture(t, skillRoot, "SKILL.md", "# skill\n")
	if got := InferProjectType(skillRoot, InferProjectScan(skillRoot)); got != "ai-agent-skill" {
		t.Fatalf("SKILL.md project = %q, want ai-agent-skill", got)
	}
}

// M2: the applicability matrix, verbatim rows.
func TestQAApplicabilityMatrix(t *testing.T) {
	api := QAApplicability("api-service", false)
	if api["functional"] != "na" || api["compatibility"] != "na" || api["contract"] != "required" {
		t.Fatalf("api-service row = %v", api)
	}
	web := QAApplicability("web-app", false)
	if web["functional"] != "required" || web["compatibility"] != "required" {
		t.Fatalf("web-app row = %v", web)
	}
	skill := QAApplicability("ai-agent-skill", false)
	if skill["functional"] != "conditional" {
		t.Fatalf("skill without UI: functional = %q, want conditional", skill["functional"])
	}
	skillUI := QAApplicability("ai-agent-skill", true)
	if skillUI["functional"] != "required" || skillUI["compatibility"] != "required" {
		t.Fatalf("skill with UI must promote both: %v", skillUI)
	}
}

// M2: strategy assembly — schema, required types dropping "na"
// rows, preset overrides from scan facts.
func TestBuildQAStrategyAssembly(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "app.py", "print('x')\n")
	writeFixture(t, root, "openapi.yaml", "openapi: 3.0.0\n")
	writeFixture(t, root, "api.postman_collection.json", "{}\n")
	s := BuildQAStrategy(root, nil, "playwright", "2026-10-23T00:00:00Z")
	if s.SchemaVersion != "1.0.2" {
		t.Fatalf("schema = %q", s.SchemaVersion)
	}
	if s.ProjectType != "api-service" {
		t.Fatalf("project type = %q", s.ProjectType)
	}
	for _, rt := range s.RequiredTypes {
		if rt == "functional" || rt == "compatibility" {
			t.Fatalf("na type %q must not be required: %v", rt, s.RequiredTypes)
		}
	}
	if !strings.HasPrefix(s.CommandBaseline["interface"], "npx newman run ") {
		t.Fatalf("postman preset must win for interface: %q", s.CommandBaseline["interface"])
	}
	if !strings.Contains(s.CommandBaseline["contract"], "schemathesis run openapi.yaml") {
		t.Fatalf("contract preset = %q", s.CommandBaseline["contract"])
	}
	if s.DefaultCommand == "" || s.DefaultCommand != s.CommandBaseline["smoke"] {
		t.Fatalf("default command = %q, want the smoke baseline", s.DefaultCommand)
	}
	if s.Assertions["unit"] != "All unit assertions pass" {
		t.Fatalf("unit assertion = %q", s.Assertions["unit"])
	}
}

// M2: legacy type names normalize; unknown types are dropped.
func TestParseQATypesNormalization(t *testing.T) {
	got := ParseQATypes([]string{"Unit", "e2e", "banana"}, "")
	joined := strings.Join(got, ",")
	if !strings.Contains(joined, "unit") {
		t.Fatalf("normalized types = %v", got)
	}
	if strings.Contains(joined, "banana") {
		t.Fatalf("unknown type must drop: %v", got)
	}
	def := ParseQATypes(nil, "web-full")
	if len(def) == 0 {
		t.Fatal("the web-full profile must select types")
	}
}
