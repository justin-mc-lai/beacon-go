package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func newUICmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Launch the Beacon Agent Desktop App (Wails native UI)",
		Long:  "Launches the Beacon Agent OpenMuse desktop workspace window. Legacy web console mode is deprecated in favor of native Wails desktop application.",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			desktopBin, err := exec.LookPath("beacon-desktop")
			if err != nil {
				// Check relative bin path
				if _, statErr := os.Stat("bin/beacon-desktop"); statErr == nil {
					desktopBin = "bin/beacon-desktop"
				} else {
					return fmt.Errorf("beacon-desktop binary not found in PATH or bin/ directory. Please run 'go build -o bin/beacon-desktop ./desktop/main.go'")
				}
			}
			_, _ = fmt.Fprintf(out, "🚀 Launching Beacon Agent Desktop Workspace via %s...\n", desktopBin)
			proc := exec.Command(desktopBin)
			proc.Stdout = os.Stdout
			proc.Stderr = os.Stderr
			return proc.Run()
		},
	}
	cmd.Flags().IntVarP(&port, "port", "p", 4178, "Port to listen on (deprecated)")
	return cmd
}
