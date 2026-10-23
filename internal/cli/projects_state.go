// projects export-state / import-state (R2c, H10): the portable
// snapshot face. export writes .beacon/state/project-state.json
// into the repo; import rebuilds a fresh machine's registry view
// from it (DB wins every conflict; runs are summaries only). A
// project directory carrying a snapshot is also auto-recognized by
// the gate-family resolver and `projects show`: unregistered no
// longer means refused when the repo itself carries its state.
package cli

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
	"beacon-go/internal/store"
)

func newProjectsExportStateCmd() *cobra.Command {
	var project string
	var asJSON bool
	c := &cobra.Command{
		Use:   "export-state",
		Short: "Export the project's state snapshot into the repo (.beacon/state/project-state.json)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			snap, abs, err := orchestrator.NewEngine(st).ExportProjectState(project)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, snap)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "state snapshot: %s (%d versions, %d features)\n",
				abs, len(snap.Versions), len(snap.Features))
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func newProjectsImportStateCmd() *cobra.Command {
	var project string
	var asJSON bool
	c := &cobra.Command{
		Use:   "import-state",
		Short: "Import the repo's state snapshot into this machine's registry (fills only what is missing; the DB wins)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			res, err := orchestrator.NewEngine(st).ImportProjectState(project)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, res)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"import: project_created=%v versions+%d features+%d lifecycles+%d gates+%d skipped_existing=%d runs_summary_only=%d\n",
				res.ProjectCreated, res.VersionsAdded, res.FeaturesAdded, res.LifecyclesRestored,
				res.GatesRestored, res.SkippedExisting, res.RunsSummarizedOnly)
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

// autoRecognizeProject gives the repo-carried snapshot its chance:
// when the root is not registered but a snapshot file exists, the
// project is imported on the spot (auto-registration from the
// repo). Used by read faces that would otherwise refuse an
// unregistered project outright. The store must be writable.
func autoRecognizeProject(st *store.Store, root string) {
	root = store.ResolveRootPath(root)
	if _, err := st.Projects().GetByRootPath(root); !errors.Is(err, sql.ErrNoRows) {
		return
	}
	orchestrator.NewEngine(st).AutoImportState(root)
}
