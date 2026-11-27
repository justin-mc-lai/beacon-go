package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	buildversion "beacon-go/internal/version"
	"beacon-go/internal/versionmanifest"
)

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
	manifestPath := strField(t, doc, "manifest")
	m, err := versionmanifest.Load(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := versionmanifest.Validate(m); err != nil {
		t.Fatal(err)
	}
	if m.Version != "v2.0.1" || m.Runtime != buildversion.Version || m.Status != versionmanifest.StatusDraft || m.Parent != "" || len(m.Lakes) != 0 {
		t.Fatalf("manifest = %+v", m)
	}
	if _, err := os.Stat(versionmanifest.Path(root, "v2.0.0")); !os.IsNotExist(err) {
		t.Fatalf("bump should not materialize a parent: %v", err)
	}
	if _, err := runCLI(t, "projects", "bump", "--path", root, "--json"); err != nil {
		t.Fatal(err)
	}
	child, err := versionmanifest.LoadVersion(root, "v2.0.2")
	if err != nil || child.Parent != m.Version {
		t.Fatalf("next bump did not inherit: %+v, %v", child, err)
	}
}

func TestProjectsBumpRefusesInvalidManifest(t *testing.T) {
	for _, probe := range []struct {
		name, version, body string
	}{
		{"malformed-parent", "v2.0.0", "{"},
		{"invalid-parent", "v2.0.0", `{"schema":"wrong","version":"v2.0.0","status":"draft"}`},
		{"wrong-parent-version", "v2.0.0", `{"schema":"beacon-version-manifest.v1","version":"v9.9.9","status":"draft"}`},
		{"existing-target", "v2.0.1", "do not overwrite"},
	} {
		t.Run(probe.name, func(t *testing.T) {
			root, _ := seedGateProject(t, true)
			path := versionmanifest.Path(root, probe.version)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(probe.body), 0o644); err != nil {
				t.Fatal(err)
			}
			out, err := runCLI(t, "projects", "bump", "--path", root, "--json")
			if err == nil {
				t.Fatalf("accepted %s: %s", probe.name, out)
			}
			body, err := os.ReadFile(path)
			if err != nil || string(body) != probe.body {
				t.Fatalf("manifest changed: %q, %v", body, err)
			}
			out, err = runCLI(t, "projects", "show", "--project", root, "--json")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, `"current_version": "v2.0.0"`) {
				t.Fatalf("refusal advanced current version: %s", out)
			}
		})
	}
}

func TestProjectsBumpInitialAndInherited(t *testing.T) {
	for _, probe := range []struct {
		level, want string
	}{
		{"patch", "v0.0.1"},
		{"minor", "v0.1.0"},
		{"major", "v1.0.0"},
	} {
		t.Run(probe.level, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("BEACON_STORE_PATH", filepath.Join(t.TempDir(), "beacon.db"))
			if _, err := runCLI(t, "projects", "register", "--project", root, "--json"); err != nil {
				t.Fatal(err)
			}
			out, err := runCLI(t, "projects", "bump", "--project", filepath.Join(root, "."), "--level", probe.level, "--json")
			if err != nil {
				t.Fatal(err)
			}
			if strField(t, parseDoc(t, out), "version") != probe.want {
				t.Fatalf("initial bump: %s", out)
			}
			m, err := versionmanifest.LoadVersion(root, probe.want)
			if err != nil || versionmanifest.Validate(m) != nil || m.Parent != "" || m.Status != versionmanifest.StatusDraft || m.Runtime != buildversion.Version {
				t.Fatalf("initial manifest: %+v, %v", m, err)
			}
			m.Lakes = []versionmanifest.Lake{{Slug: "original", Package: "contract-hash", Status: versionmanifest.LakeBaseline}}
			if err := versionmanifest.Save(versionmanifest.Path(root, probe.want), m); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(versionmanifest.Path(root, probe.want))
			if err != nil {
				t.Fatal(err)
			}
			out, err = runCLI(t, "projects", "bump", "--path", root, "--json")
			if err != nil {
				t.Fatal(err)
			}
			child, err := versionmanifest.Load(strField(t, parseDoc(t, out), "manifest"))
			if err != nil || child.Parent != m.Version || len(child.Lakes) != 1 || child.Lakes[0].Package != "contract-hash" || child.Lakes[0].Status != versionmanifest.LakeInherited {
				t.Fatalf("inherited manifest: %+v, %v", child, err)
			}
			after, err := os.ReadFile(versionmanifest.Path(root, probe.want))
			if err != nil || string(before) != string(after) {
				t.Fatalf("parent rewritten: %v", err)
			}
		})
	}
}

func TestProjectsBumpRefusesFilesystemFailures(t *testing.T) {
	for _, probe := range []string{"dangling-parent", "dangling-target", "blocked-temporary-file", "invalid-level"} {
		t.Run(probe, func(t *testing.T) {
			root, _ := seedGateProject(t, true)
			level := "patch"
			path := versionmanifest.Path(root, "v2.0.1")
			if probe == "dangling-parent" {
				path = versionmanifest.Path(root, "v2.0.0")
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			switch probe {
			case "dangling-parent", "dangling-target":
				if err := os.Symlink(filepath.Join(root, "absent.json"), path); err != nil {
					t.Fatal(err)
				}
			case "blocked-temporary-file":
				if err := os.Mkdir(path+".tmp", 0o755); err != nil {
					t.Fatal(err)
				}
			case "invalid-level":
				level = "banana"
			}
			if out, err := runCLI(t, "projects", "bump", "--project", root, "--level", level, "--json"); err == nil {
				t.Fatalf("accepted %s: %s", probe, out)
			}
			out, err := runCLI(t, "projects", "show", "--project", root, "--json")
			if err != nil || strField(t, parseDoc(t, out), "current_version") != "v2.0.0" {
				t.Fatalf("refusal advanced current: %s, %v", out, err)
			}
		})
	}
}
