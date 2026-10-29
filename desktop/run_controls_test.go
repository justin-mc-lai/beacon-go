package main

import (
	"reflect"
	"strings"
	"testing"
)

// TestDesktopRunControlsPresent (TC-007): the shell can pick an agent
// backend and push a run one step, and both go through the real
// backend rather than a local imitation of progress.
func TestDesktopRunControlsPresent(t *testing.T) {
	body := shell(t)

	// The controls themselves.
	for _, want := range []string{
		"executorSelect", "btnAdvance",
		"fillExecutorOptions", "advanceRun", "renderRunState",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("shell is missing run control %q", want)
		}
	}

	// They must call the real backend surfaces.
	for _, want := range []string{
		`invoke("Executors")`,
		`invoke("UseExecutor"`,
		`invoke("AdvanceRun"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("shell does not reach the backend via %s", want)
		}
	}

	// A park is rendered as a park with its reason code — the shell must
	// have somewhere to say so, and must not render a park as a pass.
	if !strings.Contains(body, "parked:") {
		t.Error("shell has no way to show a parked run's reason code")
	}
	if strings.Contains(body, "机器自动化验收通过") {
		t.Error("shell must not claim QA passed on its own")
	}

	// The Go bindings the shell calls must actually exist.
	appType := reflect.TypeOf(&App{})
	for _, m := range []string{"Executors", "UseExecutor", "AdvanceRun"} {
		if _, ok := appType.MethodByName(m); !ok {
			t.Errorf("App is missing binding %q that the shell calls", m)
		}
	}
}

// TestUnavailableBackendIsNotSilentlyOffered: a backend whose command
// cannot start here is shown as unavailable and disabled, rather than
// offered and then failing when the operator presses advance.
func TestUnavailableBackendIsNotSilentlyOffered(t *testing.T) {
	body := shell(t)
	if !strings.Contains(body, "op.disabled = !o.available") {
		t.Error("an unavailable backend must be disabled in the picker")
	}
	if !strings.Contains(body, "t().unavailable") {
		t.Error("an unavailable backend must be labelled as such")
	}
}
