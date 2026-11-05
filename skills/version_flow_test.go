package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTruthSkillVersionFlow(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("truth", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{"v0.0.1", "projects bump", "runtime"} {
		if !strings.Contains(text, want) {
			t.Fatalf("truth skill must mention %q", want)
		}
	}
}
