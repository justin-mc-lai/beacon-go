// sea-to-lake-proposal: the AI proposes a bounded lake set, a human
// multi-selects, and only the selected lakes become draft packages.
package orchestrator

import (
	"testing"

	"beacon-go/internal/core"
)

func validProposal() LakeProposal {
	return LakeProposal{
		Schema:    LakeProposalSchema,
		Utterance: "sell a habit tracker to busy parents",
		Lakes: []LakeProposalLake{
			{Slug: "feat-a", Outcome: "parents can join the waitlist", Value: "validated demand", Metric: ">=10 signups", Operable: "landing page + waitlist", Size: "S", AcceptanceSeed: "go test ./internal/orchestrator -run TestValidateLakeProposal -count=1"},
			{Slug: "feat-b", Outcome: "parents get a daily nudge", Value: "habit loop", Metric: "D7 >= 20%", Operable: "email digest", Size: "M", AcceptanceSeed: "go test ./internal/orchestrator -run TestProgramPropose -count=1", DependsOn: []string{"feat-a"}},
			{Slug: "feat-c", Outcome: "parents pay for premium", Value: "first revenue", Metric: ">=1 paid", Operable: "stripe checkout", Size: "M", AcceptanceSeed: "go test ./internal/orchestrator -run TestProgramSelect -count=1", DependsOn: []string{"feat-b"}},
		},
		NotProposed: []NotProposedLake{{Slug: "mobile-app", Reason: "no demand evidence yet"}},
	}
}

func codeOfErr(err error) string {
	if err == nil {
		return ""
	}
	code, _ := core.ReasonCodeOf(err)
	return code
}

func TestValidateLakeProposal(t *testing.T) {
	if err := ValidateLakeProposal(validProposal()); err != nil {
		t.Fatalf("valid proposal refused: %v", err)
	}
	bad := validProposal()
	bad.Schema = "lake-proposal.v0"
	if code := codeOfErr(ValidateLakeProposal(bad)); code != core.ReasonLakeProposalInvalid {
		t.Fatalf("schema: want %s, got %s", core.ReasonLakeProposalInvalid, code)
	}
	dup := validProposal()
	dup.Lakes[1].Slug = "feat-a"
	if code := codeOfErr(ValidateLakeProposal(dup)); code != core.ReasonLakeProposalInvalid {
		t.Fatalf("duplicate: want %s, got %s", core.ReasonLakeProposalInvalid, code)
	}
	cycle := validProposal()
	cycle.Lakes[0].DependsOn = []string{"feat-c"}
	if code := codeOfErr(ValidateLakeProposal(cycle)); code != core.ReasonLakeProposalInvalid {
		t.Fatalf("cycle: want %s, got %s", core.ReasonLakeProposalInvalid, code)
	}
}

func TestValidateLakeProposalSize(t *testing.T) {
	short := validProposal()
	short.Lakes = short.Lakes[:2]
	if code := codeOfErr(ValidateLakeProposal(short)); code != core.ReasonLakeProposalSize {
		t.Fatalf("2 lakes: want %s, got %s", core.ReasonLakeProposalSize, code)
	}
	long := validProposal()
	for i := 0; i < 5; i++ {
		l := validProposal().Lakes[2]
		l.Slug = "extra-" + string(rune('a'+i))
		long.Lakes = append(long.Lakes, l)
	}
	if code := codeOfErr(ValidateLakeProposal(long)); code != core.ReasonLakeProposalSize {
		t.Fatalf("8 lakes: want %s, got %s", core.ReasonLakeProposalSize, code)
	}
}

func TestValidateLakeProposalCommercialFields(t *testing.T) {
	noOutcome := validProposal()
	noOutcome.Lakes[0].Outcome = ""
	if code := codeOfErr(ValidateLakeProposal(noOutcome)); code != core.ReasonLakeProposalMissingField {
		t.Fatalf("missing outcome: want %s, got %s", core.ReasonLakeProposalMissingField, code)
	}
	noNotProposed := validProposal()
	noNotProposed.NotProposed = nil
	if code := codeOfErr(ValidateLakeProposal(noNotProposed)); code != core.ReasonLakeProposalNotProposedMiss {
		t.Fatalf("empty not_proposed: want %s, got %s", core.ReasonLakeProposalNotProposedMiss, code)
	}
	noUtterance := validProposal()
	noUtterance.Utterance = "  "
	if code := codeOfErr(ValidateLakeProposal(noUtterance)); code != core.ReasonLakeProposalMissingField {
		t.Fatalf("missing utterance: want %s, got %s", core.ReasonLakeProposalMissingField, code)
	}
}

