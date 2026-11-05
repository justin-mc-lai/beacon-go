package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanSkillMentionsProposalFlow(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("plan", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{"program propose", "program select", "3-7", "not_proposed", "from-proposal", "lake_proposal_required"} {
		if !strings.Contains(text, want) {
			t.Fatalf("plan skill must mention %q", want)
		}
	}
}

func TestGoalAndCoreMentionProposalGate(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("goal", "SKILL.md"):              {"propose", "multi-select", "--from-proposal", "lake_proposal_required"},
		filepath.Join("plan", "references", "core.md"): {"lake-proposal.v1", "program select", "--from-proposal", "lake_proposal_required"},
	}
	for file, wants := range checks {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		for _, want := range wants {
			if !strings.Contains(text, want) {
				t.Fatalf("%s must mention %q", file, want)
			}
		}
	}
}
