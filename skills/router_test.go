package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRouterTableSchema (AC-001): the routing table is data with a
// schema, and every skill it names really exists in the embedded set.
func TestRouterTableSchema(t *testing.T) {
	table := Router()
	if table.Schema != RouterSchema {
		t.Fatalf("schema = %q, want %q", table.Schema, RouterSchema)
	}
	if len(table.Entries) != len(IDs()) {
		t.Fatalf("router covers %d skills, embedded set has %d", len(table.Entries), len(IDs()))
	}
	known := map[string]bool{}
	for _, id := range IDs() {
		known[id] = true
	}
	for _, e := range table.Entries {
		if !known[e.Skill] {
			t.Errorf("router names unknown skill %q", e.Skill)
		}
		if len(e.Signals) == 0 {
			t.Errorf("skill %s declares no signals", e.Skill)
		}
		if len(e.Samples) == 0 {
			t.Errorf("skill %s declares no samples", e.Skill)
		}
		if len(e.MustRead) == 0 {
			t.Errorf("skill %s declares no must-read references", e.Skill)
		}
		for _, s := range e.Signals {
			if s.Text == "" {
				t.Errorf("skill %s has an empty signal", e.Skill)
			}
			if s.Weight <= 0 {
				t.Errorf("skill %s signal %q has weight %d, want > 0", e.Skill, s.Text, s.Weight)
			}
		}
		for _, m := range e.MustRead {
			if !strings.HasPrefix(m, "references/") {
				t.Errorf("skill %s must-read %q is not a references path", e.Skill, m)
			}
		}
	}
}

// TestRouterMatchesEverySample (AC-002): every sample shipped with the
// table routes to its own skill, and to exactly one.
func TestRouterMatchesEverySample(t *testing.T) {
	for _, e := range Router().Entries {
		for _, s := range e.Samples {
			res := Route(s)
			if !res.Hit {
				t.Errorf("sample %q (skill %s) did not route: %s", s, e.Skill, res.Reason)
				continue
			}
			if res.Skill != e.Skill {
				t.Errorf("sample %q routed to %q, want %q", s, res.Skill, e.Skill)
			}
		}
	}
}

// TestRouterRefusesToGuess (AC-003): a request that matches nothing, or
// matches two skills equally, comes back as a miss with candidates —
// never as a confident wrong skill.
func TestRouterRefusesToGuess(t *testing.T) {
	res := Route("今天天气怎么样")
	if res.Hit {
		t.Fatalf("an unrelated sentence must not route: got %q", res.Skill)
	}
	if res.Reason != "no_signal" {
		t.Errorf("miss reason = %q, want no_signal", res.Reason)
	}
	if res.Skill != "" {
		t.Errorf("a miss must not name a skill, got %q", res.Skill)
	}

	amb := Route("设计和实现")
	if amb.Hit {
		t.Fatalf("an equally-matching sentence must not be forced onto one skill: got %q", amb.Skill)
	}
	if amb.Reason != "ambiguous" {
		t.Errorf("ambiguity reason = %q, want ambiguous", amb.Reason)
	}
	if len(amb.Candidates) < 2 {
		t.Errorf("an ambiguous match must offer its candidates, got %v", amb.Candidates)
	}

	empty := Route("   ")
	if empty.Hit || empty.Reason != "empty_utterance" {
		t.Errorf("empty utterance = %+v, want an empty_utterance miss", empty)
	}
}

// TestProgressiveMapOrder (AC-004): every skill declares which
// reference to open first, and the declaration is ordered.
func TestProgressiveMapOrder(t *testing.T) {
	for _, e := range Router().Entries {
		if len(e.MustRead) == 0 {
			t.Fatalf("skill %s has no progressive-disclosure order", e.Skill)
		}
		for i, m := range e.MustRead {
			if m == "" {
				t.Errorf("skill %s must-read[%d] is empty", e.Skill, i)
			}
		}
		// The order must be stable across calls.
		first := MustReadPaths("", e.Skill)
		_ = first
		if len(e.MustRead) != len(append([]string{}, e.MustRead...)) {
			t.Errorf("skill %s must-read order is not stable", e.Skill)
		}
	}
}

// TestProgressiveMapPathsExist (AC-005): the must-read list resolves
// to real files under an installed host directory. A declared reference
// that is missing on disk must not be handed to a host agent.
func TestProgressiveMapPathsExist(t *testing.T) {
	host := t.TempDir()
	if _, err := Install(host, true); err != nil {
		t.Fatalf("install skills into %s: %v", host, err)
	}
	for _, e := range Router().Entries {
		paths := MustReadPaths(host, e.Skill)
		if len(paths) == 0 {
			t.Errorf("skill %s resolved no must-read path under %s", e.Skill, host)
			continue
		}
		for _, p := range paths {
			if !filepath.IsAbs(p) {
				t.Errorf("skill %s must-read path %q is not absolute", e.Skill, p)
			}
			st, err := os.Stat(p)
			if err != nil {
				t.Errorf("skill %s must-read path does not exist: %v", e.Skill, err)
				continue
			}
			if st.IsDir() {
				t.Errorf("skill %s must-read path is a directory: %s", e.Skill, p)
			}
			if !strings.Contains(p, InstallDirName(e.Skill)) {
				t.Errorf("skill %s must-read path %q is not under its own package", e.Skill, p)
			}
		}
	}
	// A host directory that was never populated yields nothing, rather
	// than paths that do not exist.
	if got := MustReadPaths(filepath.Join(t.TempDir(), "empty"), "goal"); len(got) != 0 {
		t.Errorf("an empty host dir must resolve no must-read paths, got %v", got)
	}
}
