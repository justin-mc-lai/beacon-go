package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/gate"
)

// TransitionResult represents the outcome of evaluating a state transition against the FSM.
type TransitionResult struct {
	Allowed   bool   `json:"allowed"`
	Verdict   string `json:"verdict"` // "allowed", "guard_failed", "illegal_transition", "state_not_found", "transition_not_found"
	From      string `json:"from"`
	To        string `json:"to"`
	Aggregate string `json:"aggregate,omitempty"`
	Guard     string `json:"guard,omitempty"`
	Reason    string `json:"reason"`
}

func newFSMCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "fsm",
		Short: "FSM runtime ability: state transition evaluation and inspection",
	}
	c.AddCommand(newFSMTransitionCmd())
	return c
}

func newFSMTransitionCmd() *cobra.Command {
	var project, feature, version, aggregate, from, to string
	var asJSON bool

	c := &cobra.Command{
		Use:   "transition",
		Short: "Evaluate a state transition against the feature package's FSM specification",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				return fail(ReasonFeatureNotFound, "fsm transition", "no runtime DB yet, feature not found: "+feature)
			}
			defer st.Close()

			ctx, err := resolveGateFeature(st, project, feature, version)
			if err != nil {
				return err
			}

			truthPath := filepath.Join(ctx.Project.RootPath, ctx.Feature.TruthPath)
			truthBytes, err := os.ReadFile(truthPath)
			if err != nil {
				return fail(ReasonFeatureNotFound, "fsm transition", fmt.Sprintf("cannot read truth.md at %s: %v", truthPath, err))
			}
			truthMD := string(truthBytes)

			res := evaluateFSMTransition(truthMD, aggregate, from, to)

			if asJSON {
				return printJSON(cmd, res)
			}

			if res.Allowed {
				cmd.Printf("Verdict: PASS (%s)\nFrom: %s -> To: %s\n", res.Verdict, res.From, res.To)
				if res.Aggregate != "" {
					cmd.Printf("Aggregate: %s\n", res.Aggregate)
				}
				if res.Guard != "" {
					cmd.Printf("Guard: %s\n", res.Guard)
				}
				cmd.Printf("Reason: %s\n", res.Reason)
				return nil
			}

			cmd.Printf("Verdict: REJECT (%s)\nFrom: %s -> To: %s\n", res.Verdict, res.From, res.To)
			if res.Aggregate != "" {
				cmd.Printf("Aggregate: %s\n", res.Aggregate)
			}
			cmd.Printf("Reason: %s\n", res.Reason)
			return nil
		},
	}

	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().StringVar(&aggregate, "aggregate", "", "optional aggregate root name")
	c.Flags().StringVar(&from, "from", "", "origin state (required)")
	c.Flags().StringVar(&to, "to", "", "target state (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit JSON document")

	_ = c.MarkFlagRequired("feature")
	_ = c.MarkFlagRequired("from")
	_ = c.MarkFlagRequired("to")

	return c
}

func evaluateFSMTransition(truthMD, aggregate, from, to string) TransitionResult {
	normFrom := strings.TrimSpace(from)
	normTo := strings.TrimSpace(to)
	lFrom := strings.ToLower(normFrom)
	lTo := strings.ToLower(normTo)

	// 1. Try DomainFSMProjection
	dp := gate.ParseDomainFSM(truthMD)

	// Check domain illegal slots first
	for _, ill := range dp.IllegalSlots {
		if aggregate != "" && ill.Aggregate != "" && !strings.EqualFold(ill.Aggregate, aggregate) {
			continue
		}
		desc := strings.ToLower(ill.Description)
		if (strings.Contains(desc, lFrom) && strings.Contains(desc, lTo)) ||
			strings.Contains(desc, fmt.Sprintf("%s->%s", lFrom, lTo)) ||
			strings.Contains(desc, fmt.Sprintf("%s -> %s", lFrom, lTo)) {
			return TransitionResult{
				Allowed:   false,
				Verdict:   "illegal_transition",
				From:      normFrom,
				To:        normTo,
				Aggregate: ill.Aggregate,
				Reason:    fmt.Sprintf("Matches explicit illegal transition: %s", ill.Description),
			}
		}
	}

	// 2. Try FiveTuple FSMProjection illegal slots
	tp := gate.ParseTruthFSM(truthMD)
	for _, ill := range tp.Illegal {
		if strings.EqualFold(ill.From, lFrom) && strings.EqualFold(ill.To, lTo) {
			return TransitionResult{
				Allowed: false,
				Verdict: "illegal_transition",
				From:    normFrom,
				To:      normTo,
				Reason:  fmt.Sprintf("Matches explicit illegal transition: %s -> %s (%s)", ill.From, ill.To, ill.Description),
			}
		}
	}

	// Check aggregate transitions if aggregates exist
	if len(dp.Aggregates) > 0 {
		for _, agg := range dp.Aggregates {
			if aggregate != "" && !strings.EqualFold(agg.Name, aggregate) {
				continue
			}
			for _, t := range agg.Transitions {
				if strings.EqualFold(strings.TrimSpace(t.From), lFrom) && strings.EqualFold(strings.TrimSpace(t.To), lTo) {
					return TransitionResult{
						Allowed:   true,
						Verdict:   "allowed",
						From:      normFrom,
						To:        normTo,
						Aggregate: agg.Name,
						Guard:     t.Guard,
						Reason:    "Transition is explicitly defined in domain aggregate",
					}
				}
			}
		}
	}

	// Check five-tuple transitions if five-tuple exists
	if tp.FiveTuplePresent {
		for _, t := range tp.Transitions {
			if strings.EqualFold(strings.TrimSpace(t.State), lFrom) && strings.EqualFold(strings.TrimSpace(t.To), lTo) {
				return TransitionResult{
					Allowed: true,
					Verdict: "allowed",
					From:    normFrom,
					To:      normTo,
					Guard:   t.Guard,
					Reason:  "Transition is explicitly defined in 5-tuple FSM table",
				}
			}
		}
	}

	// Not found in any transition table
	return TransitionResult{
		Allowed: false,
		Verdict: "transition_not_found",
		From:    normFrom,
		To:      normTo,
		Reason:  fmt.Sprintf("No defined transition from '%s' to '%s'", normFrom, normTo),
	}
}
