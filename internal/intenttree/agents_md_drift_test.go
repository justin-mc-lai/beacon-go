package intenttree

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentsMDNumberedComparison(t *testing.T) {
	dir := t.TempDir()
	writeDriftTree(t, dir)
	agents, mapping := filepath.Join(dir, "AGENTS.md"), filepath.Join(dir, "map.json")
	write := func(path, text string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	base := `{"schema":"beacon-agents-md-map-v2","section":"唯一真源（最高口径）","requirements":[{"item":1,"targets":[{"node_id":"child","field":"/intent"}]}],"governance":["协作"]}`
	document := "## 唯一真源（最高口径）\n1. child intent\n\n## 协作\nold process\n"
	write(mapping, base)
	write(agents, document)
	check := func(want string) AgentsMDReport {
		t.Helper()
		rep, err := AgentsMDDrift(dir, "root", agents, mapping)
		if err != nil || len(rep.Items) != 1 || rep.Items[0].Status != want {
			t.Fatalf("%s: %+v %v", want, rep, err)
		}
		return rep
	}
	if rep := check("aligned"); rep.Evaluated != 1 || rep.Blocked != 0 || len(rep.Changed) != 0 {
		t.Fatalf("clean: %+v", rep)
	}
	write(agents, strings.Replace(document, "old process", "new process", 1))
	check("aligned")
	write(agents, strings.Replace(document, "1. child intent", "1. not child intent", 1))
	if rep := check("changed"); len(rep.Changed) != 1 || rep.Changed[0].ID != "child" {
		t.Fatalf("reverse: %+v", rep)
	}
	alias, err := AgentsMDDrift(filepath.Join(dir, "."), "child", filepath.Join(dir, ".", "AGENTS.md"), mapping)
	if err != nil || len(alias.Changed) != 1 {
		t.Fatalf("aliases: %+v %v", alias, err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(cwd, dir)
	if err != nil {
		t.Fatal(err)
	}
	{
		alias, err = AgentsMDDrift(rel, "child", filepath.Join(rel, "AGENTS.md"), filepath.Join(rel, "map.json"))
		if err != nil || len(alias.Changed) != 1 {
			t.Fatalf("relative aliases: %+v %v", alias, err)
		}
	}
	write(agents, document)
	n, err := ReadNode(dir, "child")
	if err != nil {
		t.Fatal(err)
	}
	n.Status = StatusImplemented
	n, err = Stamp(n)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteNode(dir, n); err != nil {
		t.Fatal(err)
	}
	check("aligned")
	n.Intent = "not child intent"
	n, err = Stamp(n)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteNode(dir, n); err != nil {
		t.Fatal(err)
	}
	check("changed")
	write(mapping, strings.Replace(base, `"targets":`, `"gap":"contract missing","targets":`, 1))
	if rep := check("blocked"); rep.Evaluated != 0 || rep.Blocked != 1 || len(rep.Changed) != 0 {
		t.Fatalf("gap must not compare incomplete text: %+v", rep)
	}

	for name, text := range map[string]string{
		"empty set":          strings.Replace(base, `[{"item":1,"targets":[{"node_id":"child","field":"/intent"}]}]`, `[]`, 1),
		"empty targets":      strings.Replace(base, `[{"node_id":"child","field":"/intent"}]`, `[]`, 1),
		"blank field":        strings.Replace(base, `"/intent"`, `" "`, 1),
		"blank node":         strings.Replace(base, `"child"`, `" "`, 1),
		"path node alias":    strings.Replace(base, `"child"`, `"./child"`, 1),
		"duplicate key":      strings.Replace(base, `"item":1`, `"item":2,"item":1`, 1),
		"duplicate item":     strings.Replace(base, `}],"governance"`, `},{"item":1,"targets":[{"node_id":"child","field":"/intent"}]}],"governance"`, 1),
		"unknown field":      strings.Replace(base, `"/intent"`, `"/status"`, 1),
		"unknown node":       strings.Replace(base, `"child"`, `"missing"`, 1),
		"unclassified":       strings.Replace(base, `["协作"]`, `[]`, 1),
		"product governance": strings.Replace(base, `["协作"]`, `["协作","唯一真源（最高口径）"]`, 1),
		"unknown property":   strings.Replace(base, `"item":1`, `"item":1,"body":"shadow truth"`, 1),
		"trailing JSON":      base + `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			write(mapping, text)
			if _, err := AgentsMDDrift(dir, "root", agents, mapping); err == nil {
				t.Fatal("invalid mapping accepted")
			}
		})
	}
	write(mapping, base)
	for name, text := range map[string]string{
		"empty document":    " ",
		"whole H2":          "## 唯一真源（最高口径）\nchild intent\n## 协作\nx",
		"blank body":        strings.Replace(document, "child intent", " ", 1),
		"duplicate item":    strings.Replace(document, "1. child intent", "1. child intent\n1. other", 1),
		"item alias":        strings.Replace(document, "1. child intent", "01. child intent", 1),
		"extra item":        strings.Replace(document, "1. child intent", "1. child intent\n2. unmapped", 1),
		"duplicate heading": document + "\n## 协作\nx",
		"new section":       document + "\n## New\nx",
		"unclosed fence":    document + "\n```\n",
	} {
		t.Run(name, func(t *testing.T) {
			write(agents, text)
			if _, err := AgentsMDDrift(dir, "root", agents, mapping); err == nil {
				t.Fatal("incomplete source accepted")
			}
		})
	}
	write(agents, document)
	if _, err := AgentsMDDrift(dir, "root/child", agents, mapping); err == nil {
		t.Fatal("path scope alias accepted")
	}
	if _, err := AgentsMDDrift(dir, "absent", agents, mapping); err == nil {
		t.Fatal("unknown scope accepted")
	}
	write(mapping, strings.Replace(base, `"child"`, `"root"`, 1))
	if _, err := AgentsMDDrift(dir, "child", agents, mapping); err == nil {
		t.Fatal("zero evaluations accepted")
	}
}

func TestAgentsMDCanonicalFieldsAndWhitespace(t *testing.T) {
	n := Node{ID: "n", ACs: []AC{{ID: "AC-001", Text: "x"}}}
	if got, err := agentsMDField(n, "/acs/AC-001/text"); err != nil || got != "x" {
		t.Fatalf("field: %q %v", got, err)
	}
	n.ACs = append(n.ACs, n.ACs[0])
	if _, err := agentsMDField(n, "/acs/AC-001/text"); err == nil {
		t.Fatal("duplicate AC key accepted")
	}
	if agentsMDText(" \r\nx\r\n ") != "x" || agentsMDText("x  y") == agentsMDText("x y") {
		t.Fatal("whitespace normalization changed contract")
	}
	sections, err := agentsMDSections("## A\n```md\n## B\n```\n")
	if err != nil || len(sections) != 1 {
		t.Fatalf("fenced heading: %v %v", sections, err)
	}
}

func TestAgentsMDRepositoryMappingIsAligned(t *testing.T) {
	project := filepath.Join("..", "..")
	rep, err := AgentsMDDrift(filepath.Join(project, "docs", "beacon", "v0.0.1", "tree"), "", filepath.Join(project, "AGENTS.md"), filepath.Join(project, "docs", "agents-md-drift-map.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Items) != 9 || rep.Blocked != 0 || rep.Evaluated != 9 || len(rep.Changed) != 0 {
		t.Fatalf("nine canonical contracts must align: %+v", rep)
	}
	for i, item := range rep.Items {
		if item.Item != i+1 || item.Status != "aligned" || len(item.Targets) != 1 {
			t.Fatalf("missing canonical alignment: %+v", item)
		}
	}
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	raw, err := os.ReadFile(filepath.Join(project, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	for index, text := range []string{strings.Replace(string(raw), "## 工程纪律", "## 工程纪律\nchanged workflow", 1), strings.Replace(string(raw), "**worktree 是核心能力**", "**worktree 不是核心能力**", 1)} {
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := AgentsMDDrift(filepath.Join(project, "docs", "beacon", "v0.0.1", "tree"), "", path, filepath.Join(project, "docs", "agents-md-drift-map.json"))
		if err != nil {
			t.Fatal(err)
		}
		if index == 0 {
			before, _ := json.Marshal(rep)
			after, _ := json.Marshal(got)
			if string(before) != string(after) {
				t.Fatal("governance became requirement drift")
			}
		} else if got.Evaluated != 9 || got.Blocked != 0 || len(got.Changed) != 1 || got.Items[2].Status != "changed" {
			t.Fatalf("real third-rule reversal not located: %+v", got)
		}
	}
}
