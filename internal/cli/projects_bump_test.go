package cli

import "testing"

func TestProjectsBumpPatch(t *testing.T) {
	root, _ := seedGateProject(t, true) // registers v2.0.0 as current
	out, err := runCLI(t, "projects", "bump", "--project", root, "--level", "patch", "--json")
	if err != nil {
		t.Fatalf("bump: %v\n%s", err, out)
	}
	doc := parseDoc(t, out)
	if strField(t, doc, "version") != "v2.0.1" {
		t.Fatalf("bumped version = %v", doc["version"])
	}
	proj, _ := doc["project"].(map[string]any)
	if proj == nil || proj["current_version"] != "v2.0.1" {
		t.Fatalf("project view = %v", doc["project"])
	}
}
