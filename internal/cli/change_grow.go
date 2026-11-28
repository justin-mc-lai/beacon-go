package cli

import (
	"encoding/json"
	"os"
	"strings"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
	"github.com/spf13/cobra"
)

func newChangeGrowCmd() *cobra.Command {
	var project, version, node, proposal, summary, by, base string
	c := &cobra.Command{Use: "grow", Aliases: []string{"extend"}, Short: "Append governed draft children to the existing logical tree", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(version) == "" {
			return fail("tree_version_required", "change grow", "explicit --version required")
		}
		raw, err := os.ReadFile(proposal)
		if err != nil {
			return err
		}
		var p intenttree.Proposal
		if err = json.Unmarshal(raw, &p); err != nil {
			return err
		}
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
		rec, err := intenttree.GrowChange(st, dir, ver.ID, node, summary, by, base, p)
		if err != nil {
			return err
		}
		return printJSON(cmd, rec)
	}}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "explicit docs version")
	c.Flags().StringVar(&node, "node", "", "existing parent ID")
	c.Flags().StringVar(&proposal, "proposal", "", "proposal JSON file")
	c.Flags().StringVar(&summary, "summary", "", "change summary")
	c.Flags().StringVar(&by, "by", "", "requesting identity")
	c.Flags().StringVar(&base, "base-fingerprint", "", "reviewed parent fingerprint")
	c.Flags().Bool("json", true, "emit JSON")
	for _, flag := range []string{"version", "node", "proposal", "summary", "by", "base-fingerprint"} {
		_ = c.MarkFlagRequired(flag)
	}
	return c
}
