package gate

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// TaskNode is one executable row in the feature's tasks.md contract.
type TaskNode struct {
	ID       string   `json:"id"`
	Action   string   `json:"action"`
	Depends  []string `json:"depends"`
	ACs      []string `json:"acs"`
	Command  string   `json:"command"`
	Verify   string   `json:"verify"`
	Evidence string   `json:"evidence"`
	Auto     bool     `json:"auto"`
}

// TaskDAG keeps nodes in deterministic, lexicographically tied topological order.
type TaskDAG struct {
	Nodes []TaskNode `json:"nodes"`
}

var taskIdentity = regexp.MustCompile(`^TASK-[A-Za-z0-9][A-Za-z0-9_-]*$`)

// ParseTaskDAG accepts only the executable TASK table, never checkbox claims.
// Dependencies and AC lists are comma/space separated; empty or '-' means no dependencies.
func ParseTaskDAG(markdown string, definedACs map[string]bool) (TaskDAG, error) {
	header := []string{"TASK", "Action", "Depends", "AC", "Command", "Verify", "Evidence", "Auto"}
	byID := map[string]TaskNode{}
	inTable, found := false, false
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			if line != "" {
				inTable = false
			}
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		isHeader := len(cells) == len(header)
		if isHeader {
			for i := range header {
				if cells[i] != header[i] {
					isHeader = false
					break
				}
			}
		}
		if isHeader {
			if found {
				return TaskDAG{}, fmt.Errorf("task_dag_invalid: duplicate TASK table")
			}
			found, inTable = true, true
			continue
		}
		if !inTable {
			continue
		}
		separator := len(cells) == len(header)
		for _, cell := range cells {
			if strings.Trim(cell, "-:") != "" || cell == "" {
				separator = false
			}
		}
		if separator {
			continue
		}
		if len(cells) != len(header) {
			return TaskDAG{}, fmt.Errorf("task_dag_invalid: expected eight columns")
		}
		for i := range cells {
			cells[i] = strings.Trim(cells[i], "`")
		}
		n := TaskNode{ID: cells[0], Action: cells[1], Command: cells[4], Verify: cells[5], Evidence: cells[6]}
		if !taskIdentity.MatchString(n.ID) {
			return TaskDAG{}, fmt.Errorf("task_dag_invalid: invalid TASK ID %q", n.ID)
		}
		if _, exists := byID[n.ID]; exists {
			return TaskDAG{}, fmt.Errorf("task_dag_invalid: duplicate %s", n.ID)
		}
		for _, cell := range []string{n.Action, n.Command, n.Verify, n.Evidence} {
			if cell == "" || cell == "-" {
				return TaskDAG{}, fmt.Errorf("task_dag_invalid: %s needs action, command, verify and evidence", n.ID)
			}
		}
		if cells[2] != "" && cells[2] != "-" {
			n.Depends = taskList(cells[2])
		}
		n.ACs = taskList(cells[3])
		if len(n.ACs) == 0 {
			return TaskDAG{}, fmt.Errorf("task_dag_invalid: %s needs an AC", n.ID)
		}
		for _, ac := range n.ACs {
			if !definedACs[ac] {
				return TaskDAG{}, fmt.Errorf("task_dag_invalid: %s references unknown AC %s", n.ID, ac)
			}
		}
		switch cells[7] {
		case "true":
			n.Auto = true
		case "false":
			n.Auto = false
		default:
			return TaskDAG{}, fmt.Errorf("task_dag_invalid: %s Auto must be true or false", n.ID)
		}
		byID[n.ID] = n
	}
	if !found || len(byID) == 0 {
		return TaskDAG{}, fmt.Errorf("task_dag_invalid: executable TASK table missing or empty")
	}
	indegree := make(map[string]int, len(byID))
	children := make(map[string][]string, len(byID))
	ready := make([]string, 0, len(byID))
	for id, n := range byID {
		seen := map[string]bool{}
		for _, dep := range n.Depends {
			if _, ok := byID[dep]; !ok {
				return TaskDAG{}, fmt.Errorf("task_dag_invalid: %s has unknown dependency %s", id, dep)
			}
			if seen[dep] {
				return TaskDAG{}, fmt.Errorf("task_dag_invalid: %s repeats dependency %s", id, dep)
			}
			seen[dep] = true
			children[dep] = append(children[dep], id)
			indegree[id]++
		}
		if indegree[id] == 0 {
			ready = append(ready, id)
		}
	}
	out := TaskDAG{Nodes: make([]TaskNode, 0, len(byID))}
	for len(ready) > 0 {
		sort.Strings(ready)
		id := ready[0]
		ready = ready[1:]
		out.Nodes = append(out.Nodes, byID[id])
		for _, child := range children[id] {
			indegree[child]--
			if indegree[child] == 0 {
				ready = append(ready, child)
			}
		}
	}
	if len(out.Nodes) != len(byID) {
		return TaskDAG{}, fmt.Errorf("task_dag_invalid: dependency cycle")
	}
	return out, nil
}

func taskList(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
}
