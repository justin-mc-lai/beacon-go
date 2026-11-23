package gate

import (
	"reflect"
	"strings"
	"testing"
)

const taskHeaderFixture = "| TASK | Action | Depends | AC | Command | Verify | Evidence | Auto |\n|---|---|---|---|---|---|---|---|\n"

func TestTaskDAGValidationAndOrder(t *testing.T) {
	row := func(id, dep, ac, command, verify, evidence, auto string) string {
		return "|" + id + "|work|" + dep + "|" + ac + "|" + command + "|" + verify + "|" + evidence + "|" + auto + "|\n"
	}
	valid := row("TASK-C", "TASK-A, TASK-B", "AC-001", "tool do", "tool check", "result.txt", "true") +
		row("TASK-B", "-", "AC-001", "tool do", "tool check", "result.txt", "true") +
		row("TASK-A", "", "AC-001", "tool do", "tool check", "result.txt", "false")
	dag, err := ParseTaskDAG(taskHeaderFixture+valid, map[string]bool{"AC-001": true})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, n := range dag.Nodes {
		ids = append(ids, n.ID)
	}
	if !reflect.DeepEqual(ids, []string{"TASK-A", "TASK-B", "TASK-C"}) {
		t.Fatalf("order = %v", ids)
	}
	cases := map[string]string{
		"duplicate":           valid + row("TASK-A", "-", "AC-001", "tool", "tool", "out", "true"),
		"unknown dependency":  row("TASK-A", "TASK-X", "AC-001", "tool", "tool", "out", "true"),
		"unknown AC":          row("TASK-A", "-", "AC-999", "tool", "tool", "out", "true"),
		"cycle":               row("TASK-A", "TASK-B", "AC-001", "tool", "tool", "out", "true") + row("TASK-B", "TASK-A", "AC-001", "tool", "tool", "out", "true"),
		"self cycle":          row("TASK-A", "TASK-A", "AC-001", "tool", "tool", "out", "true"),
		"missing command":     row("TASK-A", "-", "AC-001", "", "tool", "out", "true"),
		"missing verify":      row("TASK-A", "-", "AC-001", "tool", "-", "out", "true"),
		"missing evidence":    row("TASK-A", "-", "AC-001", "tool", "tool", "", "true"),
		"invalid auto":        row("TASK-A", "-", "AC-001", "tool", "tool", "out", "yes"),
		"invalid ID":          row("../TASK-A", "-", "AC-001", "tool", "tool", "out", "true"),
		"repeated dependency": row("TASK-A", "-", "AC-001", "tool", "tool", "out", "true") + row("TASK-B", "TASK-A TASK-A", "AC-001", "tool", "tool", "out", "true"),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseTaskDAG(taskHeaderFixture+body, map[string]bool{"AC-001": true}); err == nil {
				t.Fatal("invalid graph accepted")
			}
		})
	}
	if _, err := ParseTaskDAG(strings.Replace(taskHeaderFixture, "Depends", "Dependencies", 1)+valid, map[string]bool{"AC-001": true}); err == nil {
		t.Fatal("non-contract header accepted")
	}
	if _, err := ParseTaskDAG("- [x] TASK-A done ac=AC-001", map[string]bool{"AC-001": true}); err == nil {
		t.Fatal("checkbox accepted as executable graph")
	}
}
