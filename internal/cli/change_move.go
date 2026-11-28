package cli

import "github.com/spf13/cobra"

func newChangeMoveCmd() *cobra.Command { return newControlledTreeChangeCmd(true) }
