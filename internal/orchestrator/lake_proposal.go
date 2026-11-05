// Sea-to-lake proposal (SDD): the AI proposes a bounded set of
// commercially meaningful lakes; the human multi-selects; the kernel
// validates, records, creates draft packages for the selected lakes
// only, and freezes nothing until the human confirms the plan.
package orchestrator

import (
	"fmt"
	"strings"
	"time"

	"beacon-go/internal/core"
)

const LakeProposalSchema = "lake-proposal.v1"

type LakeProposal struct {
	Schema         string             `json:"schema"`
	ProposalID     string             `json:"proposal_id"`
	Utterance      string             `json:"utterance"`
	Version        string             `json:"version,omitempty"`
	CommercialLens []string           `json:"commercial_lens,omitempty"`
	Lakes          []LakeProposalLake `json:"lakes"`
	NotProposed    []NotProposedLake  `json:"not_proposed"`
	OpenQuestions  []string           `json:"open_questions,omitempty"`
	Selected       []string           `json:"selected,omitempty"`
	Rejected       []string           `json:"rejected,omitempty"`
	SelectedBy     string             `json:"selected_by,omitempty"`
	SelectedAt     string             `json:"selected_at,omitempty"`
}

type LakeProposalLake struct {
	Slug           string   `json:"slug"`
	Title          string   `json:"title,omitempty"`
	Outcome        string   `json:"outcome"`
	Value          string   `json:"value"`
	Metric         string   `json:"metric"`
	Operable       string   `json:"operable"`
	DependsOn      []string `json:"depends_on,omitempty"`
	Size           string   `json:"size"`
	AcceptanceSeed string   `json:"acceptance_seed"`
	OutOfScope     []string `json:"out_of_scope,omitempty"`
	Questions      []string `json:"questions,omitempty"`
}

type NotProposedLake struct {
	Slug   string `json:"slug"`
	Reason string `json:"reason"`
}

// ValidateLakeProposal is the anti-hallucination gate: bounded lake
// count, one unique slug each, the commercial lens fields present,
// explicit exclusions, and a valid dependency DAG.
func ValidateLakeProposal(p LakeProposal) error {
	if p.Schema != "" && p.Schema != LakeProposalSchema {
		return core.NewError(core.ReasonLakeProposalInvalid, "program.propose", "unknown schema "+p.Schema)
	}
	if strings.TrimSpace(p.Utterance) == "" {
		return core.NewError(core.ReasonLakeProposalMissingField, "program.propose", "utterance is required")
	}
	if len(p.Lakes) < 3 || len(p.Lakes) > 7 {
		return core.NewError(core.ReasonLakeProposalSize, "program.propose",
			fmt.Sprintf("proposal has %d lakes; want 3-7 (split into versions/phases if bigger)", len(p.Lakes)))
	}
	seen := map[string]bool{}
	for _, l := range p.Lakes {
		slug := strings.TrimSpace(l.Slug)
		if slug == "" {
			return core.NewError(core.ReasonLakeProposalMissingField, "program.propose", "lake slug is required")
		}
		if seen[slug] {
			return core.NewError(core.ReasonLakeProposalInvalid, "program.propose", "duplicate lake slug "+slug)
		}
		seen[slug] = true
		for name, val := range map[string]string{
			"outcome": l.Outcome, "value": l.Value, "metric": l.Metric,
			"operable": l.Operable, "size": l.Size, "acceptance_seed": l.AcceptanceSeed,
		} {
			if strings.TrimSpace(val) == "" {
				return core.NewError(core.ReasonLakeProposalMissingField, "program.propose",
					"lake "+slug+" is missing "+name)
			}
		}
		switch strings.ToUpper(strings.TrimSpace(l.Size)) {
		case "S", "M", "L":
		default:
			return core.NewError(core.ReasonLakeProposalInvalid, "program.propose",
				"lake "+slug+" size must be S/M/L, got "+l.Size)
		}
	}
	if len(p.NotProposed) == 0 {
		return core.NewError(core.ReasonLakeProposalNotProposedMiss, "program.propose",
			"not_proposed must list what is deliberately excluded (with a reason)")
	}
	for _, n := range p.NotProposed {
		if strings.TrimSpace(n.Slug) == "" || strings.TrimSpace(n.Reason) == "" {
			return core.NewError(core.ReasonLakeProposalNotProposedMiss, "program.propose",
				"every not_proposed item needs slug and reason")
		}
	}
	inputs := make([]ProgramLakeInput, 0, len(p.Lakes))
	for _, l := range p.Lakes {
		inputs = append(inputs, ProgramLakeInput{Slug: l.Slug, DependsOn: l.DependsOn})
	}
	if problems := ValidateProgramGraph(inputs); len(problems) > 0 {
		return core.NewError(core.ReasonLakeProposalInvalid, "program.propose", problems[0].Detail)
	}
	return nil
}

