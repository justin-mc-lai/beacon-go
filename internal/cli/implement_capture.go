package cli

import (
	"time"

	"beacon-go/internal/orchestrator"
	"github.com/spf13/cobra"
)

func newImplementCaptureCmd() *cobra.Command {
	var project, version string
	var req orchestrator.CaptureRequest
	var seconds int
	c := &cobra.Command{Use: "capture", Short: "Capture a bound Go test Red or Green and register real process evidence", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if seconds <= 0 || seconds > 3600 {
			return fail("capture_request_invalid", "implement capture", "timeout-sec must be 1..3600")
		}
		st, err := openWriteStore()
		if err != nil {
			return err
		}
		defer st.Close()
		dir, root, _, err := resolveTree(st, project, version)
		if err != nil {
			return err
		}
		req.Root, req.TreeDir, req.Timeout = root, dir, time.Duration(seconds)*time.Second
		result, runErr := orchestrator.NewEngine(st).CaptureTest(cmd.Context(), req)
		if result.ResultPath != "" {
			if err := printJSON(cmd, result); err != nil {
				return err
			}
		}
		return runErr
	}}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "explicit docs version (required)")
	c.Flags().StringVar(&req.NodeID, "node", "", "touchpoint ID")
	c.Flags().StringVar(&req.ACID, "ac", "", "canonical AC ID")
	c.Flags().StringVar(&req.Phase, "phase", "", "red or green")
	c.Flags().StringVar(&req.By, "by", "", "maker identity")
	c.Flags().StringVar(&req.RedID, "red", "", "explicit Red artifact ID for Green")
	c.Flags().StringVar(&req.Baseline, "baseline", "", "Red only: exact ancestor commit for isolated historical replay")
	c.Flags().IntVar(&seconds, "timeout-sec", 120, "process timeout, 1..3600 seconds")
	for _, flag := range []string{"version", "node", "ac", "phase", "by"} {
		_ = c.MarkFlagRequired(flag)
	}
	return c
}
