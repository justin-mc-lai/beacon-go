// Skills surface: the six embedded public skills, listable and
// readable without a model call. This restores the v1 kept `skill`
// support surface (charter: slim, not dropped).
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/skills"
)

func newSkillsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "skills",
		Short: "List or show the six embedded public skills",
	}
	c.AddCommand(newSkillsListCmd(), newSkillsShowCmd())
	return c
}

func newSkillsListCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "list",
		Short: "List the public skills in pipeline order",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			list := skills.List()
			if asJSON {
				return printJSON(cmd, map[string]any{"skills": list})
			}
			out := cmd.OutOrStdout()
			for _, s := range list {
				fmt.Fprintf(out, "%-10s %s\n", s.ID, oneLine(s.Description))
			}
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func newSkillsShowCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "show <id>",
		Short: "Show one skill's SKILL.md",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, ok := skills.Get(args[0])
			if !ok {
				return fail("skill_not_found", "skills show", "no skill "+args[0]+" (try `beacon skills list`)")
			}
			if asJSON {
				return printJSON(cmd, s)
			}
			fmt.Fprint(cmd.OutOrStdout(), s.Body)
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 100 {
		return s[:97] + "..."
	}
	return s
}
