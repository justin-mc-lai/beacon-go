package cli

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
	"beacon-go/internal/store"
)

func newGoalAdviseCmd() *cobra.Command {
	var project, projectKey string
	var top int
	var asJSON bool
	c := &cobra.Command{
		Use:   "advise",
		Short: "Rank a project's features by facts only (blockers > in-progress > stage), no prose",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				doc := orchestrator.AdviseDoc{Schema: orchestrator.AdviseSchema,
					Version: orchestrator.AdviseVersion, Items: []orchestrator.AdviseItem{}}
				if asJSON {
					return printJSON(cmd, doc)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "no features (no runtime DB yet)")
				return nil
			}
			defer st.Close()
			filter := store.SpineListFilter{}
			key := projectKey
			if key == "" {
				root := store.ResolveRootPath(project)
				if _, err := st.Projects().GetByRootPath(root); err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						return fail(ReasonProjectNotRegistered, "goal advise", "project not registered: "+root)
					}
					return err
				}
				filter.ProjectRoot = root
			}
			snaps, err := st.Spine().List(filter)
			if err != nil {
				return err
			}
			if key == "" && len(snaps) > 0 {
				key = snaps[0].ProjectKey
			}
			doc := orchestrator.AdviseProject(snaps, key, top)
			if asJSON {
				return printJSON(cmd, doc)
			}
			if len(doc.Items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no features match")
				return nil
			}
			for i, it := range doc.Items {
				fmt.Fprintf(cmd.OutOrStdout(), "%d. %-34s score=%-3d next=%-9s blockers=%v\n",
					i+1, it.Feature, it.Score, it.SuggestedNext, it.ReasonCodes)
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&projectKey, "project-key", "", "project key filter (skips root lookup)")
	c.Flags().IntVar(&top, "top", 3, "how many items to return (0 = all)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}
