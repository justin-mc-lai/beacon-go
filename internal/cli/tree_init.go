package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

func newPlanInitCmd() *cobra.Command {
	var project, version, title, intent string
	var asJSON, localOnly bool
	c := &cobra.Command{
		Use:   "init",
		Short: "Create the root intent node for a greenfield project version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			proj, err := st.Projects().GetByRootPath(store.ResolveRootPath(project))
			if err != nil {
				return fail(intenttree.ReasonVersionNotFound, "plan init", "project not registered: "+project)
			}
			ver, err := resolveProjectVersion(st, proj.ID, version)
			if err != nil {
				return err
			}
			if title == "" {
				title = proj.Name
			}
			root, dir, err := intenttree.InitTree(st, proj.RootPath, ver.DocsRoot, ver.Version, ver.ID, title, intent)
			if err != nil {
				return err
			}
			if err := ensureArtifactsIgnored(proj.RootPath); err != nil {
				return err
			}
			if localOnly || localOnlyMode(proj.RootPath) {
				if err := ensureDocsIgnored(proj.RootPath); err != nil {
					return err
				}
				if err := setLocalOnlyMode(proj.RootPath); err != nil {
					return err
				}
			}
			tracked := []string{}
			if localOnlyMode(proj.RootPath) {
				tracked = trackedDocs(proj.RootPath)
			}
			if asJSON {
				return printJSON(cmd, map[string]any{
					"root": root, "tree_dir": dir, "local_only": localOnlyMode(proj.RootPath),
					"tracked_docs": tracked, "untrack_command": func() string {
						if len(tracked) > 0 {
							return untrackCommand()
						}
						return ""
					}(),
				})
			}
			_, err = cmd.OutOrStdout().Write([]byte("root: " + root.ID + " " + dir + "\n"))
			if err != nil {
				return err
			}
			if localOnlyMode(proj.RootPath) {
				fmt.Fprintln(cmd.OutOrStdout(), "local-only: .beacon/ and docs/beacon/ are ignored")
				if len(tracked) > 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "warning: docs/beacon already has tracked files; run: "+untrackCommand())
				}
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().StringVar(&title, "title", "", "root title")
	c.Flags().StringVar(&intent, "intent", "", "root intent statement")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	c.Flags().BoolVar(&localOnly, "local-only", false, "keep .beacon/ and docs/beacon/ local (never commit SDD state)")
	return c
}
