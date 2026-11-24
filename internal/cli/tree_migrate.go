package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

// newMigrateTreeCmd performs the snapshot-first, idempotent import of
// flat feature packages into the canonical intent tree.
func newMigrateTreeCmd() *cobra.Command {
	var project, version string
	var asJSON bool
	c := &cobra.Command{
		Use:   "tree",
		Short: "Import flat feature packages into the canonical intent tree",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			proj, err := st.Projects().GetByRootPath(store.ResolveRootPath(project))
			if err != nil {
				return fail(intenttree.ReasonVersionNotFound, "migrate tree", "project not registered: "+project)
			}
			ver, err := resolveProjectVersion(st, proj.ID, version)
			if err != nil {
				return err
			}
			all, err := st.Features().ListAll()
			if err != nil {
				return err
			}
			features := make([]intenttree.LegacyFeature, 0, len(all))
			for _, f := range all {
				if f.ProjectVersionID != ver.ID {
					continue
				}
				features = append(features, intenttree.LegacyFeature{
					Slug: f.Slug, TruthPath: f.TruthPath, TestsPath: f.TestsPath,
				})
			}
			rep, err := intenttree.Import(st, intenttree.ImportInput{
				ProjectRoot: proj.RootPath, DocsRoot: ver.DocsRoot, Version: ver.Version,
				VersionID: ver.ID, ProjectName: proj.Name, Features: features,
			})
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, rep)
			}
			verb := "migrated"
			if rep.Already {
				verb = "already migrated"
			}
			_, err = cmd.OutOrStdout().Write([]byte(
				fmt.Sprintf("%s: %s (%d nodes, index %s)\n", verb, rep.Version, rep.NodeCount, rep.IndexPath)))
			return err
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}