func (e *Engine) ProgramPropose(runID string, p LakeProposal) (LakeProposal, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return LakeProposal{}, err
	}
	if err := ValidateLakeProposal(p); err != nil {
		return LakeProposal{}, err
	}
	if strings.TrimSpace(p.ProposalID) == "" {
		p.ProposalID = fmt.Sprintf("lp_%d", time.Now().UnixNano())
	}
	m.LakeProposal = &p
	if err := e.appendEvent(run, "lake_proposal_created", map[string]any{
		"proposal_id": p.ProposalID, "utterance": p.Utterance,
		"lakes": len(p.Lakes), "not_proposed": len(p.NotProposed),
	}); err != nil {
		return LakeProposal{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return LakeProposal{}, err
	}
	return p, nil
}

func (e *Engine) ProgramSelect(runID, proposalID string, selected, rejected []string, by string) (LakeProposal, error) {
	if strings.TrimSpace(by) == "" {
		return LakeProposal{}, core.NewError(core.ReasonGateEvaluatorRequired, "program.select",
			"a human identity is required (--by)")
	}
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return LakeProposal{}, err
	}
	if m.LakeProposal == nil || m.LakeProposal.ProposalID != proposalID {
		return LakeProposal{}, core.NewError(core.ReasonLakeProposalNotFound, "program.select",
			"no proposal "+proposalID+" on run "+runID)
	}
	if len(selected) == 0 {
		return LakeProposal{}, core.NewError(core.ReasonLakeProposalSelectionEmpty, "program.select",
			"select at least one lake")
	}
	known := map[string]bool{}
	for _, l := range m.LakeProposal.Lakes {
		known[l.Slug] = true
	}
	chosen := map[string]bool{}
	for _, s := range selected {
		if !known[s] {
			return LakeProposal{}, core.NewError(core.ReasonLakeProposalUnknownLake, "program.select", "unknown lake "+s)
		}
		if chosen[s] {
			return LakeProposal{}, core.NewError(core.ReasonLakeProposalInvalid, "program.select", "duplicate selected lake "+s)
		}
		chosen[s] = true
	}
	for _, r := range rejected {
		if !known[r] {
			return LakeProposal{}, core.NewError(core.ReasonLakeProposalUnknownLake, "program.select", "unknown rejected lake "+r)
		}
		if chosen[r] {
			return LakeProposal{}, core.NewError(core.ReasonLakeProposalInvalid, "program.select", "lake "+r+" is both selected and rejected")
		}
	}
	m.LakeProposal.Selected = selected
	m.LakeProposal.Rejected = rejected
	m.LakeProposal.SelectedBy = by
	m.LakeProposal.SelectedAt = nowRFC()

	fc, ok, err := e.featureContextOf(run.FeatureID)
	if err != nil {
		return LakeProposal{}, err
	}
	if !ok {
		return LakeProposal{}, core.NewError(ReasonProjectNotRegistered, "program.select", "no feature context for run "+runID)
	}
	for _, s := range selected {
		_, cerr := e.CreateTruthPackage(fc.Project.RootPath, fc.Version.Version, s)
		if cerr != nil {
			if rc, _ := core.ReasonCodeOf(cerr); rc == ReasonTruthPackageExists || rc == ReasonTruthPackageDirExists {
				continue
			}
			return LakeProposal{}, cerr
		}
	}
	if err := e.appendEvent(run, "lake_proposal_selected", map[string]any{
		"proposal_id": proposalID, "selected": selected, "rejected": rejected, "by": by,
	}); err != nil {
		return LakeProposal{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return LakeProposal{}, err
	}
	return *m.LakeProposal, nil
}

func (e *Engine) ProgramPlanFromProposal(runID, proposalID string, freeze bool) (ProgramState, error) {
	_, m, _, err := e.loadRun(runID)
	if err != nil {
		return ProgramState{}, err
	}
	if m.LakeProposal == nil || m.LakeProposal.ProposalID != proposalID {
		return ProgramState{}, core.NewError(core.ReasonLakeProposalNotFound, "program.plan",
			"no proposal "+proposalID+" on run "+runID)
	}
	if len(m.LakeProposal.Selected) == 0 {
		return ProgramState{}, core.NewError(core.ReasonLakeProposalSelectionEmpty, "program.plan",
			"proposal has no selected lakes; run `goal program select` first")
	}
	selected := map[string]bool{}
	for _, s := range m.LakeProposal.Selected {
		selected[s] = true
	}
	bySlug := map[string]LakeProposalLake{}
	for _, l := range m.LakeProposal.Lakes {
		bySlug[l.Slug] = l
	}
	inputs := make([]ProgramLakeInput, 0, len(m.LakeProposal.Selected))
	for _, s := range m.LakeProposal.Selected {
		l := bySlug[s]
		for _, dep := range l.DependsOn {
			if !selected[dep] {
				return ProgramState{}, core.NewError(core.ReasonLakeProposalDepNotSelected, "program.plan",
					"lake "+s+" depends on "+dep+" which is not selected; select it or remove the dependency")
			}
		}
		inputs = append(inputs, ProgramLakeInput{Slug: s, Goal: l.Outcome, DependsOn: l.DependsOn, Status: "selected"})
	}
	return e.ProgramPlan(runID, ProgramPlanInput{
		Slug: "sea-" + proposalID, Goal: m.LakeProposal.Utterance, Lakes: inputs, Freeze: freeze,
	})
}
