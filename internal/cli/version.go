package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
	"beacon-go/internal/versionmanifest"
)

func newVersionCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "version",
		Short: "Docs delivery lines: baseline / open / status / freeze / release",
		Long: "A docs version is a manifest (docs/beacon/<v>/manifest.json) that\n" +
			"references immutable feature packages by contract hash. `open` inherits\n" +
			"unchanged lakes by reference; it never copies them. Runtime versioning\n" +
			"(v2.0.0-go.N) stays separate.",
	}
	c.AddCommand(
		newVersionBaselineCmd(), newVersionOpenCmd(), newVersionStatusCmd(),
		newVersionFreezeCmd(), newVersionReleaseCmd(), newVersionCompatCmd(),
		newVersionPlanCmd(),
	)
	return c
}

func newVersionBaselineCmd() *cobra.Command {
	var project, version string
	var asJSON bool
	c := &cobra.Command{
		Use:   "baseline",
		Short: "Snapshot the current docs line into a version manifest",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			m, err := orchestrator.NewEngine(st).BaselineVersion(project, version)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, m)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "baseline %s: lakes=%d seas=%d manifest=%s\n",
				m.Version, len(m.Lakes), len(m.Seas), versionmanifest.Path(project, m.Version))
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&version, "version", "", "docs version to baseline (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("version")
	return c
}

func newVersionOpenCmd() *cobra.Command {
	var project, from, level string
	var asJSON bool
	c := &cobra.Command{
		Use:   "open",
		Short: "Open the next docs line by inheriting unchanged lakes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			m, err := orchestrator.NewEngine(st).OpenVersion(project, from, level)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, m)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "opened %s (parent %s): lakes=%d seas=%d\n",
				m.Version, m.Parent, len(m.Lakes), len(m.Seas))
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&from, "from", "", "parent docs version (required)")
	c.Flags().StringVar(&level, "level", "patch", "bump level: patch|minor|major")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("from")
	return c
}

func newVersionStatusCmd() *cobra.Command {
	var project, version string
	var asJSON bool
	c := &cobra.Command{
		Use:   "status",
		Short: "Show one version manifest (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := versionmanifest.LoadVersion(project, version)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, m)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "version %s status=%s parent=%s lakes=%d seas=%d\n",
				m.Version, m.Status, m.Parent, len(m.Lakes), len(m.Seas))
			for _, l := range m.Lakes {
				fmt.Fprintf(cmd.OutOrStdout(), "  lake %-32s %-9s %s\n", l.Slug, l.Status, l.Package)
			}
			for _, s := range m.Seas {
				fmt.Fprintf(cmd.OutOrStdout(), "  sea  %-32s host=%s lakes=%d frozen=%v\n",
					s.Slug, s.HostFeature, len(s.Lakes), s.Frozen)
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&version, "version", "", "docs version (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("version")
	return c
}

func newVersionFreezeCmd() *cobra.Command {
	var project, version string
	var asJSON bool
	c := &cobra.Command{
		Use:   "freeze",
		Short: "Freeze a draft manifest (changed/new lakes need qa=pass; no open changes)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			m, err := orchestrator.NewEngine(st).FreezeVersion(project, version)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, m)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "frozen %s\n", m.Version)
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&version, "version", "", "docs version (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("version")
	return c
}

func newVersionReleaseCmd() *cobra.Command {
	var project, version string
	var asJSON bool
	c := &cobra.Command{
		Use:   "release",
		Short: "Release a frozen manifest",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			m, err := orchestrator.NewEngine(st).ReleaseVersion(project, version)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, m)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "released %s\n", m.Version)
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&version, "version", "", "docs version (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("version")
	return c
}

func newVersionCompatCmd() *cobra.Command {
	var project, version string
	var write, asJSON bool
	c := &cobra.Command{
		Use:   "compat",
		Short: "Materialize inherited lakes as ordinary feature files (opt-in compatibility view)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			missing, err := orchestrator.NewEngine(st).CompatVersion(project, version, write)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, map[string]any{"version": version, "write": write, "features": missing})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "compat %s: missing=%d write=%v\n", version, len(missing), write)
			for _, path := range missing {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", path)
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&version, "version", "", "docs version (required)")
	c.Flags().BoolVar(&write, "write", false, "write the missing feature files (default: dry run)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("version")
	return c
}

func newVersionPlanCmd() *cobra.Command {
	var project, from string
	var apply, asJSON bool
	c := &cobra.Command{
		Use:   "plan",
		Short: "Suggest the next delivery line from changed/new lakes and open changes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			eng := orchestrator.NewEngine(st)
			plan, err := eng.PlanVersion(project, from)
			if err != nil {
				return err
			}
			if apply && !plan.Draft {
				m, err := eng.OpenVersion(project, plan.From, plan.SuggestedLevel)
				if err != nil {
					return err
				}
				plan.NextVersion = m.Version
			}
			if asJSON {
				return printJSON(cmd, plan)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "plan %s -> %s (%s): changed=%d new=%d open=%d draft=%v\n",
				plan.From, plan.NextVersion, plan.SuggestedLevel, plan.LakesChanged, plan.LakesNew, plan.OpenChanges, plan.Draft)
			for _, r := range plan.Reasons {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", r)
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&from, "from", "", "version to plan from (default: current)")
	c.Flags().BoolVar(&apply, "apply", false, "open the next line when the current manifest is not draft")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}
