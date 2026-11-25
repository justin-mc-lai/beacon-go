package cli

import (
	"strings"
	"testing"
)

// #26: the seven user-facing groups exist and qa is no longer a command.
func TestSevenCommandSurface(t *testing.T) {
	root := NewRootCmd()
	top := map[string]*struct{}{}
	for _, c := range root.Commands() {
		top[c.Name()] = nil
	}
	for _, want := range []string{"plan", "truth", "change", "implement", "verify", "release", "goal"} {
		if _, ok := top[want]; !ok {
			t.Errorf("top-level command %q is missing", want)
		}
	}
	if _, ok := top["qa"]; ok {
		t.Error("qa must not be a top-level command")
	}

	for _, c := range root.Commands() {
		if c.Name() != "verify" {
			continue
		}
		sub := map[string]bool{}
		for _, sc := range c.Commands() {
			sub[sc.Name()] = true
		}
		for _, want := range []string{"status", "evaluate", "score", "autotest", "audit", "leaf", "rollup"} {
			if !sub[want] {
				t.Errorf("verify subcommand %q is missing", want)
			}
		}
	}
	if out, err := runCLI(t, "qa", "status"); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("qa must be unknown, got %v\n%s", err, out)
	}
}
