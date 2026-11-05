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
	for _, want := range []string{"program propose", "program select", "3-7", "not_proposed", "from-proposal"} {
		if !strings.Contains(text, want) {
			t.Fatalf("plan skill must mention %q", want)
		}
	}
}
