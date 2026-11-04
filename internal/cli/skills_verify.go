// skills verify (skills-distribution): report each host's beacon-go*
// install against the embedded 1+6 set. Default is report-only so a
// developer machine's drift never fails an unrelated command; --strict
// is the CI gate.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/skills"
)

func newSkillsVerifyCmd() *cobra.Command {
	var home string
	var asJSON, strict bool
	c := &cobra.Command{
		Use:   "verify",
		Short: "Check each host's beacon-go* install against the embedded 1+6 set",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vs, err := skills.VerifyAll(home)
			if err != nil {
				return err
			}
			if asJSON {
				if err := printJSON(cmd, map[string]any{"hosts": vs}); err != nil {
					return err
				}
			} else {
				out := cmd.OutOrStdout()
				for _, v := range vs {
					state := "absent"
					if v.Exists {
						state = "OK"
						if !v.OK {
							state = "DRIFT"
						}
					}
					fmt.Fprintf(out, "%-10s %-7s %s\n", v.Host, state, v.Dir)
					if len(v.Missing) > 0 {
						fmt.Fprintf(out, "  missing: %s\n", strings.Join(v.Missing, ", "))
					}
					if len(v.Extra) > 0 {
						fmt.Fprintf(out, "  extra:   %s\n", strings.Join(v.Extra, ", "))
					}
				}
			}
			if strict {
				for _, v := range vs {
					if v.Exists && !v.OK {
						return fail("skill_install_drift", "skills verify",
							"host "+v.Host+" drifts from the embedded 1+6 set; run `beacon skills install --host "+v.Host+"`")
					}
				}
			}
			return nil
		},
	}
	c.Flags().StringVar(&home, "home", "", "home directory to inspect (default: current user)")
	c.Flags().BoolVar(&strict, "strict", false, "exit non-zero when an existing host drifts")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}
