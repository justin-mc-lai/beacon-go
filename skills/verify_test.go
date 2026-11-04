// skills-distribution: manifest + drift verification. The expected
// set is the embedded 1+6 surface; v1's beacon-* names are outside
// the namespace and must never be counted.
package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func containsName(list []string, name string) bool {
	for _, s := range list {
		if s == name {
			return true
		}
	}
	return false
}

func TestPluginManifestLists1Plus6(t *testing.T) {
	pluginPath := filepath.Join("..", ".claude-plugin", "plugin.json")
	raw, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatalf("plugin.json: %v", err)
	}
	var plugin struct {
		Name   string   `json:"name"`
		Skills []string `json:"skills"`
	}
	if err := json.Unmarshal(raw, &plugin); err != nil {
		t.Fatalf("plugin.json parse: %v", err)
	}
	if plugin.Name != "beacon-go" {
		t.Fatalf("plugin name = %q", plugin.Name)
	}
	want := make([]string, 0, len(IDs()))
	for _, id := range IDs() {
		want = append(want, "./skills/"+id)
	}
	if len(plugin.Skills) != len(want) {
		t.Fatalf("plugin skills = %v want %v", plugin.Skills, want)
	}
	for i := range want {
		if plugin.Skills[i] != want[i] {
			t.Fatalf("plugin skills = %v want %v", plugin.Skills, want)
		}
	}

	marketPath := filepath.Join("..", ".claude-plugin", "marketplace.json")
	raw, err = os.ReadFile(marketPath)
	if err != nil {
		t.Fatalf("marketplace.json: %v", err)
	}
	var market struct {
		Name    string `json:"name"`
		Plugins []struct {
			Name string `json:"name"`
		} `json:"plugins"`
	}
	if err := json.Unmarshal(raw, &market); err != nil {
		t.Fatalf("marketplace.json parse: %v", err)
	}
	if market.Name != "beacon-go" || len(market.Plugins) != 1 || market.Plugins[0].Name != "beacon-go" {
		t.Fatalf("marketplace = %+v", market)
	}
}

func claudeHost(t *testing.T, home string) Host {
	t.Helper()
	hosts, err := HostDirs(home)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hosts {
		if h.Name == "claude" {
			return h
		}
	}
	t.Fatal("claude host missing from the table")
	return Host{}
}

func TestVerifyHostReportsMissing(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "skills")
	for _, name := range []string{"beacon-go", "beacon-go-goal"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	v := VerifyHost(claudeHost(t, home))
	if v.OK {
		t.Fatal("partial install must drift")
	}
	if !containsName(v.Missing, "beacon-go-plan") {
		t.Fatalf("missing must name beacon-go-plan, got %v", v.Missing)
	}
	if containsName(v.Missing, "beacon-go") || containsName(v.Missing, "beacon-go-goal") {
		t.Fatalf("installed names must not be missing, got %v", v.Missing)
	}
}

func TestVerifyIgnoresV1Names(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(filepath.Join(dir, "beacon-goal"), 0o755); err != nil {
		t.Fatal(err)
	}
	h := claudeHost(t, home)
	if containsName(h.Installed, "beacon-goal") {
		t.Fatalf("v1 name leaked into the Go namespace: %v", h.Installed)
	}
	v := VerifyHost(h)
	if containsName(v.Extra, "beacon-goal") {
		t.Fatalf("v1 name counted as extra: %v", v.Extra)
	}
}
