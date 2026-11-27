package portabledrift

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var portableBinary string
var portableRoot string

func TestMain(tests *testing.M) {
	portableRoot, _ = filepath.Abs(filepath.Join("..", ".."))
	directory, err := os.MkdirTemp("", "beacon-portable-cli-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	portableBinary = filepath.Join(directory, "beacon")
	command := exec.Command("go", "build", "-o", portableBinary, "./cmd/beacon")
	command.Dir = portableRoot
	if output, err := command.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "CLI build precondition failed: %v\n%s", err, output)
		os.RemoveAll(directory)
		os.Exit(2)
	}
	result := tests.Run()
	os.RemoveAll(directory)
	os.Exit(result)
}

func portableCommand(t *testing.T, project, ledger string, arguments ...string) map[string]any {
	t.Helper()
	command := exec.Command(portableBinary, arguments...)
	command.Dir = project
	command.Env = append(os.Environ(), "BEACON_STORE_PATH="+ledger)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("public CLI behavior failed: %v: %v\n%s", arguments, err, output)
	}
	var document map[string]any
	if err := json.Unmarshal(output, &document); err != nil {
		t.Fatalf("public CLI JSON missing: %v\n%s", err, output)
	}
	return document
}

func portableFixture(t *testing.T) (string, string, string) {
	t.Helper()
	project := t.TempDir()
	ledger := filepath.Join(t.TempDir(), "fixture.db")
	for _, relative := range []string{"AGENTS.md", "docs/beacon/v0.0.1/tree"} {
		source := filepath.Join(portableRoot, relative)
		if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkError error) error {
			if walkError != nil {
				return walkError
			}
			relativePath, _ := filepath.Rel(portableRoot, path)
			target := filepath.Join(project, relativePath)
			if entry.IsDir() {
				return os.MkdirAll(target, 0700)
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, content, 0600)
		}); err != nil {
			t.Fatal(err)
		}
	}
	agentsRaw, err := os.ReadFile(filepath.Join(project, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	product := strings.SplitN(strings.SplitN(string(agentsRaw), "## 唯一真源（最高口径）\n", 2)[1], "\n## ", 2)[0]
	pattern := regexp.MustCompile(`(?m)^([1-9])\. (.+)$`)
	matches := pattern.FindAllStringSubmatch(product, -1)
	if len(matches) != 9 {
		t.Fatalf("expected nine explicit product rules, got %d", len(matches))
	}
	criteria := []map[string]string{}
	bindings := []map[string]any{}
	for _, match := range matches {
		identifier := "NORM-00" + match[1]
		criteria = append(criteria, map[string]string{"id": identifier, "text": match[2]})
		bindings = append(bindings, map[string]any{"item": int(match[1][0] - '0'), "targets": []map[string]string{{"node_id": "n_ca6fe1322405", "field": "/acs/" + identifier + "/text"}}})
	}
	write := func(path string, value any) {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(project, "feature-patch.json"), map[string]any{"acs": criteria})
	write(filepath.Join(project, "docs/agents-md-drift-map.json"), map[string]any{
		"schema": "beacon-agents-md-map-v2", "section": "唯一真源（最高口径）", "requirements": bindings,
		"governance": []string{"目录与入口", "授权边界", "验证命令", "工程纪律"},
	})
	featureRaw, err := os.ReadFile(filepath.Join(project, "docs/beacon/v0.0.1/tree/nodes/n_ca6fe1322405.json"))
	if err != nil {
		t.Fatal(err)
	}
	var feature struct {
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.Unmarshal(featureRaw, &feature); err != nil || feature.Fingerprint == "" {
		t.Fatalf("canonical feature fingerprint missing: %v", err)
	}
	portableCommand(t, project, ledger, "projects", "register", "--path", project, "--version", "v0.0.1", "--json")
	portableCommand(t, project, ledger, "doctor", "rebuild-tree", "--project", project, "--version", "v0.0.1", "--json")
	portableCommand(t, project, ledger, "change", "request", "--project", project, "--node", "n_ca6fe1322405", "--summary", "isolated nine-rule contract fixture", "--by", "portable-fixture", "--base-fingerprint", feature.Fingerprint, "--json")
	portableCommand(t, project, ledger, "truth", "node", "update", "--project", project, "--node", "n_ca6fe1322405", "--file", filepath.Join(project, "feature-patch.json"), "--json")
	return project, ledger, string(agentsRaw)
}

func TestPortableAgentsMDGovernance(t *testing.T) {
	project, ledger, agents := portableFixture(t)
	for _, text := range []string{agents, agents + "\n治理过程增加一条说明。\n"} {
		if err := os.WriteFile(filepath.Join(project, "AGENTS.md"), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		report := portableCommand(t, project, ledger, "plan", "drift", "--project", project, "--agents-md", "--json")
		if report["evaluated"] != float64(9) || report["blocked"] != float64(0) || len(report["changed"].([]any)) != 0 {
			t.Fatalf("nine-rule alignment or governance exclusion failed: %v", report)
		}
	}
}

func TestPortableAgentsMDReverse(t *testing.T) {
	project, ledger, agents := portableFixture(t)
	reversed := strings.Replace(agents, "唯一一等能力集合", "允许 design 作为第八个一等能力", 1)
	if reversed == agents {
		t.Fatal("reverse mutation did not alter the real second product rule")
	}
	if err := os.WriteFile(filepath.Join(project, "AGENTS.md"), []byte(reversed), 0600); err != nil {
		t.Fatal(err)
	}
	report := portableCommand(t, project, ledger, "plan", "drift", "--project", project, "--agents-md", "--json")
	if report["evaluated"] != float64(9) || report["blocked"] != float64(0) || len(report["changed"].([]any)) != 1 {
		t.Fatalf("real reverse rule must be changed: %v", report)
	}
	items := report["items"].([]any)
	for index, raw := range items {
		item := raw.(map[string]any)
		wanted := "aligned"
		if index == 1 {
			wanted = "changed"
		}
		if item["status"] != wanted {
			t.Fatalf("rule %d: want %s, got %v", index+1, wanted, item)
		}
	}
}
