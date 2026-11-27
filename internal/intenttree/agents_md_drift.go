package intenttree

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type agentsMDTarget struct {
	NodeID string `json:"node_id"`
	Field  string `json:"field"`
}

type agentsMDBinding struct {
	Item    int              `json:"item"`
	Targets []agentsMDTarget `json:"targets"`
	Gap     string           `json:"gap,omitempty"`
}

type agentsMDMap struct {
	Schema       string            `json:"schema"`
	Section      string            `json:"section"`
	Requirements []agentsMDBinding `json:"requirements"`
	Governance   []string          `json:"governance"`
}

type AgentsMDItem struct {
	Item    int              `json:"item"`
	Status  string           `json:"status"`
	Targets []agentsMDTarget `json:"targets"`
	Reason  string           `json:"reason,omitempty"`
}

// AgentsMDReport is a comparison result, never a verification or release verdict.
type AgentsMDReport struct {
	Schema    string         `json:"schema"`
	NodeID    string         `json:"node_id"`
	Evaluated int            `json:"evaluated"`
	Blocked   int            `json:"blocked"`
	Items     []AgentsMDItem `json:"items"`
	Changed   []DriftEntry   `json:"changed"`
}

// AgentsMDDrift compares numbered product rules with explicitly selected canonical
// text fields. Gap rows describe incomplete contracts; they cannot claim alignment.
// The mapping contains selectors and diagnostics only, never baseline prose/hashes.
func AgentsMDDrift(dir, nodeID, agentsPath, mapPath string) (AgentsMDReport, error) {
	bad := func(message string) (AgentsMDReport, error) {
		return AgentsMDReport{}, fmt.Errorf("agents_md_drift_invalid: %s", message)
	}
	if strings.TrimSpace(agentsPath) == "" || strings.TrimSpace(mapPath) == "" {
		return bad("AGENTS.md and mapping required")
	}
	raw, err := os.ReadFile(mapPath)
	if err != nil {
		return AgentsMDReport{}, err
	}
	if err := agentsMDUniqueKeys(json.NewDecoder(bytes.NewReader(raw))); err != nil {
		return bad(err.Error())
	}
	var mapping agentsMDMap
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&mapping); err != nil {
		return bad(err.Error())
	}
	if mapping.Schema != "beacon-agents-md-map-v2" || len(mapping.Requirements) == 0 || mapping.Section != "唯一真源（最高口径）" {
		return bad("known schema, product section and nonempty requirements required")
	}
	raw, err = os.ReadFile(agentsPath)
	if err != nil {
		return AgentsMDReport{}, err
	}
	sections, err := agentsMDSections(string(raw))
	if err != nil {
		return bad(err.Error())
	}
	product, ok := sections[mapping.Section]
	if !ok {
		return bad("missing product section")
	}
	items, err := agentsMDNumberedItems(product)
	if err != nil {
		return bad(err.Error())
	}
	classified := map[string]bool{mapping.Section: true}
	for _, heading := range mapping.Governance {
		if heading == "" || strings.TrimSpace(heading) != heading || classified[heading] {
			return bad("blank/duplicate governance section")
		}
		if _, ok := sections[heading]; !ok {
			return bad("missing governance section: " + heading)
		}
		classified[heading] = true
	}
	if len(classified) != len(sections) {
		return bad("every section must be classified")
	}
	idx, err := ReadIndex(dir)
	if err != nil {
		return AgentsMDReport{}, err
	}
	if findings := ValidateIndex(idx); len(findings) != 0 {
		return bad(strings.Join(findings, "; "))
	}
	entries := map[string]IndexEntry{}
	for _, e := range idx.Nodes {
		entries[e.ID] = e
	}
	if nodeID == "" {
		nodeID = idx.RootID
	}
	if _, exists := entries[nodeID]; !exists {
		return bad("unknown or noncanonical scope: " + nodeID)
	}
	scope := subtreeIDs(nodeID, childrenByIndex(idx.Nodes))
	rep := AgentsMDReport{Schema: "beacon-agents-md-drift-v2", NodeID: nodeID, Items: []AgentsMDItem{}, Changed: []DriftEntry{}}
	seen, changed := map[int]bool{}, map[string]bool{}
	for i, binding := range mapping.Requirements {
		if binding.Item != i+1 || seen[binding.Item] {
			return bad("items must be unique consecutive canonical numbers starting at 1")
		}
		seen[binding.Item] = true
		body, exists := items[binding.Item]
		if !exists {
			return bad(fmt.Sprintf("missing product item %d", binding.Item))
		}
		if len(binding.Targets) == 0 || (binding.Gap == "" && len(binding.Targets) != 1) {
			return bad("each comparison needs one target; gap rows need at least one context target")
		}
		if strings.TrimSpace(binding.Gap) != binding.Gap {
			return bad("noncanonical gap")
		}
		inScope, targets := false, map[string]bool{}
		var canonical string
		var selected Node
		for _, target := range binding.Targets {
			key := target.NodeID + "\x00" + target.Field
			if targets[key] {
				return bad("duplicate target")
			}
			targets[key] = true
			if _, ok := entries[target.NodeID]; !ok {
				return bad("unknown or noncanonical target: " + target.NodeID)
			}
			n, err := ReadNode(dir, target.NodeID)
			if err != nil {
				return AgentsMDReport{}, err
			}
			text, err := agentsMDField(n, target.Field)
			if err != nil {
				return bad(err.Error())
			}
			if strings.TrimSpace(text) == "" {
				return bad("empty canonical requirement field")
			}
			canonical, selected = text, n
			inScope = inScope || scope[target.NodeID]
		}
		if !inScope {
			continue
		}
		item := AgentsMDItem{Item: binding.Item, Targets: binding.Targets, Status: "aligned"}
		if binding.Gap != "" {
			item.Status, item.Reason = "blocked", binding.Gap
			rep.Blocked++
		} else {
			rep.Evaluated++
			if agentsMDText(body) != agentsMDText(canonical) {
				item.Status = "changed"
				if !changed[selected.ID] {
					rep.Changed = append(rep.Changed, driftEntry(selected))
					changed[selected.ID] = true
				}
			}
		}
		rep.Items = append(rep.Items, item)
	}
	if len(seen) != len(items) {
		return bad("every product item must be mapped")
	}
	if rep.Evaluated+rep.Blocked == 0 {
		return bad("zero mapped requirements in selected scope")
	}
	sortDrift(rep.Changed)
	return rep, nil
}

