package cli

import (
	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
	"github.com/spf13/cobra"
)

func newChangeDeprecateCmd() *cobra.Command {
	var project, version, id, summary, by, base string
	c := &cobra.Command{Use: "deprecate", Short: "Deprecate an empty draft node without deleting it; record the change", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		st, err := openWriteStore()
		if err != nil {
			return err
		}
		defer st.Close()
		dir, _, idx, err := resolveTree(st, project, version)
		if err != nil {
			return err
		}
		proj, err := st.Projects().GetByRootPath(store.ResolveRootPath(project))
		if err != nil {
			return err
		}
		ver, err := resolveProjectVersion(st, proj.ID, idx.Version)
		if err != nil {
			return err
		}
		record, err := intenttree.DeprecateDraft(st, dir, ver.ID, id, summary, by, base)
		if err != nil {
			return err
		}
		return printJSON(cmd, record)
	}}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version")
	c.Flags().StringVar(&id, "node", "", "draft node ID")
	c.Flags().StringVar(&summary, "summary", "", "deprecation reason")
	c.Flags().StringVar(&by, "by", "", "requesting identity")
	c.Flags().StringVar(&base, "base-fingerprint", "", "reviewed fingerprint")
	c.Flags().Bool("json", true, "emit JSON")
	for _, name := range []string{"node", "summary", "by", "base-fingerprint"} {
		_ = c.MarkFlagRequired(name)
	}
	return c
}
