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
	Slug          string                   `json:"slug"`
	FeatureID     string                   `json:"feature_id"`
	DomainKind    string                   `json:"domain_kind"`
	FiveTuple     gate.FSMProjection       `json:"five_tuple"`
	DomainFSM     gate.DomainFSMProjection `json:"domain_fsm"`
	Findings      []gate.DomainFSMFinding  `json:"findings"`
	GraphFindings []gate.FSMFinding        `json:"graph_findings"`
	Applicable    bool                     `json:"applicable"`
	Valid         bool                     `json:"valid"`
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

			view := buildFSMCheckView(truthMD, ctx.Feature.Slug, ctx.Feature.ID)

			if asJSON {
				if err := printJSON(cmd, view); err != nil {
					return err
				}
				return fsmCheckError(view)
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
			cmd.Printf("  Findings (%d):\n", len(view.Findings)+len(view.GraphFindings))
			for _, f := range view.Findings {
				cmd.Printf("    - [%s] %s\n", f.Kind, f.Text)
			}
			for _, finding := range view.GraphFindings {
				cmd.Printf("    - [%s/%s] %s\n", finding.Severity, finding.Kind, finding.Text)
			}
			if view.Valid {
				cmd.Println("Status: VALID")
			} else if !view.Applicable {
				cmd.Println("Status: NOT APPLICABLE (fsm_required:false)")
			} else {
				cmd.Println("Status: FINDINGS REPORTED")
			}
			return fsmCheckError(view)
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

func buildFSMCheckView(truthMD, slug, featureID string) FSMCheckView {
	scalars, _, _ := gate.ParseFrontmatter(truthMD)
	projection, graphFindings, applicable, blocked := gate.CheckTruthFSM(truthMD)
	view := FSMCheckView{
		Slug: slug, FeatureID: featureID, DomainKind: scalars["domain_kind"],
		FiveTuple: projection, DomainFSM: gate.ParseDomainFSM(truthMD),
		GraphFindings: graphFindings, Applicable: applicable,
	}
	if scalars["package_contract"] != "truth-tasks" {
		_, view.Findings = gate.EvaluateDomainFSM(truthMD, "")
	}
	view.Valid = applicable && !blocked && len(view.Findings) == 0
	return view
}

func fsmCheckError(view FSMCheckView) error {
	if view.Applicable && !view.Valid {
		return fail("fsm_check_blocked", "truth fsm check", "FSM model is missing or invalid; see findings")
	}
	return nil
}
