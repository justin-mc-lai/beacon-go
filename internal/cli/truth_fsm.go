package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"beacon-go/internal/gate"
)

// FSMCheckView is the JSON output representation for `beacon truth fsm check`.
type FSMCheckView struct {
	Slug       string                   `json:"slug"`
	FeatureID  string                   `json:"feature_id"`
	DomainKind string                   `json:"domain_kind"`
	FiveTuple  gate.FSMProjection       `json:"five_tuple"`
	DomainFSM  gate.DomainFSMProjection `json:"domain_fsm"`
	Findings   []gate.DomainFSMFinding  `json:"findings"`
	Valid      bool                     `json:"valid"`
}

func newTruthFSMCheckCmd() *cobra.Command {
	var project, feature, version string
	var asJSON bool

	c := &cobra.Command{
		Use:   "check",
		Short: "Check the feature package's Domain FSM & 5-tuple FSM specification",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				return fail(ReasonFeatureNotFound, "truth fsm check", "no runtime DB yet, feature not found: "+feature)
			}
			defer st.Close()

			ctx, err := resolveGateFeature(st, project, feature, version)
			if err != nil {
				return err
			}

			truthPath := filepath.Join(ctx.Project.RootPath, ctx.Feature.TruthPath)
			truthBytes, err := os.ReadFile(truthPath)
			if err != nil {
				return fail(ReasonFeatureNotFound, "truth fsm check", fmt.Sprintf("cannot read truth.md at %s: %v", truthPath, err))
			}
			truthMD := string(truthBytes)

			scalars, _, _ := gate.ParseFrontmatter(truthMD)
			domainKindVal := ""
			if scalars != nil {
				domainKindVal = scalars["domain_kind"]
			}

			fiveTuple := gate.ParseTruthFSM(truthMD)
			domainFSM := gate.ParseDomainFSM(truthMD)
			_, findings := gate.EvaluateDomainFSM(truthMD, "")

			view := FSMCheckView{
				Slug:       ctx.Feature.Slug,
				FeatureID:  ctx.Feature.ID,
				DomainKind: domainKindVal,
				FiveTuple:  fiveTuple,
				DomainFSM:  domainFSM,
				Findings:   findings,
				Valid:      len(findings) == 0,
			}

			if asJSON {
				return printJSON(cmd, view)
			}

			cmd.Printf("FSM Check for %s (%s):\n", view.Slug, view.FeatureID)
			cmd.Printf("  Domain Kind: %s\n", view.DomainKind)
			cmd.Printf("  Entities (%d):\n", len(view.DomainFSM.Entities))
			for _, e := range view.DomainFSM.Entities {
				cmd.Printf("    - %s (key: %s, precedence: %s, requires: %s)\n", e.Entity, e.Key, e.Precedence, e.Requires)
			}
			cmd.Printf("  Aggregates (%d):\n", len(view.DomainFSM.Aggregates))
			for _, a := range view.DomainFSM.Aggregates {
				cmd.Printf("    - %s (%d transitions)\n", a.Name, len(a.Transitions))
				for _, t := range a.Transitions {
					cmd.Printf("        %s -> %s [guard: %s]\n", t.From, t.To, t.Guard)
				}
			}
			cmd.Printf("  Illegal Transition Slots (%d):\n", len(view.DomainFSM.IllegalSlots))
			for _, ill := range view.DomainFSM.IllegalSlots {
				cmd.Printf("    - %s (agg: %s)\n", ill.Description, ill.Aggregate)
			}
			cmd.Printf("  Findings (%d):\n", len(view.Findings))
			for _, f := range view.Findings {
				cmd.Printf("    - [%s] %s\n", f.Kind, f.Text)
			}
			if view.Valid {
				cmd.Println("Status: VALID")
			} else {
				cmd.Println("Status: FINDINGS REPORTED")
			}
			return nil
		},
	}

	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().BoolVar(&asJSON, "json", false, "emit JSON document")
	_ = c.MarkFlagRequired("feature")

	return c
}

func newTruthFSMCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "fsm",
		Short: "FSM tools: check status, inspect transitions, and validate models",
	}
	c.AddCommand(newTruthFSMCheckCmd())
	return c
}
