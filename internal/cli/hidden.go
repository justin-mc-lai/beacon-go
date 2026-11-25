package cli

import "github.com/spf13/cobra"

// hideAllBut keeps only the named user-facing subcommands visible.
// Everything else stays executable for maintenance and tests.
func hideAllBut(c *cobra.Command, names ...string) {
	keep := map[string]bool{}
	for _, n := range names {
		keep[n] = true
	}
	for _, sub := range c.Commands() {
		if !keep[sub.Name()] {
			sub.Hidden = true
		}
	}
}
