package skills

import (
	"regexp"
	"strings"
	"testing"
)

// Every public skill must carry the four blocks that turn a happy-path
// how-to into a contract the host can be held to: the hard gate (what
// you may not do), the mandatory load (read the reference before
// acting), the output contract (what a turn must end with) and the
// refusal list (when to stop instead of faking delivery).
//
// Without these, the agent sees only numbered steps and the cheapest
// reading of "done" wins — which is exactly the drift operators report.
func TestEverySkillCarriesTheContract(t *testing.T) {
	required := []string{
		"## HARD GATE",
		"## Load before acting",
		"## Output contract",
		"## Refuse (no fake delivery)",
		"references/core.md",
		"mode_id",
		"plain_summary",
	}
	for _, id := range IDs() {
		s, ok := Get(id)
		if !ok {
			t.Fatalf("skill %s missing", id)
		}
		for _, block := range required {
			if !strings.Contains(s.Body, block) {
				t.Errorf("skill %s: missing %q", id, block)
			}
		}
		// The refusal list must be concrete: at least six numbered stops.
		refuse := s.Body[strings.Index(s.Body, "## Refuse"):]
		if n := strings.Count(refuse, "\n1. ") + strings.Count(refuse, "\n2. ") +
			strings.Count(refuse, "\n3. ") + strings.Count(refuse, "\n4. ") +
			strings.Count(refuse, "\n5. ") + strings.Count(refuse, "\n6. "); n < 6 {
			t.Errorf("skill %s: refusal list is too thin (%d numbered items)", id, n)
		}
	}
}

// A skill that tells the agent to act without reading its reference is
// the mechanism behind "answered from the summary, skipped the rules".
func TestSkillsOrderTheReferenceRead(t *testing.T) {
	for _, id := range IDs() {
		s, _ := Get(id)
		load := s.Body[strings.Index(s.Body, "## Load before acting"):]
		if !strings.Contains(load, "completely") {
			t.Errorf("skill %s: load block does not require a complete read", id)
		}
	}
}

// AC-127/AC-144: the six public abilities each expose route examples,
// every referenced example file exists in the embedded package, and the
// set covers the four example chains the frozen surface names. Examples
// are explicitly not requirement authority.
func TestRouteExamplesCoverFourChains(t *testing.T) {
	six := []string{"plan", "truth", "design", "implement", "qa", "release"}
	fileRef := regexp.MustCompile("`(examples/[^`]+\\.md)`")
	chainRef := regexp.MustCompile("chain `([a-z-]+)`")
	seen := map[string]bool{}
	for _, id := range six {
		s, ok := Get(id)
		if !ok {
			t.Fatalf("skill %s missing", id)
		}
		i := strings.Index(s.Body, "## Route examples")
		if i < 0 {
			t.Errorf("skill %s: no ## Route examples section (AC-127)", id)
			continue
		}
		sec := s.Body[i:]
		if j := strings.Index(sec[1:], "\n## "); j >= 0 {
			sec = sec[:j+1]
		}
		if !strings.Contains(sec, "not\nrequirement authority") && !strings.Contains(sec, "not requirement authority") {
			t.Errorf("skill %s: route examples must say they are not requirement authority (AC-127)", id)
		}
		refs := fileRef.FindAllStringSubmatch(sec, -1)
		if len(refs) < 2 {
			t.Errorf("skill %s: %d example refs, want at least 2", id, len(refs))
		}
		for _, m := range refs {
			if _, err := files.ReadFile(id + "/" + m[1]); err != nil {
				t.Errorf("skill %s: referenced example %s does not exist", id, m[1])
			}
		}
		for _, m := range chainRef.FindAllStringSubmatch(sec, -1) {
			seen[m[1]] = true
		}
	}
	for _, chain := range []string{"full-delivery", "honest-block", "change-refreeze", "human-gate"} {
		if !seen[chain] {
			t.Errorf("route examples do not cover chain %q", chain)
		}
	}
}
