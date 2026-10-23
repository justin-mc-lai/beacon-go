// Package skills embeds the public ability skills into the binary
// ("edit as files, ship as binary"). Progressive disclosure lives one
// level down: the SKILL.md is the always-loaded surface; references
// are host-side files loaded on intent match.
package skills

import (
	"embed"
	"strings"
)

//go:embed goal/SKILL.md truth/SKILL.md design/SKILL.md implement/SKILL.md qa/SKILL.md release/SKILL.md
var files embed.FS

// Skill is one embedded public skill.
type Skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body,omitempty"`
}

var order = []string{"goal", "truth", "design", "implement", "qa", "release"}

// List returns the six public skills in pipeline order (bodies omitted).
func List() []Skill {
	out := make([]Skill, 0, len(order))
	for _, id := range order {
		if s, ok := Get(id); ok {
			s.Body = ""
			out = append(out, s)
		}
	}
	return out
}

// Get returns one skill including its SKILL.md body.
func Get(id string) (Skill, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	raw, err := files.ReadFile(id + "/SKILL.md")
	if err != nil {
		return Skill{}, false
	}
	s := Skill{ID: id, Body: string(raw)}
	for _, line := range strings.Split(string(raw), "\n") {
		switch {
		case strings.HasPrefix(line, "name:"):
			s.Name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		case strings.HasPrefix(line, "description:"):
			s.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		}
	}
	return s, true
}
