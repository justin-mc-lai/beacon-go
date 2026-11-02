// On-disk program graph (v1 program_graph.validate_graph_payload):
// read docs/beacon/<version>/programs/<slug>/feature-graph.json,
// report the v1 required-key set, and reuse ValidateProgramGraph for
// the dependency / cycle rules. Import folds the file into the store
// so the file graph and `goal program *` are one model.
package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"beacon-go/internal/core"
)

// ProgramGraphFile is the loadable shape of a feature-graph.json.
type ProgramGraphFile struct {
	ProgramSlug string             `json:"program_slug"`
	Version     string             `json:"version"`
	ScopeMode   string             `json:"scope_mode"`
	Goal        string             `json:"goal,omitempty"`
	Lakes       []ProgramLakeInput `json:"lakes"`
}

type rawProgramLake struct {
	Slug         *string   `json:"slug"`
	Goal         string    `json:"goal"`
	DependsOn    *[]string `json:"depends_on"`
	Touches      []string  `json:"touches"`
	ParitySubset *[]string `json:"parity_subset"`
	Status       *string   `json:"status"`
}

type rawProgramGraph struct {
	ProgramSlug *string           `json:"program_slug"`
	Version     *string           `json:"version"`
	ScopeMode   *string           `json:"scope_mode"`
	Goal        string            `json:"goal"`
	Lakes       *[]rawProgramLake `json:"lakes"`
}

// LoadProgramGraphFile reads and validates one program graph. A
// missing file is a typed file_missing error; malformed JSON is a
// feature_graph_invalid error. Structural problems are returned as
// the v1 diagnostic strings (missing:<key>, lakes[i]:missing:<key>)
// plus the existing dependency/cycle reason codes.
func LoadProgramGraphFile(path string) (ProgramGraphFile, []GraphProblem, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ProgramGraphFile{}, nil, core.NewError(core.ReasonProgramGraphFileMissing,
				"program.LoadGraph", "file_missing: "+path)
		}
		return ProgramGraphFile{}, nil, err
	}
	var raw rawProgramGraph
	if err := json.Unmarshal(b, &raw); err != nil {
		return ProgramGraphFile{}, nil, core.NewError(core.ReasonProgramGraphInvalid,
			"program.LoadGraph", "invalid JSON: "+err.Error())
	}

	g := ProgramGraphFile{Goal: raw.Goal}
	var problems []GraphProblem
	add := func(detail string) {
		problems = append(problems, GraphProblem{ReasonCode: core.ReasonProgramGraphInvalid, Detail: detail})
	}
	if raw.ProgramSlug == nil || strings.TrimSpace(*raw.ProgramSlug) == "" {
		add("missing:program_slug")
	} else {
		g.ProgramSlug = *raw.ProgramSlug
	}
	if raw.Version == nil || strings.TrimSpace(*raw.Version) == "" {
		add("missing:version")
	} else {
		g.Version = *raw.Version
	}
	if raw.ScopeMode == nil || strings.TrimSpace(*raw.ScopeMode) == "" {
		add("missing:scope_mode")
	} else {
		g.ScopeMode = *raw.ScopeMode
	}
	if raw.Lakes == nil {
		add("missing:lakes")
		return g, problems, nil
	}
	for i, l := range *raw.Lakes {
		in := ProgramLakeInput{Goal: l.Goal, Touches: l.Touches}
		if l.Slug == nil || strings.TrimSpace(*l.Slug) == "" {
			add(fmt.Sprintf("lakes[%d]:missing:slug", i))
		} else {
			in.Slug = *l.Slug
		}
		if l.DependsOn == nil {
			add(fmt.Sprintf("lakes[%d]:missing:depends_on", i))
		} else {
			in.DependsOn = *l.DependsOn
		}
		if l.ParitySubset == nil {
			add(fmt.Sprintf("lakes[%d]:missing:parity_subset", i))
		} else {
			in.ParitySubset = *l.ParitySubset
		}
		if l.Status == nil || strings.TrimSpace(*l.Status) == "" {
			add(fmt.Sprintf("lakes[%d]:missing:status", i))
		} else {
			in.Status = *l.Status
		}
		g.Lakes = append(g.Lakes, in)
	}
	if len(problems) == 0 {
		problems = append(problems, ValidateProgramGraph(g.Lakes)...)
	}
	return g, problems, nil
}

// ProgramImportFile validates an on-disk graph and records it as the
// host run's program plan. Validation runs before any store write: an
// invalid graph leaves the run's program state untouched.
func (e *Engine) ProgramImportFile(runID, path string, freeze bool) (ProgramState, error) {
	g, problems, err := LoadProgramGraphFile(path)
	if err != nil {
		return ProgramState{}, err
	}
	if len(problems) > 0 {
		return ProgramState{}, core.NewError(problems[0].ReasonCode, "program.Import", problems[0].Detail)
	}
	return e.ProgramPlan(runID, ProgramPlanInput{
		Slug: g.ProgramSlug, Goal: g.Goal, ScopeMode: g.ScopeMode, Lakes: g.Lakes, Freeze: freeze,
	})
}
