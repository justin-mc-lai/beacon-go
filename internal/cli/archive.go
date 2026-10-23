// archive (R2c, M10): the archive governance face — locate
// archived requirement packages by version, trace a feature's
// lineage across versions, and record archive status the Python
// way (governance metadata under .machine/archive/, never a move).
// Disk-based like the Python original: no registration required.
package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
)

func newArchiveCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "archive",
		Short: "Archive governance: locate archived packages by version, trace lineage, record archive status",
	}
	c.AddCommand(newArchiveListCmd(), newArchiveLineageCmd(), newArchiveApplyCmd(),
		newArchiveStatusCmd(), newArchiveExplainCmd())
	return c
}

func printArchiveEntries(cmd *cobra.Command, entries []orchestrator.ArchiveEntry) {
	out := cmd.OutOrStdout()
	if len(entries) == 0 {
		fmt.Fprintln(out, "no archive entries")
		return
	}
	for _, e := range entries {
		fmt.Fprintf(out, "%-10s %-24s %-8s %-24s ac=%d %s\n",
			e.Version, e.Slug, e.Location, e.Status, e.ACCount, e.PackageDir)
	}
}

func newArchiveListCmd() *cobra.Command {
	var project, version string
	var all, asJSON bool
	c := &cobra.Command{
		Use:   "list",
		Short: "List archived packages (default: archive roots + archived statuses; --all for the full inventory)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := absProject(project)
			entries := orchestrator.ScanArchiveEntries(root)
			var out []orchestrator.ArchiveEntry
			for _, e := range entries {
				if version != "" && e.Version != version {
					continue
				}
				if !all && e.Location != "archive" && !e.Archived {
					continue
				}
				out = append(out, e)
			}
			if out == nil {
				out = []orchestrator.ArchiveEntry{}
			}
			if asJSON {
				return printJSON(cmd, map[string]any{"entries": out, "count": len(out)})
			}
			printArchiveEntries(cmd, out)
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&version, "version", "", "only this docs version")
	c.Flags().BoolVar(&all, "all", false, "full feature x version inventory (Python archive inventory)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func newArchiveLineageCmd() *cobra.Command {
	var project string
	var asJSON bool
	c := &cobra.Command{
		Use:   "lineage <feature>",
		Short: "Trace one feature's truth evolution across active and archived version roots",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hops := orchestrator.ArchiveLineage(absProject(project), args[0])
			if asJSON {
				return printJSON(cmd, map[string]any{"feature": args[0], "hops": hops, "hop_count": len(hops)})
			}
			if len(hops) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no lineage for %s\n", args[0])
				return nil
			}
			printArchiveEntries(cmd, hops)
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func newArchiveApplyCmd() *cobra.Command {
	var project, version, feature, status, reason string
	var asJSON bool
	c := &cobra.Command{
		Use:   "apply",
		Short: "Record one feature's archive status (writes .machine/archive governance metadata; never moves the package)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := orchestrator.ApplyArchiveStatus(absProject(project), version, feature, status, reason)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, res)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "archive apply: %s %s -> %s\nmetadata: %s\nindex:    %s\nplan:     %s\n",
				res.Version, res.Feature, res.Status, res.MetadataRef, res.IndexRef, res.PlanRef)
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&version, "version", "", "docs version (required)")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&status, "status", "", "archive status: "+strings.Join(orchestrator.ArchiveStatuses, ", "))
	c.Flags().StringVar(&reason, "reason", "", "archive reason")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("version")
	_ = c.MarkFlagRequired("feature")
	_ = c.MarkFlagRequired("status")
	return c
}

func newArchiveStatusCmd() *cobra.Command {
	var project, version, feature string
	var asJSON bool
	c := &cobra.Command{
		Use:   "status",
		Short: "Show archive state for one version (or one feature's boundary within it)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			entries := orchestrator.ScanArchiveEntries(absProject(project))
			var out []orchestrator.ArchiveEntry
			for _, e := range entries {
				if version != "" && e.Version != version {
					continue
				}
				if feature != "" && e.Slug != feature {
					continue
				}
				out = append(out, e)
			}
			if out == nil {
				out = []orchestrator.ArchiveEntry{}
			}
			if asJSON {
				return printJSON(cmd, map[string]any{"entries": out, "count": len(out)})
			}
			printArchiveEntries(cmd, out)
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().StringVar(&feature, "feature", "", "feature slug filter")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func newArchiveExplainCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "explain",
		Short: "Explain the archive status model and this face's boundary",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if asJSON {
				return printJSON(cmd, map[string]any{
					"schema_version":     "1.0",
					"status_model":       orchestrator.ArchiveStatuses,
					"support_position":   "support-command",
					"lifecycle_position": "not-a-primary-stage",
					"boundary":           "archive writes governance metadata only; packages are never moved by apply (version-tree relocation is the separate version-truth-catalog surface)",
				})
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "Beacon archive explain")
			fmt.Fprintf(out, "status_model: %s\n", strings.Join(orchestrator.ArchiveStatuses, ", "))
			fmt.Fprintln(out, "support_position: support-command")
			fmt.Fprintln(out, "boundary: apply records status metadata under .machine/archive/; it never moves a package")
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

// absProject resolves a project flag to an absolute, cleaned path
// (archive/hooks faces are disk-based; no store canonicalization).
func absProject(p string) string {
	if p == "" {
		p = "."
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}
