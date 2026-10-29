// skills install + references (R2c, H9): the host discovery path.
// Hosts discover SKILL.md files on disk, not content inside a binary,
// so this face materializes the embedded set into the host's skill
// directory: <host>/beacon-<id>/SKILL.md plus its references/.
//
// Host roots live in the skills package (skills/hosts.go) so this CLI
// face and the post-release hook cannot drift. With no --host/--dir the
// installer auto-detects the first existing host directory instead of
// silently writing to a root nothing reads.
package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/core"
	"beacon-go/skills"
)

// HostSkillDir resolves a host name to its absolute skill directory.
// Kept as a thin wrapper for callers that only need the path.
func HostSkillDir(host string) (string, error) {
	dir, err := skills.HostDir(host, "")
	if err != nil {
		return "", core.NewError(core.ReasonSkillInstallHostUnknown, "skills install", err.Error())
	}
	return dir, nil
}

func newSkillsInstallCmd() *cobra.Command {
	var host, dir string
	var force, asJSON bool
	c := &cobra.Command{
		Use:   "install",
		Short: "Materialize the embedded skills into a host skill directory so the host can discover beacon",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			target := dir
			label := host
			if target == "" {
				if host == "" {
					detected, err := skills.DetectHost("")
					if err != nil {
						return fail(core.ReasonSkillInstallHostUnknown, "skills install", err.Error())
					}
					target, label = detected.Dir, detected.Name
				} else {
					var err error
					target, err = HostSkillDir(host)
					if err != nil {
						return err
					}
					label = host
				}
			}
			out := cmd.OutOrStdout()
			if !asJSON {
				fmt.Fprintf(out, "target: %s (host %s)\n", target, label)
				for _, f := range skills.PlannedFiles(target) {
					fmt.Fprintf(out, "  write %s\n", f)
				}
			}
			res, err := skills.Install(target, force)
			if err != nil {
				var conflict *skills.ConflictError
				if errors.As(err, &conflict) {
					return fail(core.ReasonSkillInstallConflict, "skills install", err.Error())
				}
				return err
			}
			if asJSON {
				return printJSON(cmd, res)
			}
			fmt.Fprintf(out, "installed %d skills (%d files) into %s\n", len(res.Skills), len(res.Files), res.TargetDir)
			return nil
		},
	}
	c.Flags().StringVar(&host, "host", "", "host skill directory: "+strings.Join(skills.HostNames(), "|")+" (default: auto-detect)")
	c.Flags().StringVar(&dir, "dir", "", "explicit target skill directory (overrides --host)")
	c.Flags().BoolVar(&force, "force", false, "overwrite existing beacon-<id> directories")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func newSkillsReferencesCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "references <id>",
		Short: "Show one skill's references/ files (the progressive-disclosure layer)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, ok := skills.Get(args[0]); !ok {
				return fail("skill_not_found", "skills references", "no skill "+args[0]+" (try `beacon skills list`)")
			}
			refs := skills.References(args[0])
			if asJSON {
				return printJSON(cmd, map[string]any{"skill": args[0], "references": refs})
			}
			out := cmd.OutOrStdout()
			for _, r := range refs {
				fmt.Fprintf(out, "===== %s/%s =====\n%s", args[0], r.Path, r.Body)
				if !strings.HasSuffix(r.Body, "\n") {
					fmt.Fprintln(out)
				}
			}
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

// newSkillsExamplesCmd exposes the worked route examples of one skill
// (AC-127/AC-144). With no id it prints the index so a host can see the
// four routed chains without loading every body.
func newSkillsExamplesCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "examples [id]",
		Short: "Show one skill's examples/ route files (or the index of all)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if len(args) == 0 {
				type idx struct {
					Skill    string   `json:"skill"`
					Examples []string `json:"examples"`
				}
				var list []idx
				for _, id := range skills.IDs() {
					var paths []string
					for _, e := range skills.Examples(id) {
						paths = append(paths, e.Path)
					}
					list = append(list, idx{Skill: id, Examples: paths})
				}
				if asJSON {
					return printJSON(cmd, map[string]any{"skills": list})
				}
				for _, it := range list {
					fmt.Fprintf(out, "%-10s %d examples\n", it.Skill, len(it.Examples))
					for _, p := range it.Examples {
						fmt.Fprintf(out, "  %s\n", p)
					}
				}
				return nil
			}
			if _, ok := skills.Get(args[0]); !ok {
				return fail("skill_not_found", "skills examples", "no skill "+args[0]+" (try `beacon skills list`)")
			}
			ex := skills.Examples(args[0])
			if asJSON {
				return printJSON(cmd, map[string]any{"skill": args[0], "examples": ex})
			}
			for _, e := range ex {
				fmt.Fprintf(out, "===== %s/%s =====\n%s", args[0], e.Path, e.Body)
				if !strings.HasSuffix(e.Body, "\n") {
					fmt.Fprintln(out)
				}
			}
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

// newSkillsHostsCmd answers "why is beacon not in my host's skill list":// it prints every known host root, whether it exists, and which
// beacon-* skills are already installed there.
func newSkillsHostsCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "hosts",
		Short: "Show known host skill directories, whether they exist, and what is installed",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			hosts, err := skills.HostDirs("")
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, map[string]any{"hosts": hosts})
			}
			out := cmd.OutOrStdout()
			for _, h := range hosts {
				state := "missing"
				if h.Exists {
					state = "present"
				}
				installed := "none"
				if len(h.Installed) > 0 {
					installed = fmt.Sprintf("%d (%s)", len(h.Installed), strings.Join(h.Installed, ", "))
				}
				fmt.Fprintf(out, "%-10s %-8s %-8s %s\n", h.Name, state, installed, h.Dir)
			}
			fmt.Fprintln(out, "\ninstall into one with: beacon skills install --host <name>")
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}
