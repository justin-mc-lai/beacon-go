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
	if !strings.Contains(out, "Usage:") || !strings.Contains(out, "ui [flags]") {
		t.Errorf("expected usage info in help output, got: %s", out)
	}
	if !strings.Contains(out, "--port") {
		t.Errorf("expected --port flag in help output")
	}
}
