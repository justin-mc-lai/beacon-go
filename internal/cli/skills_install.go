// skills install + references (R2c, H9): the host discovery path.
// Python materialized its skill package into the host's skill
// directory (doctor setup-agent-surface / the post-release install
// hook) because hosts discover SKILL.md files on disk, not content
// inside a binary. This face does the same for the embedded set:
// claude -> ~/.claude/skills, codex -> ~/.agents/skills (the
// agent_surface.py global roots), generic -> ~/.agents/skills
// unless --dir names the directory explicitly. Targets are printed
// before anything is written; an existing beacon-<id> directory is
// a conflict and refuses the whole install unless --force.
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/core"
	"beacon-go/skills"
)

// HostSkillDir resolves a host name to its global skill directory.
// The post-release install hook lands skills on these same roots;
// it spells them out itself in orchestrator/hooks.go because the
// orchestrator cannot import this CLI package.
func HostSkillDir(host string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "claude":
		return filepath.Join(home, ".claude", "skills"), nil
	case "codex", "generic", "agents":
		return filepath.Join(home, ".agents", "skills"), nil
	default:
		return "", core.NewError(core.ReasonSkillInstallHostUnknown,
			"skills install", "unknown host "+host+" (want claude|codex|generic, or pass --dir)")
	}
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
			if target == "" {
				var err error
				target, err = HostSkillDir(host)
				if err != nil {
					return err
				}
			}
			out := cmd.OutOrStdout()
			if !asJSON {
				fmt.Fprintf(out, "target: %s (host %s)\n", target, host)
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
	c.Flags().StringVar(&host, "host", "generic", "host whose skill directory to install into: claude|codex|generic")
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