func TestProgramSelectRecordsDecision(t *testing.T) {
	env := newTestEnv(t, "draft")
	writePackage(t, env.root, "truth.md", "feat-a", "draft")
	run := env.createRun(t)
	p, err := env.eng.ProgramPropose(run.RunID, validProposal())
	if err != nil {
		t.Fatal(err)
	}
	if p.ProposalID == "" {
		t.Fatal("proposal id not assigned")
	}
	got, err := env.eng.ProgramSelect(run.RunID, p.ProposalID,
		[]string{"feat-a", "feat-b"}, []string{"feat-c"}, "dahuai")
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectedBy != "dahuai" || got.SelectedAt == "" {
		t.Fatalf("decision not recorded: %+v", got)
	}
	if len(got.Selected) != 2 || len(got.Rejected) != 1 {
		t.Fatalf("counts wrong: %+v", got)
	}
	if _, err := env.eng.ProgramSelect(run.RunID, p.ProposalID,
		[]string{"nope"}, nil, "dahuai"); codeOfErr(err) != core.ReasonLakeProposalUnknownLake {
		t.Fatalf("unknown slug: want %s, got %v", core.ReasonLakeProposalUnknownLake, err)
	}
	if _, err := env.eng.ProgramSelect(run.RunID, p.ProposalID,
		nil, nil, "dahuai"); codeOfErr(err) != core.ReasonLakeProposalSelectionEmpty {
		t.Fatalf("empty selection: want %s, got %v", core.ReasonLakeProposalSelectionEmpty, err)
	}
}

func TestProgramSelectCreatesDraftsOnlyForSelected(t *testing.T) {
	env := newTestEnv(t, "draft")
	writePackage(t, env.root, "truth.md", "feat-a", "draft")
	run := env.createRun(t)
	p, err := env.eng.ProgramPropose(run.RunID, validProposal())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.ProgramSelect(run.RunID, p.ProposalID,
		[]string{"feat-a", "feat-b"}, []string{"feat-c"}, "dahuai"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.st.Features().GetBySlug(env.feature.ProjectVersionID, "feat-b"); err != nil {
		t.Fatalf("selected lake feat-b should have a draft package: %v", err)
	}
	if _, err := env.st.Features().GetBySlug(env.feature.ProjectVersionID, "feat-c"); err == nil {
		t.Fatal("rejected lake feat-c must not get a package")
	}
}

func TestProgramPlanFromProposalRefusesUnselectedDependency(t *testing.T) {
	env := newTestEnv(t, "draft")
	writePackage(t, env.root, "truth.md", "feat-a", "draft")
	run := env.createRun(t)
	p, err := env.eng.ProgramPropose(run.RunID, validProposal())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.ProgramSelect(run.RunID, p.ProposalID,
		[]string{"feat-b"}, []string{"feat-a", "feat-c"}, "dahuai"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.ProgramPlanFromProposal(run.RunID, p.ProposalID, false); codeOfErr(err) != core.ReasonLakeProposalDepNotSelected {
		t.Fatalf("dependency must be selected first: want %s, got %v", core.ReasonLakeProposalDepNotSelected, err)
	}
}

func TestProgramRouteShowsProposalCounts(t *testing.T) {
	env := newTestEnv(t, "draft")
	writePackage(t, env.root, "truth.md", "feat-a", "draft")
	run := env.createRun(t)
	p, err := env.eng.ProgramPropose(run.RunID, validProposal())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.ProgramSelect(run.RunID, p.ProposalID,
		[]string{"feat-a", "feat-b"}, []string{"feat-c"}, "dahuai"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.ProgramPlanFromProposal(run.RunID, p.ProposalID, false); err != nil {
		t.Fatal(err)
	}
	view, err := env.eng.ProgramRoute(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if view.ProposalID != p.ProposalID || view.ProposalSelected != 2 || view.ProposalRejected != 1 {
		t.Fatalf("route proposal counts wrong: %+v", view)
	}
}