func agentsMDField(n Node, field string) (string, error) {
	switch field {
	case "/intent":
		return n.Intent, nil
	case "/title":
		return n.Title, nil
	case "/doc":
		return n.Doc, nil
	}
	parts := strings.Split(field, "/")
	if len(parts) == 4 && parts[0] == "" && parts[1] == "acs" && parts[3] == "text" && strings.TrimSpace(parts[2]) == parts[2] && parts[2] != "" {
		var found []string
		for _, ac := range n.ACs {
			if ac.ID == parts[2] {
				found = append(found, ac.Text)
			}
		}
		if len(found) == 1 {
			return found[0], nil
		}
	}
	return "", fmt.Errorf("unknown/ambiguous canonical text selector: %s %s", n.ID, field)
}

func agentsMDText(s string) string { return strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n")) }

var agentsMDItemStart = regexp.MustCompile(`^([0-9]+)\. (.*)$`)

func agentsMDNumberedItems(text string) (map[int]string, error) {
	items := map[int]string{}
	current := 0
	for _, line := range strings.Split(text, "\n") {
		if match := agentsMDItemStart.FindStringSubmatch(line); match != nil {
			n, err := strconv.Atoi(match[1])
			if err != nil || n != len(items)+1 || strconv.Itoa(n) != match[1] {
				return nil, fmt.Errorf("duplicate/noncanonical product item number")
			}
			current = n
			items[n] = match[2]
		} else if current != 0 {
			items[current] += "\n" + line
		}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no numbered product requirements")
	}
	for _, body := range items {
		if agentsMDText(body) == "" {
			return nil, fmt.Errorf("blank product requirement")
		}
	}
	return items, nil
}

func agentsMDSections(text string) (map[string]string, error) {
	sections := map[string]string{}
	current, fence := "", ""
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
			marker := trim[:3]
			if fence == "" {
				fence = marker
			} else if fence == marker {
				fence = ""
			}
		}
		if fence == "" && strings.HasPrefix(line, "## ") {
			current = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if _, exists := sections[current]; exists || current == "" {
				return nil, fmt.Errorf("blank/duplicate section: %s", current)
			}
			sections[current] = ""
		} else if current != "" {
			sections[current] += line + "\n"
		}
	}
	if fence != "" || len(sections) == 0 {
		return nil, fmt.Errorf("unclosed fence or no sections")
	}
	return sections, nil
}

// encoding/json accepts duplicate object keys; mapping ambiguity must fail closed.
func agentsMDUniqueKeys(dec *json.Decoder) error {
	var value func() error
	value = func() error {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for dec.More() {
				key, err := dec.Token()
				if err != nil {
					return err
				}
				s, ok := key.(string)
				if !ok || seen[s] {
					return fmt.Errorf("duplicate/invalid JSON key: %v", key)
				}
				seen[s] = true
				if err := value(); err != nil {
					return err
				}
			}
		case '[':
			for dec.More() {
				if err := value(); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unexpected delimiter")
		}
		_, err = dec.Token()
		return err
	}
	if err := value(); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}
