package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestUICmdFlags(t *testing.T) {
	cmd := newUICmd()
	cmd.SetArgs([]string{"--help"})
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error executing ui help: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ui [flags]") {
		t.Errorf("expected usage info in help output, got: %s", out)
	}
	if !strings.Contains(out, "--project") {
		t.Errorf("expected --project in help output")
	}
	// Desktop-only decision: the browser mode is gone, not deprecated.
	for _, gone := range []string{"--web", "--port"} {
		if strings.Contains(out, gone) {
			t.Errorf("%s must not exist on a desktop-only ui command", gone)
		}
	}
}

// The desktop binary lookup must fail closed with an actionable
// message instead of silently launching whatever is on PATH.
func TestFindDesktopBinaryFailsClosed(t *testing.T) {
	t.Setenv("BEACON_DESKTOP_BIN", "/nonexistent/beacon-desktop")
	if _, err := findDesktopBinary(); err == nil {
		t.Fatal("a missing explicit binary path must be an error")
	} else if !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("unexpected error: %v", err)
	}
}
