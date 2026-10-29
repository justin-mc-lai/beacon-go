package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

// newUICmd is the single UI entry: the native desktop workspace.
// There is no browser mode — one surface, one implementation, no second
// rendering path to keep in sync (decision: docs/plan/2026-10-28-desktop-only-plan.md).
func newUICmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Launch the Beacon Agent desktop workspace (native window)",
		Long: "Launches the Beacon Agent desktop application. The binary is looked up as " +
			"$BEACON_DESKTOP_BIN, then next to the beacon binary, then on PATH, then ./bin/beacon-desktop.",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := project
			if root == "" {
				root = "."
			}
			abs, err := filepath.Abs(root)
			if err != nil {
				return err
			}
			bin, err := findDesktopBinary()
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "launching Beacon Agent Desktop (%s) for %s\n", bin, abs)
			proc := exec.Command(bin)
			proc.Dir = abs
			proc.Env = append(os.Environ(), "BEACON_PROJECT_ROOT="+abs)
			proc.Stdout = os.Stdout
			proc.Stderr = os.Stderr
			return proc.Run()
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", ".", "project root the desktop app opens")
	return cmd
}

// findDesktopBinary resolves the desktop binary in a fixed order so a
// stale copy cannot shadow the one built next to the CLI.
func findDesktopBinary() (string, error) {
	if p := os.Getenv("BEACON_DESKTOP_BIN"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("BEACON_DESKTOP_BIN=%s does not exist", p)
	}
	if self, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(self), "beacon-desktop")
		if _, err := os.Stat(sibling); err == nil {
			return sibling, nil
		}
	}
	if p, err := exec.LookPath("beacon-desktop"); err == nil {
		return p, nil
	}
	if _, err := os.Stat("bin/beacon-desktop"); err == nil {
		return "bin/beacon-desktop", nil
	}
	return "", fmt.Errorf("beacon-desktop not found; build it with: go build -o bin/beacon-desktop ./desktop")
}
